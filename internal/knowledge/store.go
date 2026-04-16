package knowledge

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"
)

// Entry represents a knowledge entry in the shared knowledge layer.
type Entry struct {
	ID        int64     `json:"id"`
	TaskType  string    `json:"task_type"`
	Approach  string    `json:"approach"`
	Outcome   string    `json:"outcome"` // "success" or "failure"
	Context   string    `json:"context"`
	CreatedAt time.Time `json:"created_at"`
}

// Store manages the shared knowledge layer.
type Store struct {
	db       *sql.DB
	maxAge   time.Duration // entries older than this are excluded from search
	embedder Embedder
}

// NewStore creates a knowledge store.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db, maxAge: 90 * 24 * time.Hour} // 90 days default
}

// WithEmbedder attaches an embedder so Record auto-populates embedding and
// VectorSearch becomes available. Stories 16.1 + 16.2.
func (s *Store) WithEmbedder(e Embedder) *Store {
	s.embedder = e
	return s
}

// Record stores a knowledge entry (success or failure).
func (s *Store) Record(ctx context.Context, taskType, approach, outcome, ctxJSON string) error {
	var embedding []byte
	if s.embedder != nil {
		vec, err := s.embedder.Embed(approach + " " + ctxJSON)
		if err != nil {
			return fmt.Errorf("embedding knowledge: %w", err)
		}
		embedding = Encode(vec)
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO knowledge (task_type, approach, outcome, context, embedding) VALUES (?, ?, ?, ?, ?)`,
		taskType, approach, outcome, ctxJSON, embedding,
	)
	if err != nil {
		return fmt.Errorf("recording knowledge: %w", err)
	}
	slog.Debug("knowledge recorded", "task_type", taskType, "outcome", outcome)
	return nil
}

// VectorSearch ranks entries by cosine similarity against the query embedding.
// Falls back to an error if no Embedder is configured.
func (s *Store) VectorSearch(ctx context.Context, query string, limit int) ([]Entry, error) {
	if s.embedder == nil {
		return nil, fmt.Errorf("vector search requires an Embedder (call WithEmbedder first)")
	}
	if limit <= 0 {
		limit = 5
	}

	qVec, err := s.embedder.Embed(query)
	if err != nil {
		return nil, fmt.Errorf("embedding query: %w", err)
	}

	cutoff := time.Now().Add(-s.maxAge)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, task_type, approach, outcome, COALESCE(context,''), created_at, embedding
		 FROM knowledge
		 WHERE created_at >= ? AND embedding IS NOT NULL
		 ORDER BY created_at DESC LIMIT 5000`,
		cutoff.Format("2006-01-02 15:04:05"))
	if err != nil {
		return nil, fmt.Errorf("loading knowledge for vector search: %w", err)
	}
	defer rows.Close()

	type scored struct {
		entry Entry
		score float32
	}
	var ranked []scored
	for rows.Next() {
		var e Entry
		var created string
		var blob []byte
		if err := rows.Scan(&e.ID, &e.TaskType, &e.Approach, &e.Outcome, &e.Context, &created, &blob); err != nil {
			continue
		}
		e.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
		score := Cosine(qVec, Decode(blob))
		if score <= 0 {
			continue
		}
		ranked = append(ranked, scored{entry: e, score: score})
	}

	// Sort descending by similarity.
	for i := 1; i < len(ranked); i++ {
		for j := i; j > 0 && ranked[j].score > ranked[j-1].score; j-- {
			ranked[j], ranked[j-1] = ranked[j-1], ranked[j]
		}
	}

	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	out := make([]Entry, len(ranked))
	for i, r := range ranked {
		out[i] = r.entry
	}
	return out, nil
}

// Search finds entries similar to the query, ranked by relevance and recency.
// Uses simple keyword matching (TF-IDF-like) — upgrade to vector embeddings in v0.3.
func (s *Store) Search(ctx context.Context, query string, limit int) ([]Entry, error) {
	if limit <= 0 {
		limit = 5
	}

	// Fetch all non-expired entries
	cutoff := time.Now().Add(-s.maxAge)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, task_type, approach, outcome, COALESCE(context, ''), created_at
		 FROM knowledge WHERE created_at >= ? ORDER BY created_at DESC LIMIT 1000`,
		cutoff.Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		return nil, fmt.Errorf("querying knowledge: %w", err)
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		var created string
		if err := rows.Scan(&e.ID, &e.TaskType, &e.Approach, &e.Outcome, &e.Context, &created); err != nil {
			continue
		}
		e.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
		entries = append(entries, e)
	}

	// Score by keyword similarity + recency
	type scored struct {
		entry Entry
		score float64
	}
	queryWords := strings.Fields(strings.ToLower(query))
	var results []scored

	for _, e := range entries {
		text := strings.ToLower(e.TaskType + " " + e.Approach + " " + e.Context)
		var matches int
		for _, w := range queryWords {
			if strings.Contains(text, w) {
				matches++
			}
		}
		if matches == 0 {
			continue
		}

		// Keyword similarity (0-1)
		similarity := float64(matches) / float64(len(queryWords))

		// Recency boost: newer entries score higher (exponential decay, half-life 30 days)
		age := time.Since(e.CreatedAt).Hours() / 24.0
		recencyBoost := math.Exp(-0.023 * age) // ln(2)/30 ≈ 0.023

		score := similarity*0.7 + recencyBoost*0.3
		results = append(results, scored{entry: e, score: score})
	}

	// Sort by score descending
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].score > results[i].score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Take top N
	if len(results) > limit {
		results = results[:limit]
	}

	var out []Entry
	for _, r := range results {
		out = append(out, r.entry)
	}
	return out, nil
}

// ListByType returns all entries for a given task type.
func (s *Store) ListByType(ctx context.Context, taskType string) ([]Entry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, task_type, approach, outcome, COALESCE(context, ''), created_at
		 FROM knowledge WHERE task_type = ? ORDER BY created_at DESC`,
		taskType,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		var created string
		if err := rows.Scan(&e.ID, &e.TaskType, &e.Approach, &e.Outcome, &e.Context, &created); err != nil {
			continue
		}
		e.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// Count returns total knowledge entries.
func (s *Store) Count(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM knowledge`).Scan(&count)
	return count, err
}
