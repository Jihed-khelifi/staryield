package mail

import (
	"bytes"
	"fmt"
	"html/template"
	"net/url"
	"strings"

	"github.com/Jihed-khelifi/staryield/backend/internal/i18n"
)

// Content is a localised e-mail before layout: every string is already in
// the recipient's language. Values are plain text; the HTML layout escapes
// them, so user-provided names can never inject markup.
type Content struct {
	Subject    string
	Preheader  string
	Heading    string
	Paragraphs []string
	Action     *Action
	// Note is small print under the action (e.g. "ignore this e-mail").
	Note string
	// Details are labelled facts shown as a table under the paragraphs (a
	// receipt's pack, amount, date and reference; revisions 5.1).
	Details []Detail
	// Links are secondary links under the action (policies). A URL starting
	// with "/" is a locale-less app path and is made absolute by Render.
	Links []Action
}

// Detail is one labelled fact of a receipt-style e-mail. Label is translated;
// Value is plain text.
type Detail struct {
	Label string
	Value string
}

// Action is the primary call-to-action button.
type Action struct {
	Label string
	URL   string
}

// Layout carries the per-recipient frame of a message.
type Layout struct {
	Locale string
	// FrontendURL is the site origin (e.g. https://staryield.net).
	FrontendURL string
	// Category is the delivery category; notification e-mails get a footer
	// link to the notification settings and a List-Unsubscribe header.
	Category string
	// SupportEmail is the support address printed in the footer of every
	// e-mail (STARYIELD_MAIL_SUPPORT_INBOX; empty: DefaultSupportEmail).
	SupportEmail string
}

// Rendered is a ready-to-send message body.
type Rendered struct {
	Subject string
	Text    string
	HTML    string
	Headers map[string]string
}

// Render lays out c as text and HTML.
func Render(c Content, l Layout) (Rendered, error) {
	locale := i18n.Normalize(l.Locale)
	t := func(s string) string { return i18n.T(locale, s) }
	settings := settingsURL(l.FrontendURL, locale)
	support := strings.TrimSpace(l.SupportEmail)
	if support == "" {
		support = DefaultSupportEmail
	}
	details := make([]Detail, 0, len(c.Details))
	for _, d := range c.Details {
		details = append(details, Detail{Label: d.Label, Value: d.Value})
	}
	links := make([]Action, 0, len(c.Links))
	for _, a := range c.Links {
		if strings.HasPrefix(a.URL, "/") {
			a.URL = AppURL(l.FrontendURL, locale, a.URL)
		}
		links = append(links, a)
	}
	view := struct {
		Lang, Subject, Preheader, Heading, Note string
		Paragraphs                              []string
		Action                                  *Action
		Home, Settings                          string
		Footer, SettingsLabel, Notification     string
		Tagline, Rights                         string
		// revisions 5.1: company footer and receipt details (§25.4, §25.5).
		Company, Address, Support, SupportLabel, Hours string
		Details                                        []Detail
		Links                                          []Action
	}{
		Lang: locale, Subject: c.Subject, Preheader: c.Preheader, Heading: c.Heading, Note: c.Note, Paragraphs: c.Paragraphs,
		Action: c.Action, Home: home(l.FrontendURL, locale), Settings: settings,
		Footer:        t("This is an automated message from Staryield. Please do not reply."),
		SettingsLabel: t("Notification settings"),
		// revisions 5.1: the tagline matches the site's "entertainment purposes only" wording (§25.6).
		Tagline: t("Astrology and psychic readings for entertainment purposes only."),
		Rights:  t("For entertainment purposes only."),
		Company: CompanyLegalName, Address: CompanyAddress, Support: support, SupportLabel: t("Support"), Hours: t(SupportHours),
		Details: details, Links: links,
	}
	if l.Category == "notification" || l.Category == "marketing" {
		view.Notification = t("You receive this email because email notifications are on for your account.")
	}
	var html bytes.Buffer
	if err := htmlLayout.Execute(&html, view); err != nil {
		return Rendered{}, fmt.Errorf("mail: render html: %w", err)
	}
	var text strings.Builder
	text.WriteString(c.Heading)
	text.WriteString("\n\n")
	for _, p := range c.Paragraphs {
		text.WriteString(p)
		text.WriteString("\n\n")
	}
	for _, d := range details {
		text.WriteString(d.Label + ": " + d.Value + "\n")
	}
	if len(details) > 0 {
		text.WriteString("\n")
	}
	if c.Action != nil {
		text.WriteString(c.Action.Label)
		text.WriteString(":\n")
		text.WriteString(c.Action.URL)
		text.WriteString("\n\n")
	}
	for _, a := range links {
		text.WriteString(a.Label + ": " + a.URL + "\n")
	}
	if len(links) > 0 {
		text.WriteString("\n")
	}
	if c.Note != "" {
		text.WriteString(c.Note)
		text.WriteString("\n\n")
	}
	text.WriteString("— Staryield\n")
	text.WriteString(view.Company + "\n" + view.Address + "\n")
	text.WriteString(view.SupportLabel + ": " + view.Support + "\n" + view.Hours + "\n")
	text.WriteString(view.Footer)
	text.WriteString("\n")
	if view.Notification != "" {
		text.WriteString(view.Notification)
		text.WriteString("\n")
		text.WriteString(view.SettingsLabel + ": " + settings + "\n")
	}
	out := Rendered{Subject: oneLine(c.Subject), Text: text.String(), HTML: html.String(),
		Headers: map[string]string{"Auto-Submitted": "auto-generated"}}
	// List-Unsubscribe must be an https (or mailto) URI.
	if view.Notification != "" && strings.HasPrefix(settings, "https://") {
		out.Headers["List-Unsubscribe"] = "<" + settings + ">"
	}
	return out, nil
}

