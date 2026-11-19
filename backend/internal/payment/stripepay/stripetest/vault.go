package stripetest

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// ------------------------------------------------------------ customers

func (s *Server) customerJSON(c *customer) map[string]any {
	if c.deleted {
		return map[string]any{"id": c.id, "object": "customer", "deleted": true}
	}
	return map[string]any{"id": c.id, "object": "customer", "email": c.email, "name": c.name, "metadata": c.metadata, "livemode": s.livemode}
}

func (s *Server) createCustomer(req Request) (int, []byte, bool) {
	f := req.Form
	c := &customer{id: s.next("cus"), email: f.Get("email"), name: f.Get("name"), metadata: map[string]string{}}
	for k, v := range f {
		if strings.HasPrefix(k, "metadata[") && len(v) > 0 {
			c.metadata[strings.TrimSuffix(strings.TrimPrefix(k, "metadata["), "]")] = v[0]
		}
	}
	s.customers[c.id] = c
	s.customerO = append(s.customerO, c.id)
	return ok(s.customerJSON(c))
}

func (s *Server) customerEndpoint(req Request, id string) (int, []byte, bool) {
	c, found := s.customers[id]
	if !found {
		return notFound("customer", id)
	}
	switch req.Method {
	case http.MethodGet:
		return ok(s.customerJSON(c))
	case http.MethodPost:
		if c.deleted {
			return notFound("customer", id)
		}
		if v, set := req.Form["email"]; set {
			c.email = v[0]
		}
		if v, set := req.Form["name"]; set {
			c.name = v[0]
		}
		return ok(s.customerJSON(c))
	case http.MethodDelete:
		if c.deleted {
			return notFound("customer", id)
		}
		c.deleted = true
		for _, m := range s.methods {
			if m.customer == id {
				m.customer = ""
			}
		}
		return ok(s.customerJSON(c))
	}
	return errorResponse(http.StatusMethodNotAllowed, "invalid_request_error", "", "", "Method not allowed.")
}

// Customer describes a customer the fake holds.
type Customer struct {
	ID, Email, Name string
	Metadata        map[string]string
	Deleted         bool
}

// Customers returns the customers created, in id order of creation.
func (s *Server) Customers() []Customer {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Customer
	for _, id := range s.customerO {
		c := s.customers[id]
		out = append(out, Customer{ID: c.id, Email: c.email, Name: c.name, Metadata: c.metadata, Deleted: c.deleted})
	}
	return out
}

// ----------------------------------------------------- payment methods

func (s *Server) methodJSON(m *method) map[string]any {
	return map[string]any{"id": m.id, "object": "payment_method", "type": "card", "card": m.spec.card(),
		"customer": nilIfEmpty(m.customer), "livemode": s.livemode}
}

func (s *Server) getMethod(id string) (int, []byte, bool) {
	m, ok2 := s.methods[id]
	if !ok2 {
		return notFound("payment method", id)
	}
	return ok(s.methodJSON(m))
}

func (s *Server) methodAction(req Request, id, action string) (int, []byte, bool) {
	m, found := s.methods[id]
	if !found {
		return notFound("payment method", id)
	}
	switch action {
	case "attach":
		cus := req.Form.Get("customer")
		if c, exists := s.customers[cus]; !exists || c.deleted {
			return notFound("customer", cus)
		}
		if m.customer != "" && m.customer != cus {
			return errorResponse(http.StatusBadRequest, "invalid_request_error", "", "payment_method",
				"The payment method you provided has already been attached to a customer.")
		}
		m.customer = cus
	case "detach":
		if m.customer == "" {
			return errorResponse(http.StatusBadRequest, "invalid_request_error", "", "payment_method",
				"This PaymentMethod is not attached to a customer.")
		}
		m.customer = ""
	}
	return ok(s.methodJSON(m))
}

// AddPaymentMethod mints a payment method of the given spec ("ok_visa_4242",
// "decline_insufficient_funds"…) belonging to customerID ("" = unattached) and
// returns its id. It is what a SetupIntent the browser confirmed leaves behind.
func (s *Server) AddPaymentMethod(customerID, specText string) string {
	sp, ok := parseSpec(specText)
	if !ok {
		panic("stripetest: unknown payment method spec " + specText)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addMethod(customerID, sp)
}

func (s *Server) addMethod(customerID string, sp spec) string {
	m := &method{id: s.next("pm"), customer: customerID, spec: sp}
	s.methods[m.id] = m
	return m.id
}

// PaymentMethodCustomer returns the customer a payment method belongs to.
func (s *Server) PaymentMethodCustomer(id string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.methods[id]
	if !ok {
		return "", false
	}
	return m.customer, true
}

// ------------------------------------------------ confirmation tokens

func (s *Server) getToken(id string) (int, []byte, bool) {
	sp, ok2 := s.parseToken(id)
	if !ok2 {
		return notFound("confirmation token", id)
	}
	expires := now() + 3600
	if sp.behaviour == "expired" {
		expires = now() - 60
	}
	out := map[string]any{"id": id, "object": "confirmation_token", "expires_at": expires, "livemode": s.livemode,
		"payment_intent": nilIfEmpty(s.used[id]), "setup_intent": nil,
		"payment_method_preview": map[string]any{"type": "card", "card": sp.card()}}
	return ok(out)
}

// ------------------------------------------------------- setup intents

func (s *Server) createSetupIntent(req Request) (int, []byte, bool) {
	cus := req.Form.Get("customer")
	if c, exists := s.customers[cus]; !exists || c.deleted {
		return notFound("customer", cus)
	}
	si := &setupIntent{id: s.next("seti"), customer: cus, usage: req.Form.Get("usage")}
	si.secret = si.id + "_secret_" + s.tag + strconv.Itoa(s.seq)
	s.setups[si.id] = si
	return ok(map[string]any{"id": si.id, "object": "setup_intent", "client_secret": si.secret, "customer": cus, "usage": si.usage,
		"status": "requires_payment_method", "payment_method_types": []string{"card"}, "livemode": s.livemode})
}

// ConfirmSetupIntent plays the browser: it confirms the SetupIntent with the
// given client secret using a card of the given spec, which creates a payment
// method attached to the SetupIntent's customer. It returns the payment
// method id (what the web app posts to POST /billing/payment-methods).
func (s *Server) ConfirmSetupIntent(clientSecret, specText string) string {
	sp, ok := parseSpec(specText)
	if !ok {
		panic("stripetest: unknown payment method spec " + specText)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, si := range s.setups {
		if si.secret == clientSecret {
			si.confirmed = true
			return s.addMethod(si.customer, sp)
		}
	}
	panic("stripetest: unknown setup intent client secret")
}

// SetupIntentRequests returns the form of every SetupIntent creation.
func (s *Server) SetupIntentRequests() []Request {
	return s.RequestsTo(http.MethodPost, "/v1/setup_intents")
}
