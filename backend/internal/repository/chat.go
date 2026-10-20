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

// ConversationRow is an inbox entry with its last message and unread count.
type ConversationRow struct {
	domain.Conversation `gorm:"embedded"`
	LastBody            *string
	LastKind            *string
	LastSenderID        *uuid.UUID
	Unread              int64
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
	FindConversation(ctx context.Context, clientID, psychicID uuid.UUID) (*domain.Conversation, error)
	ListConversations(ctx context.Context, userID uuid.UUID, asPsychic bool, limit int) ([]ConversationRow, error)
	TouchConversation(ctx context.Context, id, messageID uuid.UUID, at time.Time) error
	MarkConversationRead(ctx context.Context, id uuid.UUID, asPsychic bool, at time.Time) error

	CreateSession(ctx context.Context, s *domain.Session) error
	GetSession(ctx context.Context, id uuid.UUID) (*domain.Session, error)
	GetSessionForUpdate(ctx context.Context, id uuid.UUID) (*domain.Session, error)
	GetSessionByRequestKey(ctx context.Context, clientID uuid.UUID, key string) (*domain.Session, error)
	SaveSession(ctx context.Context, s *domain.Session) error
	OpenSessionForClient(ctx context.Context, clientID uuid.UUID) (*domain.Session, error)
	OpenSessionForPsychic(ctx context.Context, psychicID uuid.UUID) (*domain.Session, error)
	OpenSessionsForConversations(ctx context.Context, ids []uuid.UUID) ([]domain.Session, error)
	BusyPsychics(ctx context.Context) ([]uuid.UUID, error)
	ListSessions(ctx context.Context, f SessionFilter) ([]domain.Session, error)

	ClaimDueBilling(ctx context.Context, now time.Time, limit int) ([]domain.Session, error)
	ClaimDueExpiry(ctx context.Context, now time.Time, limit int) ([]domain.Session, error)
	ClaimDueGrace(ctx context.Context, now time.Time, limit int) ([]domain.Session, error)
	ClaimUnsettled(ctx context.Context, endedBefore time.Time, limit int) ([]domain.Session, error)
	ListLiveSessions(ctx context.Context, limit int) ([]domain.Session, error)

	CreateMessage(ctx context.Context, m *domain.Message) (created bool, err error)
	GetMessage(ctx context.Context, id uuid.UUID) (*domain.Message, error)
	GetMessageByClientID(ctx context.Context, senderID uuid.UUID, clientMsgID string) (*domain.Message, error)
	ListMessages(ctx context.Context, conversationID uuid.UUID, before *Cursor, limit int) ([]domain.Message, error)
	ListMessagesAfter(ctx context.Context, conversationID uuid.UUID, after time.Time, limit int) ([]domain.Message, error)
	CountSessionMessages(ctx context.Context, sessionID uuid.UUID) (int64, error)

	SaveBookmark(ctx context.Context, b *domain.SavedMessage) error
	DeleteBookmark(ctx context.Context, userID, messageID uuid.UUID) error
	ListBookmarks(ctx context.Context, userID uuid.UUID, conversationID *uuid.UUID, limit int) ([]domain.Message, error)
	BookmarkedIDs(ctx context.Context, userID uuid.UUID, messageIDs []uuid.UUID) (map[uuid.UUID]bool, error)
}

// GormChatRepository implements ChatRepository.
type GormChatRepository struct{ db *gorm.DB }

// NewChatRepository returns a GORM chat repository.
func NewChatRepository(db *gorm.DB) *GormChatRepository { return &GormChatRepository{db: db} }

