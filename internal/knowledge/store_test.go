package knowledge

import (
	"context"
	"testing"

	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupStore(t *testing.T) *Store {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })

	// Create knowledge table (v0.2 migration)
	st.DB.Exec(`CREATE TABLE IF NOT EXISTS knowledge (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_type TEXT NOT NULL,
		approach TEXT NOT NULL,
		outcome TEXT NOT NULL,
		context TEXT,
		embedding BLOB,
		created_at TEXT DEFAULT (datetime('now'))
	)`)

	return NewStore(st.DB)
}

func TestRecordAndCount(t *testing.T) {
	s := setupStore(t)

	err := s.Record(context.Background(), "code-review", "check for null pointers", "success", `{"lang":"go"}`)
	require.NoError(t, err)

	count, err := s.Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestRecordFailure(t *testing.T) {
	s := setupStore(t)

	err := s.Record(context.Background(), "deploy", "skip health check", "failure", `{"reason":"timeout"}`)
	require.NoError(t, err)

	entries, err := s.ListByType(context.Background(), "deploy")
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "failure", entries[0].Outcome)
}

func TestSearchByKeywords(t *testing.T) {
	s := setupStore(t)

	s.Record(context.Background(), "code-review", "check null pointers and error handling in Go", "success", "")
	s.Record(context.Background(), "code-review", "review Python type hints", "success", "")
	s.Record(context.Background(), "deploy", "kubernetes rolling update", "success", "")

	results, err := s.Search(context.Background(), "Go error handling", 5)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 1)
	assert.Contains(t, results[0].Approach, "Go")
}

func TestSearchEmpty(t *testing.T) {
	s := setupStore(t)

	results, err := s.Search(context.Background(), "nonexistent topic xyz", 5)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSearchLimit(t *testing.T) {
	s := setupStore(t)

	for i := 0; i < 20; i++ {
		s.Record(context.Background(), "test", "approach with keyword match", "success", "")
	}

	results, err := s.Search(context.Background(), "keyword match", 3)
	require.NoError(t, err)
	assert.Len(t, results, 3)
}

func TestListByType(t *testing.T) {
	s := setupStore(t)

	s.Record(context.Background(), "code-review", "approach 1", "success", "")
	s.Record(context.Background(), "code-review", "approach 2", "failure", "")
	s.Record(context.Background(), "deploy", "approach 3", "success", "")

	entries, err := s.ListByType(context.Background(), "code-review")
	require.NoError(t, err)
	assert.Len(t, entries, 2)
}
