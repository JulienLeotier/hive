package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/JulienLeotier/hive/internal/billing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stripeSig builds the signature header Stripe sends so we can hit
// VerifyWebhookSignature without a real Stripe account. Format:
// `t=<ts>,v1=<hex(hmac_sha256(secret, ts.body))>`.
func stripeSig(secret, body string, ts int64) string {
	signedPayload := fmt.Sprintf("%d.%s", ts, body)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signedPayload))
	return fmt.Sprintf("t=%d,v1=%s", ts, hex.EncodeToString(mac.Sum(nil)))
}

func TestStripeWebhookRejectsMissingSignature(t *testing.T) {
	srv := setupServer(t)
	gen := billing.NewGenerator(srv.db(), "USD")
	handler := StripeWebhookHandler(gen, "whsec_test")

	req := httptest.NewRequest("POST", "/webhooks/stripe", strings.NewReader(`{"type":"invoice.paid"}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestStripeWebhookRejectsBadSignature(t *testing.T) {
	srv := setupServer(t)
	gen := billing.NewGenerator(srv.db(), "USD")
	handler := StripeWebhookHandler(gen, "whsec_test")

	body := `{"type":"invoice.paid"}`
	req := httptest.NewRequest("POST", "/webhooks/stripe", strings.NewReader(body))
	req.Header.Set("Stripe-Signature",
		stripeSig("whsec_DIFFERENT", body, time.Now().Unix()))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestStripeWebhookMarksPaidOnInvoicePaid(t *testing.T) {
	srv := setupServer(t)
	// Seed a cost so GenerateForPeriod produces an invoice we can reference.
	_, err := srv.db().Exec(
		`INSERT INTO costs (agent_id, agent_name, workflow_id, task_id, cost, tenant_id, created_at)
		 VALUES ('a', 'alpha', 'w', 't', 10.0, 'tenant-x', '2026-03-15 12:00:00')`)
	require.NoError(t, err)

	gen := billing.NewGenerator(srv.db(), "USD")
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	_, err = gen.GenerateForPeriod(testContext(), start, end)
	require.NoError(t, err)

	invs, err := gen.List(testContext(), "tenant-x", 10)
	require.NoError(t, err)
	require.Len(t, invs, 1)
	hiveInvoiceID := invs[0].ID

	body := fmt.Sprintf(
		`{"id":"evt_1","type":"invoice.paid","data":{"object":{"metadata":{"hive_invoice_id":"%s"}}}}`,
		hiveInvoiceID)
	req := httptest.NewRequest("POST", "/webhooks/stripe", strings.NewReader(body))
	req.Header.Set("Stripe-Signature", stripeSig("whsec_test", body, time.Now().Unix()))

	handler := StripeWebhookHandler(gen, "whsec_test")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	invs, _ = gen.List(testContext(), "tenant-x", 10)
	assert.Equal(t, billing.StatusPaid, invs[0].Status, "invoice.paid event must flip the row")
}

func TestStripeWebhookRejectsNonPost(t *testing.T) {
	srv := setupServer(t)
	gen := billing.NewGenerator(srv.db(), "USD")
	handler := StripeWebhookHandler(gen, "whsec_test")

	req := httptest.NewRequest("GET", "/webhooks/stripe", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func testContext() (ctx testContextType) { return }

type testContextType struct{}

func (testContextType) Deadline() (time.Time, bool) { return time.Time{}, false }
func (testContextType) Done() <-chan struct{}       { return nil }
func (testContextType) Err() error                  { return nil }
func (testContextType) Value(any) any               { return nil }
