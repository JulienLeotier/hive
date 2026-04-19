package git

import (
	"context"
	"os/exec"
	"strings"
)

// AuditSnapshot capture l'état git d'un workdir : branche courante +
// HEAD sha. Utilisé pour détecter ce qu'une skill BMAD a touché : on
// snapshot AVANT, puis on compare APRÈS pour savoir si BMAD a
// commité sur main, créé une branche, etc.
type AuditSnapshot struct {
	Branch string
	HEAD   string
}

// Snapshot prend un instantané cheap de l'état git. Retourne zero
// value si workdir n'est pas un repo (pas une erreur : certains
// projets Hive tournent sans git).
func Snapshot(ctx context.Context, workdir string) AuditSnapshot {
	if workdir == "" {
		return AuditSnapshot{}
	}
	snap := AuditSnapshot{
		Branch: runGit(ctx, workdir, "rev-parse", "--abbrev-ref", "HEAD"),
		HEAD:   runGit(ctx, workdir, "rev-parse", "HEAD"),
	}
	return snap
}

// Drift compare deux snapshots pour repérer ce qui a bougé entre AVANT
// et APRÈS une skill. Renvoie une liste de mentions (strings courtes
// lisibles) à logger / stocker en metadata.
func (a AuditSnapshot) Drift(after AuditSnapshot) []string {
	var out []string
	if a.Branch != after.Branch && after.Branch != "" {
		out = append(out, "branch "+a.Branch+"→"+after.Branch)
	}
	if a.HEAD != after.HEAD && a.HEAD != "" && after.HEAD != "" {
		out = append(out,
			"HEAD "+shortSHA(a.HEAD)+"→"+shortSHA(after.HEAD))
	}
	return out
}

// IsOnDefaultBranch retourne true si on est sur main/master — utile
// pour flag "BMAD a commité sur main direct, alors qu'il devrait
// créer feat/*".
func (a AuditSnapshot) IsOnDefaultBranch() bool {
	return a.Branch == "main" || a.Branch == "master"
}

func runGit(ctx context.Context, workdir string, args ...string) string {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", workdir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func shortSHA(sha string) string {
	if len(sha) < 7 {
		return sha
	}
	return sha[:7]
}
