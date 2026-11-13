package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// revisions 5.0: pricing catalog, admin side (§24.1). Every write is audited
// and invalidates the cached catalog on all instances.

// AdminPack is a pack as the admin console sees it.
type AdminPack struct {
	ID                       uuid.UUID         `json:"id"`
	Code                     string            `json:"code"`
	Kind                     string            `json:"kind"`
	Name                     map[string]string `json:"name"`
	Tagline                  map[string]string `json:"tagline"`
	Label                    map[string]string `json:"label"`
	Credits                  int64             `json:"credits"`
	PriceCents               int64             `json:"price_cents"`
	Currency                 string            `json:"currency"`
	Active                   bool              `json:"active"`
	Featured                 bool              `json:"featured"`
	Reference                bool              `json:"reference"`
	NewClientsOnly           bool              `json:"new_clients_only"`
	SortOrder                int               `json:"sort_order"`
	Badges                   []PackBadge       `json:"badges"`
	PricePerCreditMillicents int64             `json:"price_per_credit_millicents"`
	PurchasesWindow          int64             `json:"purchases_window"`
	CreatedAt                time.Time         `json:"created_at"`
	UpdatedAt                time.Time         `json:"updated_at"`
}

// PackInput creates (POST) or updates (PATCH) a pack. On update the
// localized objects replace the whole object; code and kind are create-only.
type PackInput struct {
	Code           *string           `json:"code"`
	Kind           *string           `json:"kind"`
	Name           map[string]string `json:"name"`
	Tagline        map[string]string `json:"tagline"`
	Label          map[string]string `json:"label"`
	Credits        *int64            `json:"credits"`
	PriceCents     *int64            `json:"price_cents"`
	Active         *bool             `json:"active"`
	Featured       *bool             `json:"featured"`
	Reference      *bool             `json:"reference"`
	NewClientsOnly *bool             `json:"new_clients_only"`
	SortOrder      *int              `json:"sort_order"`
}

func adminPack(p *domain.CreditPack, badges []PackBadge, purchases int64) AdminPack {
	l := func(m domain.L10n) map[string]string {
		if m == nil {
			return map[string]string{}
		}
		return map[string]string(m)
	}
	if badges == nil {
		badges = []PackBadge{}
	}
	return AdminPack{ID: p.ID, Code: p.Code, Kind: p.Kind, Name: l(p.NameL10n), Tagline: l(p.TaglineL10n), Label: l(p.LabelL10n),
		Credits: p.Credits, PriceCents: p.PriceCents, Currency: p.Currency, Active: p.Active, Featured: p.Featured,
		Reference: p.Reference, NewClientsOnly: p.NewClientsOnly, SortOrder: p.SortOrder, Badges: badges,
		PricePerCreditMillicents: p.PriceCents * 1000 / max(p.Credits, 1), PurchasesWindow: purchases,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}
}

