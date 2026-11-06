package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// InboxRow is an inbox entry: the conversation, a preview of its last
// message and the counterpart's display identity, loaded in one query.
type InboxRow struct {
	domain.Conversation     `gorm:"embedded"`
	LastBody                *string
	LastKind                *string
	LastSenderID            *uuid.UUID
	LastMsgSeq              *int64
	CounterpartName         *string
	CounterpartNickname     *string
	CounterpartAvatarURL    *string
	CounterpartSlug         *string
	CounterpartAvailability *string
}

// InboxFilter selects one page of a user's inbox, newest activity first.
type InboxFilter struct {
	UserID    uuid.UUID
	AsPsychic bool
	Cursor    *Cursor // keyset position (last_activity_at, id)
	Limit     int     // rows to fetch (callers ask for one extra to detect more)
}

// SessionFilter narrows session listings.
type SessionFilter struct {
	ClientID       *uuid.UUID
	PsychicID      *uuid.UUID
	ConversationID *uuid.UUID
	Statuses       []domain.SessionStatus
	From, To       *time.Time
	Cursor         *Cursor
	Limit          int
}

// ChatRepository persists conversations, sessions and messages.
type ChatRepository interface {
	GetOrCreateConversation(ctx context.Context, clientID, psychicID uuid.UUID) (*domain.Conversation, error)
	GetConversation(ctx context.Context, id uuid.UUID) (*domain.Conversation, error)
	GetConversationForUpdate(ctx context.Context, id uuid.UUID) (*domain.Conversation, error)
	FindConversation(ctx context.Context, clientID, psychicID uuid.UUID) (*domain.Conversation, error)
	ListInbox(ctx context.Context, f InboxFilter) ([]InboxRow, error)
	SetReadPosition(ctx context.Context, id uuid.UUID, asPsychic bool, readSeq int64, unread int, readAt time.Time) error

	CreateSession(ctx context.Context, s *domain.Session) error
	GetSession(ctx context.Context, id uuid.UUID) (*domain.Session, error)
	GetSessionForUpdate(ctx context.Context, id uuid.UUID) (*domain.Session, error)
	GetSessionByRequestKey(ctx context.Context, clientID uuid.UUID, key string) (*domain.Session, error)
	SaveSession(ctx context.Context, s *domain.Session) error
	OpenSessionForClient(ctx context.Context, clientID uuid.UUID) (*domain.Session, error)
	OpenSessionForPsychic(ctx context.Context, psychicID uuid.UUID) (*domain.Session, error)
	OpenSessionsForConversations(ctx context.Context, ids []uuid.UUID) ([]domain.Session, error)
	OpenSessionsForPsychics(ctx context.Context, psychicIDs []uuid.UUID) ([]domain.Session, error)
	BusyPsychics(ctx context.Context) ([]uuid.UUID, error)
	ListSessions(ctx context.Context, f SessionFilter) ([]domain.Session, error)

	ClaimDueBilling(ctx context.Context, now time.Time, limit int) ([]domain.Session, error)
	ClaimDueExpiry(ctx context.Context, now time.Time, limit int) ([]domain.Session, error)
	ClaimDueGrace(ctx context.Context, now time.Time, limit int) ([]domain.Session, error)
	ClaimUnsettled(ctx context.Context, endedBefore time.Time, limit int) ([]domain.Session, error)
	ListLiveSessions(ctx context.Context, limit int) ([]domain.Session, error)

	// AppendMessage assigns the next sequence number of the message's
	// conversation, stores the message and updates the conversation's
	// counters, all inside the caller's transaction. The conversation row
	// lock taken by the UPDATE serialises writers, so seq order equals commit
	// order. ErrDuplicate means the sender reused a client message ID.
	AppendMessage(ctx context.Context, m *domain.Message) (*domain.Conversation, error)
	GetMessage(ctx context.Context, id uuid.UUID) (*domain.Message, error)
	GetMessageByClientID(ctx context.Context, senderID uuid.UUID, clientMsgID string) (*domain.Message, error)
	// ListMessagesBefore returns messages with seq < beforeSeq, newest first
	// (beforeSeq <= 0 starts from the newest message).
	ListMessagesBefore(ctx context.Context, conversationID uuid.UUID, beforeSeq int64, limit int) ([]domain.Message, error)
	// ListMessagesAfterSeq returns messages with seq > afterSeq, oldest first.
	ListMessagesAfterSeq(ctx context.Context, conversationID uuid.UUID, afterSeq int64, limit int) ([]domain.Message, error)
	// SeqAtOrBefore returns the newest seq created at or before at (0 if none).
	SeqAtOrBefore(ctx context.Context, conversationID uuid.UUID, at time.Time) (int64, error)
	// CountUnread counts messages after afterSeq that viewerID did not send.
	CountUnread(ctx context.Context, conversationID, viewerID uuid.UUID, afterSeq int64) (int64, error)
	CountSessionMessages(ctx context.Context, sessionID uuid.UUID) (int64, error)

	SaveBookmark(ctx context.Context, b *domain.SavedMessage) error
	DeleteBookmark(ctx context.Context, userID, messageID uuid.UUID) error
	// ListBookmarks returns the user's saved messages, most recently saved
	// first, after the keyset position cursor (nil for the first page).
	ListBookmarks(ctx context.Context, userID uuid.UUID, conversationID *uuid.UUID, cursor *Cursor, limit int) ([]BookmarkRow, error)
	BookmarkedIDs(ctx context.Context, userID uuid.UUID, messageIDs []uuid.UUID) (map[uuid.UUID]bool, error)
}

