package market

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
)

// Auction lifecycle states.
const (
	StatusOpen      = "open"
	StatusClosed    = "closed"
	StatusCancelled = "cancelled"
)

// Record is the persisted auction row.
type Record struct {
	ID         string
	TaskID     string
	Strategy   Strategy
	Status     string
	WinnerBID  string
	OpenedAt   time.Time
	ClosedAt   *time.Time
}

// PublishFunc matches the shim published by event.Bus.PublishErr so we avoid a
// cycle with the event package.
type PublishFunc func(ctx context.Context, eventType, source string, payload any) error

// Store persists auctions, bids, and agent token balances.
type Store struct {
	db  *sql.DB
	bus PublishFunc
}

// NewStore creates a market store.
func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// WithBus installs a publisher so auction lifecycle transitions emit events.
func (s *Store) WithBus(p PublishFunc) *Store {
	s.bus = p
	return s
}

// Open creates a new auction row and returns its ID.
func (s *Store) Open(ctx context.Context, taskID string, strategy Strategy) (string, error) {
	id := ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO auctions (id, task_id, strategy, status) VALUES (?, ?, ?, ?)`,
		id, taskID, string(strategy), StatusOpen)
	if err != nil {
		return "", fmt.Errorf("opening auction: %w", err)
	}
	return id, nil
}

// SubmitBid inserts a bid against an open auction.
func (s *Store) SubmitBid(ctx context.Context, auctionID string, bid Bid) error {
	id := ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO bids (id, auction_id, agent_id, agent_name, price, est_duration_ms, reputation)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, auctionID, bid.AgentID, bid.AgentName, bid.Price, bid.EstDuration.Milliseconds(), bid.Reputation)
	return err
}

// Bids returns all bids for an auction.
func (s *Store) Bids(ctx context.Context, auctionID string) ([]Bid, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, agent_id, agent_name, price, est_duration_ms, reputation, won, created_at
		 FROM bids WHERE auction_id = ?`, auctionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Bid
	for rows.Next() {
		var b Bid
		var durMs int64
		var won int
		var created string
		if err := rows.Scan(&b.ID, &b.AgentID, &b.AgentName, &b.Price, &durMs, &b.Reputation, &won, &created); err != nil {
			return nil, err
		}
		b.EstDuration = time.Duration(durMs) * time.Millisecond
		b.Won = won == 1
		b.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
		out = append(out, b)
	}
	return out, rows.Err()
}

// Close marks an auction as closed with the winning bid ID.
func (s *Store) Close(ctx context.Context, auctionID, winningBidID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`UPDATE auctions SET status = ?, winner_bid_id = ?, closed_at = datetime('now')
		 WHERE id = ? AND status = ?`,
		StatusClosed, winningBidID, auctionID, StatusOpen); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE bids SET won = 1 WHERE id = ?`, winningBidID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	if s.bus != nil {
		// Grab the winning bid's agent name for the event payload.
		var agentName string
		var price float64
		_ = s.db.QueryRowContext(ctx,
			`SELECT agent_name, price FROM bids WHERE id = ?`, winningBidID,
		).Scan(&agentName, &price)
		_ = s.bus(ctx, "task.auction.won", "market", map[string]any{
			"auction_id": auctionID,
			"bid_id":     winningBidID,
			"agent":      agentName,
			"price":      price,
		})
	}
	return nil
}

// Cancel voids an auction without a winner.
func (s *Store) Cancel(ctx context.Context, auctionID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE auctions SET status = ?, closed_at = datetime('now') WHERE id = ? AND status = ?`,
		StatusCancelled, auctionID, StatusOpen)
	return err
}

// ---------------- Token wallet (Story 18.3) ----------------

// Credit adds tokens to an agent's balance.
func (s *Store) Credit(ctx context.Context, agentName string, amount float64) error {
	if amount < 0 {
		return fmt.Errorf("amount must be non-negative")
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO agent_tokens (agent_name, balance, updated_at) VALUES (?, ?, datetime('now'))
		 ON CONFLICT(agent_name) DO UPDATE SET balance = balance + excluded.balance, updated_at = datetime('now')`,
		agentName, amount)
	return err
}

// Debit subtracts tokens; returns an error if the balance would go negative.
func (s *Store) Debit(ctx context.Context, agentName string, amount float64) error {
	if amount < 0 {
		return fmt.Errorf("amount must be non-negative")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var balance float64
	err = tx.QueryRowContext(ctx, `SELECT balance FROM agent_tokens WHERE agent_name = ?`, agentName).Scan(&balance)
	if err == sql.ErrNoRows {
		return fmt.Errorf("agent %s has no wallet", agentName)
	}
	if err != nil {
		return err
	}
	if balance < amount {
		return fmt.Errorf("insufficient balance: have %.2f, need %.2f", balance, amount)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE agent_tokens SET balance = balance - ?, updated_at = datetime('now') WHERE agent_name = ?`,
		amount, agentName); err != nil {
		return err
	}
	return tx.Commit()
}

// Balance returns the token balance for an agent (0 if no wallet).
func (s *Store) Balance(ctx context.Context, agentName string) (float64, error) {
	var balance float64
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(balance, 0) FROM agent_tokens WHERE agent_name = ?`, agentName).Scan(&balance)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return balance, err
}
