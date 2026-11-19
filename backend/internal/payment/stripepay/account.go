package stripepay

import (
	"context"
	"errors"
	"sort"

	"github.com/stripe/stripe-go/v87"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
)

// AccountStatus implements payment.AccountInspector: whether the account can
// take payments and receive payouts, what Stripe still needs from the
// merchant, the statement descriptor and the registered webhook endpoints.
// A restricted key without permission to list webhook endpoints still gets
// the account part (the admin page then shows no endpoint).
func (p *Provider) AccountStatus(ctx context.Context) (payment.AccountStatus, error) {
	acct, err := p.client.V1Accounts.Retrieve(ctx, nil)
	if err != nil {
		return payment.AccountStatus{}, p.mapError("retrieve account", err)
	}
	out := payment.AccountStatus{
		AccountID: acct.ID, Livemode: p.livemode, Country: acct.Country, DefaultCurrency: string(acct.DefaultCurrency),
		ChargesEnabled: acct.ChargesEnabled, PayoutsEnabled: acct.PayoutsEnabled, DetailsSubmitted: acct.DetailsSubmitted,
		RequirementsDue: []string{}, WebhookEndpoints: []payment.WebhookEndpointInfo{}, APIVersion: stripe.APIVersion, CheckedAt: p.now().UTC(),
	}
	if acct.Settings != nil && acct.Settings.Payments != nil {
		out.StatementDescriptor = acct.Settings.Payments.StatementDescriptor
	}
	if r := acct.Requirements; r != nil {
		seen := map[string]bool{}
		for _, list := range [][]string{r.PastDue, r.CurrentlyDue} {
			for _, item := range list {
				if !seen[item] {
					seen[item] = true
					out.RequirementsDue = append(out.RequirementsDue, item)
				}
			}
		}
		sort.Strings(out.RequirementsDue)
	}
	list := p.client.V1WebhookEndpoints.List(ctx, &stripe.WebhookEndpointListParams{ListParams: stripe.ListParams{Limit: stripe.Int64(100)}})
	if err := list.Err(); err != nil {
		mapped := p.mapError("list webhook endpoints", err)
		if errors.Is(mapped, payment.ErrConfiguration) {
			p.log.Warn("stripe: the API key may not list webhook endpoints; the status page shows none", zap.Error(mapped))
			return out, nil
		}
		return payment.AccountStatus{}, mapped
	}
	for _, e := range list.Data() {
		events := append([]string{}, e.EnabledEvents...)
		sort.Strings(events)
		out.WebhookEndpoints = append(out.WebhookEndpoints, payment.WebhookEndpointInfo{ID: e.ID, URL: e.URL, Status: e.Status,
			EnabledEvents: events, APIVersion: e.APIVersion})
	}
	return out, nil
}