// GormChatRepository implements ChatRepository.
type GormChatRepository struct{ db *gorm.DB }

// NewChatRepository returns a GORM chat repository.
func NewChatRepository(db *gorm.DB) *GormChatRepository { return &GormChatRepository{db: db} }

func (r *GormChatRepository) GetOrCreateConversation(ctx context.Context, clientID, psychicID uuid.UUID) (*domain.Conversation, error) {
	conn := database.Conn(ctx, r.db)
	now := time.Now().UTC()
	c := domain.Conversation{ID: domain.NewID(), ClientID: clientID, PsychicID: psychicID, LastActivityAt: now, CreatedAt: now, UpdatedAt: now}
	if err := conn.Clauses(clause.OnConflict{DoNothing: true}).Create(&c).Error; err != nil {
		return nil, err
	}
	var out domain.Conversation
	if err := conn.First(&out, "client_id = ? AND psychic_id = ?", clientID, psychicID).Error; err != nil {
		return nil, notFound(err)
	}
	return &out, nil
}

func (r *GormChatRepository) GetConversation(ctx context.Context, id uuid.UUID) (*domain.Conversation, error) {
	var c domain.Conversation
	if err := database.Conn(ctx, r.db).First(&c, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &c, nil
}

func (r *GormChatRepository) GetConversationForUpdate(ctx context.Context, id uuid.UUID) (*domain.Conversation, error) {
	var c domain.Conversation
	if err := database.Conn(ctx, r.db).Clauses(forUpdate).First(&c, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &c, nil
}

func (r *GormChatRepository) FindConversation(ctx context.Context, clientID, psychicID uuid.UUID) (*domain.Conversation, error) {
	var c domain.Conversation
	if err := database.Conn(ctx, r.db).First(&c, "client_id = ? AND psychic_id = ?", clientID, psychicID).Error; err != nil {
		return nil, notFound(err)
	}
	return &c, nil
}

// inboxQuery builds the single inbox query: one row per conversation with the
// last message preview (at most 141 characters) and the counterpart's
// identity joined in, ordered by (last_activity_at, id) for keyset paging.
// Unread counts are maintained on the row, so no per-row subquery runs.
func inboxQuery(db *gorm.DB, f InboxFilter) *gorm.DB {
	const preview = "c.*, left(m.body, 141) AS last_body, m.kind AS last_kind, m.sender_id AS last_sender_id, m.seq AS last_msg_seq"
	q := db.Table("conversations AS c").Joins("LEFT JOIN chat_messages m ON m.id = c.last_message_id")
	if f.AsPsychic {
		// The counterpart is a client: display name, nickname and avatar.
		q = q.Select(preview+", u.display_name AS counterpart_name, p.nickname AS counterpart_nickname, "+
			"p.avatar_url AS counterpart_avatar_url").
			Joins("JOIN users u ON u.id = c.client_id").
			Joins("LEFT JOIN user_profiles p ON p.user_id = c.client_id").
			Where("c.psychic_id = ?", f.UserID)
	} else {
		// The counterpart is an advisor: public name, image, slug and the
		// availability needed for the live status.
		q = q.Select(preview+", ps.name AS counterpart_name, ps.image_url AS counterpart_avatar_url, "+
			"ps.slug AS counterpart_slug, ps.availability AS counterpart_availability").
			Joins("JOIN psychics ps ON ps.user_id = c.psychic_id").
			Where("c.client_id = ?", f.UserID)
	}
	if f.Cursor != nil {
		if id, err := uuid.Parse(f.Cursor.ID); err == nil {
			q = q.Where("(c.last_activity_at, c.id) < (?, ?)", f.Cursor.At, id)
		} else {
			q = q.Where("c.last_activity_at < ?", f.Cursor.At)
		}
	}
	return q.Order("c.last_activity_at DESC, c.id DESC").Limit(clampLimit(f.Limit, 21, 51))
}

func (r *GormChatRepository) ListInbox(ctx context.Context, f InboxFilter) ([]InboxRow, error) {
	var rows []InboxRow
	err := inboxQuery(database.Conn(ctx, r.db), f).Scan(&rows).Error
	return rows, err
}

func (r *GormChatRepository) SetReadPosition(ctx context.Context, id uuid.UUID, asPsychic bool, readSeq int64, unread int, readAt time.Time) error {
	seqCol, unreadCol, atCol := "client_read_seq", "client_unread", "client_last_read_at"
	if asPsychic {
		seqCol, unreadCol, atCol = "psychic_read_seq", "psychic_unread", "psychic_last_read_at"
	}
	return database.Conn(ctx, r.db).Model(&domain.Conversation{}).Where("id = ?", id).
		Updates(map[string]any{seqCol: readSeq, unreadCol: unread, atCol: readAt}).Error
}

func (r *GormChatRepository) CreateSession(ctx context.Context, s *domain.Session) error {
	if err := database.Conn(ctx, r.db).Create(s).Error; err != nil {
		if isUnique(err, "chat_sessions_request_key") {
			return ErrDuplicate
		}
		if isUnique(err, "chat_sessions_one_open_per_client") {
			return ErrClientBusy
		}
		if isUnique(err, "chat_sessions_one_open_per_psychic") {
			return ErrPsychicBusy
		}
		return err
	}
	return nil
}

// Session uniqueness violations.
var (
	ErrClientBusy  = &busyError{"client already has an open session"}
	ErrPsychicBusy = &busyError{"psychic already has an open session"}
)

type busyError struct{ msg string }

func (e *busyError) Error() string { return "repository: " + e.msg }

func (r *GormChatRepository) GetSession(ctx context.Context, id uuid.UUID) (*domain.Session, error) {
	var s domain.Session
	if err := database.Conn(ctx, r.db).First(&s, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &s, nil
}

func (r *GormChatRepository) GetSessionForUpdate(ctx context.Context, id uuid.UUID) (*domain.Session, error) {
	var s domain.Session
	if err := database.Conn(ctx, r.db).Clauses(forUpdate).First(&s, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &s, nil
}

func (r *GormChatRepository) GetSessionByRequestKey(ctx context.Context, clientID uuid.UUID, key string) (*domain.Session, error) {
	var s domain.Session
	if err := database.Conn(ctx, r.db).First(&s, "client_id = ? AND request_key = ?", clientID, key).Error; err != nil {
		return nil, notFound(err)
	}
	return &s, nil
}

func (r *GormChatRepository) SaveSession(ctx context.Context, s *domain.Session) error {
	s.UpdatedAt = time.Now().UTC()
	return database.Conn(ctx, r.db).Save(s).Error
}

func (r *GormChatRepository) OpenSessionForClient(ctx context.Context, clientID uuid.UUID) (*domain.Session, error) {
	var s domain.Session
	if err := database.Conn(ctx, r.db).Where("client_id = ? AND status IN ?", clientID, domain.OpenSessionStatuses).
		First(&s).Error; err != nil {
		return nil, notFound(err)
	}
	return &s, nil
}

func (r *GormChatRepository) OpenSessionForPsychic(ctx context.Context, psychicID uuid.UUID) (*domain.Session, error) {
	var s domain.Session
	if err := database.Conn(ctx, r.db).Where("psychic_id = ? AND status IN ?", psychicID, domain.OpenSessionStatuses).
		First(&s).Error; err != nil {
		return nil, notFound(err)
	}
	return &s, nil
}

func (r *GormChatRepository) OpenSessionsForConversations(ctx context.Context, ids []uuid.UUID) ([]domain.Session, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var out []domain.Session
	err := database.Conn(ctx, r.db).Where("conversation_id IN ? AND status IN ?", ids, domain.OpenSessionStatuses).Find(&out).Error
	return out, err
}

func (r *GormChatRepository) OpenSessionsForPsychics(ctx context.Context, psychicIDs []uuid.UUID) ([]domain.Session, error) {
	if len(psychicIDs) == 0 {
		return nil, nil
	}
	var out []domain.Session
	// An advisor has at most one open session (chat_sessions_one_open_per_psychic).
	err := database.Conn(ctx, r.db).Where("psychic_id IN ? AND status IN ?", psychicIDs, domain.OpenSessionStatuses).Find(&out).Error
	return out, err
}

func (r *GormChatRepository) BusyPsychics(ctx context.Context) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	// Pending requests count as busy: a psychic takes one client at a time.
	err := database.Conn(ctx, r.db).Model(&domain.Session{}).
		Where("status IN ?", domain.OpenSessionStatuses).
		Distinct().Pluck("psychic_id", &ids).Error
	return ids, err
}

func (r *GormChatRepository) ListSessions(ctx context.Context, f SessionFilter) ([]domain.Session, error) {
	q := database.Conn(ctx, r.db).Model(&domain.Session{})
	if f.ClientID != nil {
		q = q.Where("client_id = ?", *f.ClientID)
	}
	if f.PsychicID != nil {
		q = q.Where("psychic_id = ?", *f.PsychicID)
	}
	if f.ConversationID != nil {
		q = q.Where("conversation_id = ?", *f.ConversationID)
	}
	if len(f.Statuses) > 0 {
		q = q.Where("status IN ?", f.Statuses)
	}
	if f.From != nil {
		q = q.Where("requested_at >= ?", *f.From)
	}
	if f.To != nil {
		q = q.Where("requested_at < ?", *f.To)
	}
	q = applyCursor(q, f.Cursor, "requested_at", "id")
	var out []domain.Session
	err := q.Order("requested_at DESC, id DESC").Limit(clampLimit(f.Limit, 20, 100)).Find(&out).Error
	return out, err
}

func (r *GormChatRepository) claim(ctx context.Context, limit int, query string, args ...any) ([]domain.Session, error) {
	var out []domain.Session
	err := database.Conn(ctx, r.db).Clauses(forUpdateSkipLocked).Where(query, args...).
		Order("updated_at ASC").Limit(clampLimit(limit, 50, 500)).Find(&out).Error
	return out, err
}

func (r *GormChatRepository) ClaimDueBilling(ctx context.Context, now time.Time, limit int) ([]domain.Session, error) {
	// Strictly before now, at database precision: the boundary of
	// domain.Session.MinuteStarted, so the worker and the end flow never
	// disagree on a started minute.
	return r.claim(ctx, limit, "status = ? AND next_billing_at < ?", domain.SessionActive, now.Truncate(domain.TimePrecision))
}

func (r *GormChatRepository) ClaimDueExpiry(ctx context.Context, now time.Time, limit int) ([]domain.Session, error) {
	return r.claim(ctx, limit, "status IN ? AND expires_at <= ?",
		[]domain.SessionStatus{domain.SessionRequested, domain.SessionAccepted}, now)
}

func (r *GormChatRepository) ClaimDueGrace(ctx context.Context, now time.Time, limit int) ([]domain.Session, error) {
	return r.claim(ctx, limit, "status = ? AND grace_expires_at <= ?", domain.SessionDisconnected, now)
}

func (r *GormChatRepository) ClaimUnsettled(ctx context.Context, endedBefore time.Time, limit int) ([]domain.Session, error) {
	return r.claim(ctx, limit, "status = ? AND ended_at <= ?", domain.SessionEnded, endedBefore)
}

func (r *GormChatRepository) ListLiveSessions(ctx context.Context, limit int) ([]domain.Session, error) {
	var out []domain.Session
	err := database.Conn(ctx, r.db).Where("status IN ?", []domain.SessionStatus{domain.SessionAccepted, domain.SessionActive}).
		Order("updated_at ASC").Limit(clampLimit(limit, 200, 2000)).Find(&out).Error
	return out, err
}

func (r *GormChatRepository) AppendMessage(ctx context.Context, m *domain.Message) (*domain.Conversation, error) {
	conn := database.Conn(ctx, r.db)
	at := m.CreatedAt
	// SET expressions read the pre-update row, so "last_seq + 1" is the new
	// sequence number in every expression. The sender has read everything up
	// to their own message; the other side gains an unread message. System
	// messages (no sender) count as unread for both participants: comparing
	// with NULL never matches, so both ELSE branches apply.
	var sender any
	if m.SenderID != nil {
		sender = *m.SenderID
	}
	var conv domain.Conversation
	res := conn.Model(&conv).Clauses(clause.Returning{}).Where("id = ?", m.ConversationID).Updates(map[string]any{
		"last_seq":             gorm.Expr("last_seq + 1"),
		"last_message_id":      m.ID,
		"last_message_at":      at,
		"last_activity_at":     at,
		"updated_at":           at,
		"client_read_seq":      gorm.Expr("CASE WHEN client_id = ? THEN last_seq + 1 ELSE client_read_seq END", sender),
		"psychic_read_seq":     gorm.Expr("CASE WHEN psychic_id = ? THEN last_seq + 1 ELSE psychic_read_seq END", sender),
		"client_unread":        gorm.Expr("CASE WHEN client_id = ? THEN 0 ELSE client_unread + 1 END", sender),
		"psychic_unread":       gorm.Expr("CASE WHEN psychic_id = ? THEN 0 ELSE psychic_unread + 1 END", sender),
		"client_last_read_at":  gorm.Expr("CASE WHEN client_id = ? THEN ?::timestamptz ELSE client_last_read_at END", sender, at),
		"psychic_last_read_at": gorm.Expr("CASE WHEN psychic_id = ? THEN ?::timestamptz ELSE psychic_last_read_at END", sender, at),
	})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	m.Seq = conv.LastSeq
	if m.UpdatedAt.IsZero() {
		m.UpdatedAt = m.CreatedAt
	}
	if err := conn.Create(m).Error; err != nil {
		// A concurrent request stored the same client message ID first; the
		// caller's transaction rolls back, so no sequence number is skipped.
		if isUnique(err, "chat_messages_client_msg_key") {
			return nil, ErrDuplicate
		}
		return nil, err
	}
	return &conv, nil
}

func (r *GormChatRepository) GetMessage(ctx context.Context, id uuid.UUID) (*domain.Message, error) {
	var m domain.Message
	if err := database.Conn(ctx, r.db).First(&m, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &m, nil
}

func (r *GormChatRepository) GetMessageByClientID(ctx context.Context, senderID uuid.UUID, clientMsgID string) (*domain.Message, error) {
	var m domain.Message
	if err := database.Conn(ctx, r.db).First(&m, "sender_id = ? AND client_msg_id = ?", senderID, clientMsgID).Error; err != nil {
		return nil, notFound(err)
	}
	return &m, nil
}

func (r *GormChatRepository) ListMessagesBefore(ctx context.Context, conversationID uuid.UUID, beforeSeq int64, limit int) ([]domain.Message, error) {
	q := database.Conn(ctx, r.db).Where("conversation_id = ?", conversationID)
	if beforeSeq > 0 {
		q = q.Where("seq < ?", beforeSeq)
	}
	var out []domain.Message
	err := q.Order("seq DESC").Limit(clampLimit(limit, 50, 201)).Find(&out).Error
	return out, err
}

func (r *GormChatRepository) ListMessagesAfterSeq(ctx context.Context, conversationID uuid.UUID, afterSeq int64, limit int) ([]domain.Message, error) {
	var out []domain.Message
	err := database.Conn(ctx, r.db).Where("conversation_id = ? AND seq > ?", conversationID, afterSeq).
		Order("seq ASC").Limit(clampLimit(limit, 200, 501)).Find(&out).Error
	return out, err
}

func (r *GormChatRepository) SeqAtOrBefore(ctx context.Context, conversationID uuid.UUID, at time.Time) (int64, error) {
	var seq int64
	err := database.Conn(ctx, r.db).Model(&domain.Message{}).
		Where("conversation_id = ? AND created_at <= ?", conversationID, at).
		Select("COALESCE(max(seq), 0)").Scan(&seq).Error
	return seq, err
}

func (r *GormChatRepository) CountUnread(ctx context.Context, conversationID, viewerID uuid.UUID, afterSeq int64) (int64, error) {
	var n int64
	err := database.Conn(ctx, r.db).Model(&domain.Message{}).
		Where("conversation_id = ? AND seq > ? AND sender_id IS DISTINCT FROM ?", conversationID, afterSeq, viewerID).
		Count(&n).Error
	return n, err
}

func (r *GormChatRepository) CountSessionMessages(ctx context.Context, sessionID uuid.UUID) (int64, error) {
	var n int64
	err := database.Conn(ctx, r.db).Model(&domain.Message{}).Where("session_id = ? AND kind = ?", sessionID, domain.MessageText).Count(&n).Error
	return n, err
}

// SaveBookmark is idempotent: an existing bookmark is left untouched, and a
// removed (soft-deleted) one is revived as a fresh save.
func (r *GormChatRepository) SaveBookmark(ctx context.Context, b *domain.SavedMessage) error {
	return database.Conn(ctx, r.db).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "message_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"created_at": gorm.Expr("CASE WHEN saved_messages.deleted_at IS NULL THEN saved_messages.created_at ELSE excluded.created_at END"),
			"updated_at": gorm.Expr("CASE WHEN saved_messages.deleted_at IS NULL THEN saved_messages.updated_at ELSE excluded.updated_at END"),
			"deleted_at": nil,
		}),
	}).Create(b).Error
}

