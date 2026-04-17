// Package intake drives the PM-agent Q&A that turns a raw idea into a PRD.
// The BMAD method starts here: before an Architect can decompose anything,
// the user needs to answer the right "what exactly do you want?" questions.
//
// The flow is web-driven (no CLI) — API endpoints append user messages and
// return agent replies, the dashboard renders the exchange as a chat. Once
// the PM has enough context, finalize() concatenates the answers into a
// PRD, stores it on the project, and flips the project to `planning` so
// Phase 3's Architect pipeline takes over.
//
// Two Agent implementations ship:
//
//   - ScriptedAgent — deterministic, asks a fixed rubric (user audience,
//     core flows, non-functional constraints, tech preferences, budget).
//     Always available, works in CI, and serves as a fallback when the
//     Claude CLI isn't on PATH. Also shapes the minimum PRD every project
//     gets even if the user rushes the intake.
//
//   - ClaudeCodeAgent — invokes the local `claude` CLI with the
//     conversation history piped on stdin and a PM system prompt. Lets
//     the real model drive depth of questioning for non-trivial ideas.
//     Falls back to ScriptedAgent on any CLI failure so the build flow
//     never dead-ends on an environment issue.
package intake

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

// Role identifiers for a project's conversations. Today only PM is wired,
// but the same table carries the Architect + Reviewer exchanges that
// Phase 3+ will spawn.
const (
	RolePM        = "pm"
	RoleArchitect = "architect"
	RoleReviewer  = "reviewer"
)

// Author identifies who wrote a message. Human authors are always "user";
// agents use their role as the author tag so multi-agent conversations
// (later phases) read cleanly.
const (
	AuthorUser    = "user"
	AuthorPM      = "pm"
)

// Conversation statuses.
const (
	StatusActive    = "active"
	StatusFinalized = "finalized"
)

// Conversation is one ongoing Q&A between the user and a single agent
// role. The message list is loaded with it — the payload is always small
// (text only), so we don't bother paginating.
type Conversation struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Messages  []Message `json:"messages,omitempty"`
}

// Message is one turn in a conversation.
type Message struct {
	ID             int64     `json:"id"`
	ConversationID string    `json:"conversation_id"`
	Author         string    `json:"author"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"created_at"`
}

// Agent is the tiny surface the intake supervisor needs to drive a
// conversation forward. Implementations decide how to pick the next
// question (static rubric, LLM call, etc.).
type Agent interface {
	// Role returns the fixed role tag the agent plays (pm, architect, …).
	Role() string
	// Greeting is the opening message the agent posts once the
	// conversation starts. Has access to the project idea so the greeting
	// can quote it back for the user.
	Greeting(ctx context.Context, projectIdea string) string
	// Reply produces the agent's next message given the running
	// conversation (first entry is the greeting, last entry is the
	// user's latest answer). Returning `done=true` signals that the
	// PRD is ready to be finalised.
	Reply(ctx context.Context, projectIdea string, history []Message) (reply string, done bool, err error)
	// FinalPRD assembles the PRD text from the complete conversation.
	// Called once the user clicks Finalize.
	FinalPRD(ctx context.Context, projectIdea string, history []Message) (string, error)
}

// Store owns the conversation and message rows. Kept slim — the API
// glue lives in internal/api.
type Store struct {
	db *sql.DB
}