func (r *GormChatRepository) GetOrCreateConversation(ctx context.Context, clientID, psychicID uuid.UUID) (*domain.Conversation, error) {
	conn := database.Conn(ctx, r.db)
	now := time.Now().UTC()
	c := domain.Conversation{ID: domain.NewID(), ClientID: clientID, PsychicID: psychicID, CreatedAt: now, UpdatedAt: now}
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

func (r *GormChatRepository) FindConversation(ctx context.Context, clientID, psychicID uuid.UUID) (*domain.Conversation, error) {
	var c domain.Conversation
	if err := database.Conn(ctx, r.db).First(&c, "client_id = ? AND psychic_id = ?", clientID, psychicID).Error; err != nil {
		return nil, notFound(err)
	}
	return &c, nil
}

func (r *GormChatRepository) ListConversations(ctx context.Context, userID uuid.UUID, asPsychic bool, limit int) ([]ConversationRow, error) {
	ownerCol, readCol := "c.client_id", "c.client_last_read_at"
	if asPsychic {
		ownerCol, readCol = "c.psychic_id", "c.psychic_last_read_at"
	}
	var rows []ConversationRow
	err := database.Conn(ctx, r.db).Raw(`
		SELECT c.*, m.body AS last_body, m.kind AS last_kind, m.sender_id AS last_sender_id,
		  (SELECT count(*) FROM chat_messages um
		    WHERE um.conversation_id = c.id AND um.kind = 'text'
		      AND um.sender_id IS DISTINCT FROM ?
		      AND um.created_at > COALESCE(`+readCol+`, 'epoch'::timestamptz)) AS unread
		FROM conversations c
		LEFT JOIN chat_messages m ON m.id = c.last_message_id
		WHERE `+ownerCol+` = ?
		ORDER BY c.last_message_at DESC NULLS LAST, c.created_at DESC
		LIMIT ?`, userID, userID, clampLimit(limit, 50, 200)).Scan(&rows).Error
	return rows, err
}

func (r *GormChatRepository) TouchConversation(ctx context.Context, id, messageID uuid.UUID, at time.Time) error {
	return database.Conn(ctx, r.db).Model(&domain.Conversation{}).Where("id = ?", id).
		Updates(map[string]any{"last_message_id": messageID, "last_message_at": at, "updated_at": at}).Error
}

func (r *GormChatRepository) MarkConversationRead(ctx context.Context, id uuid.UUID, asPsychic bool, at time.Time) error {
	col := "client_last_read_at"
	if asPsychic {
		col = "psychic_last_read_at"
	}
	// Never move the read marker backwards.
	return database.Conn(ctx, r.db).Model(&domain.Conversation{}).
		Where("id = ? AND ("+col+" IS NULL OR "+col+" < ?)", id, at).
		Update(col, at).Error
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
	return r.claim(ctx, limit, "status = ? AND next_billing_at <= ?", domain.SessionActive, now)
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

func (r *GormChatRepository) CreateMessage(ctx context.Context, m *domain.Message) (bool, error) {
	res := database.Conn(ctx, r.db).Clauses(clause.OnConflict{
		Columns:     []clause.Column{{Name: "sender_id"}, {Name: "client_msg_id"}},
		TargetWhere: clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "client_msg_id IS NOT NULL"}}},
		DoNothing:   true,
	}).Create(m)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
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

func (r *GormChatRepository) ListMessages(ctx context.Context, conversationID uuid.UUID, before *Cursor, limit int) ([]domain.Message, error) {
	q := database.Conn(ctx, r.db).Where("conversation_id = ?", conversationID)
	q = applyCursor(q, before, "created_at", "id")
	var out []domain.Message
	err := q.Order("created_at DESC, id DESC").Limit(clampLimit(limit, 50, 200)).Find(&out).Error
	return out, err
}

func (r *GormChatRepository) ListMessagesAfter(ctx context.Context, conversationID uuid.UUID, after time.Time, limit int) ([]domain.Message, error) {
	var out []domain.Message
	err := database.Conn(ctx, r.db).Where("conversation_id = ? AND created_at > ?", conversationID, after).
		Order("created_at ASC, id ASC").Limit(clampLimit(limit, 200, 500)).Find(&out).Error
	return out, err
}

func (r *GormChatRepository) CountSessionMessages(ctx context.Context, sessionID uuid.UUID) (int64, error) {
	var n int64
	err := database.Conn(ctx, r.db).Model(&domain.Message{}).Where("session_id = ? AND kind = ?", sessionID, domain.MessageText).Count(&n).Error
	return n, err
}

func (r *GormChatRepository) SaveBookmark(ctx context.Context, b *domain.SavedMessage) error {
	return database.Conn(ctx, r.db).Clauses(clause.OnConflict{DoNothing: true}).Create(b).Error
}

func (r *GormChatRepository) DeleteBookmark(ctx context.Context, userID, messageID uuid.UUID) error {
	return database.Conn(ctx, r.db).Where("user_id = ? AND message_id = ?", userID, messageID).Delete(&domain.SavedMessage{}).Error
}

func (r *GormChatRepository) ListBookmarks(ctx context.Context, userID uuid.UUID, conversationID *uuid.UUID, limit int) ([]domain.Message, error) {
	q := database.Conn(ctx, r.db).Table("chat_messages AS m").Select("m.*").
		Joins("JOIN saved_messages s ON s.message_id = m.id").Where("s.user_id = ?", userID)
	if conversationID != nil {
		q = q.Where("m.conversation_id = ?", *conversationID)
	}
	var out []domain.Message
	err := q.Order("s.created_at DESC").Limit(clampLimit(limit, 100, 200)).Scan(&out).Error
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
