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
// 1 à 4). Chaque slash-command est exécutée dans sa propre invocation
// `claude --print` — la doc BMAD indique explicitement que chaque
// skill doit tourner dans un chat neuf (« fresh chat each »).

// AnalysisSequence (Phase 1) — vide par design. Avant, on lançait
// /bmad-agent-analyst + /bmad-product-brief pour générer un product
// brief à partir de l'idée + l'intake chat. Mais l'agent Analyst
// élargissait systématiquement la portée ("todolist basique" →
// concurrent de Notion avec IA de triage) parce que son rôle
// intrinsèque est de prospecter marché et positionnement.
//
// Maintenant, Hive pré-écrit lui-même product-brief-<slug>.md via le
// PM agent du chat d'intake (voir intake/claude_code.go FinalPRD qui
// produit un brief SCOPE LOCKED). BMAD démarre donc directement à
// /bmad-agent-pm + /bmad-create-prd, qui lit le brief pré-écrit et
// s'y tient.
var AnalysisSequence = []string{}

// PlanningSequence (Phase 2) — PM prend la main, rédige le PRD,
// valide, puis passe le relais au designer UX (bmad considère l'UX
// comme systématique ; la skill elle-même saute le travail quand le
// projet est sans UI).
var PlanningSequence = []string{
	"/bmad-agent-pm",
	"/bmad-create-prd",
	"/bmad-validate-prd",
	"/bmad-agent-ux-designer",
	"/bmad-create-ux-design",
}

// SolutioningSequence (Phase 3) — Architecte rédige l'architecture,
// PM décompose en epics et stories, Architecte valide la readiness.
// Suit exactement la doc BMAD : « bmad-agent-architect +
// bmad-create-architecture ; bmad-agent-pm +
// bmad-create-epics-and-stories ; bmad-agent-architect +
// bmad-check-implementation-readiness ».
var SolutioningSequence = []string{
	"/bmad-agent-architect",
	"/bmad-create-architecture",
	"/bmad-agent-pm",
	"/bmad-create-epics-and-stories",
	"/bmad-agent-architect",
	"/bmad-check-implementation-readiness",
}

// ImplementationInitSequence (Phase 4 init) — le Dev prend la main
// et initialise sprint-status.yaml via bmad-sprint-planning.
var ImplementationInitSequence = []string{
	"/bmad-agent-dev",
	"/bmad-sprint-planning",
}

// FullPlanningPipeline concatène Analysis + Planning + Solutioning +
// ImplementationInit. C'est ce que Hive lance à partir du finalize
// intake, une seule fois par projet.
var FullPlanningPipeline = concat(
	AnalysisSequence,
	PlanningSequence,
	SolutioningSequence,
	ImplementationInitSequence,
)

// StorySequence est le cycle de build per-story de la Phase 4 BMAD.
// Doc BMAD : « Build cycle (per story, fresh chat each): create-story,
// dev-story, code-review ». On scinde code-review parce que Hive le
// traite comme un agent séparé (ReviewerAgent), et on ajoute
// bmad-qa-generate-e2e-tests au bout du cycle dev pour que la story
// livre ses tests e2e (BMAD le fait spontanément sur certains
// projets).
var StorySequence = []string{
	"/bmad-create-story",
	"/bmad-dev-story",
	"/bmad-qa-generate-e2e-tests",
}

// ReviewSequence : juste l'étape de revue, tenue à part parce que
// Hive l'invoque depuis son ReviewerAgent pour logger dev vs review
// distinctement dans le tableau de bord.
var ReviewSequence = []string{
	"/bmad-code-review",
}

// RetrospectiveSequence s'exécute à la fin de chaque epic per les
// docs BMAD (« bmad-agent-dev + bmad-retrospective »).
var RetrospectiveSequence = []string{
	"/bmad-agent-dev",
	"/bmad-retrospective",
}

