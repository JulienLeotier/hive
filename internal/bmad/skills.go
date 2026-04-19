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
		Description: "Régénère le PRD depuis le product brief. À lancer quand le PRD actuel est vide, obsolète ou a dérivé du scope utilisateur.",
		Scope:       ScopeProject, Phase: "planning",
	},
	{
		Command: "/bmad-validate-prd", Name: "Validate PRD",
		Description: "Audite le PRD contre les checklists BMAD (ambiguïté, scope, testabilité). Utile après une Edit PRD ou pour checker la cohérence avant solutioning.",
		Scope:       ScopeProject, Phase: "planning",
	},
	{
		Command: "/bmad-create-ux-design", Name: "UX Design",
		Description: "Produit la spec UX (wireframes texte, flows utilisateur). À skip pour les outils CLI / API sans UI.",
		Scope:       ScopeProject, Phase: "planning",
	},
	{
		Command: "/bmad-edit-prd", Name: "Edit PRD",
		Description: "Étend le PRD existant sans tout réécrire. À utiliser pour une itération brownfield ou pour ajouter une feature à un projet shipped.",
		Scope:       ScopeProject, Phase: "iteration",
	},

	// Solutioning — niveau projet.
	{
		Command: "/bmad-create-architecture", Name: "Create Architecture",
		Description: "Rédige l'architecture technique (stack, modules, contrats). Doit passer après PRD validé. Si l'arch existe, l'amende plutôt que la recréer.",
		Scope:       ScopeProject, Phase: "solutioning",
	},
	{
		Command: "/bmad-create-epics-and-stories", Name: "Create Epics & Stories",
		Description: "Décompose le PRD + architecture en epics → stories → ACs. À lancer après Create Architecture, ou après une Edit PRD pour ajouter les nouvelles stories.",
		Scope:       ScopeProject, Phase: "solutioning",
	},
	{
		Command: "/bmad-check-implementation-readiness", Name: "Check Readiness",
		Description: "Audit final avant Phase 4 (dev) — vérifie que chaque story a ACs + contexte tech. Dernier garde-fou avant de brûler des tokens en dev-story.",
		Scope:       ScopeProject, Phase: "solutioning",
	},
	{
		Command: "/bmad-document-project", Name: "Document Project",
		Description: "Scan un repo existant (brownfield) pour extraire stack + conventions. À lancer UNE fois au début d'une itération brownfield.",
		Scope:       ScopeProject, Phase: "iteration",
	},
	{
		Command: "/bmad-generate-project-context", Name: "Generate Context",
		Description: "Construit le fichier de contexte projet que BMAD passe aux autres skills. Normalement auto-déclenché après document-project.",
		Scope:       ScopeProject, Phase: "iteration",
	},

	// Implementation — niveau projet (global) ou story.
	{
		Command: "/bmad-sprint-planning", Name: "Sprint Planning",
		Description: "Recompose sprint-status.yaml à partir de l'arbre epics/stories. À relancer si Hive et BMAD ont divergé sur les statuts de story.",
		Scope:       ScopeProject, Phase: "implementation",
	},

	// Story-level.
	{
		Command: "/bmad-create-story", Name: "Create Story",
		Description: "Génère le fichier story (ACs détaillés, notes dev, technical context) avant que /bmad-dev-story ne le code. Run automatiquement par le devloop.",
		Scope:       ScopeStory, Phase: "story",
	},
	{
		Command: "/bmad-dev-story", Name: "Dev Story",
		Description: "Fait coder la story par le Dev agent (écrit le code, les tests, commite). Le devloop l'invoque automatiquement ; rarement utile en manuel.",
		Scope:       ScopeStory, Phase: "story",
	},
	{
		Command: "/bmad-code-review", Name: "Code Review",
		Description: "Revoit le code produit par dev-story contre chaque AC. À relancer manuellement si tu veux un 2e avis ou si le reviewer a dérivé.",
		Scope:       ScopeStory, Phase: "review",
	},
	{
		Command: "/bmad-qa-generate-e2e-tests", Name: "QA Tests",
		Description: "Génère des tests e2e Playwright pour la story. Utile quand BMAD n'en a pas écrit spontanément et que tu veux couvrir un happy path.",
		Scope:       ScopeStory, Phase: "story",
	},
	{
		Command: "/bmad-correct-course", Name: "Correct Course",
		Description: "Fait trancher par l'Architect un finding decision-needed bloquant. Modifie la story.md, ne touche PAS au code. À utiliser quand le reviewer pose une vraie question d'architecture.",
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
