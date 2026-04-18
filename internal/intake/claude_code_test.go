package intake

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestBuildReplyPrompt vérifie que les règles stack defaults + scope
// lock sont bien présentes dans le prompt. C'est un canari contre
// les régressions qui effaceraient les garde-fous anti-drift.
func TestBuildReplyPrompt(t *testing.T) {
	history := []Message{
		{Author: AuthorUser, Content: "Je veux une todolist simple"},
		{Author: "pm", Content: "Ok, quelle stack ?"},
	}
	out := buildReplyPrompt("Simple todolist web", history)

	// Instructions de base.
	assert.Contains(t, out, "PM agent")
	assert.Contains(t, out, "one clarifying question at a time")

	// Garde-fous stack (ancrage anti-Go-by-default).
	assert.Contains(t, out, "STACK DEFAULTS")
	assert.Contains(t, out, "vanilla HTML")
	assert.Contains(t, out, "localStorage")
	assert.Contains(t, out, "Do NOT default to Go")

	// Format de sortie JSON.
	assert.Contains(t, out, `"reply"`)
	assert.Contains(t, out, `"done"`)

	// Transcript présent.
	assert.Contains(t, out, "User: Je veux une todolist simple")
	assert.Contains(t, out, "PM: Ok, quelle stack ?")
}

func TestBuildPRDPrompt(t *testing.T) {
	out := buildPRDPrompt("todolist", []Message{
		{Author: AuthorUser, Content: "simple"},
	})

	// Structure du brief scope-locked.
	assert.Contains(t, out, "SCOPE LOCK")
	assert.Contains(t, out, "In-scope")
	assert.Contains(t, out, "Non-goals")
	assert.Contains(t, out, "Definition of Done")

	// Marqueurs d'extraction.
	assert.Contains(t, out, "<<<PRD")
	assert.Contains(t, out, "PRD>>>")

	// Stack lean forcé.
	assert.Contains(t, out, "leanest viable")
	assert.Contains(t, out, "DEFAULT STACK")
}

func TestTruncateIntake(t *testing.T) {
	assert.Equal(t, "hi", truncate("hi", 10))
	assert.Equal(t, "hello…", truncate("hello world", 5))
	assert.Equal(t, "", truncate("", 10))
}

// TestBuildReplyPromptAuthorTags : si l'auteur n'est pas "user", on le
// marque "PM" dans le transcript. Canari : un éventuel refactor qui
// casserait la convention mélangerait les tours.
func TestBuildReplyPromptAuthorTags(t *testing.T) {
	out := buildReplyPrompt("idea", []Message{
		{Author: AuthorUser, Content: "human says"},
		{Author: "assistant", Content: "pm says"},
	})
	assert.Contains(t, out, "User: human says")
	assert.Contains(t, out, "PM: pm says")
	// Ordre chronologique préservé.
	userIdx := strings.Index(out, "User: human says")
	pmIdx := strings.Index(out, "PM: pm says")
	assert.Less(t, userIdx, pmIdx)
}