// NewStore builds a store backed by the hive DB.
func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// GetOrStart returns the PM conversation for the project, starting a fresh
// one and seeding the agent greeting when no conversation exists. Safe
// to call multiple times — idempotent on the (project, role) pair.
func (s *Store) GetOrStart(ctx context.Context, projectID, projectIdea string, agent Agent) (*Conversation, error) {
	conv, err := s.findActive(ctx, projectID, agent.Role())
	if err != nil {
		return nil, err
	}
	if conv != nil {
		return s.Load(ctx, conv.ID)
	}

	id := "conv_" + ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO project_conversations (id, project_id, role, status)
		 VALUES (?, ?, ?, ?)`,
		id, projectID, agent.Role(), StatusActive,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting conversation: %w", err)
	}

	greeting := agent.Greeting(ctx, projectIdea)
	if err := s.appendMessage(ctx, id, agent.Role(), greeting); err != nil {
		return nil, err
	}
	return s.Load(ctx, id)
}

// AppendUserMessage records a user reply and then asks the agent for its
// next turn. The agent's reply is stored too before returning the full
// updated conversation. If the agent signals `done=true`, the caller can
// prompt the user to hit Finalize.
func (s *Store) AppendUserMessage(
	ctx context.Context,
	conversationID, projectIdea, content string,
	agent Agent,
) (*Conversation, bool, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, false, fmt.Errorf("message is empty")
	}
	if err := s.appendMessage(ctx, conversationID, AuthorUser, content); err != nil {
		return nil, false, err
	}
	conv, err := s.Load(ctx, conversationID)
	if err != nil {
		return nil, false, err
	}
	if conv.Status == StatusFinalized {
		return conv, true, nil
	}

	reply, done, err := agent.Reply(ctx, projectIdea, conv.Messages)
	if err != nil {
		return nil, false, fmt.Errorf("agent reply: %w", err)
	}
	if err := s.appendMessage(ctx, conversationID, agent.Role(), reply); err != nil {
		return nil, false, err
	}
	conv, err = s.Load(ctx, conversationID)
	if err != nil {
		return nil, false, err
	}
	return conv, done, nil
}

// Finalize asks the agent for a PRD, marks the conversation finalised,
// and returns the PRD text. Caller persists it on the project.
func (s *Store) Finalize(
	ctx context.Context,
	conversationID, projectIdea string,
	agent Agent,
) (string, error) {
	conv, err := s.Load(ctx, conversationID)
	if err != nil {
		return "", err
	}
	// Re-finalise is idempotent — we recompute and overwrite. The stored
	// messages are the source of truth, not the conversation status.
	prd, err := agent.FinalPRD(ctx, projectIdea, conv.Messages)
	if err != nil {
		return "", fmt.Errorf("building PRD: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE project_conversations SET status = ?, updated_at = datetime('now') WHERE id = ?`,
		StatusFinalized, conversationID,
	)
	if err != nil {
		return "", err
	}
	return prd, nil
}

// Load fetches the conversation with all messages attached.
func (s *Store) Load(ctx context.Context, conversationID string) (*Conversation, error) {
	var c Conversation
	var created, updated string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, project_id, role, status, created_at, updated_at
		 FROM project_conversations WHERE id = ?`,
		conversationID,
	).Scan(&c.ID, &c.ProjectID, &c.Role, &c.Status, &created, &updated)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("conversation %s not found", conversationID)
	}
	if err != nil {
		return nil, err
	}
	c.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
	c.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updated)

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, conversation_id, author, content, created_at
		 FROM project_messages WHERE conversation_id = ? ORDER BY id ASC`,
		conversationID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var m Message
		var mCreated string
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Author, &m.Content, &mCreated); err != nil {
			return nil, err
		}
		m.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", mCreated)
		c.Messages = append(c.Messages, m)
	}
	return &c, rows.Err()
}

// findActive looks up an existing active conversation for (project, role).
// Returns nil, nil when none exists.
func (s *Store) findActive(ctx context.Context, projectID, role string) (*Conversation, error) {
	var id, status string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, status FROM project_conversations
		 WHERE project_id = ? AND role = ?
		 ORDER BY created_at DESC LIMIT 1`,
		projectID, role,
	).Scan(&id, &status)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &Conversation{ID: id, Status: status}, nil
}

// appendMessage inserts a row + bumps the conversation's updated_at.
func (s *Store) appendMessage(ctx context.Context, conversationID, author, content string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // rollback after Commit is a no-op
	_, err = tx.ExecContext(ctx,
		`INSERT INTO project_messages (conversation_id, author, content)
		 VALUES (?, ?, ?)`,
		conversationID, author, content,
	)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx,
		`UPDATE project_conversations SET updated_at = datetime('now') WHERE id = ?`,
		conversationID,
	)
	if err != nil {
		return err
	}
	return tx.Commit()
}
