package adapter

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildAdapter_DispatchesByType proves that a stored agent row gets
// rebuilt as the right Adapter at runtime. Before the factory existed, the
// workflow engine only knew how to construct HTTPAdapter, which made
// claude-code / crewai / autogen / langchain / mcp registrations look
// valid but silently fail on first Invoke.
func TestBuildAdapter_DispatchesByType(t *testing.T) {
	cases := []struct {
		name    string
		spec    AgentSpec
		want    string // prefix of fmt.Sprintf("%T", a)
		wantErr bool
	}{
		{
			name: "http with base_url",
			spec: AgentSpec{Type: TypeHTTP, Config: `{"base_url":"http://x"}`},
			want: "*adapter.HTTPAdapter",
		},
		{
			name:    "http missing base_url errors",
			spec:    AgentSpec{Type: TypeHTTP, Config: `{}`, Name: "bad"},
			wantErr: true,
		},
		{
			name: "claude-code with path",
			spec: AgentSpec{Type: TypeClaude, Config: `{"path":"/tmp"}`, Name: "c"},
			want: "*adapter.ClaudeCodeAdapter",
		},
		{
			name: "crewai with path",
			spec: AgentSpec{Type: TypeCrewAI, Config: `{"path":"/tmp"}`, Name: "c"},
			want: "*adapter.CrewAIAdapter",
		},
		{
			name: "autogen with path → file:// url",
			spec: AgentSpec{Type: TypeAutoGen, Config: `{"path":"/tmp"}`, Name: "a"},
			want: "*adapter.AutoGenAdapter",
		},
		{
			name: "langchain with path",
			spec: AgentSpec{Type: TypeLangChain, Config: `{"path":"/tmp"}`, Name: "l"},
			want: "*adapter.LangChainAdapter",
		},
		{
			name: "mcp with path",
			spec: AgentSpec{Type: TypeMCP, Config: `{"path":"/tmp/server"}`, Name: "m"},
			want: "*adapter.MCPAdapter",
		},
		{
			name:    "unknown type errors",
			spec:    AgentSpec{Type: "nonsense", Name: "x"},
			wantErr: true,
		},
		{
			name:    "openai without env key errors",
			spec:    AgentSpec{Type: TypeOpenAI, Config: `{"path":"asst_abc"}`, Name: "o"},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("OPENAI_API_KEY", "") // scrub host env so the openai case is deterministic
			a, err := BuildAdapter(tc.spec)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			got := fmt.Sprintf("%T", a)
			assert.True(t, strings.HasPrefix(got, tc.want),
				"expected type prefix %q, got %q", tc.want, got)
		})
	}
}

func TestBuildAdapter_OpenAIWithKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test")
	a, err := BuildAdapter(AgentSpec{Type: TypeOpenAI, Config: `{"path":"asst_xyz"}`, Name: "o"})
	require.NoError(t, err)
	assert.NotNil(t, a)
}