// adminPacks renders every pack with the badges the public page shows now.
func (s *PricingService) adminPacks(ctx context.Context) ([]AdminPack, error) {
	settings, err := s.settings(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	packs, err := s.d.Repo.Packs(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	counts, err := s.d.Repo.PurchaseCounts(ctx, s.d.Clock.Now().AddDate(0, 0, -settings.PopularWindowDays))
	if err != nil {
		return nil, apperror.Internal(err)
	}
	byPack := make(map[uuid.UUID]int64, len(counts))
	for _, c := range counts {
		byPack[c.PackID] = c.Purchases
	}
	var bp []badgePack
	for i := range packs {
		if packs[i].Active {
			bp = append(bp, badgePackOf(&packs[i]))
		}
	}
	badges := computeBadges(bp, byPack, settings.PopularMinPurchases)
	out := make([]AdminPack, 0, len(packs))
	for i := range packs {
		out = append(out, adminPack(&packs[i], badges[packs[i].ID], byPack[packs[i].ID]))
	}
	return out, nil
}

// AdminPacks lists every pack (inactive included) in canonical order.
func (s *PricingService) AdminPacks(ctx context.Context) ([]AdminPack, error) {
	return s.adminPacks(ctx)
}

func (s *PricingService) adminPackByID(ctx context.Context, id uuid.UUID) (*AdminPack, error) {
	all, err := s.adminPacks(ctx)
	if err != nil {
		return nil, err
	}
	for i := range all {
		if all[i].ID == id {
			return &all[i], nil
		}
	}
	return nil, apperror.NotFound("Credit pack")
}

func packWriteErr(err error) error {
	switch {
	case errors.Is(err, repository.ErrPackCodeTaken):
		return apperror.Validation(map[string]string{"code": msgPackCodeTaken})
	case errors.Is(err, repository.ErrIntroActive):
		return apperror.Conflict(apperror.CodeConflict, msgPackIntroActive)
	case errors.Is(err, repository.ErrReferenceTaken):
		return apperror.Conflict(apperror.CodeConflict, "Another pack was just made the reference. Reload and try again.")
	}
	return mapRepoErr(err, "Credit pack")
}

// CreatePack adds a pack. An intro pack defaults to new_clients_only.
func (s *PricingService) CreatePack(ctx context.Context, in PackInput) (*AdminPack, error) {
	fields := map[string]string{}
	code := ""
	if in.Code == nil {
		fields["code"] = msgBannerRequired
	} else if code = trimPackCode(*in.Code); !validPackCode(code) {
		fields["code"] = msgPackCode
	}
	kind := domain.PackStandard
	if in.Kind != nil {
		if kind = strings.TrimSpace(*in.Kind); kind != domain.PackStandard && kind != domain.PackIntro {
			fields["kind"] = msgPackKind
		}
	}
	if in.Name == nil {
		fields["name.en"] = msgPackNameEN
	}
	name := validatePackL10n("name", in.Name, domain.PackNameMax, true, fields)
	tagline := validatePackL10n("tagline", in.Tagline, domain.PackTaglineMax, false, fields)
	label := validatePackL10n("label", in.Label, domain.PackLabelMax, false, fields)
	validPackNumbers(in.Credits, in.PriceCents, true, fields)
	if in.SortOrder != nil && (*in.SortOrder < 0 || *in.SortOrder > 100000) {
		fields["sort_order"] = msgPackSortOrder
	}
	now := s.d.Clock.Now()
	p := &domain.CreditPack{ID: domain.NewID(), Code: code, Kind: kind, NameL10n: name, TaglineL10n: tagline, LabelL10n: label,
		Currency: s.d.Cfg.Currency, Active: true, CreatedAt: now, UpdatedAt: now}
	if len(fields) == 0 {
		p.Name, p.Credits, p.PriceCents = name["en"], *in.Credits, *in.PriceCents
	}
	if in.Active != nil {
		p.Active = *in.Active
	}
	if in.Featured != nil {
		p.Featured = *in.Featured
	}
	if in.Reference != nil {
		p.Reference = *in.Reference
	}
	p.NewClientsOnly = kind == domain.PackIntro
	if in.NewClientsOnly != nil {
		p.NewClientsOnly = *in.NewClientsOnly
	}
	if in.SortOrder != nil {
		p.SortOrder = *in.SortOrder
	}
	packRules(p, fields)
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.d.Repo.LockPacks(ctx); err != nil {
			return err
		}
		if in.SortOrder == nil {
			all, err := s.d.Repo.Packs(ctx)
			if err != nil {
				return err
			}
			for i := range all {
				if all[i].Active || all[i].SortOrder < 1000 {
					p.SortOrder = max(p.SortOrder, all[i].SortOrder+1)
				}
			}
		}
		if p.Reference {
			if err := s.d.Repo.ClearReference(ctx, p.ID); err != nil {
				return err
			}
		}
		return s.d.Repo.CreatePack(ctx, p)
	})
	if err != nil {
		return nil, packWriteErr(err)
	}
	s.changed(ctx)
	s.record(ctx, "pack.created", "credit_pack", p.ID.String(), map[string]any{"code": p.Code, "kind": p.Kind,
		"credits": p.Credits, "price_cents": p.PriceCents, "currency": p.Currency, "active": p.Active, "reference": p.Reference})
	return s.adminPackByID(ctx, p.ID)
}

