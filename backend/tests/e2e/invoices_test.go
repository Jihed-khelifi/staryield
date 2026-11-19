package e2e

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/mail"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// revisions 6.0 (docs/platform-extensions.md §26.6): invoices and credit notes.

const (
	invoicesPath      = "/api/v1/billing/invoices/"
	adminInvoicesPath = "/api/v1/admin/billing/invoices"
	testCard          = "sbx_a_visa_4242_12_2035"
)

// buyView buys a test pack with a card and returns the purchase as the client sees it.
func buyView(t *testing.T, c *client, credits int64, card service.PaymentMethodView) service.PurchaseView {
	t.Helper()
	p := pack(t, c, credits)
	var out service.PurchaseView
	c.must(http.StatusCreated, http.MethodPost, purchasesPath, map[string]any{"pack_id": p.ID, "payment_method_id": card.ID,
		"idempotency_key": uniq("purchase-key-"), "consent": purchaseConsent()}, &out)
	return out
}

func (e *env) invoiceYearPrefix(kind domain.InvoiceKind) string {
	return fmt.Sprintf("%s-%d-", kind.NumberPrefix(), e.clock.Now().UTC().Year())
}

func (e *env) invoicesOf(userID uuid.UUID) []domain.Invoice {
	e.t.Helper()
	var rows []domain.Invoice
	if err := e.app.DB.Where("user_id = ?", userID).Order("issued_at, number").Find(&rows).Error; err != nil {
		e.t.Fatal(err)
	}
	return rows
}

func (e *env) invoiceCount(where string, args ...any) int64 {
	e.t.Helper()
	var n int64
	if err := e.app.DB.Model(&domain.Invoice{}).Where(where, args...).Count(&n).Error; err != nil {
		e.t.Fatal(err)
	}
	return n
}

func (c *client) invoice(id uuid.UUID) service.InvoiceView {
	c.e.t.Helper()
	var v service.InvoiceView
	c.must(http.StatusOK, http.MethodGet, invoicesPath+id.String(), nil, &v)
	return v
}

// sequence is the numeric part of a number ("SY-2026-000042" -> 42).
func sequence(t *testing.T, number string) int {
	t.Helper()
	n, err := strconv.Atoi(number[strings.LastIndex(number, "-")+1:])
	if err != nil {
		t.Fatalf("number %q: %v", number, err)
	}
	return n
}

