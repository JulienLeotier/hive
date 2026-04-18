package bmad

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseStreamLine covers toutes les shapes d'events que le CLI
// claude émet avec --output-format stream-json. Les shapes sont stables
// dans la v2.x mais on garde la tolérance : event inconnu ne doit PAS
// crasher, juste renvoyer un StreamEvent avec un Type tagué.
func TestParseStreamLine(t *testing.T) {
	cases := []struct {
		name        string
		line        string
		wantType    string
		textContains string
	}{
		{
			name:         "system init",
			line:         `{"type":"system","subtype":"init","cwd":"/x"}`,
			wantType:     "system",
			textContains: "init",
		},
		{
			name: "assistant text block",
			line: `{"type":"assistant","message":{"content":[{"type":"text","text":"Hello BMAD"}]}}`,
			wantType:     "assistant",
			textContains: "Hello BMAD",
		},
		{
			name: "assistant tool_use Read",
			line: `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"/Users/x/y/z/foo.md"}}]}}`,
			wantType:     "assistant",
			textContains: "Read",
		},
		{
			name: "user tool_result string",
			line: `{"type":"user","message":{"content":[{"type":"tool_result","content":"file contents here","is_error":false}]}}`,
			wantType:     "tool_result",
			textContains: "file contents",
		},
		{
			name: "user tool_result error",
			line: `{"type":"user","message":{"content":[{"type":"tool_result","content":"boom","is_error":true}]}}`,
			wantType:     "tool_result",
			textContains: "✗",
		},
		{
			name: "user tool_result array",
			line: `{"type":"user","message":{"content":[{"type":"tool_result","content":[{"type":"text","text":"output line"}]}]}}`,
			wantType:     "tool_result",
			textContains: "output line",
		},
		{
			name:         "result final",
			line:         `{"type":"result","subtype":"success","result":"Voici la skill terminée.","total_cost_usd":0.5}`,
			wantType:     "result",
			textContains: "skill terminée",
		},
		{
			name:     "unknown type",
			line:     `{"type":"rate_limit_event","rate_limit_info":{}}`,
			wantType: "rate_limit_event",
		},
		{
			name:     "malformed JSON",
			line:     `{not-json`,
			wantType: "raw",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			evt := parseStreamLine([]byte(c.line))
			assert.Equal(t, c.wantType, evt.Type)
			if c.textContains != "" {
				assert.Contains(t, evt.Text, c.textContains)
			}
		})
	}
}

// TestSummariseToolUse vérifie que chaque tool du registre Claude
// donne une ligne lisible humainement au lieu du raw JSON. Canari
// pour les évolutions futures du format.
func TestSummariseToolUse(t *testing.T) {
	cases := map[string]struct {
		name  string
		input string
		want  string
	}{
		"Read":     {"Read", `{"file_path":"/Users/x/repo/src/main.go"}`, "Read .../repo/src/main.go"},
		"Edit":     {"Edit", `{"file_path":"/short.md"}`, "Edit /short.md"},
		"Bash":     {"Bash", `{"command":"go test ./..."}`, "Bash $ go test"},
		"Grep":     {"Grep", `{"pattern":"foo.*bar"}`, "Grep foo"},
		"Glob":     {"Glob", `{"pattern":"**/*.go"}`, "Glob"},
		"Task":     {"Task", `{"description":"Do X"}`, "Task: Do X"},
		"WebFetch": {"WebFetch", `{"url":"https://example.com"}`, "WebFetch"},
		"Skill":    {"Skill", `{"skill":"bmad-create-prd"}`, "Skill bmad-create-prd"},
		"TodoWrite": {"TodoWrite", `{"todos":[]}`, "TodoWrite"},
		"Unknown small input": {"Custom", `{"x":1}`, "Custom"},
		"Unknown large input":  {"Custom", `{"x":"` + strings.Repeat("a", 100) + `"}`, "Custom"},
	}
	for label, c := range cases {
		t.Run(label, func(t *testing.T) {
			got := summariseToolUse(c.name, json.RawMessage(c.input))
			assert.Contains(t, got, c.want)
		})
	}
}

func TestSummariseToolResult(t *testing.T) {
	t.Run("string short", func(t *testing.T) {
		got := summariseToolResult("just one line")
		assert.Equal(t, "just one line", got)
	})
	t.Run("string multiline", func(t *testing.T) {
		got := summariseToolResult("first\nsecond\nthird")
		assert.Contains(t, got, "first")
		assert.Contains(t, got, "(+2 lignes)")
	})
	t.Run("string empty", func(t *testing.T) {
		assert.Equal(t, "", summariseToolResult(""))
	})
	t.Run("array of text blocks", func(t *testing.T) {
		content := []any{
			map[string]any{"type": "text", "text": "line one\nline two"},
		}
		got := summariseToolResult(content)
		assert.Contains(t, got, "line one")
	})
	t.Run("unknown type", func(t *testing.T) {
		assert.Equal(t, "", summariseToolResult(42))
	})
}

func TestShortPath(t *testing.T) {
	cases := map[string]string{
		"short":                                          "short",
		"a/b":                                            "a/b",
		"/Users/a/b/c/d/e/f/g.md":                        ".../e/f/g.md",
		"/very/long/path/to/some/deeply/nested/file.txt": ".../deeply/nested/file.txt",
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			assert.Equal(t, want, shortPath(in))
		})
	}
}

func TestFirstLine(t *testing.T) {
	assert.Equal(t, "a", firstLine("a"))
	assert.Equal(t, "first", firstLine("first\nsecond"))
	assert.Equal(t, "", firstLine(""))
}

func TestJoinPath(t *testing.T) {
	assert.Equal(t, "/abs/path", joinPath("/ignored", "/abs/path"))
	assert.Equal(t, "dir/file.txt", joinPath("dir", "file.txt"))
}

func TestFileExistsNonEmpty(t *testing.T) {
	tmp := t.TempDir()
	empty := filepath.Join(tmp, "empty.txt")
	full := filepath.Join(tmp, "full.txt")
	require.NoError(t, os.WriteFile(empty, nil, 0o644))
	require.NoError(t, os.WriteFile(full, []byte("hi"), 0o644))

	assert.False(t, fileExistsNonEmpty(filepath.Join(tmp, "nope.txt")))
	assert.False(t, fileExistsNonEmpty(tmp), "dir is not a file")
	assert.False(t, fileExistsNonEmpty(empty), "empty file")
	assert.True(t, fileExistsNonEmpty(full))
}
