package devloop

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/JulienLeotier/hive/internal/bmad"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCountDecisionNeeded : detection du keyword "decision-needed" sous
// ses diverses orthographes. Conditionne l'escalation architect.
func TestCountDecisionNeeded(t *testing.T) {
	cases := map[string]int{
		"":                              0,
		"everything ok":                 0,
		"decision-needed on foo":        1,
		"DECISION-NEEDED here":          1,
		"three: decision-needed, decision-needed, decision-needed": 3,
		"decision_needed underscore":    1,
		"decision needed no dash":       1,
	}
	for input, want := range cases {
		assert.Equal(t, want, countDecisionNeeded(input), "input=%q", input)
	}
}

// TestFirstLineFromClaudeCode : helper interne distinct du firstLine
// de stream.go — situé dans devloop/claude_code.go.
func TestFirstLineFromClaudeCode(t *testing.T) {
	assert.Equal(t, "", firstLine(""))
	assert.Equal(t, "only", firstLine("only"))
	assert.Equal(t, "first", firstLine("first\nsecond\nthird"))
	assert.Equal(t, "trim", firstLine("   trim   "))
}

// TestSnapshotSprint : renvoie copie du development_status ou map vide
// quand le fichier n'existe pas.
func TestSnapshotSprint(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		got := snapshotSprint(t.TempDir())
		assert.Empty(t, got)
	})
	t.Run("with file", func(t *testing.T) {
		workdir := t.TempDir()
		dir := filepath.Join(workdir, "_bmad-output", "implementation-artifacts")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(dir, "sprint-status.yaml"),
			[]byte("development_status:\n  1-1: ready-for-dev\n  1-2: done\n"),
			0o644,
		))
		got := snapshotSprint(workdir)
		assert.Equal(t, map[string]string{"1-1": "ready-for-dev", "1-2": "done"}, got)
	})
}

// TestActiveBMADKey détecte la clé que BMAD vient de traiter en
// diffant une snapshot pre-run contre l'état courant.
func TestActiveBMADKey(t *testing.T) {
	workdir := t.TempDir()
	dir := filepath.Join(workdir, "_bmad-output", "implementation-artifacts")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	t.Run("ready-for-dev → in-progress", func(t *testing.T) {
		require.NoError(t, os.WriteFile(
			filepath.Join(dir, "sprint-status.yaml"),
			[]byte("development_status:\n  1-1: in-progress\n"),
			0o644,
		))
		pre := map[string]string{"1-1": "ready-for-dev"}
		assert.Equal(t, "1-1", activeBMADKey(pre, workdir))
	})
	t.Run("fresh entry ready-for-dev → review", func(t *testing.T) {
		require.NoError(t, os.WriteFile(
			filepath.Join(dir, "sprint-status.yaml"),
			[]byte("development_status:\n  2-3: review\n"),
			0o644,
		))
		assert.Equal(t, "2-3", activeBMADKey(map[string]string{}, workdir))
	})
	t.Run("no change", func(t *testing.T) {
		require.NoError(t, os.WriteFile(
			filepath.Join(dir, "sprint-status.yaml"),
			[]byte("development_status:\n  1-1: done\n"),
			0o644,
		))
		assert.Equal(t, "", activeBMADKey(map[string]string{"1-1": "done"}, workdir))
	})
}

// TestClaudeCodeAgentsSetters exerce les With* fluent setters de chaque
// agent. Ce sont des triviaux mais ça ajoute du coverage pour pas cher.
func TestClaudeCodeAgentsSetters(t *testing.T) {
	t.Run("dev", func(t *testing.T) {
		d := &ClaudeCodeDev{}
		assert.Equal(t, "bmad-dev", d.Name())
		d.WithDB(nil).WithPublisher(nil).WithCancelRegistry(nil)
	})
	t.Run("reviewer", func(t *testing.T) {
		r := &ClaudeCodeReviewer{}
		assert.Equal(t, "bmad-reviewer", r.Name())
		r.WithDB(nil).WithPublisher(nil).WithCancelRegistry(nil)
	})
	t.Run("architect", func(t *testing.T) {
		a := &ClaudeCodeArchitect{}
		assert.Equal(t, "bmad-architect", a.Name())
		a.WithDB(nil).WithPublisher(nil).WithCancelRegistry(nil)
	})
}

// TestMakeDevloopObserverNoDB : quand db==nil, l'observer renvoyé est
// vide (tous callbacks nil) — ça n'explose pas si appelé quand même.
func TestMakeDevloopObserverNoDB(t *testing.T) {
	obs := makeDevloopObserver(nil, nil, nil, "prj", "", "story")
	assert.Nil(t, obs.OnStart)
	assert.Nil(t, obs.OnFinish)
	assert.Nil(t, obs.OnChunk)

	// Et si projectID est vide avec une DB : même résultat.
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	obs2 := makeDevloopObserver(db, nil, nil, "", "", "story")
	assert.Nil(t, obs2.OnStart)
}

// TestResumeBuildStreamEventExposed : check simple que StreamEvent
// passe bien entre bmad et devloop sans conversion.
func TestResumeBuildStreamEventExposed(t *testing.T) {
	_ = bmad.StreamEvent{Type: "assistant", Text: "hello"}
}

// TestParseACVerdicts : per-AC parsing du reply BMAD. Heuristique,
// pas censé être parfaite — juste mieux que "toutes ACs same verdict".
func TestParseACVerdicts(t *testing.T) {
	t.Run("no ACs", func(t *testing.T) {
		assert.Empty(t, parseACVerdicts("anything", 0, false))
	})

	t.Run("empty reply", func(t *testing.T) {
		r := parseACVerdicts("", 3, false)
		assert.Len(t, r, 3)
		for _, ac := range r {
			assert.False(t, ac.decided)
		}
	})

	t.Run("explicit AC pass/fail", func(t *testing.T) {
		reply := "AC1 : ✓ pass\nAC2 : ✗ fail\nAC3 : pas mentionné"
		r := parseACVerdicts(reply, 3, false)
		assert.True(t, r[0].decided)
		assert.True(t, r[0].passed)
		assert.True(t, r[1].decided)
		assert.False(t, r[1].passed)
		assert.False(t, r[2].decided, "AC3 pas de signal fort")
	})

	t.Run("Acceptance N syntax", func(t *testing.T) {
		reply := "Acceptance 1 est validé dans le code. Acceptance 2 manque encore."
		r := parseACVerdicts(reply, 2, false)
		assert.True(t, r[0].decided && r[0].passed)
		assert.True(t, r[1].decided && !r[1].passed)
	})

	t.Run("no signal → fallback", func(t *testing.T) {
		reply := "AC1 apparaît mais sans verdict clair."
		r := parseACVerdicts(reply, 1, false)
		assert.False(t, r[0].decided)
	})
}

func TestFindACMention(t *testing.T) {
	assert.Equal(t, -1, findACMention("nothing", 1))
	assert.Equal(t, 0, findACMention("ac1 blabla", 1))
	assert.Equal(t, 0, findACMention("acceptance 3", 3))
	// Plusieurs patterns → le PLUS PROCHE du début gagne.
	assert.Equal(t, 5, findACMention("blah ac-2 plus tard [ac2]", 2))
}

func TestStrOf(t *testing.T) {
	assert.Equal(t, "1", strOf(1))
	assert.Equal(t, "9", strOf(9))
	assert.Equal(t, "10", strOf(10))
	assert.Equal(t, "42", strOf(42))
}