// receiptsOf waits until the outbox delivered n e-mails whose subject contains
// the text to the address and returns them, oldest first.
func receiptsOf(t *testing.T, e *env, to, subject string, n int) []mail.Message {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var got []mail.Message
		for _, m := range sentTo(e.flushMail(), to) {
			if strings.Contains(m.Subject, subject) {
				got = append(got, m)
			}
		}
		if len(got) >= n {
			return got
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected %d e-mails %q to %s, got %d", n, subject, to, len(got))
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestPurchaseIssuesOneInvoiceWithItsPDFAndReceipt(t *testing.T) {
	e := newEnv(t)
	e.ensurePacks()
	e.disableSignupTrial()
	c := e.registerClient()
	card := c.attachCard(testCard)
	if err := e.app.DB.Model(&domain.User{}).Where("id = ?", c.user.ID).Update("locale", "fr").Error; err != nil {
		t.Fatal(err)
	}

	view := buyView(t, c, 250, card)
	if view.Status != "succeeded" || view.Invoice == nil || !strings.HasPrefix(view.Invoice.Number, e.invoiceYearPrefix(domain.InvoiceKindInvoice)) {
		t.Fatalf("a paid purchase carries its invoice: %+v", view)
	}
	rows := e.invoicesOf(c.user.ID)
	if len(rows) != 1 || rows[0].ID != view.Invoice.ID || rows[0].Kind != domain.InvoiceKindInvoice {
		t.Fatalf("exactly one invoice: %+v", rows)
	}

	// The document: a snapshot of the buyer, the seller, the payment and the line.
	inv := c.invoice(view.Invoice.ID)
	if inv.Number != view.Invoice.Number || inv.Kind != "invoice" || inv.Currency != "USD" || inv.SubtotalCents != 999 || inv.TaxCents != 0 || inv.TotalCents != 999 {
		t.Fatalf("totals: %+v", inv)
	}
	if len(inv.Lines) != 1 || inv.Lines[0].Quantity != 1 || inv.Lines[0].AmountCents != 999 || inv.Lines[0].Credits != 250 || inv.Lines[0].Description == "" {
		t.Fatalf("lines: %+v", inv.Lines)
	}
	if inv.Buyer.Email != c.user.Email || inv.Buyer.Name != c.user.DisplayName || inv.Locale != "fr" || inv.OriginalInvoice != nil {
		t.Fatalf("buyer: %+v locale %q", inv.Buyer, inv.Locale)
	}
	if inv.Seller.LegalName != mail.CompanyLegalName || strings.Join(inv.Seller.Address, ", ") != mail.CompanyAddress || inv.Seller.Email == "" || inv.Seller.Website == "" {
		t.Fatalf("seller: %+v", inv.Seller)
	}
	if inv.Payment.Provider != "sandbox_a" || inv.Payment.Card == nil || inv.Payment.Card.Last4 != "4242" || inv.Payment.Card.Brand != "visa" ||
		inv.Payment.Reference == "" || inv.Payment.PaidAt == nil || inv.Payment.RefundedAt != nil {
		t.Fatalf("payment: %+v", inv.Payment)
	}
	if time.Since(inv.IssuedAt) > time.Minute || inv.TransactionID == uuid.Nil {
		t.Fatalf("issued_at / transaction: %+v", inv)
	}
	// The buyer is sealed at rest.
	if strings.Contains(rows[0].BuyerSealed, c.user.Email) || strings.Contains(rows[0].BuyerSealed, c.user.DisplayName) || rows[0].BuyerSealed == "" {
		t.Fatal("the buyer snapshot must be stored sealed")
	}
	var dump string
	e.app.DB.Raw("SELECT row_to_json(i)::text FROM invoices i WHERE id = ?", inv.ID).Scan(&dump)
	if strings.Contains(dump, c.user.Email) {
		t.Fatal("the buyer's e-mail appears in clear in the invoices table")
	}

	// A replay of the purchase returns the same invoice; nothing is issued twice.
	key := uniq("replay-")
	p := pack(t, c, 600)
	body := map[string]any{"pack_id": p.ID, "payment_method_id": card.ID, "idempotency_key": key, "consent": purchaseConsent()}
	var first, replay service.PurchaseView
	c.must(http.StatusCreated, http.MethodPost, purchasesPath, body, &first)
	c.must(http.StatusCreated, http.MethodPost, purchasesPath, body, &replay)
	if first.Invoice == nil || replay.Invoice == nil || first.Invoice.ID != replay.Invoice.ID || e.invoiceCount("user_id = ?", c.user.ID) != 2 {
		t.Fatalf("replay: %+v / %+v", first.Invoice, replay.Invoice)
	}

	// The purchase history carries the invoice.
	var history service.Page[service.PurchaseView]
	c.must(http.StatusOK, http.MethodGet, purchasesPath, nil, &history)
	if len(history.Items) != 2 {
		t.Fatalf("history: %+v", history.Items)
	}
	for _, it := range history.Items {
		if it.Invoice == nil {
			t.Fatalf("purchase %s has no invoice in the history", it.ID)
		}
	}

	// PDF: owner only, with the headers of the contract, in the invoice's language.
	r := c.call(http.MethodGet, invoicesPath+inv.ID.String()+"/pdf", nil)
	if r.status != http.StatusOK || r.header.Get("Content-Type") != "application/pdf" ||
		r.header.Get("Content-Disposition") != `attachment; filename="`+inv.Number+`.pdf"` || r.header.Get("Cache-Control") != "private, no-store" {
		t.Fatalf("pdf: %d %v", r.status, r.header)
	}
	if !bytes.HasPrefix(r.body, []byte("%PDF-")) || !bytes.Contains(r.body, []byte(inv.Number)) || !bytes.Contains(r.body, []byte("FACTURE")) ||
		!bytes.Contains(r.body, []byte("Non applicable")) {
		t.Fatal("the PDF must start with %PDF, carry the number and be in French")
	}
	if again := c.call(http.MethodGet, invoicesPath+inv.ID.String()+"/pdf", nil); !bytes.Equal(again.body, r.body) {
		t.Fatal("the same invoice must render to the same bytes")
	}

	// Other accounts, anonymous callers and bad ids get nothing.
	other := e.registerClient()
	for _, path := range []string{inv.ID.String(), inv.ID.String() + "/pdf"} {
		if r := other.call(http.MethodGet, invoicesPath+path, nil); r.status != http.StatusNotFound {
			t.Fatalf("another account read %s: %d", path, r.status)
		}
		if r := e.client().call(http.MethodGet, invoicesPath+path, nil); r.status != http.StatusUnauthorized {
			t.Fatalf("anonymous read %s: %d", path, r.status)
		}
	}
	for _, id := range []string{"not-a-uuid", uuid.NewString(), uuid.NewString() + "/pdf"} {
		if r := c.call(http.MethodGet, invoicesPath+id, nil); r.status != http.StatusNotFound {
			t.Fatalf("%s: %d", id, r.status)
		}
	}
	if r := other.raw(http.MethodGet, invoicesPath+inv.ID.String(), nil, map[string]string{"X-Locale": "fr"}); !strings.Contains(string(r.body), "Facture introuvable.") {
		t.Fatalf("french 404: %s", r.body)
	}

	// The receipt e-mail attaches the PDF and links the invoice page.
	msgs := receiptsOf(t, e, c.user.Email, "", 2)
	var receipt mail.Message
	for _, m := range msgs {
		if len(m.Attachments) == 1 && m.Attachments[0].Filename == inv.Number+".pdf" {
			receipt = m
		}
	}
	if receipt.To == "" {
		t.Fatalf("no receipt carries %s.pdf: %+v", inv.Number, msgs)
	}
	att := receipt.Attachments[0]
	if att.ContentType != "application/pdf" || !bytes.HasPrefix(att.Content, []byte("%PDF-")) || !bytes.Equal(att.Content, r.body) {
		t.Fatal("the attachment is the invoice PDF")
	}
	if !strings.Contains(receipt.Text, "/fr/profile/invoices/"+inv.ID.String()) || !strings.Contains(receipt.HTML, inv.Number) {
		t.Fatalf("the receipt links the invoice page: %s", receipt.Text)
	}
}

func TestDeclinedPurchaseHasNoInvoice(t *testing.T) {
	e := newEnv(t)
	e.ensurePacks()
	c := e.registerClient()
	card := c.attachCard(testCard)
	p := pack(t, c, 40) // 1.13: the sandbox declines prices ending in 13 cents
	r := c.call(http.MethodPost, purchasesPath, map[string]any{"pack_id": p.ID, "payment_method_id": card.ID, "idempotency_key": uniq("k-"), "consent": purchaseConsent()})
	if r.status != http.StatusPaymentRequired {
		t.Fatalf("declined: %d %s", r.status, r.body)
	}
	if n := e.invoiceCount("user_id = ?", c.user.ID); n != 0 {
		t.Fatalf("a declined payment was invoiced: %d", n)
	}
	// The backfill never invents one either.
	e.clock.Advance(2 * time.Minute)
	if err := e.app.Scheduler.RunJob(context.Background(), service.InvoiceBackfillJob); err != nil {
		t.Fatal(err)
	}
	if n := e.invoiceCount("user_id = ?", c.user.ID); n != 0 {
		t.Fatalf("backfill invoiced a declined payment: %d", n)
	}
	var tx struct{ Items []service.TransactionView }
	c.must(http.StatusOK, http.MethodGet, "/api/v1/billing/transactions", nil, &tx)
	for _, it := range tx.Items {
		if it.Invoice != nil || it.CreditNote != nil {
			t.Fatalf("a failed payment row shows a document: %+v", it)
		}
	}
}

func TestInvoiceIsIssuedExactlyOnceUnderConcurrencyAndReplays(t *testing.T) {
	e := newEnv(t)
	e.ensurePacks()
	c := e.registerClient()
	view := buyView(t, c, 250, c.attachCard(testCard))
	svc := e.app.Services.Invoices
	ctx := context.Background()

	// 24 simultaneous issuers (webhook duplicates, replays, the backfill) all get the one invoice.
	var wg sync.WaitGroup
	refs := make([]*service.InvoiceRef, 24)
	errs := make([]error, 24)
	for i := range refs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			refs[i], errs[i] = svc.IssueForPurchase(ctx, view.ID)
		}()
	}
	wg.Wait()
	for i := range refs {
		if errs[i] != nil || refs[i] == nil || refs[i].ID != view.Invoice.ID || refs[i].Number != view.Invoice.Number {
			t.Fatalf("issuer %d: %+v %v (want %+v)", i, refs[i], errs[i], view.Invoice)
		}
	}
	if n := e.invoiceCount("user_id = ?", c.user.ID); n != 1 {
		t.Fatalf("%d invoices for one purchase", n)
	}
	// Not issuable: unknown purchase, a pending payment.
	if _, err := svc.IssueForPurchase(ctx, uuid.New()); err == nil {
		t.Fatal("an unknown purchase cannot be invoiced")
	}
	// Concurrent completion of a purchase: the invoice is still unique.
	p := pack(t, c, 600)
	var buy service.PurchaseView
	c.must(http.StatusCreated, http.MethodPost, purchasesPath, map[string]any{"pack_id": p.ID, "payment_method_id": c.cards()[0].ID,
		"idempotency_key": uniq("k-"), "consent": purchaseConsent()}, &buy)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = e.app.Services.Credits.CompletePurchase(ctx, buy.ID, "")
			_, _ = svc.IssueForPurchase(ctx, buy.ID)
		}()
	}
	wg.Wait()
	if n := e.invoiceCount("credit_purchase_id = ? AND kind = 'invoice'", buy.ID); n != 1 {
		t.Fatalf("%d invoices after concurrent completion", n)
	}
}

