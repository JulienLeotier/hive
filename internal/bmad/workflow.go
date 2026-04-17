package bmad

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Les séquences ci-dessous reproduisent scrupuleusement le workflow
// officiel BMAD-METHOD (github.com/bmad-code-org/BMAD-METHOD, Phases
// 2 + 3 + 4). Chaque slash-command est exécutée dans sa propre
// invocation `claude --print` — BMAD indique explicitement que chaque
// skill doit tourner dans un chat neuf.

// PlanningSequence enchaîne les phases Planning + Solutioning + l'init
// d'Implementation (sprint-planning) :
//
//	 1. bmad-agent-pm             (active le PM)
//	 2. bmad-create-prd           (PRD)
//	 3. bmad-agent-architect      (active l'Architecte)
//	 4. bmad-create-architecture  (Architecture)
//	 5. bmad-agent-pm             (repasse PM)
//	 6. bmad-create-epics-and-stories
//	 7. bmad-agent-architect      (repasse Architecte)
//	 8. bmad-check-implementation-readiness
//	 9. bmad-agent-dev            (active le Dev)
//	10. bmad-sprint-planning      (initialise sprint-status.yaml)
var PlanningSequence = []string{
	"/bmad-agent-pm",
	"/bmad-create-prd",
	"/bmad-agent-architect",
	"/bmad-create-architecture",
	"/bmad-agent-pm",
	"/bmad-create-epics-and-stories",
	"/bmad-agent-architect",
	"/bmad-check-implementation-readiness",
	"/bmad-agent-dev",
	"/bmad-sprint-planning",
}

// StorySequence est le cycle de build per-story de la Phase 4 BMAD.
// Doc BMAD : "Build cycle (per story, fresh chat each): create-story,
// dev-story, code-review". On scinde code-review parce que Hive le
// traite comme un agent séparé (ReviewerAgent).
var StorySequence = []string{
	"/bmad-create-story",
	"/bmad-dev-story",
}

// ReviewSequence : juste l'étape de revue, tenue à part parce que
// Hive l'invoque depuis son ReviewerAgent pour logger dev vs review
// distinctement dans le tableau de bord.
var ReviewSequence = []string{
	"/bmad-code-review",
}

// RetrospectiveSequence s'exécute à la fin de chaque epic per les
// docs BMAD ("bmad-agent-dev + bmad-retrospective").
var RetrospectiveSequence = []string{
	"/bmad-agent-dev",
	"/bmad-retrospective",
}

// RunSequence exécute une liste de slash-commands dans l'ordre, une
// invocation `claude --print` par commande. Retourne l'historique —
// useful pour log + événements dashboard. Stoppe à la première
// erreur et renvoie ce qui a été fait jusque-là.
func (r *Runner) RunSequence(ctx context.Context, workdir string, cmds []string) ([]PhaseStep, error) {
	if r == nil {
		return nil, errors.New("bmad: runner indisponible")
	}
	var history []PhaseStep
	for _, cmd := range cmds {
		res, err := r.Invoke(ctx, workdir, cmd, nil)
		history = append(history, PhaseStep{Command: cmd, Reply: res.Text})
		if err != nil {
			return history, fmt.Errorf("exec %s: %w", cmd, err)
		}
	}
	return history, nil
}

// --- Lecture fiable des artefacts BMAD (remplace les regex
// approximatifs qu'on utilisait avant). ---

// SprintStatus représente le fichier sprint-status.yaml que
// bmad-sprint-planning génère. On se limite aux champs que Hive
// exploite ; BMAD en ajoute d'autres qu'on ignore sans erreur.
type SprintStatus struct {
	DevelopmentStatus map[string]string `yaml:"development_status"`
	LastUpdated       string            `yaml:"last_updated"`
}

// ReadSprintStatus lit le sprint-status.yaml canonique sous
// implementation-artifacts. Retourne (nil, nil) quand il n'existe
// pas encore — c'est normal avant que sprint-planning ait tourné.
func ReadSprintStatus(workdir string) (*SprintStatus, error) {
	path := filepath.Join(workdir, "_bmad-output", "implementation-artifacts", "sprint-status.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var s SprintStatus
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse sprint-status.yaml: %w", err)
	}
	return &s, nil
}

// StoryStatus retourne le statut BMAD d'une story (ex. ready-for-dev,
// in-progress, review, ready-for-done, done). Retourne "" si la clé
// n'existe pas.
func (s *SprintStatus) StoryStatus(key string) string {
	if s == nil || s.DevelopmentStatus == nil {
		return ""
	}
	return s.DevelopmentStatus[key]
}

// StoryFile représente les métadonnées qu'on extrait d'un fichier
// story BMAD (implementation-artifacts/{story_key}.md). BMAD écrit
// une front-matter YAML avec les champs utiles — notamment pr_url
// quand la skill dev-story a poussé une PR.
type StoryFile struct {
	StoryKey string `yaml:"story_key"`
	Branch   string `yaml:"branch"`
	PRURL    string `yaml:"pr_url"`
	Status   string `yaml:"status"`
}

// ReadStoryFile parse la front-matter YAML du fichier story BMAD
// correspondant à storyKey. Retourne (nil, nil) quand le fichier
// n'existe pas.
func ReadStoryFile(workdir, storyKey string) (*StoryFile, error) {
	path := filepath.Join(workdir, "_bmad-output", "implementation-artifacts", storyKey+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	// Front-matter YAML entre deux `---`.
	s := string(data)
	if !strings.HasPrefix(s, "---\n") {
		return &StoryFile{}, nil
	}
	rest := s[4:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return &StoryFile{}, nil
	}
	var sf StoryFile
	if err := yaml.Unmarshal([]byte(rest[:end]), &sf); err != nil {
		// Pas fatal — le fichier peut ne pas avoir de front-matter
		// parseable selon la skill qui l'a écrit.
		return &StoryFile{}, nil //nolint:nilerr // best-effort
	}
	return &sf, nil
}

// --- Extraction de secours pour la PR URL : regex sur stdout. ---

// ExtractPRURL tente d'extraire une URL de PR GitHub du texte
// rendu par bmad-dev-story quand BMAD ne l'a pas écrite dans la
// story file. Renvoie "" si aucune URL github pull détectée.
func ExtractPRURL(reply string) string {
	for _, line := range strings.Split(reply, "\n") {
		line = strings.TrimSpace(line)
		for _, token := range strings.Fields(line) {
			token = strings.Trim(token, "`\"'(),.;:")
			if strings.HasPrefix(token, "https://github.com/") && strings.Contains(token, "/pull/") {
				return token
			}
		}
	}
	return ""
}

