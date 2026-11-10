package service

import (
	"context"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// Private client nicknames (revisions 4.0, §23.4). An advisor can give a
// client a nickname that only that advisor sees. The alias is sealed at rest
// like notes, is delivered in its own field on advisor-scoped responses
// only (the advisor's inbox rows, the client card and the alias lookup) and
// is never merged into a name, a session view or an event the client or
// another advisor receives.

const (
	// maxAliasLookup bounds GET /chat/aliases?client_ids=.
	maxAliasLookup  = 50
	msgAliasInvalid = "Use 1 to 40 characters, without special characters."
	msgAliasIDs     = "Send between 1 and 50 comma-separated client ids."
)

// ClientAliasView is an advisor's alias for one client.
type ClientAliasView struct {
	ClientID  uuid.UUID `json:"client_id"`
	Alias     string    `json:"alias"`
	UpdatedAt time.Time `json:"updated_at"`
}

// aliasStore groups the alias collaborators wired by WithAliases.
type aliasStore struct {
	repo     repository.AliasRepository
	sealer   *security.Sealer
	audit    repository.AuditRepository
	ipSecret string
}

// WithAliases enables client aliases (composition root). Without it the
// feature is off: no alias is ever returned and the endpoints answer 503.
func (c *ChatService) WithAliases(repo repository.AliasRepository, sealer *security.Sealer, audit repository.AuditRepository, ipSecret string) *ChatService {
	c.aliases = &aliasStore{repo: repo, sealer: sealer, audit: audit, ipSecret: ipSecret}
	return c
}

// aliasAD binds a sealed alias to its advisor and client, so a ciphertext
// copied to another row does not open.
func aliasAD(psychicID, clientID uuid.UUID) string {
	return "alias:" + psychicID.String() + ":" + clientID.String()
}

// NormalizeAlias trims the alias and collapses inner whitespace to single
// spaces. It must have 1 to 40 characters (code points) and no control or
// invisible formatting characters (those are refused, never silently
// stripped): 422 validation_failed on field "alias".
func NormalizeAlias(raw string) (string, error) {
	invalid := apperror.Validation(map[string]string{"alias": msgAliasInvalid})
	if !utf8.ValidString(raw) {
		return "", invalid
	}
	for _, r := range raw {
		if (unicode.IsControl(r) && !unicode.IsSpace(r)) || unicode.Is(unicode.Cf, r) {
			return "", invalid
		}
	}
	alias := strings.Join(strings.Fields(raw), " ")
	if n := utf8.RuneCountInString(alias); n < 1 || n > domain.ClientAliasMaxChars {
		return "", invalid
	}
	return alias, nil
}

func (c *ChatService) aliasesEnabled() error {
	if c.aliases == nil {
		return apperror.FeatureUnavailable("Nicknames are not available yet.")
	}
	return nil
}

// advisorConversation returns the conversation when the caller is its
// advisor; anyone else (clients, other advisors, unknown ids) gets 404.
func (c *ChatService) advisorConversation(ctx context.Context, p reqctx.Principal, id uuid.UUID) (*domain.Conversation, error) {
	conv, err := c.chat.GetConversation(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "Conversation")
	}
	if conv.PsychicID != p.UserID {
		return nil, apperror.NotFound("Conversation")
	}
	return conv, nil
}

func (c *ChatService) publishAlias(ctx context.Context, advisor, clientID uuid.UUID, alias *string, at time.Time) {
	var v any
	if alias != nil {
		v = *alias
	}
	// Durable, to the advisor's own user only.
	warnErr(ctx, "chat: publish alias", c.events.Publish(ctx, realtime.EventClientAliasUpdated,
		map[string]any{"client_id": clientID, "alias": v, "updated_at": at}, advisor))
}

func (c *ChatService) auditAlias(ctx context.Context, action string, conv *domain.Conversation) {
	// The alias text is never logged.
	recordAudit(ctx, c.aliases.audit, c.clock, c.aliases.ipSecret, action, "client_alias", conv.ID.String(),
		map[string]any{"client_id": conv.ClientID.String(), "conversation_id": conv.ID.String()})
}

