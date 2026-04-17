// Package git wraps the local `git` and `gh` CLIs for the one-off
// operations Hive does on a project's workdir : clone an existing
// GitHub repo, or create a fresh one. The per-story flow
// (branches, commits, PRs, merges) stays inside internal/devloop
// where it belongs with the supervisor; this package is only for
// the project-level bootstrap.
package git

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// GhStatus reports whether the `gh` CLI is installed and the user is
// authenticated. Used by the dashboard to enable/disable GitHub
// options in the project-creation form.
type GhStatus struct {
	Installed     bool   `json:"installed"`
	Authenticated bool   `json:"authenticated"`
	Login         string `json:"login,omitempty"`
	Error         string `json:"error,omitempty"`
}

// CheckGh probes `gh` for readiness. Never returns an error — failure
// is reported via the Status fields so the UI can present a
// remediation message.
func CheckGh(ctx context.Context) GhStatus {
	path, err := exec.LookPath("gh")
	if err != nil {
		return GhStatus{Error: "gh n'est pas installé — https://cli.github.com"}
	}
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(callCtx, path, "api", "user", "--jq", ".login")
	out, err := cmd.Output()
	if err != nil {
		return GhStatus{
			Installed: true,
			Error:     "gh non authentifié — lance `gh auth login` dans ton terminal",
		}
	}
	login := strings.TrimSpace(string(out))
	return GhStatus{
		Installed:     true,
		Authenticated: true,
		Login:         login,
	}
}

// CloneRepo clones a GitHub repo URL (or owner/name shorthand) into
// workdir. Uses `gh repo clone` which handles both HTTPS and SSH
// auth via the user's gh credentials. workdir must not exist or
// must be empty.
func CloneRepo(ctx context.Context, target, workdir string) error {
	if target == "" || workdir == "" {
		return errors.New("git: repo target ou workdir vide")
	}
	if info, err := os.Stat(workdir); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("git: %s existe et n'est pas un répertoire", workdir)
		}
		// Répertoire existe — vérifier qu'il est vide.
		entries, derr := os.ReadDir(workdir)
		if derr != nil {
			return fmt.Errorf("git: lecture de %s: %w", workdir, derr)
		}
		if len(entries) > 0 {
			return fmt.Errorf("git: %s n'est pas vide — choisis un autre workdir", workdir)
		}
	}
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(callCtx, "gh", "repo", "clone", target, workdir)
	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gh repo clone: %w — %s", err, truncate(combined.String(), 200))
	}
	return nil
}

// CreateRepo creates a new GitHub repo populated with an initial
// commit from workdir. Calls:
//
//	git init (if workdir isn't already a repo)
//	git add -A && git commit
//	gh repo create <name> --source=workdir --remote=origin --push --<visibility>
//
// Returns the canonical HTTPS URL of the created repo.
func CreateRepo(ctx context.Context, name, workdir, visibility string) (string, error) {
	if name == "" || workdir == "" {
		return "", errors.New("git: nom de repo ou workdir vide")
	}
	if visibility != "public" && visibility != "private" && visibility != "internal" {
		visibility = "private"
	}
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		return "", fmt.Errorf("git: préparer workdir: %w", err)
	}
	// Init local si pas déjà un repo.
	if _, err := os.Stat(workdir + "/.git"); err != nil {
		if err := runIn(ctx, workdir, "git", "init", "-b", "main"); err != nil {
			return "", err
		}
		// Seed un README pour éviter le repo vide (gh repo create
		// --push refuse le repo sans commit).
		seed := "# " + name + "\n\nGéré par Hive BMAD.\n"
		if err := os.WriteFile(workdir+"/README.md", []byte(seed), 0o644); err != nil {
			return "", fmt.Errorf("git: seed README: %w", err)
		}
		_ = runIn(ctx, workdir, "git", "add", "-A")
		_ = runIn(ctx, workdir, "git", "-c", "user.email=bmad@hive.local",
			"-c", "user.name=Hive BMAD", "commit", "-m", "chore: initial Hive BMAD scaffold")
	}
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	args := []string{
		"repo", "create", name,
		"--source", workdir,
		"--remote", "origin",
		"--push",
		"--" + visibility,
	}
	cmd := exec.CommandContext(callCtx, "gh", args...)
	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("gh repo create: %w — %s", err, truncate(combined.String(), 200))
	}

	// Récupérer l'URL canonique.
	url, err := getRemoteURL(ctx, workdir)
	if err != nil {
		// Fallback : reconstruire depuis gh api
		login, _ := ghLogin(ctx)
		if login != "" {
			return fmt.Sprintf("https://github.com/%s/%s", login, name), nil
		}
		return "", err
	}
	return url, nil
}

