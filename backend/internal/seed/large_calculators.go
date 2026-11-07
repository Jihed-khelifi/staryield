package seed

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Load-test data for the calculator usage and conversion reports (revisions
// 3.0, §17): anonymous visitors of the three calculators, their per-day
// visits, and a share who signed up, so the report and conversion query
// plans are checked at realistic sizes. Run it alone with
//
//	go run ./cmd/seed -profile=large -extend "calculator visitors"
func init() {
	RegisterLarge(Generator{Name: "calculator visitors", Run: func(ctx context.Context, d *Dataset) error {
		o := CalculatorSeedOptions{Visitors: d.Opts.Users, Tag: d.Tag, Now: d.Now, ConvertedShare: 0.05, Days: 365, UserEmailLike: "%@" + LoadEmailDomain}
		res, err := SeedCalculatorVisitors(ctx, d.DB, o)
		if err != nil {
			return err
		}
		d.Counts["web_visitors"] += int(res.Visitors)
		d.Counts["calculator_visits"] += int(res.Visits)
		d.Counts["short_links (calculators)"] += int(res.Links)
		d.Log.Info("calculator visitors generated", zap.Int64("visitors", res.Visitors), zap.Int64("visits", res.Visits),
			zap.Int64("converted", res.Converted))
		return nil
	}})
}

// CalculatorSeedOptions size the synthetic calculator traffic.
type CalculatorSeedOptions struct {
	Visitors       int       // anonymous visitors (about five visit rows each)
	Tag            string    // namespaces visitor ids and link codes (letters and digits)
	Now            time.Time // the end of the history
	Days           int       // history length, at most 700
	ConvertedShare float64   // share of visitors linked to a client account
	// UserEmailLike selects the client accounts visitors convert into.
	UserEmailLike string
}

// CalculatorSeedResult counts the inserted rows.
type CalculatorSeedResult struct{ Visitors, Visits, Links, Converted int64 }

// calculatorSources are (utm_source, medium, weight in percent, linkable).
var calculatorSources = []struct {
	source, medium string
	weight         int
	linked         bool
}{
	{"", "", 40, false}, {"google", "organic", 18, false}, {"instagram", "social", 12, true}, {"tiktok", "paid_social", 9, true},
	{"newsletter", "email", 8, true}, {"facebook", "social", 7, true}, {"youtube", "video", 6, true},
}

// SeedCalculatorVisitors inserts web_visitors and calculator_visits with
// generate_series: visits spread over the history, three locales and kinds,
// a share of visitors from UTM sources and short links, and ConvertedShare of
// them linked to existing client accounts (first seen shortly before the
// account was created). Visitor ids are prefixed "lv<Tag>-"; running it again
// with the same Tag adds nothing.
func SeedCalculatorVisitors(ctx context.Context, db *gorm.DB, o CalculatorSeedOptions) (CalculatorSeedResult, error) {
	var res CalculatorSeedResult
	if o.Visitors < 1 || o.Days < 1 || o.Days > 700 || o.ConvertedShare < 0 || o.ConvertedShare > 1 {
		return res, errors.New("seed: calculator visitors need Visitors >= 1, 1 <= Days <= 700 and 0 <= ConvertedShare <= 1")
	}
	for _, c := range o.Tag {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			return res, errors.New("seed: tag must be letters and digits")
		}
	}
	if o.Now.IsZero() {
		o.Now = time.Now().UTC()
	}
	now := o.Now.UTC()
	converted := int(float64(o.Visitors) * o.ConvertedShare)
	prefix := "lv" + o.Tag + "-"

	// Short links of a synthetic campaign, for the by_link breakdown.
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		r := tx.Exec(`INSERT INTO campaigns (id, name, slug, status, created_at, updated_at)
			VALUES (gen_random_uuid(), ?, ?, 'active', ?, ?) ON CONFLICT (slug) DO NOTHING`, "Calculator load "+o.Tag, "calc-load-"+lower(o.Tag), now, now)
		if r.Error != nil {
			return r.Error
		}
		for i, s := range calculatorSources {
			if !s.linked {
				continue
			}
			for n := 1; n <= 2; n++ {
				code := fmt.Sprintf("cl%s%d%d", o.Tag, i, n)
				if len(code) > 32 {
					code = code[:32]
				}
				r := tx.Exec(`INSERT INTO short_links (id, code, campaign_id, destination, utm_source, utm_medium, utm_campaign, label, created_at, updated_at)
					SELECT gen_random_uuid(), ?, c.id, '/en/calculators/natal-chart', ?, ?, c.slug, ?, ?, ? FROM campaigns c WHERE c.slug = ?
					ON CONFLICT (lower(code)) DO NOTHING`, code, s.source, s.medium, "Calculator load "+s.source, now, now, "calc-load-"+lower(o.Tag))
				if r.Error != nil {
					return r.Error
				}
				res.Links += r.RowsAffected
			}
		}
		return nil
	})
	if err != nil {
		return res, fmt.Errorf("calculator links: %w", err)
	}

	// Visitors. Source buckets by cumulative weight.
	sourceCase, mediumCase, linkedCase := "CASE", "CASE", "CASE"
	cum := 0
	for _, s := range calculatorSources {
		cum += s.weight
		sourceCase += fmt.Sprintf(" WHEN g.r_src < %d THEN '%s'", cum, s.source)
		mediumCase += fmt.Sprintf(" WHEN g.r_src < %d THEN '%s'", cum, s.medium)
		linkedCase += fmt.Sprintf(" WHEN g.r_src < %d THEN %t", cum, s.linked)
	}
	sourceCase, mediumCase, linkedCase = sourceCase+" END", mediumCase+" END", linkedCase+" END"
	r := db.WithContext(ctx).Exec(`
		WITH cu AS (
			SELECT row_number() OVER (ORDER BY id) AS rn, id, created_at FROM users
			WHERE role = 'client' AND email LIKE ? AND created_at <= ? ORDER BY id LIMIT ?
		), g AS (
			SELECT i, random() AS r_t, random() * 100 AS r_src, random() AS r_loc, random() AS r_link, random() AS r_len, random() AS r_ref
			FROM generate_series(1, ?::int) AS i
		), base AS (
			SELECT g.*, cu.id AS user_id, cu.created_at AS user_at,
			       CASE WHEN cu.id IS NOT NULL THEN cu.created_at - (g.r_t * interval '3 days')
			            ELSE ?::timestamptz - (g.r_t * (?::int * interval '1 day')) END AS first_seen
			FROM g LEFT JOIN cu ON cu.rn = g.i
		)
		INSERT INTO web_visitors (anonymous_id, first_seen_at, last_seen_at, first_locale, utm_source, utm_medium, utm_campaign,
		                          link_id, campaign_id, referrer_host, user_id, linked_at, signed_up_at)
		SELECT ? || lpad(g.i::text, 8, '0'), g.first_seen, LEAST(?::timestamptz, g.first_seen + (g.r_len * interval '20 days')),
		       CASE WHEN g.r_loc < 0.5 THEN 'en' WHEN g.r_loc < 0.8 THEN 'fr' ELSE 'es' END,
		       `+sourceCase+`, `+mediumCase+`,
		       CASE WHEN `+linkedCase+` THEN 'calc-load-' || ? ELSE '' END,
		       sl.id, sl.campaign_id,
		       CASE WHEN g.r_ref < 0.3 THEN 'www.google.com' WHEN g.r_ref < 0.4 THEN 'l.instagram.com' ELSE '' END,
		       g.user_id, g.user_at, g.user_at
		FROM base g
		LEFT JOIN LATERAL (
			SELECT s.id, s.campaign_id FROM short_links s
			WHERE `+linkedCase+` AND s.utm_source = `+sourceCase+` AND s.code LIKE ?
			ORDER BY s.code LIMIT 1 OFFSET (floor(g.r_link * 2))::int
		) sl ON TRUE
		ON CONFLICT (anonymous_id) DO NOTHING`,
		o.UserEmailLike, now, converted, o.Visitors, now, o.Days, prefix, now, lower(o.Tag), "cl"+o.Tag+"%")
	if r.Error != nil {
		return res, fmt.Errorf("web visitors: %w", r.Error)
	}
	res.Visitors = r.RowsAffected
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM web_visitors WHERE anonymous_id LIKE ? AND user_id IS NOT NULL`, prefix+"%").
		Row().Scan(&res.Converted); err != nil {
		return res, err
	}

	// Visits are generated once per tag: a second run adds none (an
	// anti-join against a million rows is quadratic without statistics).
	var haveVisits bool
	if err := db.WithContext(ctx).Raw(`SELECT EXISTS (SELECT 1 FROM calculator_visits WHERE anonymous_id LIKE ?)`, prefix+"%").Row().Scan(&haveVisits); err != nil {
		return res, err
	}
	if haveVisits {
		return res, db.WithContext(ctx).Exec("ANALYZE web_visitors").Error
	}

	// Visits: 1-9 rows per visitor (about five), mostly in the visitor's
	// locale. Values derive from a hash of (visitor, n) so every row differs
	// (uncorrelated random() would be evaluated once) and a run is reproducible.
	r = db.WithContext(ctx).Exec(`
		INSERT INTO calculator_visits (day, anonymous_id, kind, locale, views, started, completed, signed_in)
		SELECT LEAST(?::date, (v.first_seen_at + CASE
		           -- converted visitors browse before they sign up
		           WHEN v.signed_up_at IS NOT NULL THEN ((v.h / 13) % GREATEST(1, extract(epoch FROM (v.signed_up_at - v.first_seen_at))::bigint)) * interval '1 second'
		           ELSE (v.h / 13) % 30 * interval '1 day' END)::date), v.anonymous_id,
		       CASE WHEN v.h % 100 < 50 THEN 'natal-chart' WHEN v.h % 100 < 70 THEN 'path-of-life' ELSE 'astrology' END,
		       CASE WHEN (v.h / 100) % 100 < 95 THEN v.first_locale ELSE (ARRAY['en','fr','es'])[1 + (v.h / 10000) % 3] END,
		       (1 + (v.h / 1000000) % 3)::int,
		       CASE WHEN (v.h / 7) % 100 < 45 THEN 1 ELSE 0 END,
		       CASE WHEN (v.h / 7) % 100 < 45 AND (v.h / 19) % 100 < 60 THEN 1 ELSE 0 END,
		       (v.h / 17) % 100 < 4
		FROM (
			SELECT w.anonymous_id, w.first_seen_at, w.first_locale, w.signed_up_at, abs(hashtext(w.anonymous_id || ':' || n)::bigint) AS h
			FROM web_visitors w
			CROSS JOIN LATERAL generate_series(1, 1 + (abs(hashtext(w.anonymous_id)::bigint) % 9)::int) AS n
			WHERE w.anonymous_id LIKE ?
		) v
		ON CONFLICT (day, anonymous_id, kind, locale) DO NOTHING`, now, prefix+"%")
	if r.Error != nil {
		return res, fmt.Errorf("calculator visits: %w", r.Error)
	}
	res.Visits = r.RowsAffected
	// last_seen_at covers the last visit (the roll-up writes both together,
	// and the reports rely on it).
	if err := db.WithContext(ctx).Exec(`UPDATE web_visitors w SET last_seen_at = LEAST(?::timestamptz, GREATEST(w.last_seen_at, m.d::timestamp AT TIME ZONE 'UTC' + interval '1 day' - interval '1 second'))
		FROM (SELECT anonymous_id, max(day) AS d FROM calculator_visits WHERE anonymous_id LIKE ? GROUP BY anonymous_id) m
		WHERE w.anonymous_id = m.anonymous_id AND w.last_seen_at < m.d::timestamp AT TIME ZONE 'UTC' + interval '1 day' - interval '1 second'`, now, prefix+"%").Error; err != nil {
		return res, fmt.Errorf("web visitors last seen: %w", err)
	}
	return res, db.WithContext(ctx).Exec("ANALYZE web_visitors").Error
}

func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
