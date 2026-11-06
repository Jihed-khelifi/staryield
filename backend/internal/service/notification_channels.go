package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/i18n"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// Notification kinds added for account, export, waitlist and membership
// events. Account security e-mails (e-mail change, closure, reactivation)
// are sent directly through MailService by the account services because
// they go to specific addresses with one-time links; their in-app
// notifications use these kinds.
const (
	NotifyWaitlistAvailable           = "waitlist.available"
	NotifyExportReady                 = "export.ready"
	NotifyAccountEmailChangeRequested = "account.email_change_requested"
	NotifyAccountEmailChanged         = "account.email_changed"
	NotifyAccountClosureScheduled     = "account.closure_scheduled"
	NotifyAccountReactivated          = "account.reactivated"
	NotifySubscriptionRenewed         = "subscription.renewed"
	NotifySubscriptionPaymentFailed   = "subscription.payment_failed"
	NotifySubscriptionCanceled        = "subscription.canceled"
)

// ChannelPolicy says how a notification kind reaches people outside the app.
// E-mail of the security, billing and account categories is always sent;
// notification e-mail follows the user's e-mail switch; marketing needs the
// marketing opt-in (and is cancelled by the outbox while the provider is
// transactional-only). Push follows the push switch (plus the marketing
// opt-in for marketing).
type ChannelPolicy struct {
	Email    bool
	Category domain.EmailCategory
	Push     bool
}

var channelPolicies = map[string]ChannelPolicy{
	NotifyChatRequested:  {Push: true}, // time-critical: e-mail would arrive after the request expired
	NotifyChatMissed:     {Email: true, Category: domain.EmailNotification, Push: true},
	NotifyReviewPrompt:   {Email: true, Category: domain.EmailNotification, Push: true},
	NotifyReviewReceived: {Email: true, Category: domain.EmailNotification, Push: true},
	NotifyRequestCreated: {Email: true, Category: domain.EmailNotification, Push: true},
	NotifyRequestUpdated: {Email: true, Category: domain.EmailNotification, Push: true},
	// Receipts are e-mailed by checkout to the address the buyer chose.
	NotifyPurchase:                  {Push: true},
	NotifyCreditsAdjusted:           {Email: true, Category: domain.EmailBilling, Push: true},
	NotifyWithdrawal:                {Email: true, Category: domain.EmailBilling, Push: true},
	NotifyBroadcast:                 {Email: true, Category: domain.EmailMarketing, Push: true},
	NotifyWaitlistAvailable:         {Email: true, Category: domain.EmailNotification, Push: true},
	NotifyExportReady:               {Email: true, Category: domain.EmailAccount, Push: true},
	NotifySubscriptionRenewed:       {Email: true, Category: domain.EmailBilling, Push: true},
	NotifySubscriptionPaymentFailed: {Email: true, Category: domain.EmailBilling, Push: true},
	NotifySubscriptionCanceled:      {Email: true, Category: domain.EmailBilling, Push: true},
	// account.* kinds: in-app only here (their e-mails are sent directly).
}

// PolicyFor returns the channel policy of a kind (zero value: in-app only).
func PolicyFor(kind string) ChannelPolicy { return channelPolicies[kind] }

// wantsEmail applies the e-mail policy to a recipient.
func (p ChannelPolicy) wantsEmail(c repository.Contact) bool {
	if !p.Email {
		return false
	}
	switch p.Category {
	case domain.EmailSecurity, domain.EmailBilling, domain.EmailAccount:
		return true
	case domain.EmailMarketing:
		return c.NotifyMarketing
	}
	return c.NotifyEmail
}

// wantsPush applies the push policy to a recipient.
func (p ChannelPolicy) wantsPush(c repository.Contact) bool {
	if !p.Push || !c.NotifyPush {
		return false
	}
	return p.Category != domain.EmailMarketing || c.NotifyMarketing
}

// NotificationChannels fans persisted in-app notifications out to e-mail
// (outbox) and Web Push, following each recipient's preferences.
type NotificationChannels struct {
	contacts    repository.ContactRepository
	mail        EmailQueue
	push        *PushService
	frontendURL string
	log         *zap.Logger
}

// NewNotificationChannels wires the fan-out; mail or push may be nil.
func NewNotificationChannels(contacts repository.ContactRepository, mail EmailQueue, push *PushService, frontendURL string,
	log *zap.Logger) *NotificationChannels {
	return &NotificationChannels{contacts: contacts, mail: mail, push: push, frontendURL: frontendURL, log: log}
}

// Deliver fans out a batch of persisted notifications. Failures are logged:
// the in-app notification already exists.
func (n *NotificationChannels) Deliver(ctx context.Context, rows []domain.Notification) {
	if n == nil || len(rows) == 0 {
		return
	}
	ids := make([]uuid.UUID, 0, len(rows))
	seen := map[uuid.UUID]bool{}
	relevant := false
	for i := range rows {
		p := PolicyFor(rows[i].Kind)
		if !p.Email && !p.Push {
			continue
		}
		relevant = true
		if !seen[rows[i].UserID] {
			seen[rows[i].UserID] = true
			ids = append(ids, rows[i].UserID)
		}
	}
	if !relevant {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	contacts, err := n.contacts.Contacts(ctx, ids)
	if err != nil {
		n.log.Error("notifications: recipient lookup failed", zap.Error(err))
		return
	}
	byID := make(map[uuid.UUID]repository.Contact, len(contacts))
	for _, c := range contacts {
		byID[c.UserID] = c
	}
	var emails []EmailRequest
	for i := range rows {
		row := &rows[i]
		c, ok := byID[row.UserID]
		if !ok || c.Status != domain.UserActive {
			continue
		}
		p := PolicyFor(row.Kind)
		locale := i18n.Normalize(c.Locale)
		if n.mail != nil && p.wantsEmail(c) {
			link := mail.AppURL(n.frontendURL, locale, row.Link)
			if content, ok := mail.Notification(locale, row.Kind, row.Data, link); ok {
				uid := row.UserID
				emails = append(emails, EmailRequest{UserID: &uid, To: c.Email, Locale: locale, Category: p.Category,
					Template: "notification:" + row.Kind, Content: content, DedupeKey: "notification:" + row.ID.String()})
			}
		}
		if n.push != nil && p.wantsPush(c) {
			n.push.NotifyUser(row.UserID, locale, row)
		}
	}
	if len(emails) > 0 {
		if err := n.mail.Enqueue(ctx, emails...); err != nil {
			n.log.Error("notifications: e-mail enqueue failed", zap.Error(err), zap.Int("count", len(emails)))
		}
	}
}
