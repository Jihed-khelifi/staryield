package service

import (
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// Growth: notification kinds of the referral loop. The data never names the
// friend: joined carries nothing, reward_granted the minutes added. (A
// qualified referral is an analytics event only; the grant announces it.)
const (
	NotifyReferralJoined        = "referral.joined"
	NotifyReferralRewardGranted = "referral.reward_granted"
)

// referralLink is where the notifications lead (an app path; the mail
// builder and the web app add the locale).
const referralLink = "/profile/invite"

func init() {
	// In-app and push for both; e-mail only when minutes are added (account mail).
	channelPolicies[NotifyReferralJoined] = ChannelPolicy{Push: true}
	channelPolicies[NotifyReferralRewardGranted] = ChannelPolicy{Email: true, Category: domain.EmailAccount, Push: true}
}
