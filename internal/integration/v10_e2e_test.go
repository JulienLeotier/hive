package integration

import (
	"context"
	"testing"
	"time"

	"github.com/JulienLeotier/hive/internal/audit"
	"github.com/JulienLeotier/hive/internal/auth"
	"github.com/JulienLeotier/hive/internal/cluster"
	"github.com/JulienLeotier/hive/internal/federation"
	"github.com/JulienLeotier/hive/internal/market"
	"github.com/JulienLeotier/hive/internal/optimizer"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestV10FeaturesEndToEnd exercises the full v1.0 surface across modules:
// market auctions, federation links, RBAC users, audit log, cluster roster,
// and optimizer trends — all backed by a single SQLite DB with migration 006.
func TestV10FeaturesEndToEnd(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()
	ctx := context.Background()

	// --- Market: open auction, submit bids, close with a winner.
	mkt := market.NewStore(st.DB)
	auctionID, err := mkt.Open(ctx, "task-1", market.StrategyLowestCost)
	require.NoError(t, err)

	require.NoError(t, mkt.SubmitBid(ctx, auctionID, market.Bid{
		AgentID: "a1", AgentName: "cheap", Price: 0.10, EstDuration: time.Second, Reputation: 0.8,
	}))
	require.NoError(t, mkt.SubmitBid(ctx, auctionID, market.Bid{
		AgentID: "a2", AgentName: "fast", Price: 0.25, EstDuration: 500 * time.Millisecond, Reputation: 0.95,
	}))

	bids, err := mkt.Bids(ctx, auctionID)
	require.NoError(t, err)
	require.Len(t, bids, 2)

	winner, err := market.NewAuction(nil).SelectWinner(bids, market.StrategyLowestCost)
	require.NoError(t, err)
	require.NoError(t, mkt.Close(ctx, auctionID, winner.ID))

	require.NoError(t, mkt.Credit(ctx, "cheap", 100))
	balance, _ := mkt.Balance(ctx, "cheap")
	assert.Equal(t, 100.0, balance)

	// --- Federation: persist a link and hydrate a Manager from it.
	fed := federation.NewStore(st.DB)
	require.NoError(t, fed.Add(ctx, &federation.Link{
		Name: "peer-1", URL: "https://peer", Status: "active", SharedCaps: []string{"code-review"},
	}, "", "", ""))
	m := federation.NewManager()
	require.NoError(t, fed.Hydrate(ctx, m))
	assert.Len(t, m.ListLinks(), 1)

	// --- RBAC: store and read back a user.
	users := auth.NewUserStore(st.DB)
	require.NoError(t, users.Upsert(ctx, auth.UserRecord{
		Subject: "alice@example.com", Role: auth.RoleOperator, TenantID: "acme",
	}))
	got, err := users.Get(ctx, "alice@example.com")
	require.NoError(t, err)
	assert.Equal(t, auth.RoleOperator, got.Role)
	assert.Equal(t, "acme", got.TenantID)

	// --- Audit: write + read.
	logger := audit.NewLogger(st.DB)
	require.NoError(t, logger.Log(ctx, "agent.register", "alice@example.com", "agents/worker", "initial setup"))
	entries, err := logger.Query(ctx, time.Now().Add(-time.Hour), 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "alice@example.com", entries[0].Actor)

	// --- Cluster: heartbeat, list, mark stale.
	roster := cluster.NewRoster(st.DB)
	require.NoError(t, roster.Heartbeat(ctx, &cluster.Node{
		ID: "node-A", Hostname: "host-A", Address: ":7777", Status: "active",
	}))
	nodes, err := roster.List(ctx)
	require.NoError(t, err)
	assert.Len(t, nodes, 1)

	// --- Optimizer: a Trend call must succeed even on empty tasks.
	an := optimizer.NewAnalyzer(st.DB)
	cur, prev, err := an.Trend(ctx, 7)
	require.NoError(t, err)
	assert.Equal(t, "7d", cur.Window)
	assert.Equal(t, 0, cur.TasksRun)
	assert.Equal(t, 0, prev.TasksRun)
}