func TestFiftyParallelPurchasesGetGaplessNumbers(t *testing.T) {
	e := newEnv(t)
	e.ensurePacks()
	const buyers = 50
	type buyer struct {
		c    *client
		card service.PaymentMethodView
	}
	bs := make([]buyer, buyers)
	for i := range bs {
		c := e.registerClient()
		bs[i] = buyer{c: c, card: c.attachCard(testCard)}
	}
	packID := pack(t, bs[0].c, 250).ID
	start := make(chan struct{})
	var wg sync.WaitGroup
	views := make([]service.PurchaseView, buyers)
	statuses := make([]int, buyers)
	for i := range bs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			r := bs[i].c.call(http.MethodPost, purchasesPath, map[string]any{"pack_id": packID, "payment_method_id": bs[i].card.ID,
				"idempotency_key": uniq("par-"), "consent": purchaseConsent()})
			statuses[i] = r.status
			_ = jsonDecode(r.body, &views[i])
		}()
	}
	close(start)
	wg.Wait()

	var nums []int
	seen := map[string]bool{}
	for i := range views {
		if statuses[i] != http.StatusCreated || views[i].Invoice == nil {
			t.Fatalf("buyer %d: %d %+v", i, statuses[i], views[i])
		}
		if seen[views[i].Invoice.Number] {
			t.Fatalf("number %s issued twice", views[i].Invoice.Number)
		}
		seen[views[i].Invoice.Number] = true
		nums = append(nums, sequence(t, views[i].Invoice.Number))
	}
	sort.Ints(nums)
	if nums[len(nums)-1]-nums[0] != buyers-1 {
		t.Fatalf("numbers are not gapless: %v", nums)
	}
	var documents int64
	ids := make([]uuid.UUID, 0, buyers)
	for _, b := range bs {
		ids = append(ids, b.c.user.ID)
	}
	e.app.DB.Model(&domain.Invoice{}).Where("user_id IN ?", ids).Count(&documents)
	if documents != buyers {
		t.Fatalf("%d documents for %d purchases", documents, buyers)
	}
	var next int64
	e.app.DB.Raw("SELECT next FROM invoice_sequences WHERE scope = ?", domain.InvoiceScope(domain.InvoiceKindInvoice, e.clock.Now())).Scan(&next)
	if int(next-1) != nums[len(nums)-1] {
		t.Fatalf("the counter (%d) is not the last number issued (%d)", next, nums[len(nums)-1])
	}
}

