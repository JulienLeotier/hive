package market

import (
	"context"
	"testing"
	"time"

	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupStore(t *testing.T) *Store {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	return NewStore(st.DB)
}

func TestAuctionLifecycle(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	auctionID, err := s.Open(ctx, "task-1", StrategyLowestCost)
	require.NoError(t, err)

	require.NoError(t, s.SubmitBid(ctx, auctionID, Bid{
		AgentID: "a1", AgentName: "cheap", Price: 0.10, EstDuration: time.Second, Reputation: 0.9,
	}))
	require.NoError(t, s.SubmitBid(ctx, auctionID, Bid{
		AgentID: "a2", AgentName: "fast", Price: 0.20, EstDuration: 500 * time.Millisecond, Reputation: 0.95,
	}))

	bids, err := s.Bids(ctx, auctionID)
	require.NoError(t, err)
	assert.Len(t, bids, 2)

	// Pick the lowest-cost bid and close.
	winner, err := NewAuction(nil).SelectWinner(bids, StrategyLowestCost)
	require.NoError(t, err)
	assert.Equal(t, "cheap", winner.AgentName)

	require.NoError(t, s.Close(ctx, auctionID, winner.ID))

	// Re-close is a no-op (status is now closed, not open).
	require.NoError(t, s.Close(ctx, auctionID, winner.ID))
}

func TestTokensCreditDebitBalance(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	require.NoError(t, s.Credit(ctx, "worker", 10))
	require.NoError(t, s.Credit(ctx, "worker", 5))

	bal, err := s.Balance(ctx, "worker")
	require.NoError(t, err)
	assert.Equal(t, 15.0, bal)

	require.NoError(t, s.Debit(ctx, "worker", 4))
	bal, _ = s.Balance(ctx, "worker")
	assert.Equal(t, 11.0, bal)

	// Can't debit below zero
	err = s.Debit(ctx, "worker", 1000)
	assert.Error(t, err)
}

func TestTokensDebitUnknownAgent(t *testing.T) {
	s := setupStore(t)
	err := s.Debit(context.Background(), "nobody", 1)
	assert.Error(t, err)
}
