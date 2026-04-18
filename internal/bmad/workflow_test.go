package bmad

import (
	"testing"
)

// Tests unitaires pour les helpers purs du workflow BMAD : parser de
// sprint-status.yaml, extraction de PR URL, lecture de story file.
// Les parties qui appellent `claude --print` / `gh` sont testées via
// scripts/claude-e2e.sh (build tag claude_e2e).

func TestPlanningSequenceNonEmpty(t *testing.T) {
	if len(PlanningSequence) == 0 {
		t.Fatal("PlanningSequence vide — la doc BMAD attend les 5 étapes de Phase 2")
	}
	for _, cmd := range PlanningSequence {
		if len(cmd) < 6 || cmd[:6] != "/bmad-" {
			t.Errorf("commande malformée : %q", cmd)
		}
	}
}

func TestFullPlanningPipelineConcatOrdered(t *testing.T) {
	// Le pipeline démarre désormais directement sur /bmad-agent-pm.
	// AnalysisSequence est vide par design : Hive pré-écrit lui-même le
	// Product Brief via son PM agent d'intake pour éviter que l'Analyst
	// BMAD élargisse systématiquement la portée.
	if FullPlanningPipeline[0] != "/bmad-agent-pm" {
		t.Errorf("FullPlanningPipeline doit commencer par /bmad-agent-pm, pas %q", FullPlanningPipeline[0])
	}
	// Vérifie que /bmad-product-brief n'est PLUS dans le pipeline —
	// sinon la régression "analyst réinvente la portée" revient.
	for _, cmd := range FullPlanningPipeline {
		if cmd == "/bmad-product-brief" || cmd == "/bmad-agent-analyst" {
			t.Errorf("FullPlanningPipeline ne doit plus contenir %q — Hive pré-écrit le brief via son PM", cmd)
		}
	}
	// sprint-planning en dernier = init Phase 4.
	last := FullPlanningPipeline[len(FullPlanningPipeline)-1]
	if last != "/bmad-sprint-planning" {
		t.Errorf("FullPlanningPipeline doit finir par /bmad-sprint-planning, pas %q", last)
	}
}

func TestIterationPipelineStartsWithDocumentProject(t *testing.T) {
	if IterationPipeline[0] != "/bmad-document-project" {
		t.Errorf("IterationPipeline doit commencer par /bmad-document-project, pas %q", IterationPipeline[0])
	}
}

func TestExtractPRURL(t *testing.T) {
	cases := map[string]string{
		"ouverte : https://github.com/me/repo/pull/42\nend":                        "https://github.com/me/repo/pull/42",
		"PR: https://github.com/org/proj/pull/7 merged":                            "https://github.com/org/proj/pull/7",
		"aucune URL ici":                                                           "",
		"plusieurs : https://github.com/x/y/pull/1 puis https://github.com/x/y/pull/2": "https://github.com/x/y/pull/1",
	}
	for in, want := range cases {
		got := ExtractPRURL(in)
		if got != want {
			t.Errorf("ExtractPRURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStoryStatus(t *testing.T) {
	s := &SprintStatus{DevelopmentStatus: map[string]string{
		"1.1": "ready-for-done",
		"1.2": "ready-for-dev",
	}}
	if s.StoryStatus("1.1") != "ready-for-done" {
		t.Error("1.1 devrait être ready-for-done")
	}
	if s.StoryStatus("missing") != "" {
		t.Error("clé inconnue devrait renvoyer vide")
	}
	var nilStatus *SprintStatus
	if nilStatus.StoryStatus("whatever") != "" {
		t.Error("nil receiver devrait être safe et renvoyer vide")
	}
}
