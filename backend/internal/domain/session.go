package domain

import (
	"time"

	"github.com/google/uuid"
)

// SessionStatus is a live chat session state (spec §6).
type SessionStatus string

// Session states.
const (
	SessionRequested    SessionStatus = "requested"
	SessionAccepted     SessionStatus = "accepted"
	SessionActive       SessionStatus = "active"
	SessionDisconnected SessionStatus = "disconnected"
	SessionEnded        SessionStatus = "ended"
	SessionSettled      SessionStatus = "settled"
	SessionDeclined     SessionStatus = "declined"
	SessionExpired      SessionStatus = "expired"
	SessionCancelled    SessionStatus = "cancelled"
)

// EndReason explains how a session left the open states.
type EndReason string

// End reasons.
const (
	EndClientEnded      EndReason = "client_ended"
	EndPsychicEnded     EndReason = "psychic_ended"
	EndCreditsExhausted EndReason = "credits_exhausted"
	EndGraceExpired     EndReason = "grace_expired"
	EndAdminTerminated  EndReason = "admin_terminated"
	EndAccountDisabled  EndReason = "account_disabled"
	EndDeclined         EndReason = "declined"
	EndRequestTimeout   EndReason = "request_timeout"
	EndNotStarted       EndReason = "not_started"
	EndCancelled        EndReason = "cancelled"
)

// sessionTransitions encodes the state diagram. Extensions beyond the spec:
// Requested→Cancelled (client withdraws a pending request), Accepted→Expired
// (accepted but both parties never connected) and Accepted→Ended (a party
// ends before the session started; nothing is charged).
var sessionTransitions = map[SessionStatus][]SessionStatus{
	SessionRequested:    {SessionAccepted, SessionDeclined, SessionExpired, SessionCancelled},
	SessionAccepted:     {SessionActive, SessionExpired, SessionEnded},
	SessionActive:       {SessionDisconnected, SessionEnded},
	SessionDisconnected: {SessionActive, SessionEnded},
	SessionEnded:        {SessionSettled},
}

// CanTransition reports whether from→to is allowed.
func (s SessionStatus) CanTransition(to SessionStatus) bool {
	for _, next := range sessionTransitions[s] {
		if next == to {
			return true
		}
	}
	return false
}

// IsOpen reports whether the session still blocks new sessions.
func (s SessionStatus) IsOpen() bool {
	switch s {
	case SessionRequested, SessionAccepted, SessionActive, SessionDisconnected:
		return true
	}
	return false
}

// IsLive reports whether the session has been accepted and not yet ended.
func (s SessionStatus) IsLive() bool {
	return s == SessionAccepted || s == SessionActive || s == SessionDisconnected
}

// IsFinal reports whether no further transition is possible.
func (s SessionStatus) IsFinal() bool { return len(sessionTransitions[s]) == 0 }

// OpenSessionStatuses lists the open states (for SQL filters).
var OpenSessionStatuses = []SessionStatus{SessionRequested, SessionAccepted, SessionActive, SessionDisconnected}