// UpdatePack edits a pack under the catalog lock. Price and credit changes
// are audited with before and after.
func (s *PricingService) UpdatePack(ctx context.Context, id uuid.UUID, in PackInput) (*AdminPack, error) {
	fields := map[string]string{}
	var name, tagline, label domain.L10n
	if in.Name != nil {
		name = validatePackL10n("name", in.Name, domain.PackNameMax, true, fields)
	}
	if in.Tagline != nil {
		tagline = validatePackL10n("tagline", in.Tagline, domain.PackTaglineMax, false, fields)
	}
	if in.Label != nil {
		label = validatePackL10n("label", in.Label, domain.PackLabelMax, false, fields)
	}
	validPackNumbers(in.Credits, in.PriceCents, false, fields)
	if in.SortOrder != nil && (*in.SortOrder < 0 || *in.SortOrder > 100000) {
		fields["sort_order"] = msgPackSortOrder
	}
	empty := in.Code == nil && in.Kind == nil && in.Name == nil && in.Tagline == nil && in.Label == nil && in.Credits == nil &&
		in.PriceCents == nil && in.Active == nil && in.Featured == nil && in.Reference == nil && in.NewClientsOnly == nil && in.SortOrder == nil
	if empty {
		fields["name"] = msgPackNothing
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}

	var changed []string
	var before, after domain.CreditPack
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		changed = nil
		if err := s.d.Repo.LockPacks(ctx); err != nil {
			return err
		}
		cur, err := s.d.Repo.PackForUpdate(ctx, id)
		if err != nil {
			return err
		}
		before = *cur
		next := *cur
		bad := map[string]string{}
		if in.Code != nil && trimPackCode(*in.Code) != cur.Code {
			bad["code"] = msgPackImmutable
		}
		if in.Kind != nil && strings.TrimSpace(*in.Kind) != cur.Kind {
			bad["kind"] = msgPackImmutable
		}
		set := func(field string, differs bool, apply func()) {
			if differs {
				apply()
				changed = append(changed, field)
			}
		}
		if in.Name != nil {
			set("name", !equalL10n(name, cur.NameL10n), func() { next.NameL10n, next.Name = name, name["en"] })
		}
		if in.Tagline != nil {
			set("tagline", !equalL10n(tagline, cur.TaglineL10n), func() { next.TaglineL10n = tagline })
		}
		if in.Label != nil {
			set("label", !equalL10n(label, cur.LabelL10n), func() { next.LabelL10n = label })
		}
		if in.Credits != nil {
			set("credits", *in.Credits != cur.Credits, func() { next.Credits = *in.Credits })
		}
		if in.PriceCents != nil {
			set("price_cents", *in.PriceCents != cur.PriceCents, func() { next.PriceCents = *in.PriceCents })
		}
		if in.Active != nil {
			set("active", *in.Active != cur.Active, func() { next.Active = *in.Active })
		}
		if in.Featured != nil {
			set("featured", *in.Featured != cur.Featured, func() { next.Featured = *in.Featured })
		}
		if in.Reference != nil {
			set("reference", *in.Reference != cur.Reference, func() { next.Reference = *in.Reference })
		}
		if in.NewClientsOnly != nil {
			set("new_clients_only", *in.NewClientsOnly != cur.NewClientsOnly, func() { next.NewClientsOnly = *in.NewClientsOnly })
		}
		if in.SortOrder != nil {
			set("sort_order", *in.SortOrder != cur.SortOrder, func() { next.SortOrder = *in.SortOrder })
		}
		packRules(&next, bad)
		if next.Reference && !next.Active && cur.Reference && in.Active != nil && !*in.Active {
			bad["active"] = msgPackReference
		}
		if len(bad) > 0 {
			return apperror.Validation(bad)
		}
		after = next
		if len(changed) == 0 {
			return nil
		}
		if next.Reference && !cur.Reference {
			if err := s.d.Repo.ClearReference(ctx, id); err != nil {
				return err
			}
		}
		return s.d.Repo.SavePack(ctx, &next)
	})
	if err != nil {
		return nil, packWriteErr(err)
	}
	if len(changed) > 0 {
		s.changed(ctx)
		meta := map[string]any{"code": after.Code, "fields": strings.Join(changed, ",")}
		for _, f := range changed {
			switch f {
			case "credits":
				meta["credits_before"], meta["credits_after"] = before.Credits, after.Credits
			case "price_cents":
				meta["price_before"], meta["price_after"] = before.PriceCents, after.PriceCents
			case "active":
				meta["active"] = after.Active
			case "reference":
				meta["reference"] = after.Reference
			}
		}
		s.record(ctx, "pack.updated", "credit_pack", id.String(), meta)
	}
	return s.adminPackByID(ctx, id)
}

