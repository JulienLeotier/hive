package market

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"crypto/rand"

	"github.com/oklog/ulid/v2"
)

// Strategy defines how the winning bid is selected.
type Strategy string

const (
	StrategyLowestCost  Strategy = "lowest-cost"
	StrategyFastest     Strategy = "fastest"
	StrategyBestReputation Strategy = "best-reputation"
)

// Bid represents an agent's offer to complete a task.
type Bid struct {
	ID            string        `json:"id"`
	TaskID        string        `json:"task_id"`
	AgentID       string        `json:"agent_id"`
	AgentName     string        `json:"agent_name"`
	Price         float64       `json:"price"`
	EstDuration   time.Duration `json:"est_duration"`
	Reputation    float64       `json:"reputation"` // 0.0-1.0
	Won           bool          `json:"won"`
	CreatedAt     time.Time     `json:"created_at"`
}

// Auction manages the bidding process for market-based task allocation.
type Auction struct {
	db *sql.DB
}

// NewAuction creates a market auction engine.
func NewAuction(db *sql.DB) *Auction {
	return &Auction{db: db}
}

// SubmitBid records an agent's bid for a task.
func (a *Auction) SubmitBid(ctx context.Context, taskID, agentID, agentName string, price float64, estDuration time.Duration, reputation float64) (*Bid, error) {
	id, err := ulid.New(ulid.Timestamp(time.Now()), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating bid ID: %w", err)
	}

	bid := &Bid{
		ID:          id.String(),
		TaskID:      taskID,
		AgentID:     agentID,
		AgentName:   agentName,
		Price:       price,
		EstDuration: estDuration,
		Reputation:  reputation,
		CreatedAt:   time.Now(),
	}

	slog.Debug("bid submitted", "task", taskID, "agent", agentName, "price", price)
	return bid, nil
}

// SelectWinner picks the winning bid based on strategy.
func (a *Auction) SelectWinner(bids []Bid, strategy Strategy) (*Bid, error) {
	if len(bids) == 0 {
		return nil, fmt.Errorf("no bids received")
	}

	sorted := make([]Bid, len(bids))
	copy(sorted, bids)

	switch strategy {
	case StrategyLowestCost:
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].Price < sorted[j].Price })
	case StrategyFastest:
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].EstDuration < sorted[j].EstDuration })
	case StrategyBestReputation:
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].Reputation > sorted[j].Reputation })
	default:
		// Default: weighted score (40% cost, 30% speed, 30% reputation)
		sort.Slice(sorted, func(i, j int) bool {
			scoreI := (1.0-sorted[i].Price)*0.4 + (1.0/float64(sorted[i].EstDuration.Seconds()+1))*0.3 + sorted[i].Reputation*0.3
			scoreJ := (1.0-sorted[j].Price)*0.4 + (1.0/float64(sorted[j].EstDuration.Seconds()+1))*0.3 + sorted[j].Reputation*0.3
			return scoreI > scoreJ
		})
	}

	winner := sorted[0]
	winner.Won = true
	slog.Info("auction won", "task", winner.TaskID, "agent", winner.AgentName, "price", winner.Price, "strategy", strategy)
	return &winner, nil
}