// oneLine strips CR/LF so a subject can never split into extra headers.
func oneLine(s string) string {
	s = strings.NewReplacer("\r", " ", "\n", " ").Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

func home(frontend, locale string) string {
	if frontend == "" {
		return ""
	}
	return strings.TrimRight(frontend, "/") + "/" + locale
}

func settingsURL(frontend, locale string) string {
	if frontend == "" {
		return ""
	}
	return strings.TrimRight(frontend, "/") + "/" + locale + "/profile/settings#notifications"
}

// AppURL turns a locale-less in-app path ("/chatroom?reader=x") into an
// absolute, localised URL on the frontend. Anything that is not a
// same-site path yields the localised home page.
func AppURL(frontend, locale, path string) string {
	locale = i18n.Normalize(locale)
	base := strings.TrimRight(frontend, "/")
	if path == "" || !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") || strings.Contains(path, "\\") {
		return base + "/" + locale
	}
	u, err := url.Parse(path)
	if err != nil || u.IsAbs() || u.Host != "" {
		return base + "/" + locale
	}
	return base + "/" + locale + path
}

// The layout uses tables and inline styles (the lowest common denominator of
// mail clients) in the brand palette: cream #E0D5C3, paper #F4F0EA, gold
// #AA8E62, CTA #C29A3D, ink #15100B.
var htmlLayout = template.Must(template.New("layout").Parse(`<!doctype html>
<html lang="{{.Lang}}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="color-scheme" content="light">
<title>{{.Subject}}</title>
</head>
<body style="margin:0;padding:0;background:#E0D5C3;color:#15100B;">
<div style="display:none;max-height:0;overflow:hidden;opacity:0;">{{.Preheader}}</div>
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#E0D5C3;">
<tr><td align="center" style="padding:32px 16px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;background:#F4F0EA;border:1px solid #AA8E62;border-radius:16px;">
<tr><td style="padding:28px 32px 8px 32px;font-family:Castellar,Cinzel,Georgia,'Times New Roman',serif;font-size:20px;letter-spacing:3px;color:#15100B;">
{{if .Home}}<a href="{{.Home}}" style="color:#15100B;text-decoration:none;">STARYIELD</a>{{else}}STARYIELD{{end}}
</td></tr>
<tr><td style="padding:8px 32px 0 32px;font-family:'Source Serif 4',Georgia,'Times New Roman',serif;">
<h1 style="margin:0 0 16px 0;font-family:Castellar,Cinzel,Georgia,'Times New Roman',serif;font-weight:normal;font-size:24px;line-height:1.3;color:#15100B;">{{.Heading}}</h1>
{{range .Paragraphs}}<p style="margin:0 0 14px 0;font-size:16px;line-height:1.6;color:#15100B;">{{.}}</p>
{{end}}</td></tr>
{{if .Details}}<tr><td style="padding:0 32px 8px 32px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="border-top:1px solid #E0D5C3;">
{{range .Details}}<tr><td style="padding:8px 0;border-bottom:1px solid #E0D5C3;font-family:'Source Serif 4',Georgia,serif;font-size:14px;line-height:1.5;color:#978F6E;width:38%;vertical-align:top;">{{.Label}}</td><td style="padding:8px 0;border-bottom:1px solid #E0D5C3;font-family:'Source Serif 4',Georgia,serif;font-size:14px;line-height:1.5;color:#15100B;vertical-align:top;word-break:break-word;">{{.Value}}</td></tr>
{{end}}</table>
</td></tr>{{end}}
{{if .Action}}<tr><td style="padding:8px 32px 8px 32px;">
<table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="background:#C29A3D;border-radius:8px;box-shadow:0 4px 15px rgba(194,154,61,0.42);">
<a href="{{.Action.URL}}" style="display:inline-block;padding:12px 28px;font-family:Castellar,Cinzel,Georgia,serif;font-size:15px;letter-spacing:1px;color:#15100B;text-decoration:none;">{{.Action.Label}}</a>
</td></tr></table>
<p style="margin:12px 0 0 0;font-family:'Source Serif 4',Georgia,serif;font-size:12px;line-height:1.5;color:#978F6E;word-break:break-all;">{{.Action.URL}}</p>
</td></tr>{{end}}
{{if .Links}}<tr><td style="padding:12px 32px 0 32px;font-family:'Source Serif 4',Georgia,serif;font-size:13px;line-height:1.7;color:#978F6E;">
{{range .Links}}<a href="{{.URL}}" style="color:#AA8E62;">{{.Label}}</a><br>
{{end}}</td></tr>{{end}}
{{if .Note}}<tr><td style="padding:8px 32px 0 32px;font-family:'Source Serif 4',Georgia,serif;font-size:13px;line-height:1.5;color:#978F6E;">{{.Note}}</td></tr>{{end}}
<tr><td style="padding:24px 32px 28px 32px;font-family:'Source Serif 4',Georgia,serif;font-size:12px;line-height:1.5;color:#978F6E;border-top:1px solid #E0D5C3;">
<p style="margin:16px 0 6px 0;">{{.Tagline}}</p>
<p style="margin:0 0 6px 0;"><strong>{{.Company}}</strong><br>{{.Address}}</p>
<p style="margin:0 0 6px 0;">{{.SupportLabel}}: <a href="mailto:{{.Support}}" style="color:#AA8E62;">{{.Support}}</a><br>{{.Hours}}</p>
<p style="margin:0 0 6px 0;">{{.Footer}}</p>
{{if .Notification}}<p style="margin:0 0 6px 0;">{{.Notification}}{{if .Settings}} <a href="{{.Settings}}" style="color:#AA8E62;">{{.SettingsLabel}}</a>{{end}}</p>{{end}}
<p style="margin:0;">© {{.Company}} · {{.Rights}}</p>
</td></tr>
</table>
</td></tr>
</table>
</body>
</html>
`))
