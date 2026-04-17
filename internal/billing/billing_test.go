package billing

import (
	"context"
	"testing"
	"time"

	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateForPeriodAggregatesPerTenant(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()

	// Two tenants, three cost rows in the period, one row outside.
	when := "2026-03-15 12:00:00"
	outside := "2026-02-10 12:00:00"
	_, err = st.DB.Exec(
		`INSERT INTO costs (agent_id, agent_name, workflow_id, task_id, cost, tenant_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"a1", "alpha", "w1", "t1", 1.50, "tenant-a", when)
	require.NoError(t, err)
	_, err = st.DB.Exec(
		`INSERT INTO costs (agent_id, agent_name, workflow_id, task_id, cost, tenant_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"a1", "alpha", "w1", "t2", 0.75, "tenant-a", when)
	require.NoError(t, err)
	_, err = st.DB.Exec(
		`INSERT INTO costs (agent_id, agent_name, workflow_id, task_id, cost, tenant_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"a2", "beta", "w2", "t3", 2.25, "tenant-b", when)
	require.NoError(t, err)
	_, err = st.DB.Exec(
		`INSERT INTO costs (agent_id, agent_name, workflow_id, task_id, cost, tenant_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"a1", "alpha", "w1", "t4", 999.00, "tenant-a", outside)
	require.NoError(t, err)

	g := NewGenerator(st.DB, "USD")
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	n, err := g.GenerateForPeriod(context.Background(), start, end)
	require.NoError(t, err)
	assert.Equal(t, 2, n, "one invoice per tenant")

	invs, err := g.List(context.Background(), "", 50)
	require.NoError(t, err)
	require.Len(t, invs, 2)

	totals := map[string]float64{}
	for _, inv := range invs {
		totals[inv.TenantID] = inv.TotalAmount
		assert.Equal(t, StatusDraft, inv.Status, "new invoices start draft")
		assert.Equal(t, "USD", inv.Currency)
	}
	assert.InDelta(t, 2.25, totals["tenant-a"], 0.001)
	assert.InDelta(t, 2.25, totals["tenant-b"], 0.001)
}

func TestGenerateIsIdempotent(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()

	_, err = st.DB.Exec(
		`INSERT INTO costs (agent_id, agent_name, workflow_id, task_id, cost, tenant_id, created_at)
		 VALUES ('a', 'alpha', 'w', 't', 5.0, 'tenant-a', '2026-03-15 12:00:00')`)
	require.NoError(t, err)

	g := NewGenerator(st.DB, "EUR")
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	// Run twice — the UNIQUE constraint on the period keeps us at one row.
	_, err = g.GenerateForPeriod(context.Background(), start, end)
	require.NoError(t, err)
	_, err = g.GenerateForPeriod(context.Background(), start, end)
	require.NoError(t, err)

	invs, err := g.List(context.Background(), "tenant-a", 10)
	require.NoError(t, err)
	assert.Len(t, invs, 1, "repeat generations must not duplicate")
}

// stubGateway lets us verify the gateway hook fires without pulling in
// Stripe. Name is used by log tagging inside Issue().
type stubGateway struct {
	calls    int
	lastInv  Invoice
	externID string
}

func (s *stubGateway) Name() string { return "stub" }
func (s *stubGateway) Register(_ context.Context, inv Invoice) (string, error) {
	s.calls++
	s.lastInv = inv
	if s.externID == "" {
		s.externID = "ext_" + inv.ID[:6]
	}
	return s.externID, nil
}

func TestIssuePushesToGateway(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()

	_, err = st.DB.Exec(
		`INSERT INTO costs (agent_id, agent_name, workflow_id, task_id, cost, tenant_id, created_at)
		 VALUES ('a', 'alpha', 'w', 't', 1.23, 'tenant-a', '2026-03-15 12:00:00')`)
	require.NoError(t, err)

	gw := &stubGateway{}
	g := NewGenerator(st.DB, "USD").WithGateway(gw)
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	_, err = g.GenerateForPeriod(context.Background(), start, end)
	require.NoError(t, err)

	invs, err := g.List(context.Background(), "tenant-a", 10)
	require.NoError(t, err)
	require.Len(t, invs, 1)

	require.NoError(t, g.Issue(context.Background(), invs[0].ID))
	assert.Equal(t, 1, gw.calls, "gateway Register should fire exactly once")

	// Re-fetch to check status + external id.
	invs, _ = g.List(context.Background(), "tenant-a", 10)
	assert.Equal(t, StatusIssued, invs[0].Status)
	assert.NotEmpty(t, invs[0].ExternalID)
}

func TestMarkPaidRequiresIssued(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()

	_, err = st.DB.Exec(
		`INSERT INTO costs (agent_id, agent_name, workflow_id, task_id, cost, tenant_id, created_at)
		 VALUES ('a', 'alpha', 'w', 't', 2.0, 'tenant-a', '2026-03-15 12:00:00')`)
	require.NoError(t, err)

	g := NewGenerator(st.DB, "USD")
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	_, err = g.GenerateForPeriod(context.Background(), start, end)
	require.NoError(t, err)

	invs, _ := g.List(context.Background(), "tenant-a", 10)
	// Drafts can be marked paid — covers the offline-payment path where
	// an admin records a wire transfer without going through Issue().
	require.NoError(t, g.MarkPaid(context.Background(), invs[0].ID))

	invs, _ = g.List(context.Background(), "tenant-a", 10)
	assert.Equal(t, StatusPaid, invs[0].Status)

	// Marking an already-paid invoice paid is rejected with a clear error.
	err = g.MarkPaid(context.Background(), invs[0].ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in issued/draft state")
}
