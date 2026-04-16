package dialog

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
)

// Thread represents a dialog between two agents.
type Thread struct {
	ID                 string    `json:"id"`
	InitiatorAgentID   string    `json:"initiator_agent_id"`
	ParticipantAgentID string    `json:"participant_agent_id"`
	Topic              string    `json:"topic"`
	Status             string    `json:"status"`
	MessageCount       int       `json:"message_count"`
	CreatedAt          time.Time `json:"created_at"`
}

// Message represents a single message in a dialog thread.
type Message struct {
	ID            int64     `json:"id"`
	ThreadID      string    `json:"thread_id"`
	SenderAgentID string    `json:"sender_agent_id"`
	Content       string    `json:"content"`
	CreatedAt     time.Time `json:"created_at"`
}

// Manager handles dialog thread lifecycle.
type Manager struct {
	db *sql.DB
}

// NewManager creates a dialog manager.
func NewManager(db *sql.DB) *Manager {
	return &Manager{db: db}
}

// CreateThread starts a new dialog between two agents.
func (m *Manager) CreateThread(ctx context.Context, initiatorID, participantID, topic string) (*Thread, error) {
	id := ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader)

	_, err := m.db.ExecContext(ctx,
		`INSERT INTO dialog_threads (id, initiator_agent_id, participant_agent_id, topic) VALUES (?, ?, ?, ?)`,
		id.String(), initiatorID, participantID, topic,
	)
	if err != nil {
		return nil, fmt.Errorf("creating thread: %w", err)
	}

	return &Thread{
		ID: id.String(), InitiatorAgentID: initiatorID,
		ParticipantAgentID: participantID, Topic: topic, Status: "active",
	}, nil
}

// AddMessage adds a message to a thread.
func (m *Manager) AddMessage(ctx context.Context, threadID, senderAgentID, content string) (*Message, error) {
	result, err := m.db.ExecContext(ctx,
		`INSERT INTO dialog_messages (thread_id, sender_agent_id, content) VALUES (?, ?, ?)`,
		threadID, senderAgentID, content,
	)
	if err != nil {
		return nil, fmt.Errorf("adding message: %w", err)
	}
	msgID, _ := result.LastInsertId()
	return &Message{ID: msgID, ThreadID: threadID, SenderAgentID: senderAgentID, Content: content}, nil
}

// CloseThread marks a thread as completed.
func (m *Manager) CloseThread(ctx context.Context, threadID string) error {
	result, err := m.db.ExecContext(ctx,
		`UPDATE dialog_threads SET status = 'completed', completed_at = datetime('now') WHERE id = ?`, threadID,
	)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return fmt.Errorf("thread %s not found", threadID)
	}
	return nil
}

// ListThreads returns all threads with message counts.
func (m *Manager) ListThreads(ctx context.Context) ([]Thread, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT t.id, t.initiator_agent_id, t.participant_agent_id, t.topic, t.status,
		       COUNT(m.id) as msg_count, t.created_at
		FROM dialog_threads t
		LEFT JOIN dialog_messages m ON m.thread_id = t.id
		GROUP BY t.id
		ORDER BY t.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var threads []Thread
	for rows.Next() {
		var t Thread
		var created string
		if err := rows.Scan(&t.ID, &t.InitiatorAgentID, &t.ParticipantAgentID,
			&t.Topic, &t.Status, &t.MessageCount, &created); err != nil {
			continue
		}
		t.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
		threads = append(threads, t)
	}
	return threads, rows.Err()
}

// GetMessages returns all messages in a thread.
func (m *Manager) GetMessages(ctx context.Context, threadID string) ([]Message, error) {
	rows, err := m.db.QueryContext(ctx,
		`SELECT id, thread_id, sender_agent_id, content, created_at FROM dialog_messages WHERE thread_id = ? ORDER BY id`,
		threadID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		var created string
		if err := rows.Scan(&msg.ID, &msg.ThreadID, &msg.SenderAgentID, &msg.Content, &created); err != nil {
			continue
		}
		msg.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
		messages = append(messages, msg)
	}
	return messages, rows.Err()
}
