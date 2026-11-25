package service

import (
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// Growth: notification kinds of the referral loop. The data never names the
// friend: joined carries nothing, qualified the minutes earned and whether
// the rewards already launched, reward_granted the minutes added.
const (
	NotifyReferralJoined        = "referral.joined"
	NotifyReferralQualified     = "referral.qualified"
	NotifyReferralRewardGranted = "referral.reward_granted"
)

// referralLink is where the notifications lead (an app path; the mail
// builder and the web app add the locale).
const referralLink = "/profile/invite"

func init() {
	// In-app and push for all three; e-mail only when minutes are added (account mail).
	channelPolicies[NotifyReferralJoined] = ChannelPolicy{Push: true}
	channelPolicies[NotifyReferralQualified] = ChannelPolicy{Push: true}
	channelPolicies[NotifyReferralRewardGranted] = ChannelPolicy{Email: true, Category: domain.EmailAccount, Push: true}
}
