package service

import "github.com/Jihed-khelifi/staryield/backend/internal/domain"

// NotifyAutoAway tells an advisor that the platform set them to Away after
// repeated unanswered requests. Data: {"missed": n}. In-app, push and e-mail
// (the notification category: it follows the advisor's e-mail switch).
const NotifyAutoAway = "availability.auto_away"

// autoAwayLink is where the notification leads: the chatroom, which holds the
// advisor's availability control (like chat.missed).
const autoAwayLink = "/chatroom"

func init() {
	channelPolicies[NotifyAutoAway] = ChannelPolicy{Email: true, Category: domain.EmailNotification, Push: true}
}
