package api

import (
	"io"
	"log/slog"
	"net/http"

	"github.com/JulienLeotier/hive/internal/billing"
)

// StripeWebhookHandler accepts Stripe's POST-and-forget payment events and
// translates the ones we care about into Hive billing state transitions.
// Signature verification is mandatory — anyone can hit the public URL, so
// only events signed with the shared webhook secret are honoured.
//
// Handled event types:
//
//   - invoice.paid  → flip the matching Hive invoice to status=paid
//   - invoice.payment_failed → recorded in slog, no state change (the
//     operator typically retries via Stripe's own dunning workflow)
//
// Everything else is acknowledged with 200 so Stripe doesn't retry; the
// server is intentionally permissive here because the integration surface
// should grow (credit notes, refunds) without breaking inbound deliveries.
func StripeWebhookHandler(gen *billing.Generator, secret string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if gen == nil {
			http.Error(w, "billing subsystem not configured", http.StatusServiceUnavailable)
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
			return
		}

		sig := r.Header.Get("Stripe-Signature")
		eventType, hiveInvoiceID, err := billing.VerifyWebhookSignature(body, sig, secret)
		if err != nil {
			slog.Warn("stripe webhook rejected", "error", err)
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}

		switch eventType {
		case "invoice.paid":
			if hiveInvoiceID == "" {
				slog.Warn("stripe invoice.paid missing hive_invoice_id metadata — dropping")
			} else if err := gen.MarkPaid(r.Context(), hiveInvoiceID); err != nil {
				slog.Warn("stripe: mark paid failed", "hive_invoice_id", hiveInvoiceID, "error", err)
			} else {
				slog.Info("stripe: invoice marked paid", "hive_invoice_id", hiveInvoiceID)
			}
		case "invoice.payment_failed":
			slog.Warn("stripe: payment failed", "hive_invoice_id", hiveInvoiceID)
		default:
			// Unhandled event types still return 200 so Stripe stops retrying.
			slog.Debug("stripe: unhandled event", "type", eventType)
		}
		w.WriteHeader(http.StatusOK)
	})
}
