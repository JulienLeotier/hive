package architect

import (
	"context"
	"strings"
	"testing"

	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const samplePRD = `# Product Requirements Document

## Summary

App for indie novelists with inline AI assistance.

## Audience & problem

Novelists drafting their first book need cheaper help than a human editor.

## Core flows

1. Capture idea → outline → draft.
2. Inline edit suggestions from the assistant.
3. Export a clean markdown chapter.

## Constraints & non-goals

No social features. No hosted copyright-protected text.

## Tech notes

SvelteKit front, Go back, Postgres.

## Definition of done

Ten beta users finish a chapter within 4 weeks of release.
`

func TestExtractSectionHandlesAliases(t *testing.T) {
	body := ExtractSection(samplePRD, "Audience & problem", "Audience")
	assert.Contains(t, body, "Novelists drafting")
	assert.NotContains(t, body, "## Core flows", "section must stop before the next heading")
}

func TestExtractSectionMissing(t *testing.T) {
	assert.Equal(t, "", ExtractSection("# no headings here", "Audience"))
}

func TestScriptedDecomposeCoversAllRubricEpicsWhenPRDIsComplete(t *testing.T) {
	a := NewScripted()
	epics, err := a.Decompose(context.Background(), "writers app", samplePRD)
	require.NoError(t, err)
	// Foundations + Audience + Core Flows + Non-Goals + Definition of Done = 5.
	require.Len(t, epics, 5)
	titles := make([]string, len(epics))
	for i, e := range epics {
		titles[i] = e.Title
	}
	assert.Equal(t, "Foundations", titles[0], "Foundations always runs first")
	assert.Contains(t, titles, "Core user flows")
	assert.Contains(t, titles, "Definition of done")

	// Every story has at least one AC.
	for _, e := range epics {
		for _, s := range e.Stories {
			assert.NotEmpty(t, s.AcceptanceCriteria, "story %q needs at least one AC", s.Title)
		}
	}
}

func TestScriptedDecomposeSkipsMissingSections(t *testing.T) {
	prd := `# PRD

## Summary

thin idea

## Tech notes

Go + Svelte
`
	a := NewScripted()
	epics, err := a.Decompose(context.Background(), "thin idea", prd)
	require.NoError(t, err)
	// Should have Foundations (always) + maybe that's it since other
	// sections missing.
	require.GreaterOrEqual(t, len(epics), 1)
	assert.Equal(t, "Foundations", epics[0].Title)
	for _, e := range epics {
		assert.NotEqual(t, "Core user flows", e.Title, "no core flows section → no core flows epic")
	}
}

func TestScriptedDecomposeRejectsEmptyPRD(t *testing.T) {
	_, err := NewScripted().Decompose(context.Background(), "x", "")
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "empty"))
}

func TestDispatcherPersistsEpicsStoriesACs(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()

	// Seed a project.
	_, err = st.DB.Exec(
		`INSERT INTO projects (id, name, idea, status, tenant_id) VALUES (?, ?, ?, ?, ?)`,
		"prj_test", "demo", "writers app", "planning", "default",
	)
	require.NoError(t, err)

	d := NewDispatcher(st.DB, NewScripted())
	nEpics, nStories, err := d.Run(context.Background(), "prj_test", "writers app", samplePRD)
	require.NoError(t, err)
	assert.Equal(t, 5, nEpics)
	assert.Greater(t, nStories, 5)

	var actualEpics, actualStories, actualACs int
	require.NoError(t, st.DB.QueryRow(`SELECT COUNT(*) FROM epics WHERE project_id = ?`, "prj_test").Scan(&actualEpics))
	require.NoError(t, st.DB.QueryRow(`SELECT COUNT(*) FROM stories s
		JOIN epics e ON e.id = s.epic_id WHERE e.project_id = ?`, "prj_test").Scan(&actualStories))
	require.NoError(t, st.DB.QueryRow(`SELECT COUNT(*) FROM acceptance_criteria ac
		JOIN stories s ON s.id = ac.story_id
		JOIN epics e ON e.id = s.epic_id WHERE e.project_id = ?`, "prj_test").Scan(&actualACs))

	assert.Equal(t, 5, actualEpics)
	assert.Greater(t, actualStories, 5)
	assert.Greater(t, actualACs, actualStories, "each story should have at least 1 AC, total > story count")
}

func TestDispatcherIsIdempotent(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()

	_, err = st.DB.Exec(
		`INSERT INTO projects (id, name, idea, status, tenant_id) VALUES (?, ?, ?, ?, ?)`,
		"prj_test", "demo", "x", "planning", "default",
	)
	require.NoError(t, err)

	d := NewDispatcher(st.DB, NewScripted())
	nEpics1, _, err := d.Run(context.Background(), "prj_test", "x", samplePRD)
	require.NoError(t, err)
	nEpics2, _, err := d.Run(context.Background(), "prj_test", "x", samplePRD)
	require.NoError(t, err)

	assert.Greater(t, nEpics1, 0)
	assert.Equal(t, 0, nEpics2, "re-run must not duplicate epics")
}