// DeleteBookmark soft-deletes the bookmark (gorm.DeletedAt).
func (r *GormChatRepository) DeleteBookmark(ctx context.Context, userID, messageID uuid.UUID) error {
	return database.Conn(ctx, r.db).Where("user_id = ? AND message_id = ?", userID, messageID).Delete(&domain.SavedMessage{}).Error
}

// BookmarkRow is a saved message with the time it was saved (the keyset
// position of the saved-messages list).
type BookmarkRow struct {
	domain.Message `gorm:"embedded"`
	SavedAt        time.Time
}

func (r *GormChatRepository) ListBookmarks(ctx context.Context, userID uuid.UUID, conversationID *uuid.UUID, cursor *Cursor, limit int) ([]BookmarkRow, error) {
	q := database.Conn(ctx, r.db).Table("chat_messages AS m").Select("m.*, s.created_at AS saved_at").
		Joins("JOIN saved_messages s ON s.message_id = m.id").Where("s.user_id = ? AND s.deleted_at IS NULL", userID)
	if conversationID != nil {
		q = q.Where("m.conversation_id = ?", *conversationID)
	}
	q = applyCursor(q, cursor, "s.created_at", "s.message_id")
	var out []BookmarkRow
	err := q.Order("s.created_at DESC, s.message_id DESC").Limit(clampLimit(limit, 100, 201)).Scan(&out).Error
	return out, err
}

func (r *GormChatRepository) BookmarkedIDs(ctx context.Context, userID uuid.UUID, messageIDs []uuid.UUID) (map[uuid.UUID]bool, error) {
	out := map[uuid.UUID]bool{}
	if len(messageIDs) == 0 {
		return out, nil
	}
	var ids []uuid.UUID
	if err := database.Conn(ctx, r.db).Model(&domain.SavedMessage{}).Where("user_id = ? AND message_id IN ?", userID, messageIDs).
		Pluck("message_id", &ids).Error; err != nil {
		return nil, err
	}
	for _, id := range ids {
		out[id] = true
	}
	return out, nil
}