// DeletePack removes a pack no purchase references (409 otherwise).
func (s *PricingService) DeletePack(ctx context.Context, id uuid.UUID) error {
	var code string
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.d.Repo.LockPacks(ctx); err != nil {
			return err
		}
		p, err := s.d.Repo.PackForUpdate(ctx, id)
		if err != nil {
			return err
		}
		code = p.Code
		used, err := s.d.Repo.PackInUse(ctx, id)
		if err != nil {
			return err
		}
		if used {
			return apperror.Conflict(apperror.CodeConflict, "Purchases reference this pack. Deactivate it instead.")
		}
		return s.d.Repo.DeletePack(ctx, id)
	})
	if err != nil {
		if errors.Is(err, repository.ErrInUse) {
			return apperror.Conflict(apperror.CodeConflict, "Purchases reference this pack. Deactivate it instead.")
		}
		return mapRepoErr(err, "Credit pack")
	}
	s.changed(ctx)
	s.record(ctx, "pack.deleted", "credit_pack", id.String(), map[string]any{"code": code})
	return nil
}

// ReorderPacks sets the display order: ids must list every pack exactly once.
func (s *PricingService) ReorderPacks(ctx context.Context, ids []uuid.UUID) ([]AdminPack, error) {
	seen := map[uuid.UUID]bool{}
	for _, id := range ids {
		if seen[id] {
			return nil, apperror.Validation(map[string]string{"ids": msgPackOrder})
		}
		seen[id] = true
	}
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.d.Repo.LockPacks(ctx); err != nil {
			return err
		}
		current, err := s.d.Repo.Packs(ctx)
		if err != nil {
			return err
		}
		if len(current) != len(ids) {
			return apperror.Validation(map[string]string{"ids": msgPackOrder})
		}
		for i := range current {
			if !seen[current[i].ID] {
				return apperror.Validation(map[string]string{"ids": msgPackOrder})
			}
		}
		return s.d.Repo.SetPackOrder(ctx, ids)
	})
	if err != nil {
		return nil, mapRepoErr(err, "Credit pack")
	}
	s.changed(ctx)
	strIDs := make([]string, len(ids))
	for i, id := range ids {
		strIDs[i] = id.String()
	}
	s.record(ctx, "pack.reordered", "credit_pack", "", map[string]any{"ids": strings.Join(strIDs, ",")})
	return s.adminPacks(ctx)
}

// Settings returns the stored pricing settings (database, never the cache).
func (s *PricingService) Settings(ctx context.Context) (*PricingSettings, error) {
	st, err := s.d.Site.Setting(ctx, domain.SiteSettingPricing)
	if err != nil {
		return nil, mapRepoErr(err, "Setting")
	}
	out := pricingSettingsFrom(*st)
	if st.UpdatedBy != nil {
		if names, err := s.d.Site.UserNames(ctx, []uuid.UUID{*st.UpdatedBy}); err == nil {
			if n, ok := names[*st.UpdatedBy]; ok {
				out.UpdatedBy = &UserRef{ID: *st.UpdatedBy, Name: n}
			}
		}
	}
	return &out, nil
}

// UpdateSettings replaces the pricing settings. A stale expected_version is
// a 409 version_conflict.
func (s *PricingService) UpdateSettings(ctx context.Context, in PricingSettingsInput) (*PricingSettings, error) {
	next, fields := validatePricingSettings(in)
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	var before PricingSettings
	changed := false
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		changed = false
		st, err := s.d.Site.SettingForUpdate(ctx, domain.SiteSettingPricing)
		if err != nil {
			return err
		}
		before = pricingSettingsFrom(*st)
		if in.ExpectedVersion != nil && *in.ExpectedVersion != st.Version {
			return apperror.VersionConflict(st.Version)
		}
		next.Version, next.UpdatedAt = before.Version, before.UpdatedAt
		if next == before {
			return nil
		}
		changed = true
		now := s.d.Clock.Now()
		return s.d.Site.SaveSetting(ctx, domain.SiteSettingPricing, next.toMap(), st.Version+1, actor(ctx), now)
	})
	if err != nil {
		return nil, mapRepoErr(err, "Setting")
	}
	if changed {
		s.changed(ctx)
		view := func(p PricingSettings) map[string]any {
			return map[string]any{"rate_min": p.RateMin, "rate_max": p.RateMax, "popular_min_purchases": p.PopularMinPurchases,
				"popular_window_days": p.PopularWindowDays, "processing_fee_bps": p.ProcessingFeeBps,
				"processing_fixed_cents": p.ProcessingFixedCents}
		}
		s.record(ctx, "pricing.settings_updated", "site_setting", domain.SiteSettingPricing,
			map[string]any{"before": view(before), "after": view(next)})
	}
	return s.Settings(ctx)
}
