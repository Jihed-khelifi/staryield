package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
)

// Admin oversight of private client nicknames (§23.4 / §9.5 pattern). The
// nickname is only ever returned by this explicit, step-up protected call,
// with a stated reason, and every call is audited without the nickname text.

const msgAliasAdminReason = "State why you need to see this nickname."

// AdminAliasParty names one side of the session in the alias response.
type AdminAliasParty struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Slug string    `json:"slug,omitempty"`
}

// AdminClientAliasView is the advisor's nickname for the session's client.
type AdminClientAliasView struct {
	Alias     *string         `json:"alias"`
	UpdatedAt *time.Time      `json:"updated_at,omitempty"`
	Client    AdminAliasParty `json:"client"`
	Advisor   AdminAliasParty `json:"advisor"`
}

// ClientAlias reveals the nickname the session's advisor gave the session's
// client (nil when none). The reason is required and audited as
// client_alias.viewed.
func (s *AdminService) ClientAlias(ctx context.Context, sessionID uuid.UUID, reason string) (*AdminClientAliasView, error) {
	why, err := adminReason(reason, msgAliasAdminReason)
	if err != nil {
		return nil, err
	}
	if s.chat == nil || s.chat.aliases == nil {
		return nil, apperror.FeatureUnavailable("Nicknames are not available yet.")
	}
	sess, err := s.chatRepo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, mapRepoErr(err, "Session")
	}
	ps := s.chat.newParticipants()
	client, advisor := ps.client(ctx, sess.ClientID), ps.psychic(ctx, sess.PsychicID)
	out := &AdminClientAliasView{
		Client:  AdminAliasParty{ID: client.ID, Name: client.Name},
		Advisor: AdminAliasParty{ID: advisor.ID, Name: advisor.Name, Slug: advisor.Slug},
	}
	row, err := s.chat.aliases.repo.Get(ctx, sess.PsychicID, sess.ClientID)
	switch {
	case err == nil:
		if alias, ok := s.chat.openAlias(ctx, row.PsychicID, row.ClientID, row.AliasEnc); ok {
			out.Alias, out.UpdatedAt = &alias, &row.UpdatedAt
		}
	case !isNotFound(err):
		return nil, apperror.Internal(err)
	}
	// The nickname text is never logged.
	s.record(ctx, "client_alias.viewed", "user", sess.ClientID.String(), map[string]any{
		"session_id": sess.ID.String(), "conversation_id": sess.ConversationID.String(),
		"client_id": sess.ClientID.String(), "psychic_id": sess.PsychicID.String(),
		"reason": why, "found": out.Alias != nil,
	})
	return out, nil
}
