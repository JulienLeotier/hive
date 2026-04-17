package bmad

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Dispatcher appelle un petit modèle (Claude Haiku) entre chaque
// slash-command BMAD. Il lit la réponse de la skill précédente et
// répond uniquement par la slash-command suivante à exécuter, ou par
// `done` quand la phase est terminée. C'est ce petit LLM
// d'aiguillage qui remplace une séquence hardcodée : la phase
// planning se déroule tant qu'il y a une skill suivante, de même
// pour la phase story.
//
// On n'ajoute pas de JSON contract ou de prompt engineering : la
// réponse attendue tient en une ligne.
const dispatcherModel = "claude-haiku-4-5-20251001"

// Phase décrit ce que le dispatcher doit accomplir. Le prompt liste
// les skills BMAD disponibles et le but final ; au dispatcher de
// sélectionner la prochaine étape.
type Phase struct {
	Name        string // "planning" | "story" | "autre"
	Goal        string // phrase courte : "produire PRD + epics + stories"
	MaxSteps    int    // garde-fou contre les boucles : 0 → 8
}

// NextStep rend la prochaine slash-command à exécuter (ex.
// "/bmad-create-prd") ou "" quand le dispatcher juge la phase
// terminée. `lastReply` est la réponse texte de la skill précédente
// (ou vide au premier appel).
func (r *Runner) NextStep(ctx context.Context, workdir string, phase Phase, lastCommand, lastReply string) (string, error) {
	if r == nil {
		return "", fmt.Errorf("bmad: dispatcher indisponible (claude CLI absent)")
	}
	callCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	prompt := dispatcherPrompt(phase, lastCommand, lastReply)
	cmd := exec.CommandContext(callCtx, r.cliPath,
		"--print", "--output-format", "json",
		"--model", dispatcherModel)
	cmd.Dir = workdir
	cmd.Stdin = strings.NewReader(prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("dispatcher invoke: %w (%s)", err, truncate(stderr.String(), 200))
	}
	var env struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		return "", fmt.Errorf("dispatcher parse envelope: %w", err)
	}
	reply := strings.TrimSpace(env.Result)
	// On tolère quelques variations : première ligne, majuscules,
	// backticks autour de la commande.
	if i := strings.IndexAny(reply, "\n"); i >= 0 {
		reply = strings.TrimSpace(reply[:i])
	}
	reply = strings.Trim(reply, "`\"' ")
	lower := strings.ToLower(reply)
	if lower == "done" || lower == "terminé" || lower == "fini" {
		return "", nil
	}
	if strings.HasPrefix(reply, "/") {
		return reply, nil
	}
	return "", fmt.Errorf("dispatcher n'a pas retourné de slash-command: %q", truncate(reply, 200))
}

func dispatcherPrompt(phase Phase, lastCommand, lastReply string) string {
	var b strings.Builder
	b.WriteString("Tu es l'aiguilleur du workflow BMAD. Tu lis la dernière sortie de skill ")
	b.WriteString("et tu choisis la prochaine slash-command à exécuter. Réponds par UNE SEULE ligne : ")
	b.WriteString("soit une slash-command (ex. /bmad-create-prd), soit le mot `done` si la phase est atteinte. ")
	b.WriteString("AUCUN commentaire, AUCUN JSON, AUCUNE explication.\n\n")
	fmt.Fprintf(&b, "Phase : %s\nBut : %s\n\n", phase.Name, phase.Goal)
	b.WriteString("Slash-commands BMAD disponibles :\n")
	b.WriteString("- /bmad-create-prd — rédige le PRD\n")
	b.WriteString("- /bmad-create-architecture — rédige l'architecture\n")
	b.WriteString("- /bmad-create-epics-and-stories — produit l'arbre epics + stories\n")
	b.WriteString("- /bmad-check-implementation-readiness — vérifie avant dev\n")
	b.WriteString("- /bmad-sprint-planning — planifie le sprint\n")
	b.WriteString("- /bmad-create-story — génère un fichier story prêt pour le dev\n")
	b.WriteString("- /bmad-dev-story — implémente la story courante\n")
	b.WriteString("- /bmad-code-review — relit la story codée\n")
	b.WriteString("- /bmad-correct-course — corrige de cap quand quelque chose bloque\n")
	b.WriteString("- /bmad-retrospective — rétrospective après un epic\n\n")
	if lastCommand != "" {
		fmt.Fprintf(&b, "Dernière commande exécutée : %s\n", lastCommand)
	} else {
		b.WriteString("Dernière commande exécutée : aucune (début de phase)\n")
	}
	if lastReply != "" {
		fmt.Fprintf(&b, "\nSortie de cette skill (tronquée à 4000 caractères) :\n%s\n",
			truncate(strings.TrimSpace(lastReply), 4000))
	}
	return b.String()
}
