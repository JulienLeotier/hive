package bmad

// Skill décrit un skill BMAD invocable manuellement via l'UI Hive.
// Distinct des séquences (AnalysisSequence, StorySequence...) : ces
// constantes sont pour l'automation ; la Registry ci-dessous expose
// chaque skill individuellement pour que l'opérateur puisse firer
// "/bmad-correct-course sur story 0.3" ou "/bmad-validate-prd" sans
// passer par le devloop.
type Skill struct {
	Command     string      `json:"command"`     // "/bmad-code-review"
	Name        string      `json:"name"`        // "Code Review"
	Description string      `json:"description"` // une phrase
	Scope       SkillScope  `json:"scope"`       // project|epic|story
	Phase       string      `json:"phase"`       // tag pour bmad_phase_steps.phase
	Dangerous   bool        `json:"dangerous"`   // exige confirmation UI
}

// SkillScope restreint les contextes où un skill peut être lancé.
// Un skill story-scoped ne doit pas apparaître dans le menu d'un
// projet en status=planning.
type SkillScope string

const (
	ScopeProject SkillScope = "project"
	ScopeEpic    SkillScope = "epic"
	ScopeStory   SkillScope = "story"
)

// SkillRegistry énumère tous les skills BMAD que l'UI expose à
// l'opérateur. L'ordre est celui du menu — du plus courant au plus
// rare. Les skills "dangerous" (retrospective, correct-course en
// mid-dev) demandent une confirmation UI avant de tourner.
//
// La liste suit les phases officielles BMAD-METHOD : Analyse, Planning,
// Solutioning, Implementation, Iteration. On n'expose PAS les skills
// de pure activation d'agent (/bmad-agent-pm, /bmad-agent-architect)
// individuellement — elles sont consommées automatiquement par les
// skills de production (create-prd, create-architecture, etc.).
var SkillRegistry = []Skill{
	// Planning — niveau projet.
	{
		Command: "/bmad-create-prd", Name: "Create PRD",
		Description: "Régénère le PRD depuis le product brief.",
		Scope:       ScopeProject, Phase: "planning",
	},
	{
		Command: "/bmad-validate-prd", Name: "Validate PRD",
		Description: "Audite le PRD contre les checklists BMAD.",
		Scope:       ScopeProject, Phase: "planning",
	},
	{
		Command: "/bmad-create-ux-design", Name: "UX Design",
		Description: "Produit la spec UX (wireframes, flows).",
		Scope:       ScopeProject, Phase: "planning",
	},
	{
		Command: "/bmad-edit-prd", Name: "Edit PRD",
		Description: "Étend le PRD existant (mode itération brownfield).",
		Scope:       ScopeProject, Phase: "iteration",
	},

	// Solutioning — niveau projet.
	{
		Command: "/bmad-create-architecture", Name: "Create Architecture",
		Description: "Rédige l'architecture technique.",
		Scope:       ScopeProject, Phase: "solutioning",
	},
	{
		Command: "/bmad-create-epics-and-stories", Name: "Create Epics & Stories",
		Description: "Décompose le PRD en epics + stories.",
		Scope:       ScopeProject, Phase: "solutioning",
	},
	{
		Command: "/bmad-check-implementation-readiness", Name: "Check Readiness",
		Description: "Audit solutioning avant Phase 4.",
		Scope:       ScopeProject, Phase: "solutioning",
	},
	{
		Command: "/bmad-document-project", Name: "Document Project",
		Description: "Scan un repo existant (brownfield).",
		Scope:       ScopeProject, Phase: "iteration",
	},
	{
		Command: "/bmad-generate-project-context", Name: "Generate Context",
		Description: "Construit le contexte projet pour BMAD.",
		Scope:       ScopeProject, Phase: "iteration",
	},

	// Implementation — niveau projet (global) ou story.
	{
		Command: "/bmad-sprint-planning", Name: "Sprint Planning",
		Description: "Recompose sprint-status.yaml.",
		Scope:       ScopeProject, Phase: "implementation",
	},

	// Story-level.
	{
		Command: "/bmad-create-story", Name: "Create Story",
		Description: "Génère une story file à partir des ACs.",
		Scope:       ScopeStory, Phase: "story",
	},
	{
		Command: "/bmad-dev-story", Name: "Dev Story",
		Description: "Fait coder la story par le Dev agent.",
		Scope:       ScopeStory, Phase: "story",
	},
	{
		Command: "/bmad-code-review", Name: "Code Review",
		Description: "Revoit le code produit par dev-story.",
		Scope:       ScopeStory, Phase: "review",
	},
	{
		Command: "/bmad-qa-generate-e2e-tests", Name: "QA Tests",
		Description: "Génère des tests e2e pour la story.",
		Scope:       ScopeStory, Phase: "story",
	},
	{
		Command: "/bmad-correct-course", Name: "Correct Course",
		Description: "Tranche un finding decision-needed.",
		Scope:       ScopeStory, Phase: "architect", Dangerous: true,
	},

	// Epic-level.
	{
		Command: "/bmad-retrospective", Name: "Retrospective",
		Description: "Rétrospective de fin d'epic.",
		Scope:       ScopeEpic, Phase: "retrospective", Dangerous: true,
	},
}

// FindSkill retourne un skill par sa commande, ou nil si absent.
// Utilisé par l'API pour valider qu'une commande demandée fait bien
// partie du registre exposé (refus des injections arbitraires).
func FindSkill(command string) *Skill {
	for i := range SkillRegistry {
		if SkillRegistry[i].Command == command {
			return &SkillRegistry[i]
		}
	}
	return nil
}