func TestNumbersRestartEachYearAndCreditNotesCountSeparately(t *testing.T) {
	e := newEnv(t)
	e.ensurePacks()
	// Jump to just before the next New Year (UTC), then across it.
	year := e.clock.Now().UTC().Year()
	newYear := time.Date(year+1, 1, 1, 0, 0, 0, 0, time.UTC)
	e.clock.Advance(newYear.Sub(e.clock.Now()) - 2*time.Minute)
	c := e.registerClient()
	card := c.attachCard(testCard)
	last := buyView(t, c, 250, card)
	if !strings.HasPrefix(last.Invoice.Number, fmt.Sprintf("SY-%d-", year)) {
		t.Fatalf("before midnight UTC: %s", last.Invoice.Number)
	}
	e.clock.Advance(3 * time.Minute)
	first := buyView(t, c, 250, card)
	if !strings.HasPrefix(first.Invoice.Number, fmt.Sprintf("SY-%d-", year+1)) {
		t.Fatalf("after midnight UTC: %s", first.Invoice.Number)
	}
	var before int64
	e.app.DB.Raw("SELECT count(*) FROM invoices WHERE number LIKE ?", fmt.Sprintf("SY-%d-%%", year+1)).Scan(&before)
	if sequence(t, first.Invoice.Number) != int(before) {
		t.Fatalf("the new year's counter restarted at 1 and counts the documents of the year: %s with %d documents", first.Invoice.Number, before)
	}
	// A credit note takes its own counter: SYC-<year+1>-…
	admin := e.createAdmin()
	admin.stepUp()
	tx := e.uuidOf("SELECT payment_transaction_id FROM invoices WHERE id = ?", first.Invoice.ID)
	admin.must(http.StatusOK, http.MethodPost, "/api/v1/admin/billing/transactions/"+tx.String()+"/refund", map[string]any{"amount_cents": 100, "reason": "year"}, nil)
	var cn domain.Invoice
	if err := e.app.DB.Where("kind = 'credit_note' AND original_invoice_id = ?", first.Invoice.ID).First(&cn).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(cn.Number, fmt.Sprintf("SYC-%d-", year+1)) {
		t.Fatalf("credit note number %s", cn.Number)
	}
}

