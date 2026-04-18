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
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	callCtx, cancel := context.WithCancel(ctx)
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
	if err := validateWorkdir(workdir); err != nil {
		return err
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
	callCtx, cancel := context.WithCancel(ctx)
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
	if err := validateWorkdir(workdir); err != nil {
		return "", err
	}
	if visibility != "public" && visibility != "private" && visibility != "internal" {
		visibility = "private"
	}
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		return "", fmt.Errorf("git: préparer workdir: %w", err)
	}
	// Init local si pas déjà un repo.
	if _, err := os.Stat(filepath.Join(workdir, ".git")); err != nil {
		// Avant d'init + add -A, on vérifie que le dossier est soit vide
		// soit ne contient QUE notre scaffold (README + .bmad-output).
		// Sinon on pourrait committer des fichiers personnels sans rapport.
		if err := assertSafeForGitInit(workdir); err != nil {
			return "", err
		}
		if err := runIn(ctx, workdir, "git", "init", "-b", "main"); err != nil {
			return "", err
		}
	}
	// Seed un README si absent (sinon gh repo create --push refuse un repo vide).
	readmePath := filepath.Join(workdir, "README.md")
	if _, err := os.Stat(readmePath); err != nil {
		seed := "# " + name + "\n\nGéré par Hive BMAD.\n"
		if err := os.WriteFile(readmePath, []byte(seed), 0o644); err != nil {
			return "", fmt.Errorf("git: seed README: %w", err)
		}
	}
	// Stage et commit si quelque chose n'est pas déjà committé.
	if err := ensureInitialCommit(ctx, workdir); err != nil {
		return "", fmt.Errorf("git: impossible de créer un commit initial : %w", err)
	}
	callCtx, cancel := context.WithCancel(ctx)
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
	callCtx, cancel := context.WithCancel(ctx)
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
	callCtx, cancel := context.WithCancel(ctx)
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

// DeviceFlow — GitHub OAuth device flow côté client. On appelle
// directement les endpoints GitHub (plus fiable que de parser le
// output de `gh auth login --web`), puis une fois le token obtenu
// on le pipe à `gh auth login --with-token` pour qu'il persiste
// dans ~/.config/gh comme si l'user avait fait un login standard.
//
// Client ID : celui du gh CLI public (178c6fc778ccc68e1d6a) — c'est
// le même qu'utiliserait `gh auth login --web`, aucune secret
// nécessaire puisque c'est une OAuth App publique.
const ghDeviceClientID = "178c6fc778ccc68e1d6a"

// DeviceStart kicks off the device flow and returns the user code,
// verification URL, opaque device code (to pass to DevicePoll), and
// polling interval.
type DeviceStart struct {
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	DeviceCode      string `json:"device_code"`
	Interval        int    `json:"interval"`
	ExpiresIn       int    `json:"expires_in"`
}

func StartDeviceFlow(ctx context.Context) (*DeviceStart, error) {
	form := "client_id=" + ghDeviceClientID + "&scope=" + httpEscapeScopes("repo workflow read:org")
	req, err := httpNewRequest(ctx, "POST", "https://github.com/login/device/code", form)
	if err != nil {
		return nil, err
	}
	var out DeviceStart
	if err := httpDoJSON(req, &out); err != nil {
		return nil, fmt.Errorf("device/code: %w", err)
	}
	if out.Interval <= 0 {
		out.Interval = 5
	}
	return &out, nil
}

