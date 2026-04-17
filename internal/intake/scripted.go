package intake

import (
	"context"
	"fmt"
	"strings"
)

// ScriptedAgent is a deterministic PM. It asks a fixed rubric of five
// questions (audience, flows, constraints, tech, budget) before declaring
// done. The PRD it produces is a structured markdown doc that restates
// the idea then interleaves each question with the user's answer.
//
// Why ship it: BMAD's "zero human" contract means a build must not
// dead-end because the Claude CLI is missing, rate-limited, or offline.
// ScriptedAgent is the safety net + the CI-friendly behaviour. When the
// real ClaudeCodeAgent works, it's used; otherwise this kicks in.
type ScriptedAgent struct{}

// NewScriptedAgent returns a fresh deterministic PM agent.
func NewScriptedAgent() *ScriptedAgent { return &ScriptedAgent{} }

// Role identifies this agent's conversation slot.
func (*ScriptedAgent) Role() string { return RolePM }

// rubric is the question list the PM walks through in order. Each slot
// teases out one dimension of scope the Architect will need.
var rubric = []string{
	"Who is this built for? (primary users, context they use the product in, pain point it addresses)",
	"Walk me through the 2–4 core user flows. For each: who starts it, what they do, what they get.",
	"What must it NOT do? (non-goals, constraints — data sensitivity, offline, accessibility, regulatory)",
	"Tech preferences or fixed points? (existing stack, hosting environment, auth provider, language/framework constraints)",
	"What does done look like? (first-release scope, success metric, any deadline to be aware of)",
}

// Greeting opens the conversation by restating the idea and asking the
// first rubric question.
func (*ScriptedAgent) Greeting(_ context.Context, projectIdea string) string {
	return fmt.Sprintf(
		"Hi — I'm the PM agent. You described the idea as:\n\n> %s\n\n"+
			"I'll ask a short series of questions so the rest of the BMAD fleet has a crisp PRD to work from. %s",
		strings.TrimSpace(projectIdea), rubric[0],
	)
}

// Reply picks the next rubric question based on how many user replies the
// conversation has received. When the rubric is exhausted it signals
// done=true.
func (*ScriptedAgent) Reply(
	_ context.Context,
	_ string,
	history []Message,
) (string, bool, error) {
	userAnswers := 0
	for _, m := range history {
		if m.Author == AuthorUser {
			userAnswers++
		}
	}
	// userAnswers is the index of the NEXT rubric question (0-based),
	// because the greeting already posted rubric[0] and the first user
	// reply is the answer to it.
	if userAnswers >= len(rubric) {
		return "Thanks — I have enough to write the PRD. When you're ready, click **Finalize PRD** to kick off the Architect phase. If anything else comes to mind before then, add it and I'll fold it in.",
			true, nil
	}
	return fmt.Sprintf("Got it. %s", rubric[userAnswers]), false, nil
}

// FinalPRD stitches the conversation into a markdown PRD. Each rubric
// answer becomes a section; the opening idea is kept as the Summary.
func (*ScriptedAgent) FinalPRD(
	_ context.Context,
	projectIdea string,
	history []Message,
) (string, error) {
	// Pair each rubric question with the user's answer in order.
	type pair struct{ question, answer string }
	var pairs []pair
	qIndex := 0
	lastQuestion := ""
	for _, m := range history {
		switch m.Author {
		case RolePM:
			if qIndex < len(rubric) {
				lastQuestion = rubric[qIndex]
				qIndex++
			}
		case AuthorUser:
			pairs = append(pairs, pair{question: lastQuestion, answer: m.Content})
			lastQuestion = ""
		}
	}

	var b strings.Builder
	b.WriteString("# Product Requirements Document\n\n")
	b.WriteString("## Summary\n\n")
	b.WriteString(strings.TrimSpace(projectIdea))
	b.WriteString("\n\n")

	sectionTitles := []string{
		"Audience & problem",
		"Core flows",
		"Constraints & non-goals",
		"Tech notes",
		"Definition of done",
	}
	for i, p := range pairs {
		title := fmt.Sprintf("Section %d", i+1)
		if i < len(sectionTitles) {
			title = sectionTitles[i]
		}
		fmt.Fprintf(&b, "## %s\n\n", title)
		if p.question != "" {
			fmt.Fprintf(&b, "_Prompt:_ %s\n\n", p.question)
		}
		fmt.Fprintf(&b, "%s\n\n", strings.TrimSpace(p.answer))
	}
	b.WriteString("---\n\n")
	b.WriteString("_PRD drafted by the scripted PM agent. Edit in place on the project page before the Architect runs, or click Re-intake to walk the rubric again._\n")
	return b.String(), nil
}

// Compile-time Agent interface check.
var _ Agent = (*ScriptedAgent)(nil)