func TestRefundIssuesCreditNotesThatReferenceTheInvoice(t *testing.T) {
	e := newEnv(t)
	e.ensurePacks()
	e.disableSignupTrial()
	c := e.registerClient()
	card := c.attachCard(testCard)
	purchase := buyView(t, c, 250, card)
	tx := e.uuidOf("SELECT payment_transaction_id FROM invoices WHERE id = ?", purchase.Invoice.ID)
	admin := e.createAdmin()
	admin.stepUp()
	refund := func(cents int64) {
		admin.must(http.StatusOK, http.MethodPost, "/api/v1/admin/billing/transactions/"+tx.String()+"/refund",
			map[string]any{"amount_cents": cents, "reason": "Unused credits"}, nil)
	}
	refund(300)

	var notes []domain.Invoice
	e.app.DB.Where("kind = 'credit_note' AND user_id = ?", c.user.ID).Find(&notes)
	if len(notes) != 1 {
		t.Fatalf("one refund, one credit note: %d", len(notes))
	}
	cn := c.invoice(notes[0].ID)
	if cn.Kind != "credit_note" || !strings.HasPrefix(cn.Number, e.invoiceYearPrefix(domain.InvoiceKindCreditNote)) || cn.TotalCents != 300 ||
		cn.SubtotalCents != 300 || cn.TaxCents != 0 || cn.Currency != "USD" {
		t.Fatalf("credit note: %+v", cn)
	}
	if cn.OriginalInvoice == nil || cn.OriginalInvoice.ID != purchase.Invoice.ID || cn.OriginalInvoice.Number != purchase.Invoice.Number {
		t.Fatalf("the credit note references the invoice: %+v", cn.OriginalInvoice)
	}
	if cn.Payment.RefundedAt == nil || cn.Payment.PaidAt != nil || cn.Payment.Card == nil || cn.Payment.Card.Last4 != "4242" || cn.Buyer.Email != c.user.Email {
		t.Fatalf("credit note payment/buyer: %+v %+v", cn.Payment, cn.Buyer)
	}
	pdf := c.call(http.MethodGet, invoicesPath+cn.ID.String()+"/pdf", nil)
	if pdf.status != http.StatusOK || !bytes.Contains(pdf.body, []byte("CREDIT NOTE")) || !bytes.Contains(pdf.body, []byte("Original invoice: "+purchase.Invoice.Number)) ||
		pdf.header.Get("Content-Disposition") != `attachment; filename="`+cn.Number+`.pdf"` {
		t.Fatalf("credit note pdf: %d %v", pdf.status, pdf.header)
	}

	// The refund e-mail links and attaches it.
	msgs := receiptsOf(t, e, c.user.Email, "Your refund was issued", 1)
	m := msgs[0]
	if len(m.Attachments) != 1 || m.Attachments[0].Filename != cn.Number+".pdf" || !bytes.Equal(m.Attachments[0].Content, pdf.body) ||
		!strings.Contains(m.Text, "/en/profile/invoices/"+cn.ID.String()) {
		t.Fatalf("refund e-mail: %+v / %s", m.Attachments, m.Text)
	}

	// A second partial refund: a second credit note, numbered next, same invoice.
	refund(200)
	e.app.DB.Where("kind = 'credit_note' AND user_id = ?", c.user.ID).Order("number").Find(&notes)
	if len(notes) != 2 || sequence(t, notes[1].Number) != sequence(t, notes[0].Number)+1 || *notes[1].OriginalInvoiceID != purchase.Invoice.ID || notes[1].TotalCents != 200 {
		t.Fatalf("second credit note: %+v", notes)
	}
	// Issuing again changes nothing.
	refundTx := e.uuidOf("SELECT payment_transaction_id FROM invoices WHERE id = ?", notes[1].ID)
	ref, err := e.app.Services.Invoices.IssueCreditNote(context.Background(), refundTx)
	if err != nil || ref == nil || ref.ID != notes[1].ID {
		t.Fatalf("idempotent credit note: %+v %v", ref, err)
	}
	if n := e.invoiceCount("kind = 'credit_note' AND user_id = ?", c.user.ID); n != 2 {
		t.Fatalf("%d credit notes", n)
	}
	// The history: the purchase shows its invoice, each refund its credit note.
	var history struct {
		Items []service.TransactionView `json:"items"`
	}
	c.must(http.StatusOK, http.MethodGet, "/api/v1/billing/transactions", nil, &history)
	var purchases, refunds int
	for _, it := range history.Items {
		switch it.Kind {
		case "credit_purchase":
			purchases++
			if it.Invoice == nil || it.Invoice.ID != purchase.Invoice.ID || it.CreditNote != nil {
				t.Fatalf("purchase row: %+v", it)
			}
		case "refund":
			refunds++
			if it.CreditNote == nil || it.Invoice != nil {
				t.Fatalf("refund row: %+v", it)
			}
		}
	}
	if purchases != 1 || refunds != 2 {
		t.Fatalf("rows: %d purchases, %d refunds", purchases, refunds)
	}
}

func TestTransactionListsLoadDocumentsWithOneQueryPerPage(t *testing.T) {
	e := newEnv(t)
	e.ensurePacks()
	c := e.registerClient()
	card := c.attachCard(testCard)
	admin := e.createAdmin()
	admin.stepUp()
	for i := 0; i < 6; i++ {
		v := buyView(t, c, 250, card)
		if i%2 == 0 {
			tx := e.uuidOf("SELECT payment_transaction_id FROM invoices WHERE id = ?", v.Invoice.ID)
			admin.must(http.StatusOK, http.MethodPost, "/api/v1/admin/billing/transactions/"+tx.String()+"/refund", map[string]any{"amount_cents": 100, "reason": "x"}, nil)
		}
	}
	var page struct {
		Items []service.TransactionView `json:"items"`
	}
	log := recordSQL(e, func() { c.must(http.StatusOK, http.MethodGet, "/api/v1/billing/transactions?limit=100", nil, &page) })
	if len(page.Items) != 9 {
		t.Fatalf("rows: %d", len(page.Items))
	}
	if n := log.count(`FROM "invoices"`); n != 1 {
		t.Fatalf("%d invoice queries for %d rows, want one per page", n, len(page.Items))
	}
	var adminPage struct {
		Items []service.AdminTransactionRow `json:"items"`
	}
	log = recordSQL(e, func() {
		admin.must(http.StatusOK, http.MethodGet, "/api/v1/admin/billing/transactions?limit=100&user_id="+c.user.ID.String(), nil, &adminPage)
	})
	if n := log.count(`FROM "invoices"`); n != 1 || len(adminPage.Items) != 9 {
		t.Fatalf("admin: %d invoice queries, %d rows", n, len(adminPage.Items))
	}
	for _, it := range adminPage.Items {
		if (it.Kind == "credit_purchase") != (it.Invoice != nil) || (it.Kind == "refund") != (it.CreditNote != nil) {
			t.Fatalf("admin row %s: invoice %v credit note %v", it.Kind, it.Invoice, it.CreditNote)
		}
	}
	// The purchase history is one query too.
	log = recordSQL(e, func() { c.must(http.StatusOK, http.MethodGet, purchasesPath+"?limit=100", nil, nil) })
	if n := log.count(`FROM "invoices"`); n != 1 {
		t.Fatalf("purchase history: %d invoice queries", n)
	}
}

