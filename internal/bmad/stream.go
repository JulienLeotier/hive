package bmad

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// StreamEvent est un snapshot d'un événement claude CLI stream-json
// parsé et normalisé pour l'UI Hive. Le CLI émet NDJSON sur stdout
// avec des shapes différentes par type (system / assistant / user /
// result) ; on extrait juste ce dont l'opérateur a besoin à voir
// défiler dans la console : un "type" lisible, le texte ou le résumé
// d'outil, et le JSON brut pour les power users.
type StreamEvent struct {
	Type string // system | assistant | tool_use | tool_result | result | rate_limit | ...
	Text string // contenu lisible à afficher (extrait selon le type)
	Raw  []byte // ligne NDJSON brute (pour debug / affichage expert)
}

// InvokeStream runs a BMAD skill avec streaming. onEvent est appelé
// pour CHAQUE ligne NDJSON reçue du claude CLI au fur et à mesure.
// onEvent est OPTIONNEL — nil = équivalent à Invoke (on consomme tout
// et on renvoie à la fin).
//
// Claude CLI stream-json émet des events par TURN, pas par token :
// chaque message assistant complet arrive d'un coup, chaque résultat
// d'outil aussi. Pour une skill BMAD qui lit des fichiers + édite +
// commente, ça donne typiquement 5-20 events répartis sur la durée
// de la skill — suffisant pour que l'UI affiche un fil en live.
//
// Implementation notes :
//   - --verbose est requis par le CLI avec stream-json (sinon silence).
//   - On scan ligne par ligne avec un buffer XL (1MB → 10MB cap) :
//     certains assistant messages avec de gros diffs dépassent le
//     default bufio.Scanner de 64KB.
//   - L'event final "result" contient le grand total cost/usage ; on
//     l'extrait et on le renvoie dans Result. Les erreurs du CLI lui-
//     même (exit != 0) sont distinctes des erreurs logiques (is_error=true).
func (r *Runner) InvokeStream(
	ctx context.Context,
	workdir, goal string,
	expectedOutputs []string,
	onEvent func(StreamEvent),
) (Result, error) {
	if r == nil {
		return Result{}, errors.New("bmad: runner unavailable")
	}
	callCtx := ctx
	cancel := func() {}
	if r.timeout > 0 {
		callCtx, cancel = context.WithTimeout(ctx, r.timeout)
	}
	defer cancel()

	prompt := buildPrompt(goal)
	cmd := exec.CommandContext(callCtx, r.cliPath,
		"--print",
		"--output-format", "stream-json",
		"--verbose",
		"--dangerously-skip-permissions")
	configureProcessGroup(cmd)
	cmd.Dir = workdir
	cmd.Stdin = strings.NewReader(prompt)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Result{}, fmt.Errorf("claude stdout pipe: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return Result{}, fmt.Errorf("claude start: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	var accumulated strings.Builder
	var final struct {
		Result       string  `json:"result"`
		IsError      bool    `json:"is_error"`
		TotalCostUSD float64 `json:"total_cost_usd"`
		Usage        struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	sawResult := false

	for scanner.Scan() {
		line := scanner.Bytes()
		evt := parseStreamLine(line)
		if evt.Type == "result" {
			// Le CLI émet un event type=result à la toute fin.
			_ = json.Unmarshal(line, &final)
			sawResult = true
		}
		if evt.Text != "" {
			accumulated.WriteString(evt.Text)
			accumulated.WriteString("\n")
		}
		if onEvent != nil {
			// Copie la ligne pour que onEvent puisse la garder après le
			// prochain scan (scanner.Bytes() est réutilisé).
			raw := make([]byte, len(line))
			copy(raw, line)
			evt.Raw = raw
			onEvent(evt)
		}
	}
	if err := scanner.Err(); err != nil {
		_ = cmd.Wait()
		return Result{}, fmt.Errorf("stream scan: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		return Result{}, fmt.Errorf("claude invoke: %w\nstderr: %s",
			err, truncate(stderr.String(), 300))
	}

	// Si on a vu un event result, on l'utilise comme source de vérité
	// pour le texte final (c'est ce que json mode renvoyait avant).
	// Sinon on se replie sur l'accumulé des events assistant.
	finalText := accumulated.String()
	if sawResult && final.Result != "" {
		finalText = final.Result
	}
	if sawResult && final.IsError {
		return Result{
				Text:         finalText,
				InputTokens:  final.Usage.InputTokens,
				OutputTokens: final.Usage.OutputTokens,
				CostUSD:      final.TotalCostUSD,
			},
			fmt.Errorf("skill reported error: %s", truncate(finalText, 300))
	}

	var landed []string
	for _, rel := range expectedOutputs {
		abs := joinPath(workdir, rel)
		if fileExistsNonEmpty(abs) {
			landed = append(landed, abs)
		}
	}
	return Result{
		Text:         finalText,
		Outputs:      landed,
		InputTokens:  final.Usage.InputTokens,
		OutputTokens: final.Usage.OutputTokens,
		CostUSD:      final.TotalCostUSD,
	}, nil
}

// parseStreamLine extrait un StreamEvent lisible d'une ligne NDJSON
// du CLI claude stream-json. Tolère les shapes inconnues : retourne
// un event avec Type="other" et Text vide plutôt que de propager
// l'erreur — le consumer affiche le Raw s'il veut.
func parseStreamLine(line []byte) StreamEvent {
	var head struct {
		Type    string `json:"type"`
		Subtype string `json:"subtype"`
	}
	if err := json.Unmarshal(line, &head); err != nil {
		return StreamEvent{Type: "raw", Text: string(line)}
	}
	switch head.Type {
	case "system":
		// init + tool list : trop verbeux pour l'UI. On marque
		// simplement le démarrage de session. Le type "system" portera
		// le badge ; pas de texte additionnel nécessaire.
		return StreamEvent{Type: "system", Text: "session " + head.Subtype}
	case "assistant":
		// message.content[] contient soit des blocks text, soit des
		// tool_use. On les rassemble mais on ÉMET deux StreamEvent
		// distincts pour que l'UI puisse styler texte et tool_use
		// différemment. Le tool_use est parsé humainement (Read path
		// au lieu de raw JSON) par summariseToolUse.
		var env struct {
			Message struct {
				Content []struct {
					Type  string          `json:"type"`
					Text  string          `json:"text"`
					Name  string          `json:"name"`
					Input json.RawMessage `json:"input"`
				} `json:"content"`
			} `json:"message"`
		}
		_ = json.Unmarshal(line, &env)
		// On fusionne en un seul event pour préserver l'ordre temporel
		// (l'assistant fait parfois "text block → tool_use" dans le
		// même message). Une ligne par sous-block.
		var b strings.Builder
		for _, c := range env.Message.Content {
			switch c.Type {
			case "text":
				if c.Text != "" {
					if b.Len() > 0 {
						b.WriteString("\n")
					}
					b.WriteString(c.Text)
				}
			case "tool_use":
				if b.Len() > 0 {
					b.WriteString("\n")
				}
				b.WriteString("→ ")
				b.WriteString(summariseToolUse(c.Name, c.Input))
			}
		}
		return StreamEvent{Type: "assistant", Text: strings.TrimSpace(b.String())}
	case "user":
		// tool results — garde juste une première ligne lisible par
		// tool_result pour ne pas noyer la console.
		var env struct {
			Message struct {
				Content []struct {
					Type      string `json:"type"`
					ToolUseID string `json:"tool_use_id"`
					Content   any    `json:"content"`
					IsError   bool   `json:"is_error"`
				} `json:"content"`
			} `json:"message"`
		}
		_ = json.Unmarshal(line, &env)
		var b strings.Builder
		for _, c := range env.Message.Content {
			if c.Type != "tool_result" {
				continue
			}
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			preview := summariseToolResult(c.Content)
			if preview == "" {
				preview = "(ok)"
			}
			if c.IsError {
				b.WriteString("✗ ")
			} else {
				b.WriteString("← ")
			}
			b.WriteString(preview)
		}
		return StreamEvent{Type: "tool_result", Text: strings.TrimSpace(b.String())}
	case "result":
		var env struct {
			Result string `json:"result"`
		}
		_ = json.Unmarshal(line, &env)
		text := env.Result
		if text == "" {
			text = "skill terminée"
		}
		return StreamEvent{Type: "result", Text: text}
	default:
		return StreamEvent{Type: head.Type, Text: ""}
	}
}

// summariseToolResult ne garde que la PREMIÈRE ligne non vide du
// résultat, tronquée. Les tool results peuvent faire des milliers de
// lignes (cat d'un gros fichier, output d'un grep) — l'opérateur veut
// juste voir "file contents: ..." sans se taper tout le dump qui
// noie la console. S'il veut le détail complet, il regarde le step
// dans la console du terminal / les logs.
func summariseToolResult(content any) string {
	var text string
	switch v := content.(type) {
	case string:
		text = v
	case []any:
		var b strings.Builder
		for _, part := range v {
			m, ok := part.(map[string]any)
			if !ok {
				continue
			}
			if t, _ := m["type"].(string); t == "text" {
				if s, _ := m["text"].(string); s != "" {
					b.WriteString(s)
				}
			}
		}
		text = b.String()
	}
	// Première ligne significative, comptage de lignes pour
	// contextualiser.
	lines := strings.Split(text, "\n")
	nonEmpty := 0
	var first string
	for _, ln := range lines {
		trimmed := strings.TrimSpace(ln)
		if trimmed == "" {
			continue
		}
		if first == "" {
			first = trimmed
		}
		nonEmpty++
	}
	if first == "" {
		return ""
	}
	first = truncate(first, 140)
	if nonEmpty > 1 {
		return fmt.Sprintf("%s (+%d lignes)", first, nonEmpty-1)
	}
	return first
}

// summariseToolUse transforme un tool_use.input (JSON brut) en une
// ligne lisible humainement, en fonction du tool. "Read src/foo.ts"
// au lieu de "Read {\"file_path\":\"...\"}". Les tools inconnus
// retombent sur "Name <raw input>" tronqué.
func summariseToolUse(name string, rawInput json.RawMessage) string {
	if len(rawInput) == 0 {
		return name
	}
	var m map[string]any
	if err := json.Unmarshal(rawInput, &m); err != nil {
		return name
	}
	switch name {
	case "Read", "Edit", "Write", "NotebookEdit":
		if p, _ := m["file_path"].(string); p != "" {
			return name + " " + shortPath(p)
		}
	case "Bash":
		if cmd, _ := m["command"].(string); cmd != "" {
			cmd = firstLine(cmd)
			return "Bash $ " + truncate(cmd, 100)
		}
	case "Grep":
		if p, _ := m["pattern"].(string); p != "" {
			return "Grep " + truncate(p, 80)
		}
	case "Glob":
		if p, _ := m["pattern"].(string); p != "" {
			return "Glob " + truncate(p, 80)
		}
	case "Task", "Agent":
		if d, _ := m["description"].(string); d != "" {
			return name + ": " + truncate(d, 80)
		}
	case "WebFetch":
		if u, _ := m["url"].(string); u != "" {
			return "WebFetch " + truncate(u, 80)
		}
	case "Skill":
		if s, _ := m["skill"].(string); s != "" {
			return "Skill " + s
		}
	case "TodoWrite":
		return "TodoWrite"
	}
	// Fallback : le name seul, ou avec input compact si court.
	raw := string(rawInput)
	if len(raw) < 80 {
		return name + " " + raw
	}
	return name
}

// shortPath tronque un chemin absolu aux 2 derniers segments pour
// que la console reste lisible — "/Users/julien/.../skills/foo.md"
// au lieu de "/Users/julien/Documents/todolist/.claude/skills/foo.md".
func shortPath(p string) string {
	parts := strings.Split(p, "/")
	if len(parts) <= 3 {
		return p
	}
	return ".../" + strings.Join(parts[len(parts)-3:], "/")
}

func firstLine(s string) string {
	if i := strings.Index(s, "\n"); i >= 0 {
		return s[:i]
	}
	return s
}

func joinPath(dir, rel string) string {
	if filepath.IsAbs(rel) {
		return rel
	}
	return filepath.Join(dir, rel)
}

func fileExistsNonEmpty(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir() && info.Size() > 0
}