// PollDeviceFlow checks if the user has authorized the device code.
// Returns the access token when granted, or (empty, pendingErr) while
// waiting. Callers poll at the interval returned by StartDeviceFlow.
func PollDeviceFlow(ctx context.Context, deviceCode string) (string, error) {
	form := "client_id=" + ghDeviceClientID +
		"&device_code=" + deviceCode +
		"&grant_type=urn:ietf:params:oauth:grant-type:device_code"
	req, err := httpNewRequest(ctx, "POST", "https://github.com/login/oauth/access_token", form)
	if err != nil {
		return "", err
	}
	var resp struct {
		AccessToken      string `json:"access_token"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := httpDoJSON(req, &resp); err != nil {
		return "", err
	}
	if resp.AccessToken != "" {
		// Persiste via gh pour que les prochaines commandes `gh …` soient auth.
		if err := LoginWithToken(ctx, resp.AccessToken); err != nil {
			return resp.AccessToken, fmt.Errorf("token obtenu mais gh auth failed: %w", err)
		}
		return resp.AccessToken, nil
	}
	// Cas pending : on relaie l'erreur GitHub pour que le caller sache quoi faire.
	return "", errors.New(resp.Error)
}

// Logout supprime l'auth gh locale (`gh auth logout --hostname github.com`).
func Logout(ctx context.Context) error {
	if _, err := exec.LookPath("gh"); err != nil {
		return nil
	}
	callCtx, cancel := context.WithCancel(ctx)
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
	callCtx, cancel := context.WithCancel(ctx)
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
	callCtx, cancel := context.WithCancel(ctx)
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
	callCtx, cancel := context.WithCancel(ctx)
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

// EnsureStoryPushed is the fallback Hive runs after a /bmad-dev-story
// invocation when we can't find a PR URL in BMAD's output. BMAD is
// supposed to commit+push+PR by itself, but Claude occasionally skips
// one of the steps — so we defensively :
//
//  1. Check that workdir is a git repo; bail politely if not.
//  2. If there are uncommitted changes, stage and commit them with a
//     standard message keyed on the story title.
//  3. If the branch has no upstream, push with -u to create it.
//  4. If no PR exists for this branch yet, create it via `gh pr create`
//     and return its URL.
//
// Idempotent : re-running is safe (commit is skipped when workdir is
// clean, push is skipped when upstream is already set, gh pr create is
// skipped when one already exists).
//
// Returns the PR URL on success, or ("", nil) when the workdir is
// simply not a git repo (greenfield scaffold, local-only project).
func EnsureStoryPushed(ctx context.Context, workdir, branch, storyTitle string) (string, error) {
	if workdir == "" {
		return "", errors.New("git: empty workdir")
	}
	// Not a repo → nothing to do, silently.
	if _, err := os.Stat(filepath.Join(workdir, ".git")); err != nil {
		return "", nil //nolint:nilerr // non-repo is a valid state
	}

	// Check tree state AVANT tout switch de branche : si working tree
	// clean ET pas de commits ahead d'origin, il n'y a littéralement
	// rien à PR-er (BMAD a probablement déjà committé ET pushé sur
	// main). On s'arrête là silencieusement — pas de faux PR vide.
	statusCtx, cancel := context.WithCancel(ctx)
	statusCmd := exec.CommandContext(statusCtx, "git", "-C", workdir, "status", "--porcelain")
	statusOut, err := statusCmd.Output()
	cancel()
	if err != nil {
		return "", fmt.Errorf("git status: %w", err)
	}
	hasUncommitted := len(strings.TrimSpace(string(statusOut))) > 0
	hasAheadCommits := commitsAheadOfOrigin(ctx, workdir)
	if !hasUncommitted && !hasAheadCommits {
		return "", nil
	}

	// Ensure a feature branch is checked out. Cas 1 : BMAD a fourni un
	// nom de branche → on y switch. Cas 2 : branch vide ET on est sur
	// la branche par défaut (main/master) → on fabrique un feat/<slug>
	// à partir du storyTitle. Sans ce fallback, `gh pr create` fail
	// plus bas avec "could not find any commits between origin/main
	// and main" puisque main == default branch on ne peut pas ouvrir
	// de PR vers elle-même.
	if branch == "" {
		cur := currentBranch(ctx, workdir)
		if cur == "" || cur == "main" || cur == "master" {
			branch = "feat/" + branchSlug(storyTitle)
		}
	}
	if branch != "" {
		_ = runIn(ctx, workdir, "git", "checkout", "-B", branch)
	}

	// Stage + commit if there are changes.
	if hasUncommitted {
		if err := runIn(ctx, workdir, "git", "add", "-A"); err != nil {
			return "", err
		}
		msg := "feat: " + storyTitle
		if storyTitle == "" {
			msg = "chore: BMAD dev-story update"
		}
		if err := runIn(ctx, workdir,
			"git", "-c", "user.email=bmad@hive.local", "-c", "user.name=Hive BMAD",
			"commit", "-m", msg); err != nil {
			// Rien à commit (fichiers déjà staged/committés) — on tolère.
			if !strings.Contains(err.Error(), "nothing to commit") {
				return "", err
			}
		}
	}

	// Push with upstream if needed.
	upCtx, upCancel := context.WithCancel(ctx)
	upCmd := exec.CommandContext(upCtx, "git", "-C", workdir, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	_, upErr := upCmd.Output()
	upCancel()
	if upErr != nil {
		// No upstream → push with -u.
		currentBranch := branch
		if currentBranch == "" {
			brCtx, brCancel := context.WithCancel(ctx)
			brOut, _ := exec.CommandContext(brCtx, "git", "-C", workdir, "rev-parse", "--abbrev-ref", "HEAD").Output()
			brCancel()
			currentBranch = strings.TrimSpace(string(brOut))
		}
		if currentBranch != "" {
			if err := runIn(ctx, workdir, "git", "push", "-u", "origin", currentBranch); err != nil {
				return "", fmt.Errorf("git push: %w", err)
			}
		}
	} else {
		// Upstream set → plain push.
		_ = runIn(ctx, workdir, "git", "push")
	}

	// Does a PR already exist?
	if _, err := exec.LookPath("gh"); err != nil {
		return "", nil // pas de gh → on s'arrête après le push
	}
	lookCtx, lookCancel := context.WithCancel(ctx)
	lookCmd := exec.CommandContext(lookCtx, "gh", "pr", "view", "--json", "url", "--jq", ".url")
	lookCmd.Dir = workdir
	if out, err := lookCmd.Output(); err == nil {
		lookCancel()
		if u := strings.TrimSpace(string(out)); u != "" {
			return u, nil
		}
	} else {
		lookCancel()
	}

	// Create a PR.
	title := storyTitle
	if title == "" {
		title = "BMAD dev-story"
	}
	prCtx, prCancel := context.WithCancel(ctx)
	defer prCancel()
	prCmd := exec.CommandContext(prCtx, "gh", "pr", "create",
		"--fill", "--title", title)
	prCmd.Dir = workdir
	out, err := prCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh pr create: %w — %s", err, truncate(string(out), 200))
	}
	// Extract PR URL from output.
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "https://github.com/") && strings.Contains(line, "/pull/") {
			return line, nil
		}
	}
	return strings.TrimSpace(string(out)), nil
}

// validateWorkdir refuse les workdir qui sont manifestement dangereux :
// le home de l'user, les racines systèmes, les dossiers "personnels"
// usuels (Documents, Downloads, Desktop, Pictures...) où l'user a
// probablement plein de fichiers sans rapport que Hive n'a aucune
// raison de `git add -A`.
//
// Incident qui a motivé ces garde-fous : un workdir=/Users/X/Documents
// a causé un git init + commit de 86 fichiers personnels (photos,
// PDFs, .DS_Store) et un push sur GitHub via gh repo create --push.
// Le repo était privé — mais le risque est sérieux et irréversible.
func validateWorkdir(workdir string) error {
	clean := filepath.Clean(workdir)
	if !filepath.IsAbs(clean) {
		return fmt.Errorf("git: workdir doit être un chemin absolu : %s", workdir)
	}
	if len(clean) < 4 {
		return fmt.Errorf("git: workdir %q trop court, risque d'écraser une racine système", clean)
	}
	home, _ := os.UserHomeDir()
	if home != "" && clean == home {
		return fmt.Errorf("git: workdir %q est ton home directory — crée un sous-dossier dédié au projet (ex. %s/projets/<nom>)", clean, home)
	}
	// Sous-dossiers personnels macOS/Linux : on refuse le dossier lui-
	// même mais on accepte un sous-dossier dédié (ex. ~/Documents/my-app).
	if home != "" {
		for _, personalDir := range []string{"Documents", "Downloads", "Desktop", "Pictures", "Music", "Movies", "Videos", "Library", ".config", ".local", ".ssh", ".gnupg"} {
			full := filepath.Join(home, personalDir)
			if clean == full {
				return fmt.Errorf("git: workdir %q contient probablement des fichiers personnels — crée un sous-dossier dédié (ex. %s/<nom-du-projet>)", clean, full)
			}
		}
	}
	// Racines / arborescences systèmes à bannir explicitement.
	for _, forbidden := range []string{"/", "/Users", "/home", "/etc", "/var", "/tmp", "/usr", "/bin", "/sbin", "/opt", "/private", "/System", "/Library", "/Applications"} {
		if clean == forbidden {
			return fmt.Errorf("git: workdir %q est une racine système, choisis un sous-dossier dédié", clean)
		}
	}
	return nil
}

// assertSafeForGitInit refuse de git init sur un workdir qui contient
// des fichiers qui n'ont manifestement pas été créés par Hive. Un dossier
// vide est OK. Un dossier qui ne contient que README.md / _bmad-output
// est OK (scaffold Hive légitime). Tout le reste déclenche un refus.
//
// Prévient le scénario où l'operateur saisit un path pointant sur un
// dossier personnel (Documents, Desktop) qui aurait passé l'allowlist
// ; le check contenu est un garde-fou de secours.
func assertSafeForGitInit(workdir string) error {
	entries, err := os.ReadDir(workdir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // sera créé par os.MkdirAll plus haut
		}
		return fmt.Errorf("git: lecture workdir: %w", err)
	}
	safe := map[string]bool{
		"README.md":      true,
		"_bmad-output":   true,
		"_bmad":          true,
		".claude":        true,
		".bmad":          true,
		".git":           true,
		".gitignore":     true,
		"LICENSE":        true,
		".DS_Store":      true, // tolérance macOS
	}
	for _, e := range entries {
		if !safe[e.Name()] {
			return fmt.Errorf("git: workdir %q contient %q qui n'est pas un artefact Hive — probablement un dossier personnel, refus du git init pour éviter de committer des fichiers sans rapport. Crée un sous-dossier vide dédié au projet", workdir, e.Name())
		}
	}
	return nil
}

// ensureInitialCommit stage tout ce qui n'est pas déjà committé et
// crée un commit si le repo n'a pas de HEAD (ou si des fichiers sont
// staged). Idempotent : si HEAD pointe déjà sur un commit et que tout
// est clean, c'est un no-op.
//
// On utilise user.email / user.name locaux pour ne pas dépendre de la
// config git globale du user (qui peut être absente ou volontairement
// différente).
func ensureInitialCommit(ctx context.Context, workdir string) error {
	// Stage tout le workdir.
	if err := runIn(ctx, workdir, "git", "add", "-A"); err != nil {
		return err
	}
	// Regarde s'il y a un HEAD valide.
	headCtx, headCancel := context.WithCancel(ctx)
	headCmd := exec.CommandContext(headCtx, "git", "-C", workdir, "rev-parse", "--verify", "HEAD")
	headErr := headCmd.Run()
	headCancel()
	hasHEAD := headErr == nil
	// Regarde s'il y a quelque chose à committer.
	diffCtx, diffCancel := context.WithCancel(ctx)
	diffCmd := exec.CommandContext(diffCtx, "git", "-C", workdir, "diff", "--cached", "--quiet")
	diffErr := diffCmd.Run()
	diffCancel()
	somethingStaged := diffErr != nil // --quiet + exit code != 0 = diff present

	if hasHEAD && !somethingStaged {
		return nil // rien à faire, HEAD pointe déjà vers un commit et tout est clean
	}
	// Commit. Utilise un config local pour ne pas polluer global.
	if err := runIn(ctx, workdir,
		"git", "-c", "user.email=bmad@hive.local", "-c", "user.name=Hive BMAD",
		"commit", "--allow-empty-message", "-m", "chore: initial Hive BMAD scaffold",
	); err != nil {
		// Si git refuse encore (ex. nothing to commit et hasHEAD false —
		// situation pathologique), remonte l'erreur claire.
		return fmt.Errorf("git commit: %w", err)
	}
	return nil
}

// commitsAheadOfOrigin retourne true si HEAD a des commits qui ne sont
// pas encore sur origin/<current branch>. Best-effort : si on n'a pas
// d'upstream configuré, on considère qu'il y a potentiellement du
// travail à push (plus safe que d'abandonner silencieusement).
func commitsAheadOfOrigin(ctx context.Context, workdir string) bool {
	c, cancel := context.WithCancel(ctx)
	defer cancel()
	// rev-list @{u}..HEAD count : nb de commits ahead de l'upstream.
	out, err := exec.CommandContext(c, "git", "-C", workdir,
		"rev-list", "--count", "@{u}..HEAD").Output()
	if err != nil {
		// Pas d'upstream : il y a (a priori) du travail non pushé.
		return true
	}
	return strings.TrimSpace(string(out)) != "0"
}

// currentBranch retourne la branche courante du repo, ou "" si on est
// en detached HEAD / pas un repo / git indisponible. Best-effort :
// erreur silencieuse, le caller décide quoi faire.
func currentBranch(ctx context.Context, workdir string) string {
	brCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	out, err := exec.CommandContext(brCtx, "git", "-C", workdir, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// branchSlug construit un nom de branche feature lisible à partir
// d'un titre de story. "Bootstrap the monorepo" → "bootstrap-the-monorepo".
// Fallback sur un suffixe horodaté si rien d'exploitable.
func branchSlug(title string) string {
	title = strings.ToLower(strings.TrimSpace(title))
	var b strings.Builder
	for _, r := range title {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ', r == '-', r == '_', r == '/':
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	// Collapse multiples "-" consécutifs en un seul.
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	if len(out) > 50 {
		out = strings.TrimRight(out[:50], "-")
	}
	if out == "" {
		return fmt.Sprintf("story-%d", os.Getpid())
	}
	return out
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

// HTTP helpers for the GitHub OAuth device flow. Small hand-rolled
// wrappers so we avoid pulling in an OAuth SDK just for two endpoints.

func httpEscapeScopes(s string) string {
	return url.QueryEscape(s)
}

func httpNewRequest(ctx context.Context, method, urlStr, body string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, urlStr, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req, nil
}

func httpDoJSON(req *http.Request, out any) error {
	// Pas de timeout client — le ctx fourni par le caller pilote l'arrêt.
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 && resp.StatusCode != http.StatusUnauthorized {
		return fmt.Errorf("http %d: %s", resp.StatusCode, truncate(string(data), 200))
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("parse json: %w (body=%s)", err, truncate(string(data), 200))
	}
	return nil
}