func TestAdminInvoicesListFilterPDFAndExport(t *testing.T) {
	e := newEnv(t)
	e.ensurePacks()
	a, b := e.registerClient(), e.registerClient()
	ca, cb := a.attachCard(testCard), b.attachCard(testCard)
	va1, va2, vb := buyView(t, a, 250, ca), buyView(t, a, 600, ca), buyView(t, b, 250, cb)
	admin := e.createAdmin()
	admin.stepUp()
	txA := e.uuidOf("SELECT payment_transaction_id FROM invoices WHERE id = ?", va1.Invoice.ID)
	admin.must(http.StatusOK, http.MethodPost, "/api/v1/admin/billing/transactions/"+txA.String()+"/refund", map[string]any{"amount_cents": 250, "reason": "x"}, nil)

	list := func(query string) service.Page[service.AdminInvoiceView] {
		var p service.Page[service.AdminInvoiceView]
		admin.must(http.StatusOK, http.MethodGet, adminInvoicesPath+"?"+query, nil, &p)
		return p
	}
	all := list("user_id=" + a.user.ID.String())
	if len(all.Items) != 3 {
		t.Fatalf("user a has two invoices and a credit note: %d", len(all.Items))
	}
	top := all.Items[0]
	if top.User.ID != a.user.ID || top.User.Email != a.user.Email || top.User.DisplayName != a.user.DisplayName || top.Buyer.Email != a.user.Email {
		t.Fatalf("row user: %+v", top)
	}
	if got := list("user_id=" + a.user.ID.String() + "&kind=credit_note"); len(got.Items) != 1 || got.Items[0].Kind != "credit_note" || got.Items[0].OriginalInvoice.ID != va1.Invoice.ID {
		t.Fatalf("kind filter: %+v", got.Items)
	}
	if got := list("q=" + va2.Invoice.Number); len(got.Items) != 1 || got.Items[0].ID != va2.Invoice.ID {
		t.Fatalf("number search: %+v", got.Items)
	}
	if got := list("q=" + strings.ToLower(vb.Invoice.Number)); len(got.Items) != 1 || got.Items[0].ID != vb.Invoice.ID {
		t.Fatalf("number search is case-insensitive: %+v", got.Items)
	}
	prefix := va1.Invoice.Number[:len(va1.Invoice.Number)-3]
	if got := list("q=" + prefix + "&kind=invoice"); len(got.Items) < 3 {
		t.Fatalf("number prefix search: %d", len(got.Items))
	}
	if got := list("q=SY%25"); len(got.Items) != 0 { // a LIKE wildcard is data, not syntax
		t.Fatalf("wildcards must be escaped: %d", len(got.Items))
	}
	today := e.clock.Now().UTC().Format("2006-01-02")
	if got := list("user_id=" + a.user.ID.String() + "&from=" + today + "&to=" + today); len(got.Items) != 3 {
		t.Fatalf("date range of today: %d", len(got.Items))
	}
	if got := list("user_id=" + a.user.ID.String() + "&from=" + e.clock.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")); len(got.Items) != 0 {
		t.Fatalf("from tomorrow: %d", len(got.Items))
	}
	// Pagination: one at a time, newest first, no repeats.
	var seen []uuid.UUID
	cursor := ""
	for i := 0; i < 5; i++ {
		p := list("user_id=" + a.user.ID.String() + "&limit=1&cursor=" + cursor)
		for _, it := range p.Items {
			seen = append(seen, it.ID)
		}
		if p.NextCursor == "" {
			break
		}
		cursor = p.NextCursor
	}
	if len(seen) != 3 || seen[0] != top.ID {
		t.Fatalf("pagination: %v", seen)
	}
	for _, q := range []string{"kind=receipt", "from=yesterday", "to=2026-13-40", "cursor=!!!", "user_id=zzz"} {
		if r := admin.call(http.MethodGet, adminInvoicesPath+"?"+q, nil); r.status != http.StatusUnprocessableEntity && r.status != http.StatusBadRequest {
			t.Fatalf("%s: %d %s", q, r.status, r.body)
		}
	}

	// Detail and PDF of any invoice; clients cannot use the admin routes.
	var one service.AdminInvoiceView
	admin.must(http.StatusOK, http.MethodGet, adminInvoicesPath+"/"+vb.Invoice.ID.String(), nil, &one)
	if one.ID != vb.Invoice.ID || one.User.ID != b.user.ID {
		t.Fatalf("detail: %+v", one)
	}
	pdf := admin.call(http.MethodGet, adminInvoicesPath+"/"+vb.Invoice.ID.String()+"/pdf", nil)
	if pdf.status != http.StatusOK || !bytes.Contains(pdf.body, []byte(vb.Invoice.Number)) || pdf.header.Get("Cache-Control") != "private, no-store" {
		t.Fatalf("admin pdf: %d", pdf.status)
	}
	if r := admin.call(http.MethodGet, adminInvoicesPath+"/"+uuid.NewString(), nil); r.status != http.StatusNotFound {
		t.Fatalf("unknown invoice: %d", r.status)
	}
	if r := a.call(http.MethodGet, adminInvoicesPath, nil); r.status == http.StatusOK {
		t.Fatal("a client reached the admin invoice list")
	}

	// Accounting export: step-up, audited, signed amounts, range limits.
	exportQuery := "?from=" + today + "&to=" + e.clock.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")
	e.clock.Advance(11 * time.Minute) // the step-up window passes
	if r := admin.call(http.MethodGet, adminInvoicesPath+"/export.csv"+exportQuery, nil); r.status != http.StatusForbidden || r.errCode(t) != "step_up_required" {
		t.Fatalf("export without step-up: %d %s", r.status, r.body)
	}
	admin.stepUp()
	if r := admin.call(http.MethodGet, adminInvoicesPath+"/export.csv", nil); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("export without a range: %d", r.status)
	}
	long := "?from=" + e.clock.Now().UTC().AddDate(-2, 0, 0).Format("2006-01-02") + "&to=" + today
	if r := admin.call(http.MethodGet, adminInvoicesPath+"/export.csv"+long, nil); r.status != http.StatusUnprocessableEntity || errField(t, r, "to") == "" {
		t.Fatalf("export over 366 days: %d %s", r.status, r.body)
	}
	var auditBefore int64
	e.app.DB.Model(&domain.AuditLog{}).Where("action = 'invoices.exported'").Count(&auditBefore)
	r := admin.call(http.MethodGet, adminInvoicesPath+"/export.csv"+exportQuery, nil)
	if r.status != http.StatusOK || !strings.HasPrefix(r.header.Get("Content-Type"), "text/csv") || r.header.Get("Cache-Control") != "private, no-store" ||
		!strings.Contains(r.header.Get("Content-Disposition"), "attachment") {
		t.Fatalf("export: %d %v", r.status, r.header)
	}
	records, err := csv.NewReader(bytes.NewReader(r.body)).ReadAll()
	if err != nil || len(records) < 5 {
		t.Fatalf("csv: %v %d rows", err, len(records))
	}
	if strings.Join(records[0], ",") != "number,kind,issued_at,original_number,buyer_country,currency,subtotal,tax,total,provider,provider_reference" {
		t.Fatalf("header: %v", records[0])
	}
	byNumber := map[string][]string{}
	for _, rec := range records[1:] {
		byNumber[rec[0]] = rec
	}
	if rec := byNumber[va2.Invoice.Number]; rec == nil || rec[1] != "invoice" || rec[5] != "USD" || rec[6] != "19.99" || rec[7] != "0.00" || rec[8] != "19.99" || rec[9] != "sandbox_a" {
		t.Fatalf("invoice row: %v", rec)
	}
	var creditNote domain.Invoice
	e.app.DB.Where("kind = 'credit_note' AND user_id = ?", a.user.ID).First(&creditNote)
	if rec := byNumber[creditNote.Number]; rec == nil || rec[1] != "credit_note" || rec[3] != va1.Invoice.Number || rec[8] != "-2.50" {
		t.Fatalf("credit note row: %v", rec)
	}
	var auditAfter int64
	e.app.DB.Model(&domain.AuditLog{}).Where("action = 'invoices.exported'").Count(&auditAfter)
	if auditAfter != auditBefore+1 {
		t.Fatalf("the export is audited once: %d -> %d", auditBefore, auditAfter)
	}
	if strings.Contains(string(r.body), a.user.Email) {
		t.Fatal("the accounting export carries no e-mail address")
	}
}