// SetAlias gives the conversation's client a private nickname (replacing any
// earlier one).
func (c *ChatService) SetAlias(ctx context.Context, p reqctx.Principal, conversationID uuid.UUID, raw string) (*ClientAliasView, error) {
	if err := c.aliasesEnabled(); err != nil {
		return nil, err
	}
	alias, err := NormalizeAlias(raw)
	if err != nil {
		return nil, err
	}
	conv, err := c.advisorConversation(ctx, p, conversationID)
	if err != nil {
		return nil, err
	}
	now := c.clock.Now()
	enc, err := c.aliases.sealer.Seal(alias, aliasAD(conv.PsychicID, conv.ClientID))
	if err != nil {
		return nil, apperror.Internal(err)
	}
	row := &domain.ClientAlias{PsychicID: conv.PsychicID, ClientID: conv.ClientID, AliasEnc: enc, CreatedAt: now, UpdatedAt: now}
	if err := c.aliases.repo.Upsert(ctx, row); err != nil {
		return nil, apperror.Internal(err)
	}
	c.auditAlias(ctx, "client_alias.set", conv)
	c.publishAlias(ctx, conv.PsychicID, conv.ClientID, &alias, now)
	return &ClientAliasView{ClientID: conv.ClientID, Alias: alias, UpdatedAt: now}, nil
}

// ClearAlias removes the nickname (idempotent).
func (c *ChatService) ClearAlias(ctx context.Context, p reqctx.Principal, conversationID uuid.UUID) error {
	if err := c.aliasesEnabled(); err != nil {
		return err
	}
	conv, err := c.advisorConversation(ctx, p, conversationID)
	if err != nil {
		return err
	}
	existed, err := c.aliases.repo.Delete(ctx, conv.PsychicID, conv.ClientID)
	if err != nil {
		return apperror.Internal(err)
	}
	if existed {
		c.auditAlias(ctx, "client_alias.cleared", conv)
		c.publishAlias(ctx, conv.PsychicID, conv.ClientID, nil, c.clock.Now())
	}
	return nil
}

// openAlias unseals a stored alias; an alias that cannot be opened is
// skipped (logged), never fatal.
func (c *ChatService) openAlias(ctx context.Context, psychicID, clientID uuid.UUID, enc string) (string, bool) {
	if c.aliases == nil || enc == "" {
		return "", false
	}
	alias, err := c.aliases.sealer.Open(enc, aliasAD(psychicID, clientID))
	if err != nil {
		warnErr(ctx, "chat: open alias", err)
		return "", false
	}
	return alias, true
}

// LookupAliases returns the caller's aliases for up to 50 clients in one
// query (only the ones that exist). The result is scoped by the caller's id,
// so it can never expose another advisor's aliases.
func (c *ChatService) LookupAliases(ctx context.Context, p reqctx.Principal, clientIDs []uuid.UUID) ([]ClientAliasView, error) {
	if err := c.aliasesEnabled(); err != nil {
		return nil, err
	}
	if n := len(clientIDs); n < 1 || n > maxAliasLookup {
		return nil, apperror.Validation(map[string]string{"client_ids": msgAliasIDs})
	}
	rows, err := c.aliases.repo.List(ctx, p.UserID, clientIDs)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := make([]ClientAliasView, 0, len(rows))
	for _, r := range rows {
		if alias, ok := c.openAlias(ctx, r.PsychicID, r.ClientID, r.AliasEnc); ok {
			out = append(out, ClientAliasView{ClientID: r.ClientID, Alias: alias, UpdatedAt: r.UpdatedAt})
		}
	}
	return out, nil
}

// aliasFor returns the advisor's alias for one client ("" when none).
func (c *ChatService) aliasFor(ctx context.Context, psychicID, clientID uuid.UUID) string {
	if c.aliases == nil {
		return ""
	}
	row, err := c.aliases.repo.Get(ctx, psychicID, clientID)
	if err != nil {
		if !isNotFound(err) {
			warnErr(ctx, "chat: load alias", err)
		}
		return ""
	}
	alias, _ := c.openAlias(ctx, row.PsychicID, row.ClientID, row.AliasEnc)
	return alias
}

// PurgeForUser hard-deletes the aliases written by or about a user and the
// user's likes (account erasure; the messages themselves are kept like the
// rest of the transcript).
func (c *ChatService) PurgeForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	var total int64
	if c.aliases != nil {
		n, err := c.aliases.repo.PurgeForUser(ctx, userID)
		if err != nil {
			return total, err
		}
		total += n
	}
	n, err := c.chat.PurgeLikesForUser(ctx, userID)
	return total + n, err
}
