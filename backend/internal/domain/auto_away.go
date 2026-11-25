package domain

// SiteSettingAdvisorAutoAway is the auto-away document of site_settings:
// {"enabled": bool, "missed_requests": int (2..10)}. An Online advisor whose
// live-chat requests expire unanswered that many times in a row is set Away
// (never Offline). Declines are answers, not misses.
const SiteSettingAdvisorAutoAway = "advisor_auto_away"