// Session is a live chat session between one client and one psychic.
type Session struct {
	ID                uuid.UUID     `gorm:"type:uuid;primaryKey"`
	ConversationID    uuid.UUID     `gorm:"type:uuid;not null"`
	ClientID          uuid.UUID     `gorm:"type:uuid;not null"`
	PsychicID         uuid.UUID     `gorm:"type:uuid;not null"`
	Status            SessionStatus `gorm:"not null"`
	EndReason         EndReason     `gorm:"not null"`
	RatePerMinute     int64         `gorm:"not null"`
	RequestKey        string        `gorm:"not null"`
	RequestedAt       time.Time     `gorm:"not null"`
	ExpiresAt         time.Time     `gorm:"not null"`
	AcceptedAt        *time.Time
	StartedAt         *time.Time
	LastResumedAt     *time.Time
	DisconnectedAt    *time.Time
	DisconnectedParty string `gorm:"not null"`
	GraceExpiresAt    *time.Time
	EndedAt           *time.Time
	SettledAt         *time.Time
	NextBillingAt     *time.Time
	ActiveSeconds     int64 `gorm:"not null"`
	BilledMinutes     int   `gorm:"not null"`
	FreeMinutesUsed   int   `gorm:"not null"`
	CreditsCharged    int64 `gorm:"not null"`
	PsychicEarnings   int64 `gorm:"not null"`
	Reconnects        int   `gorm:"not null"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (Session) TableName() string { return "chat_sessions" }

// IsParticipant reports whether userID takes part in the session.
func (s *Session) IsParticipant(userID uuid.UUID) bool {
	return s.ClientID == userID || s.PsychicID == userID
}

// Counterpart returns the other participant.
func (s *Session) Counterpart(userID uuid.UUID) uuid.UUID {
	if s.ClientID == userID {
		return s.PsychicID
	}
	return s.ClientID
}

// ElapsedActive returns accumulated active time at now, including the
// current active stretch.
func (s *Session) ElapsedActive(now time.Time) time.Duration {
	d := time.Duration(s.ActiveSeconds) * time.Second
	if s.Status == SessionActive && s.LastResumedAt != nil && now.After(*s.LastResumedAt) {
		d += now.Sub(*s.LastResumedAt)
	}
	return d
}

// PaidCoverage returns how much active time the billed minutes cover.
func (s *Session) PaidCoverage(interval time.Duration) time.Duration {
	return time.Duration(s.BilledMinutes) * interval
}

// StartedMinutes returns the number of started billing minutes for an
// active duration (BR-02: every started minute counts, no pro-rating).
func StartedMinutes(active, interval time.Duration) int {
	if active <= 0 {
		return 0
	}
	n := int(active / interval)
	if active%interval != 0 {
		n++
	}
	return n
}

// CustomRequestStatus is the lifecycle of an ordered custom request (§5.1).
type CustomRequestStatus string

// Custom request statuses.
const (
	RequestPending   CustomRequestStatus = "pending"
	RequestAccepted  CustomRequestStatus = "accepted"
	RequestDelivered CustomRequestStatus = "delivered"
	RequestDeclined  CustomRequestStatus = "declined"
	RequestCancelled CustomRequestStatus = "cancelled"
	RequestExpired   CustomRequestStatus = "expired"
	RequestDisputed  CustomRequestStatus = "disputed"
	RequestRefunded  CustomRequestStatus = "refunded"
)

var requestTransitions = map[CustomRequestStatus][]CustomRequestStatus{
	RequestPending:   {RequestAccepted, RequestDeclined, RequestCancelled, RequestExpired, RequestRefunded},
	RequestAccepted:  {RequestDelivered, RequestExpired, RequestRefunded},
	RequestDelivered: {RequestDisputed},
	RequestDisputed:  {RequestRefunded, RequestDelivered},
}

// CanTransition reports whether from→to is allowed.
func (s CustomRequestStatus) CanTransition(to CustomRequestStatus) bool {
	for _, next := range requestTransitions[s] {
		if next == to {
			return true
		}
	}
	return false
}

// HoldsCredits reports whether credits are still held for this request.
func (s CustomRequestStatus) HoldsCredits() bool {
	return s == RequestPending || s == RequestAccepted
}

// CustomRequest is an order for a psychic's listed service.
type CustomRequest struct {
	ID             uuid.UUID           `gorm:"type:uuid;primaryKey"`
	ClientID       uuid.UUID           `gorm:"type:uuid;not null"`
	PsychicID      uuid.UUID           `gorm:"type:uuid;not null"`
	ServiceID      uuid.UUID           `gorm:"type:uuid;not null"`
	ServiceName    string              `gorm:"not null"`
	PriceCredits   int64               `gorm:"not null"`
	Status         CustomRequestStatus `gorm:"not null"`
	ClientNote     string              `gorm:"not null"`
	PreferredAt    *time.Time
	Delivery       string `gorm:"not null"`
	DeclineReason  string `gorm:"not null"`
	DisputeReason  string `gorm:"not null"`
	ResolutionNote string `gorm:"not null"`
	IdempotencyKey string `gorm:"not null"`
	AcceptBy       time.Time
	DeliverBy      *time.Time
	AcceptedAt     *time.Time
	DeliveredAt    *time.Time
	ClosedAt       *time.Time
	ResolvedBy     *uuid.UUID `gorm:"type:uuid"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (CustomRequest) TableName() string { return "custom_requests" }