// RemoteURL retourne l'URL `origin` du repo dans workdir. Retourne
// "" sans erreur si le repo n'a pas de remote.
func RemoteURL(ctx context.Context, workdir string) (string, error) {
	return getRemoteURL(ctx, workdir)
}

// LoginWithToken authentifie la CLI `gh` avec un personal access
// token via `gh auth login --with-token` (stdin). Le token est écrit
// dans ~/.config/gh/hosts.yml par gh — on ne le stocke pas dans
// Hive. Errors remontent verbose pour que l'UI puisse diagnostiquer
// (token manquant de scope `repo`, etc.).
func LoginWithToken(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("token GitHub vide")
	}
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("gh non installé — https://cli.github.com")
	}
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(callCtx, "gh", "auth", "login",
		"--hostname", "github.com", "--git-protocol", "https", "--with-token")
	cmd.Stdin = strings.NewReader(token + "\n")
	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gh auth login: %w — %s", err, truncate(combined.String(), 300))
	}
	// Sanity-check : on doit pouvoir hit /user maintenant.
	if _, err := ghLogin(ctx); err != nil {
		return fmt.Errorf("login accepté mais /user inaccessible — vérifie les scopes du token (repo, workflow): %w", err)
	}
	return nil
}

// Repo résume un repo GitHub accessible à l'utilisateur courant,
// formaté pour l'UI (sélecteur de clone).
type Repo struct {
	NameWithOwner string `json:"name_with_owner"`
	Description   string `json:"description,omitempty"`
	URL           string `json:"url"`
	Private       bool   `json:"private"`
	UpdatedAt     string `json:"updated_at,omitempty"`
}

// ListRepos invoque `gh repo list` et retourne jusqu'à 200 repos
// visibles du user, triés par date de mise à jour (gh fait le sort
// lui-même). Sert à l'UI pour proposer un autocomplete dans le champ
// clone au lieu d'obliger l'opérateur à taper l'URL à la main.
func ListRepos(ctx context.Context) ([]Repo, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return nil, errors.New("gh non installé")
	}
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(callCtx, "gh", "repo", "list",
		"--limit", "200",
		"--json", "nameWithOwner,description,url,isPrivate,updatedAt")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh repo list: %w", err)
	}
	var raw []struct {
		NameWithOwner string `json:"nameWithOwner"`
		Description   string `json:"description"`
		URL           string `json:"url"`
		IsPrivate     bool   `json:"isPrivate"`
		UpdatedAt     string `json:"updatedAt"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parse gh repo list: %w", err)
	}
	repos := make([]Repo, 0, len(raw))
	for _, r := range raw {
		repos = append(repos, Repo{
			NameWithOwner: r.NameWithOwner,
			Description:   r.Description,
			URL:           r.URL,
			Private:       r.IsPrivate,
			UpdatedAt:     r.UpdatedAt,
		})
	}
	return repos, nil
}

// Logout supprime l'auth gh locale (`gh auth logout --hostname github.com`).
func Logout(ctx context.Context) error {
	if _, err := exec.LookPath("gh"); err != nil {
		return nil
	}
	callCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(callCtx, "gh", "auth", "logout",
		"--hostname", "github.com")
	// `--hostname` seul déconnecte sans prompt sur les versions récentes.
	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gh auth logout: %w — %s", err, truncate(combined.String(), 200))
	}
	return nil
}

func getRemoteURL(ctx context.Context, workdir string) (string, error) {
	callCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(callCtx, "git", "-C", workdir, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return "", nil //nolint:nilerr // pas de remote == pas une erreur
	}
	url := strings.TrimSpace(string(out))
	// Normaliser les URLs SSH vers HTTPS pour affichage.
	if strings.HasPrefix(url, "git@github.com:") {
		rest := strings.TrimSuffix(strings.TrimPrefix(url, "git@github.com:"), ".git")
		url = "https://github.com/" + rest
	}
	return url, nil
}

func ghLogin(ctx context.Context) (string, error) {
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(callCtx, "gh", "api", "user")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	var resp struct {
		Login string `json:"login"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return "", err
	}
	return resp.Login, nil
}

func runIn(ctx context.Context, workdir, name string, args ...string) error {
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(callCtx, name, args...)
	cmd.Dir = workdir
	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w — %s",
			name, strings.Join(args, " "), err, truncate(combined.String(), 200))
	}
	return nil
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