func TestBackfillIssuesMissingInvoicesAndCreditNotes(t *testing.T) {
	e := newEnv(t)
	e.ensurePacks()
	c := e.registerClient()
	card := c.attachCard(testCard)
	v1, v2 := buyView(t, c, 250, card), buyView(t, c, 600, card)
	admin := e.createAdmin()
	admin.stepUp()
	tx1 := e.uuidOf("SELECT payment_transaction_id FROM invoices WHERE id = ?", v1.Invoice.ID)
	admin.must(http.StatusOK, http.MethodPost, "/api/v1/admin/billing/transactions/"+tx1.String()+"/refund", map[string]any{"amount_cents": 400, "reason": "x"}, nil)

	// Simulate lost documents (a crash right after the money moved).
	mine := func() []uuid.UUID {
		var ids []uuid.UUID
		e.app.DB.Model(&domain.Invoice{}).Where("user_id = ?", c.user.ID).Pluck("id", &ids)
		return ids
	}
	if len(mine()) != 3 {
		t.Fatalf("documents before: %d", len(mine()))
	}
	e.app.DB.Exec("DELETE FROM invoices WHERE user_id = ? AND kind = 'credit_note'", c.user.ID)
	e.app.DB.Exec("DELETE FROM invoices WHERE id = ?", v2.Invoice.ID)
	run := func() {
		t.Helper()
		if err := e.app.Scheduler.RunJob(context.Background(), service.InvoiceBackfillJob); err != nil {
			t.Fatal(err)
		}
	}
	// A payment settled seconds ago is still being invoiced inline: not touched yet.
	run()
	if n := e.invoiceCount("user_id = ?", c.user.ID); n != 1 {
		t.Fatalf("the backfill ran too early: %d documents", n)
	}
	e.clock.Advance(2 * time.Minute)
	run()
	rows := e.invoicesOf(c.user.ID)
	if len(rows) != 3 {
		t.Fatalf("after the backfill: %d documents", len(rows))
	}
	var kinds []string
	for _, r := range rows {
		kinds = append(kinds, string(r.Kind))
		if r.Kind == domain.InvoiceKindCreditNote && (r.TotalCents != 400 || r.OriginalInvoiceID == nil || *r.OriginalInvoiceID != v1.Invoice.ID) {
			t.Fatalf("backfilled credit note: %+v", r)
		}
		if r.Kind == domain.InvoiceKindInvoice && r.ID == v1.Invoice.ID {
			continue
		}
	}
	sort.Strings(kinds)
	if strings.Join(kinds, ",") != "credit_note,invoice,invoice" {
		t.Fatalf("kinds: %v", kinds)
	}
	// Running again finds nothing to do and issues nothing.
	run()
	if len(e.invoicesOf(c.user.ID)) != 3 {
		t.Fatal("the backfill is idempotent")
	}
	// The purchase view shows the re-issued invoice.
	var history service.Page[service.PurchaseView]
	c.must(http.StatusOK, http.MethodGet, purchasesPath, nil, &history)
	for _, it := range history.Items {
		if it.Invoice == nil {
			t.Fatalf("purchase %s still has no invoice", it.ID)
		}
	}
	// Payments older than the window are left alone (the lookup stays proportional to recent activity).
	e.app.DB.Exec("DELETE FROM invoices WHERE user_id = ? AND kind = 'credit_note'", c.user.ID)
	e.app.DB.Exec("DELETE FROM invoices WHERE user_id = ? AND kind = 'invoice'", c.user.ID)
	e.clock.Advance(60 * 24 * time.Hour)
	run()
	if n := e.invoiceCount("user_id = ?", c.user.ID); n != 0 {
		t.Fatalf("the backfill reached back beyond its window: %d", n)
	}
}

