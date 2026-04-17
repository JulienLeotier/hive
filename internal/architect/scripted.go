package architect

import (
	"context"
	"fmt"
	"strings"
)

// ScriptedAgent decomposes a PRD deterministically. It walks the BMAD
// rubric sections the ScriptedAgent in internal/intake produces, mapping
// each to an epic with a hand-rolled story template. The output is
// always valid (even on a thin PRD) so the autonomous build flow never
// dead-ends waiting for better decomposition.
type ScriptedAgent struct{}

// NewScripted returns a deterministic architect.
func NewScripted() *ScriptedAgent { return &ScriptedAgent{} }

// Name identifies this agent in log/event tags.
func (*ScriptedAgent) Name() string { return "scripted-architect" }

// rubricEpic binds a PRD section alias to the epic + story template it
// produces. Keep the alias list generous so hand-written PRDs (not from
// the scripted PM) still match — "Audience & problem", "Audience", or
// "Users" all work.
type rubricEpic struct {
	title       string
	aliases     []string
	description string
	stories     []scriptedStoryTemplate
}

type scriptedStoryTemplate struct {
	title, description string
	acs                []string
}

var rubricEpics = []rubricEpic{
	{
		title:   "Foundations",
		aliases: []string{"Tech Notes", "Tech notes", "Tech"},
		description: "Project scaffolding, CI baseline, and the top-level structure derived from the PRD's Tech Notes. " +
			"Gets built first because every other epic depends on it.",
		stories: []scriptedStoryTemplate{
			{
				title: "Scaffold the project",
				description: "Create the initial directory layout, commit a README, set up the chosen stack as declared in the PRD Tech Notes.",
				acs: []string{
					"A fresh clone of the repo runs a hello-world with the stack called out in the PRD",
					"A README documents how to install deps and run the dev loop",
					"git log shows an initial commit with a meaningful message",
				},
			},
			{
				title: "Wire CI basics",
				description: "Lint + test jobs that run on push. Just enough to catch regressions from this point forward.",
				acs: []string{
					"A workflow file (or equivalent) runs lint and tests on push",
					"A failing test fails CI",
					"README documents how to run the same checks locally",
				},
			},
		},
	},
	{
		title:   "Audience alignment",
		aliases: []string{"Audience & problem", "Audience", "Users"},
		description: "Features that directly serve the target user per the PRD Audience section.",
		stories: []scriptedStoryTemplate{
			{
				title: "Primary user landing",
				description: "The primary entry point the target audience sees on first use — no account required.",
				acs: []string{
					"Hitting the root route renders content directly relevant to the primary audience",
					"Copy matches the tone/problem described in the PRD's Audience section",
					"The landing page links to the first Core Flow",
				},
			},
		},
	},
	{
		title:   "Core user flows",
		aliases: []string{"Core flows", "Core Flows", "User flows"},
		description: "One story per numbered user flow in the PRD. These are the build's main value delivery.",
		stories: []scriptedStoryTemplate{
			{
				title: "Flow 1 — primary happy path",
				description: "Implement the first user flow end-to-end. Error cases can be polish in a later epic.",
				acs: []string{
					"A user can complete flow 1 from start to finish without seeing an error",
					"State persists across a page reload",
					"An integration test exercises the full flow",
				},
			},
			{
				title: "Flow 2 — secondary path",
				description: "The second user flow described in the PRD.",
				acs: []string{
					"A user can complete flow 2 from start to finish",
					"Flow 2 reuses shared components from flow 1 where sensible",
					"An integration test covers flow 2",
				},
			},
		},
	},
	{
		title:   "Constraints & hardening",
		aliases: []string{"Constraints & non-goals", "Non-Goals", "Non-goals", "Constraints"},
		description: "Explicit PRD non-goals treated as invariants the build must not violate.",
		stories: []scriptedStoryTemplate{
			{
				title: "Enforce declared non-goals",
				description: "Each non-goal in the PRD becomes a negative test or a runtime guard.",
				acs: []string{
					"Each non-goal from the PRD has at least one test that would fail if the non-goal were violated",
					"The README lists the non-goals next to the feature scope",
				},
			},
		},
	},
	{
		title:   "Definition of done",
		aliases: []string{"Definition of done", "Definition of Done", "Success"},
		description: "Everything required to call the build shippable per the PRD's DoD section.",
		stories: []scriptedStoryTemplate{
			{
				title: "Ship-ready packaging",
				description: "Binary/container/web bundle the user can actually run, matching the DoD artefact spec.",
				acs: []string{
					"A single command builds the artefact described in the PRD DoD",
					"The artefact runs on a fresh host without developer tools installed",
					"README includes a copy-paste install command",
				},
			},
			{
				title: "Metric instrumentation",
				description: "Log or count the metric the PRD defines as success.",
				acs: []string{
					"The success metric is computable from local logs or metrics",
					"An end-to-end test asserts the metric updates after a user action",
				},
			},
		},
	},
}

// Decompose walks the rubric epics. Each produces an epic iff the PRD
// has content for its section alias (so a PRD missing Non-Goals won't
// get a stub Constraints epic with fabricated ACs). Always returns at
// least the Foundations epic because every build needs scaffolding.
func (a *ScriptedAgent) Decompose(_ context.Context, projectIdea, prd string) ([]EpicDraft, error) {
	if strings.TrimSpace(prd) == "" {
		return nil, fmt.Errorf("PRD is empty — architect needs something to decompose")
	}
	var epics []EpicDraft
	for _, re := range rubricEpics {
		section := ExtractSection(prd, re.aliases...)
		// Foundations always ships; others need a non-empty PRD section.
		if section == "" && re.title != "Foundations" {
			continue
		}
		desc := re.description
		if section != "" {
			desc += "\n\nReferenced PRD section:\n" + section
		} else if projectIdea != "" {
			desc += "\n\nNo matching PRD section — falling back to the original idea:\n" + projectIdea
		}
		epic := EpicDraft{Title: re.title, Description: desc}
		for _, st := range re.stories {
			epic.Stories = append(epic.Stories, StoryDraft{
				Title:              st.title,
				Description:        st.description,
				AcceptanceCriteria: append([]string(nil), st.acs...),
			})
		}
		epics = append(epics, epic)
	}
	return epics, nil
}

// Compile-time Agent interface check.
var _ Agent = (*ScriptedAgent)(nil)
