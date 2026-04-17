package storage

import (
	"strings"
	"testing"
)

func TestSplitSimple(t *testing.T) {
	got := splitSQLStatements("SELECT 1; SELECT 2;")
	if len(got) != 2 {
		t.Fatalf("want 2 statements, got %d: %v", len(got), got)
	}
	if strings.TrimSpace(got[0]) != "SELECT 1" {
		t.Errorf("got[0] = %q", got[0])
	}
}

func TestSplitIgnoresSemicolonInLineComment(t *testing.T) {
	src := `
-- commentaire avec ; dedans qui ne doit pas couper
SELECT 1;
`
	got := splitSQLStatements(src)
	// Un seul vrai statement SQL, mais le commentaire fait partie du
	// chunk courant — SQLite ignore les -- comments donc c'est safe.
	if len(got) != 1 {
		t.Fatalf("want 1 statement, got %d: %#v", len(got), got)
	}
	if !strings.Contains(got[0], "SELECT 1") {
		t.Errorf("SELECT 1 dropped: %q", got[0])
	}
}

func TestSplitIgnoresSemicolonInBlockComment(t *testing.T) {
	src := `/* plusieurs statements ici ; mais commented ; */ SELECT 1;`
	got := splitSQLStatements(src)
	if len(got) != 1 {
		t.Fatalf("want 1, got %d: %#v", len(got), got)
	}
}

func TestSplitIgnoresSemicolonInStringLiteral(t *testing.T) {
	src := `INSERT INTO t VALUES ('hello; world'); SELECT 2;`
	got := splitSQLStatements(src)
	if len(got) != 2 {
		t.Fatalf("want 2, got %d: %#v", len(got), got)
	}
	if !strings.Contains(got[0], "hello; world") {
		t.Errorf("string literal broken: %q", got[0])
	}
}

func TestSplitHandlesEscapedQuote(t *testing.T) {
	src := `INSERT INTO t VALUES ('it''s fine'); SELECT 2;`
	got := splitSQLStatements(src)
	if len(got) != 2 {
		t.Fatalf("want 2, got %d: %#v", len(got), got)
	}
}

func TestSplitIgnoresSemicolonInDoubleQuoteIdent(t *testing.T) {
	src := `CREATE TABLE "weird;name" (x INT); SELECT 1;`
	got := splitSQLStatements(src)
	if len(got) != 2 {
		t.Fatalf("want 2, got %d: %#v", len(got), got)
	}
}

func TestSplitIgnoresSemicolonInBackticks(t *testing.T) {
	src := "CREATE TABLE `mix;up` (x INT); SELECT 1;"
	got := splitSQLStatements(src)
	if len(got) != 2 {
		t.Fatalf("want 2, got %d: %#v", len(got), got)
	}
}

func TestSplitTrailingStatementWithoutSemicolon(t *testing.T) {
	src := `SELECT 1; SELECT 2`
	got := splitSQLStatements(src)
	if len(got) != 2 {
		t.Fatalf("want 2, got %d: %#v", len(got), got)
	}
	if !strings.Contains(got[1], "SELECT 2") {
		t.Errorf("trailing no-; lost: %q", got[1])
	}
}

func TestSplitEmpty(t *testing.T) {
	if got := splitSQLStatements(""); len(got) != 0 {
		t.Fatalf("empty → %v", got)
	}
	if got := splitSQLStatements("   \n\n  "); len(got) != 0 {
		t.Fatalf("whitespace-only → %v", got)
	}
}

// Regression P49 : le piège historique qui a forcé la réécriture du
// commentaire de migration 025. Ce cas exact doit maintenant passer.
func TestSplitRegressionFrenchCommentWithSemicolon(t *testing.T) {
	src := `-- Le code a été supprimé en P43 ; les tables restaient car dropper
-- des tables est irréversible.
DROP TABLE IF EXISTS agent_tokens;
DROP TABLE IF EXISTS agents;`
	got := splitSQLStatements(src)
	// Deux DROP → deux statements.
	if len(got) != 2 {
		t.Fatalf("want 2, got %d: %#v", len(got), got)
	}
	for i, s := range got {
		if !strings.Contains(s, "DROP TABLE IF EXISTS") {
			t.Errorf("got[%d] corrupted: %q", i, s)
		}
	}
}