// The backfill is also registered with the scheduler under its documented name.
func TestInvoiceBackfillJobIsRegistered(t *testing.T) {
	e := newEnv(t)
	if service.InvoiceBackfillJob != "invoices-backfill" {
		t.Fatalf("job name %q", service.InvoiceBackfillJob)
	}
	if err := e.app.Scheduler.RunJob(context.Background(), service.InvoiceBackfillJob); err != nil {
		t.Fatal(err)
	}
}

func TestInvoicesSurviveAccountErasure(t *testing.T) {
	e := newEnv(t)
	e.ensurePacks()
	c := e.registerClient()
	v := buyView(t, c, 250, c.attachCard(testCard))
	admin := e.createAdmin()
	admin.stepUp()
	admin.must(http.StatusNoContent, http.MethodDelete, "/api/v1/admin/users/"+c.user.ID.String(), map[string]any{"reason": "GDPR erasure"}, nil)

	if n := e.invoiceCount("user_id = ?", c.user.ID); n != 1 {
		t.Fatalf("an erased account's invoices are kept for accounting: %d", n)
	}
	var one service.AdminInvoiceView
	admin.must(http.StatusOK, http.MethodGet, adminInvoicesPath+"/"+v.Invoice.ID.String(), nil, &one)
	if one.Buyer.Email != c.user.Email || one.Number != v.Invoice.Number {
		t.Fatalf("the snapshot is immutable: %+v", one.Buyer)
	}
	if one.User.DisplayName != "Deleted user" {
		t.Fatalf("the live account is anonymised: %+v", one.User)
	}
	pdf := admin.call(http.MethodGet, adminInvoicesPath+"/"+v.Invoice.ID.String()+"/pdf", nil)
	if pdf.status != http.StatusOK || !bytes.Contains(pdf.body, []byte(v.Invoice.Number)) {
		t.Fatalf("pdf after erasure: %d", pdf.status)
	}
}

func TestDocumentsAreImmutable(t *testing.T) {
	e := newEnv(t)
	e.ensurePacks()
	c := e.registerClient()
	v := buyView(t, c, 250, c.attachCard(testCard))
	before := c.invoice(v.Invoice.ID)
	// The account changes name, e-mail and language afterwards: the invoice does not.
	e.app.DB.Model(&domain.User{}).Where("id = ?", c.user.ID).Updates(map[string]any{"display_name": "Someone Else", "email": uniq("new") + "@example.com", "locale": "es"})
	after := c.invoice(v.Invoice.ID)
	if after.Buyer != before.Buyer || after.Locale != before.Locale {
		t.Fatalf("snapshot changed: %+v -> %+v", before.Buyer, after.Buyer)
	}
	// A second issue attempt after the change returns the same document.
	ref, err := e.app.Services.Invoices.IssueForPurchase(context.Background(), v.ID)
	if err != nil || ref.ID != v.Invoice.ID {
		t.Fatalf("%+v %v", ref, err)
	}
	// The database refuses duplicates of (transaction, kind) and of numbers.
	var inv domain.Invoice
	e.app.DB.First(&inv, "id = ?", v.Invoice.ID)
	dup := inv
	dup.ID = domain.NewID()
	if err := e.app.DB.Create(&dup).Error; err == nil {
		t.Fatal("a second invoice for the same payment must violate the unique key")
	}
	if err := e.app.DB.Exec("UPDATE invoices SET total_cents = 1 WHERE id = ?", inv.ID).Error; err == nil {
		t.Fatal("totals must stay consistent: total = subtotal + tax")
	}
}

func jsonDecode(b []byte, v any) error { return json.Unmarshal(b, v) }
