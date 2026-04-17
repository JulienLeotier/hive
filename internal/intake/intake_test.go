package intake

import (
	"context"
	"strings"
	"testing"

	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setup(t *testing.T) (*Store, string) {
	t.Helper()
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })

	// Seed a project to anchor the conversation.
	_, err = st.DB.Exec(
		`INSERT INTO projects (id, name, idea, status, tenant_id) VALUES (?, ?, ?, ?, ?)`,
		"prj_test", "demo", "an app for writers", "draft", "default",
	)
	require.NoError(t, err)
	return NewStore(st.DB), "prj_test"
}

func TestGetOrStartSeedsGreeting(t *testing.T) {
	store, projectID := setup(t)
	ctx := context.Background()

	conv, err := store.GetOrStart(ctx, projectID, "an app for writers", NewScriptedAgent())
	require.NoError(t, err)
	require.NotNil(t, conv)
	assert.Equal(t, StatusActive, conv.Status)
	require.Len(t, conv.Messages, 1, "greeting is seeded")
	assert.Equal(t, RolePM, conv.Messages[0].Author)
	assert.Contains(t, conv.Messages[0].Content, "an app for writers",
		"greeting should quote the idea back")
}

func TestGetOrStartIsIdempotent(t *testing.T) {
	store, projectID := setup(t)
	ctx := context.Background()
	agent := NewScriptedAgent()

	conv1, err := store.GetOrStart(ctx, projectID, "idea", agent)
	require.NoError(t, err)
	conv2, err := store.GetOrStart(ctx, projectID, "idea", agent)
	require.NoError(t, err)
	assert.Equal(t, conv1.ID, conv2.ID, "second call must return the same conversation")
}

func TestAppendUserMessageWalksTheRubric(t *testing.T) {
	store, projectID := setup(t)
	ctx := context.Background()
	agent := NewScriptedAgent()

	conv, err := store.GetOrStart(ctx, projectID, "idea", agent)
	require.NoError(t, err)

	// Walk through every rubric question except the last answer.
	answers := []string{
		"indie novelists drafting their first book",
		"(1) capture idea → (2) outline → (3) draft with inline AI",
		"no social features, no hosting of copyrighted text",
		"SvelteKit + Go backend, Postgres",
		"MVP in 4 weeks, success = 10 beta users finishing a chapter",
	}
	for i, ans := range answers {
		updated, done, err := store.AppendUserMessage(ctx, conv.ID, "idea", ans, agent)
		require.NoError(t, err, "answer %d", i)
		if i < len(answers)-1 {
			assert.False(t, done, "not done yet, still rubric slots to cover (answer %d)", i)
		} else {
			assert.True(t, done, "final answer must trigger done=true")
		}
		conv = updated
	}

	// Count user and agent messages.
	userCount, agentCount := 0, 0
	for _, m := range conv.Messages {
		if m.Author == AuthorUser {
			userCount++
		} else {
			agentCount++
		}
	}
	assert.Equal(t, 5, userCount)
	assert.Equal(t, 6, agentCount, "greeting + 5 follow-ups (the 5th being the 'enough to write the PRD' ack)")
}

func TestFinalizeStitchesPRD(t *testing.T) {
	store, projectID := setup(t)
	ctx := context.Background()
	agent := NewScriptedAgent()

	conv, err := store.GetOrStart(ctx, projectID, "app for writers", agent)
	require.NoError(t, err)
	answers := []string{
		"indie novelists",
		"capture, outline, draft",
		"no hosting",
		"Svelte + Go",
		"MVP in 4 weeks",
	}
	for _, ans := range answers {
		_, _, err := store.AppendUserMessage(ctx, conv.ID, "app for writers", ans, agent)
		require.NoError(t, err)
	}

	prd, err := store.Finalize(ctx, conv.ID, "app for writers", agent)
	require.NoError(t, err)

	assert.Contains(t, prd, "# Product Requirements Document")
	assert.Contains(t, prd, "## Audience & problem")
	assert.Contains(t, prd, "## Core flows")
	assert.Contains(t, prd, "## Definition of done")
	assert.Contains(t, prd, "indie novelists")
	assert.Contains(t, prd, "MVP in 4 weeks")

	// Conversation is flipped to finalized.
	reloaded, err := store.Load(ctx, conv.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusFinalized, reloaded.Status)
}

func TestAppendUserMessageRejectsEmpty(t *testing.T) {
	store, projectID := setup(t)
	ctx := context.Background()
	agent := NewScriptedAgent()

	conv, err := store.GetOrStart(ctx, projectID, "idea", agent)
	require.NoError(t, err)

	_, _, err = store.AppendUserMessage(ctx, conv.ID, "idea", "   ", agent)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "empty"))
}