// IterationPipeline est la séquence BMAD brownfield : ajouter une
// nouvelle feature / itération à un projet DÉJÀ livré. Au lieu de
// /bmad-create-prd qui part de zéro, on utilise /bmad-edit-prd qui
// étend le PRD existant. Même logique pour l'architecture : on
// repasse /bmad-create-architecture en mode « amend » (BMAD détecte
// la présence du fichier et met à jour plutôt que de ré-écrire).
//
// Doc BMAD brownfield : Phase d'analyse remplacée par
// bmad-document-project (récolte le contexte du repo existant),
// puis le même parcours PM → Architect → Dev que Phase 2+3+4, mais
// avec edit-prd à la place de create-prd.
var IterationPipeline = concat(
	// Analyse brownfield : documenter le projet existant.
	[]string{
		"/bmad-document-project",
		"/bmad-generate-project-context",
	},
	// Planning update.
	[]string{
		"/bmad-agent-pm",
		"/bmad-edit-prd",
		"/bmad-validate-prd",
		"/bmad-agent-ux-designer",
		"/bmad-create-ux-design",
	},
	// Solutioning update — Architecture amend + ajout d'epics/stories
	// pour la nouvelle feature.
	[]string{
		"/bmad-agent-architect",
		"/bmad-create-architecture",
		"/bmad-agent-pm",
		"/bmad-create-epics-and-stories",
		"/bmad-agent-architect",
		"/bmad-check-implementation-readiness",
	},
	// Re-sprint : sprint-planning recompose sprint-status.yaml avec
	// les stories existantes (done) + les nouvelles (ready-for-dev).
	[]string{
		"/bmad-agent-dev",
		"/bmad-sprint-planning",
	},
)

// CorrectCourseSequence reste à disposition : quand le loop
// dev/review se coince (max iterations atteint, review refuse de
// valider), on peut lancer /bmad-correct-course pour que BMAD re-cadre
// la story. Laissé disponible aux callers qui voudraient la câbler
// dans leur gestion d'erreur — non appelée automatiquement pour
// l'instant.
var CorrectCourseSequence = []string{
	"/bmad-correct-course",
}

// ArchitectEscalationSequence est invoquée quand un code-review BMAD
// a tagged des findings "decision-needed" et que le devloop Hive veut
// trancher de manière autonome plutôt que de bloquer la story. On
// réveille l'agent Architect puis on ouvre /bmad-correct-course pour
// qu'il committe sa décision dans la story.md. Le Dev reprend ensuite
// au prochain tick avec la nouvelle spec.
var ArchitectEscalationSequence = []string{
	"/bmad-agent-architect",
	"/bmad-correct-course",
}

func concat(slices ...[]string) []string {
	total := 0
	for _, s := range slices {
		total += len(s)
	}
	out := make([]string, 0, total)
	for _, s := range slices {
		out = append(out, s...)
	}
	return out
}

// StepObserver est notifié à l'entrée et à la sortie de chaque
// invocation de skill pendant RunSequence. Le supervisor s'en sert
// pour persister la progression + émettre des événements WebSocket
// "skill X/Y started", "skill done", etc. Les deux callbacks sont
// optionnels.
type StepObserver struct {
	// OnStart reçoit l'index (1-based) et la commande qui va démarrer.
	OnStart func(index, total int, command string)
	// OnFinish reçoit l'index, la commande, le résultat et l'erreur
	// éventuelle. Sert à persister tokens/cost/status=done|failed.
	OnFinish func(index, total int, command string, res Result, err error)
}

// RunSequence exécute une liste de slash-commands dans l'ordre, une
// invocation `claude --print` par commande. Retourne l'historique —
// useful pour log + événements dashboard. Stoppe à la première
// erreur et renvoie ce qui a été fait jusque-là.
func (r *Runner) RunSequence(ctx context.Context, workdir string, cmds []string) ([]PhaseStep, error) {
	return r.RunSequenceObserved(ctx, workdir, cmds, StepObserver{})
}

// RunSequenceObserved est le même flux, instrumenté via un
// StepObserver. Les observers nil sont tolérés : RunSequence est une
// shorthand qui passe un StepObserver vide.
func (r *Runner) RunSequenceObserved(ctx context.Context, workdir string, cmds []string, obs StepObserver) ([]PhaseStep, error) {
	if r == nil {
		return nil, errors.New("bmad: runner indisponible")
	}
	var history []PhaseStep
	total := len(cmds)
	for i, cmd := range cmds {
		if obs.OnStart != nil {
			obs.OnStart(i+1, total, cmd)
		}
		res, err := r.Invoke(ctx, workdir, cmd, nil)
		history = append(history, PhaseStep{Command: cmd, Reply: res.Text})
		if obs.OnFinish != nil {
			obs.OnFinish(i+1, total, cmd, res, err)
		}
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

