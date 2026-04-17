package billing

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/customer"
	"github.com/stripe/stripe-go/v82/invoice"
	"github.com/stripe/stripe-go/v82/invoiceitem"
	"github.com/stripe/stripe-go/v82/webhook"
)

// StripeGateway is the billing.Gateway implementation that mints a Stripe
// customer per hive tenant, attaches one InvoiceItem per Hive invoice, and
// finalises the Stripe invoice so Stripe's own payment/email flow takes
// over. Exists as a plug-in so operators who don't want Stripe can skip
// this file entirely — the billing package core has zero Stripe imports.
//
// Idempotency: the Stripe invoice id is persisted back into
// invoices.external_id. A re-run of Generator.Issue() on the same row
// will call Register again, but the Stripe API's idempotency key support
// (keyed on the Hive invoice id) prevents duplicate Stripe rows.
type StripeGateway struct {
	secretKey string
	currency  stripe.Currency
	// customerResolver turns a Hive tenant_id into a Stripe customer id.
	// Defaults to a lazy create-or-lookup by tenant_id metadata. Operators
	// with richer customer records (product DB, CRM) can override this so
	// the Stripe customer carries their canonical email/name instead of
	// the tenant string.
	customerResolver func(ctx context.Context, tenantID string) (string, error)
}

// NewStripeGateway builds a gateway. secretKey is the Stripe restricted key;
// currency is the ISO 4217 code (usd, eur, gbp, …).
func NewStripeGateway(secretKey, currency string) *StripeGateway {
	if currency == "" {
		currency = "usd"
	}
	g := &StripeGateway{
		secretKey: secretKey,
		currency:  stripe.Currency(strings.ToLower(currency)),
	}
	g.customerResolver = g.lookupOrCreateCustomer
	return g
}

// Name satisfies billing.Gateway.
func (g *StripeGateway) Name() string { return "stripe" }

// Register satisfies billing.Gateway: creates (or finds) a Stripe customer
// for the Hive tenant, attaches an InvoiceItem for the total amount, then
// creates + finalises the Stripe invoice. Returns the Stripe invoice id so
// Hive can store it in invoices.external_id for later reconciliation.
func (g *StripeGateway) Register(ctx context.Context, inv Invoice) (string, error) {
	stripe.Key = g.secretKey

	customerID, err := g.customerResolver(ctx, inv.TenantID)
	if err != nil {
		return "", fmt.Errorf("resolving stripe customer for tenant %s: %w", inv.TenantID, err)
	}

	// InvoiceItem amount is in the currency's smallest unit (cents for USD).
	// Round half-up via int64 cast; fractional cents below 0.5 are lost,
	// above are captured — consistent with Stripe's own rounding guidance.
	amountMinor := int64(inv.TotalAmount*100 + 0.5)
	periodLabel := inv.PeriodStart.Format("2006-01-02") + " → " + inv.PeriodEnd.Format("2006-01-02")

	_, err = invoiceitem.New(&stripe.InvoiceItemParams{
		Customer:    stripe.String(customerID),
		Amount:      stripe.Int64(amountMinor),
		Currency:    stripe.String(string(g.currency)),
		Description: stripe.String("Hive usage — " + periodLabel),
		Metadata: map[string]string{
			"hive_invoice_id": inv.ID,
			"hive_tenant_id":  inv.TenantID,
			"period_start":    inv.PeriodStart.Format("2006-01-02"),
			"period_end":      inv.PeriodEnd.Format("2006-01-02"),
			"task_count":      fmt.Sprintf("%d", inv.TaskCount),
		},
	})
	if err != nil {
		return "", fmt.Errorf("creating stripe invoice item: %w", err)
	}

	params := &stripe.InvoiceParams{
		Customer:         stripe.String(customerID),
		CollectionMethod: stripe.String(string(stripe.InvoiceCollectionMethodSendInvoice)),
		DaysUntilDue:     stripe.Int64(14),
		Metadata: map[string]string{
			"hive_invoice_id": inv.ID,
			"hive_tenant_id":  inv.TenantID,
		},
	}
	// Idempotency by our Hive invoice id — rerunning Issue on the same row
	// won't create a second Stripe invoice.
	params.SetIdempotencyKey("hive-inv-" + inv.ID)

	stripeInv, err := invoice.New(params)
	if err != nil {
		return "", fmt.Errorf("creating stripe invoice: %w", err)
	}
	// Finalise so Stripe can send the email + expose the hosted invoice URL.
	_, err = invoice.FinalizeInvoice(stripeInv.ID, &stripe.InvoiceFinalizeInvoiceParams{})
	if err != nil {
		return "", fmt.Errorf("finalising stripe invoice: %w", err)
	}
	slog.Info("stripe: invoice finalised",
		"hive_invoice_id", inv.ID, "stripe_id", stripeInv.ID, "amount_minor", amountMinor)
	return stripeInv.ID, nil
}

// lookupOrCreateCustomer finds a Stripe customer by tenant_id metadata or
// creates one. Kept internal so operators with richer customer records can
// swap customerResolver without rewriting Register.
func (g *StripeGateway) lookupOrCreateCustomer(_ context.Context, tenantID string) (string, error) {
	stripe.Key = g.secretKey

	params := &stripe.CustomerSearchParams{}
	params.Query = fmt.Sprintf("metadata['hive_tenant_id']:'%s'", stripeEscape(tenantID))
	iter := customer.Search(params)
	for iter.Next() {
		c := iter.Customer()
		if c != nil {
			return c.ID, nil
		}
	}
	if err := iter.Err(); err != nil {
		slog.Warn("stripe: customer search failed — falling back to create", "error", err)
	}

	// Not found: create. Description doubles as the tenant label.
	c, err := customer.New(&stripe.CustomerParams{
		Description: stripe.String("Hive tenant " + tenantID),
		Metadata:    map[string]string{"hive_tenant_id": tenantID},
	})
	if err != nil {
		return "", fmt.Errorf("creating stripe customer: %w", err)
	}
	return c.ID, nil
}

// stripeEscape scrubs single-quotes in tenant ids so they can safely ride
// inside the Stripe search query DSL.
func stripeEscape(s string) string {
	return strings.ReplaceAll(s, "'", `\'`)
}

// VerifyWebhookSignature wraps stripe-go's ConstructEventWithOptions so
// callers in internal/api don't import the Stripe package directly. Returns
// the parsed event type + the `hive_invoice_id` metadata when present.
//
// IgnoreAPIVersionMismatch is enabled: we only consume the event type and
// a single metadata field, neither of which changes shape across Stripe API
// versions. Without this, webhook endpoints from older integrations throw
// errors even when the payload itself is fine.
func VerifyWebhookSignature(payload []byte, signature, secret string) (eventType, hiveInvoiceID string, err error) {
	evt, err := webhook.ConstructEventWithOptions(payload, signature, secret,
		webhook.ConstructEventOptions{IgnoreAPIVersionMismatch: true})
	if err != nil {
		return "", "", fmt.Errorf("invalid stripe signature: %w", err)
	}
	eventType = string(evt.Type)

	// Stripe's event payload is a {data: {object: {...}}} shape. For
	// invoice-flavoured events the object carries our metadata.
	if raw, ok := evt.Data.Object["metadata"].(map[string]interface{}); ok {
		if id, ok := raw["hive_invoice_id"].(string); ok {
			hiveInvoiceID = id
		}
	}
	return eventType, hiveInvoiceID, nil
}
