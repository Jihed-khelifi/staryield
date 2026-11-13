package service

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/i18n"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
)

// revisions 5.1: purchase receipts and refund notices of the hosted-checkout
// path (§25.5).

// receiptData is the data of purchase.succeeded: what the receipt e-mail and
// the in-app notification show. The pack and card come from one query; when it
// fails the notification still goes out with what the purchase itself knows.
func (s *CreditService) receiptData(ctx context.Context, p *domain.CreditPurchase) map[string]any {
	data := map[string]any{"purchase_id": p.ID, "credits": p.Credits, "price_cents": p.PriceCents, "currency": p.Currency}
	if p.CompletedAt != nil {
		data["completed_at"] = *p.CompletedAt
	} else {
		data["completed_at"] = s.clock.Now()
	}
	if r, err := s.credits.PurchaseReceipt(ctx, p.ID); err == nil {
		data["pack_name"] = r.PackName
		if r.Brand != "" && r.Last4 != "" {
			data["card_brand"], data["card_last4"] = r.Brand, r.Last4
		}
	} else {
		logger.FromContext(ctx).Warn("credits: receipt details unavailable", zap.Error(err), zap.String("purchase_id", p.ID.String()))
	}
	if s.settings.StatementDescriptor != "" {
		data["statement_descriptor"] = s.settings.StatementDescriptor
	}
	return data
}

// mailReceiptCopy e-mails the receipt to the purchase's receipt_email when the
// buyer gave one that differs from the account address (the account's own
// receipt goes through the notification channels). The copy has no account
// link: it may go to someone else.
func (s *CreditService) mailReceiptCopy(ctx context.Context, p *domain.CreditPurchase, data map[string]any) {
	if s.mailer == nil || s.contacts == nil || strings.TrimSpace(p.ReceiptEmail) == "" {
		return
	}
	to, ok := NormalizeEmail(p.ReceiptEmail)
	if !ok {
		return
	}
	contacts, err := s.contacts.Contacts(ctx, []uuid.UUID{p.UserID})
	if err != nil || len(contacts) == 0 {
		logger.FromContext(ctx).Warn("credits: receipt copy skipped, account lookup failed", zap.Error(err), zap.String("purchase_id", p.ID.String()))
		return
	}
	c := contacts[0]
	if strings.EqualFold(strings.TrimSpace(c.Email), to) {
		return
	}
	locale := i18n.Normalize(c.Locale)
	content, ok := mail.Notification(locale, NotifyPurchase, data, "")
	if !ok {
		return
	}
	content.Action = nil
	uid := p.UserID
	if err := s.mailer.Enqueue(ctx, EmailRequest{UserID: &uid, To: to, Locale: locale, Category: domain.EmailBilling,
		Template: "notification:" + NotifyPurchase + ":receipt_email", Content: content, DedupeKey: "purchase_receipt_email:" + p.ID.String()}); err != nil {
		logger.FromContext(ctx).Error("credits: receipt copy enqueue failed", zap.Error(err), zap.String("purchase_id", p.ID.String()))
	}
}

// notifyRefunded tells the buyer that a hosted-checkout purchase was refunded
// in full by the provider (payment.refunded webhook).
func (s *CreditService) notifyRefunded(ctx context.Context, p *domain.CreditPurchase) {
	if p == nil {
		return
	}
	data := map[string]any{"amount_cents": p.PriceCents, "currency": p.Currency, "purchase_id": p.ID, "credits": p.Credits,
		"purchased_at": p.CreatedAt}
	if r, err := s.credits.PurchaseReceipt(ctx, p.ID); err == nil {
		data["pack_name"] = r.PackName
	}
	if s.settings.StatementDescriptor != "" {
		data["statement_descriptor"] = s.settings.StatementDescriptor
	}
	s.notify.Notify(ctx, NotificationSpec{Kind: NotifyPaymentRefunded, Data: data, Link: "/profile/settings#billing"}, p.UserID)
}
