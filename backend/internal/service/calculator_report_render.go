package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/astro/geo"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
)

// revisions 6.0 (§26.4): recalculation of the report from the calculator's own
// request, validated by the calculator's own rules.

// SunSignInput is POST /astrology/sun-sign (and the "astrology" report input).
type SunSignInput struct {
	BirthDate string `json:"birth_date"`
	BirthTime string `json:"birth_time"`
	PlaceID   string `json:"place_id"`
}

// builtReport is a recalculated report, before the lead-dependent parts (the
// unsubscribe link) are known.
type builtReport struct {
	kind domain.ReportKind
	data mail.CalculatorReportData
}

// reportStrictJSON decodes raw into v, refusing unknown fields and trailing data
// exactly like the calculators' own endpoints (handler.bind).
func reportStrictJSON(raw []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("trailing data")
	}
	return nil
}

var errReportInput = apperror.Validation(map[string]string{"input": "Check the details of the calculation and try again."})

// build recalculates the report of one calculator from its request body.
func (s *CalculatorReportService) build(kind domain.ReportKind, raw []byte) (*builtReport, error) {
	out := &builtReport{kind: kind, data: mail.CalculatorReportData{Kind: string(kind), FrontendURL: s.d.Cfg.FrontendURL,
		Disclaimer: chartDisclaimer}}
	var date, clock, place, house string
	switch kind {
	case domain.ReportNatalChart:
		var in BirthInput
		if err := reportStrictJSON(raw, &in); err != nil {
			return nil, errReportInput
		}
		// The report is the on-screen result: the spread with its houses. Anything
		// else the visitor asked the calculator for does not change it.
		if _, msg := parseIncludes(in.Include); msg != "" {
			return nil, apperror.Validation(map[string]string{"include": msg})
		}
		in.Include = []string{includeTarot}
		view, b, err := s.d.Astro.natal(in)
		if err != nil {
			return nil, err
		}
		n := &mail.NatalReport{Spread: *view.Tarot, TimeKnown: view.Chart.TimeKnown, HouseSystem: string(view.Chart.HouseSystemUsed),
			FallbackReason: view.Chart.FallbackReason != "", WallTimeStatus: view.Chart.WallTimeStatus, BirthTime: view.Chart.BirthTime}
		if n.HouseSystem == "" {
			n.HouseSystem = string(view.Tarot.HouseSystem)
		}
		if view.Chart.Place != nil {
			n.PlaceLabel = view.Chart.Place.Label()
		}
		out.data.Natal, out.data.Attribution = n, view.Attribution
		date, clock, place, house = view.Chart.BirthDate, b.clock, strings.TrimSpace(in.PlaceID), string(b.houseSystem)
	case domain.ReportAstrology:
		var in SunSignInput
		if err := reportStrictJSON(raw, &in); err != nil {
			return nil, errReportInput
		}
		view, b, err := s.d.Astro.sunSign(BirthInput{BirthDate: in.BirthDate, BirthTime: in.BirthTime, PlaceID: in.PlaceID})
		if err != nil {
			return nil, err
		}
		sun := &mail.SunReport{Sign: view.SunSign, Uncertain: view.Uncertain, Alternatives: view.Alternatives,
			Element: view.Element, Modality: view.Modality, BirthTime: b.clock}
		if b.place != nil {
			sun.PlaceLabel, out.data.Attribution = b.place.Label(), geo.Attribution
		}
		out.data.Sun = sun
		date, clock, place = b.date.Format("2006-01-02"), b.clock, strings.TrimSpace(in.PlaceID)
	default:
		var in NumerologyInput
		if err := reportStrictJSON(raw, &in); err != nil {
			return nil, errReportInput
		}
		view, err := s.d.Astro.numerology(in)
		if err != nil {
			return nil, err
		}
		out.data.Numbers = &mail.NumbersReport{LifePath: view.LifePathReduction, PersonalYear: view.PersonalYear,
			PersonalMonth: view.PersonalMonth, AsOf: view.AsOf}
		date = strings.TrimSpace(in.BirthDate)
	}
	out.data.BirthDate = date
	out.data.ViewPath = "/calculators/" + string(kind) + mail.ReportFragment(string(kind), date, clock, place, house)
	return out, nil
}

// render builds the e-mail content for a recalculated report and the lead it
// is sent to: the unsubscribe link is only included while the address is
// subscribed to marketing e-mail.
func (s *CalculatorReportService) render(b *builtReport, locale string, lead *domain.EmailLead) (mail.Content, error) {
	data := b.data
	data.Locale = locale
	if lead.MarketingStatus == domain.MarketingSubscribed {
		data.UnsubscribePath = "/unsubscribe?token=" + s.UnsubscribeToken(lead.ID)
	}
	return mail.CalculatorReport(data)
}
