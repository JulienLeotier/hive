package api

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/JulienLeotier/hive/internal/bmad"
	"github.com/JulienLeotier/hive/internal/metrics"
)

// stepObserver construit un bmad.StepObserver qui :
//   - insère une row `bmad_phase_steps` en `running` au start ;
//   - la finalise avec status=done/failed + tokens + cost au finish ;
//   - met à jour projects.total_cost_usd pour que le dashboard
//     puisse afficher le cumul ;
//   - émet un événement project.bmad_step (start + finish) pour que
//     le WS push la progression en temps réel.
//
// Les insertions échouées ne plantent pas le pipeline — au pire le
// dashboard n'a pas l'entrée, mais BMAD continue d'avancer.
// trackedInvoke wraps runner.Invoke with the same phase tracking that
// stepObserver does for RunSequence skills. Used for one-shot calls
// (ingest-json, retrospective, etc.) that wouldn't otherwise show up
// in the UI's phase panel — leaving the operator staring at "100%
// avancement" while Claude is still clearly working.
func (s *Server) trackedInvoke(
	ctx context.Context,
	runner *bmad.Runner,
	projectID, phase, label, workdir, goal string,
) (bmad.Result, error) {
	start := time.Now()
	res, err := s.db().ExecContext(ctx,
		`INSERT INTO bmad_phase_steps (project_id, phase, command, status)
		 VALUES (?, ?, ?, 'running')`,
		projectID, phase, label)
	var stepID int64
	if err == nil {
		stepID, _ = res.LastInsertId()
	}
	// Child ctx + registre step-level : permet le cancel chirurgical
	// via POST /api/v1/phases/{id}/cancel. Pas un sous-ctx du parent
	// fourni — on wrap directement le parent pour que l'annulation
	// parent-level reste effective.
	skillCtx, skillCancel := context.WithCancel(ctx)
	defer skillCancel()
	s.RegisterStepCancel(stepID, skillCancel)
	defer s.ClearStepCancel(stepID)

	if s.eventBus != nil {
		_, _ = s.eventBus.Publish(ctx, "project.bmad_step_started", "api", map[string]any{
			"project_id": projectID,
			"phase":      phase,
			"step_id":    stepID,
			"command":    label,
		})
	}
	// Streaming : chaque event NDJSON émis par le CLI Claude est
	// flushé en DB (reply_full) + broadcast WS pour que la console UI
	// défile en live. SQLite tient des UPDATE par chunk sans broncher.
	var streamBuf strings.Builder
	out, invErr := runner.InvokeStream(skillCtx, workdir, goal, nil,
		func(evt bmad.StreamEvent) {
			if evt.Text == "" {
				return
			}
			line := "[" + evt.Type + "] " + evt.Text + "\n"
			streamBuf.WriteString(line)
			if stepID > 0 {
				_, _ = s.db().ExecContext(ctx,
					`UPDATE bmad_phase_steps SET reply_full = ? WHERE id = ?`,
					streamBuf.String(), stepID)
			}
			if s.eventBus != nil {
				_, _ = s.eventBus.Publish(ctx, "project.bmad_step_output", "api", map[string]any{
					"project_id": projectID,
					"step_id":    stepID,
					"command":    label,
					"chunk":      line,
					"event_type": evt.Type,
				})
			}
		})
	status := "done"
	errText := ""
	if invErr != nil {
		status = "failed"
		errText = invErr.Error()
	}
	preview := out.Text
	if len(preview) > 600 {
		preview = preview[:600] + "…"
	}
	_, _ = s.db().ExecContext(ctx,
		`UPDATE bmad_phase_steps
		 SET finished_at = datetime('now'), status = ?,
		     input_tokens = ?, output_tokens = ?, cost_usd = ?,
		     reply_preview = ?, reply_full = ?, error_text = ?
		 WHERE id = ?`,
		status, out.InputTokens, out.OutputTokens, out.CostUSD,
		preview, out.Text, errText, stepID)
	if out.CostUSD > 0 {
		_, _ = s.db().ExecContext(ctx,
			`UPDATE projects SET total_cost_usd = total_cost_usd + ?,
			 updated_at = datetime('now') WHERE id = ?`,
			out.CostUSD, projectID)
	}
	if s.eventBus != nil {
		_, _ = s.eventBus.Publish(ctx, "project.bmad_step_finished", "api", map[string]any{
			"project_id": projectID,
			"phase":      phase,
			"step_id":    stepID,
			"command":    label,
			"status":     status,
			"cost_usd":   out.CostUSD,
			"error":      errText,
		})
	}
	_ = start
	return out, invErr
}

func (s *Server) stepObserver(ctx context.Context, projectID, phase string) bmad.StepObserver {
	// stepStart[command] → time.Time pour mesurer la durée exacte par
	// skill. On stocke sous lock léger car OnStart + OnFinish tournent
	// séquentiellement par séquence, mais on reste thread-safe si un
	// jour on parallélise.
	stepStart := make(map[string]time.Time)
	// Buffers de streaming : on accumule les events stream-json au fur
	// et à mesure et on flush en DB + WS pour que l'UI voie la console
	// défiler en live. Un par (command) car les OnStart/OnFinish
	// tournent séquentiellement dans une séquence mais on veut des
	// clés distinctes au cas où une future parallélisation les
	// entrelacerait.
	buffers := make(map[string]*strings.Builder)
	stepIDs := make(map[string]int64)
	var mu sync.Mutex

	return bmad.StepObserver{
		OnStart: func(index, total int, command string, stepCancel context.CancelFunc) {
			mu.Lock()
			stepStart[command] = time.Now()
			buffers[command] = &strings.Builder{}
			mu.Unlock()
			res, err := s.db().ExecContext(ctx,
				`INSERT INTO bmad_phase_steps (project_id, phase, command, status)
				 VALUES (?, ?, ?, 'running')`,
				projectID, phase, command)
			if err != nil {
				slog.Warn("bmad step log insert failed", "project", projectID, "cmd", command, "error", err)
				return
			}
			id, _ := res.LastInsertId()
			mu.Lock()
			stepIDs[command] = id
			mu.Unlock()
			// Registre step-level → l'UI peut firer POST /phases/{id}/cancel
			// pour tuer CE skill précis sans passer par le cancel du projet.
			s.RegisterStepCancel(id, stepCancel)
			if s.eventBus != nil {
				_, _ = s.eventBus.Publish(ctx, "project.bmad_step_started", "api", map[string]any{
					"project_id": projectID,
					"phase":      phase,
					"step_id":    id,
					"index":      index,
					"total":      total,
					"command":    command,
				})
			}
		},
		OnChunk: func(_, _ int, command string, evt bmad.StreamEvent) {
			if evt.Text == "" {
				return
			}
			mu.Lock()
			buf := buffers[command]
			id := stepIDs[command]
			mu.Unlock()
			if buf == nil {
				return
			}
			line := "[" + evt.Type + "] " + evt.Text + "\n"
			mu.Lock()
			buf.WriteString(line)
			accumulated := buf.String()
			mu.Unlock()
			// Flush DB à chaque chunk — SQLite + une skill qui émet
			// 10-50 events au total tient sans broncher. Évite la
			// complexité d'un batcher + timer.
			if id > 0 {
				_, _ = s.db().ExecContext(ctx,
					`UPDATE bmad_phase_steps SET reply_full = ? WHERE id = ?`,
					accumulated, id)
			}
			if s.eventBus != nil {
				_, _ = s.eventBus.Publish(ctx, "project.bmad_step_output", "api", map[string]any{
					"project_id": projectID,
					"step_id":    id,
					"command":    command,
					"chunk":      line,
					"event_type": evt.Type,
				})
			}
		},
		OnFinish: func(index, total int, command string, res bmad.Result, err error) {
			status := "done"
			errText := ""
			if err != nil {
				status = "failed"
				errText = err.Error()
			}
			// Prometheus : coût cumulé + durée par skill.
			metrics.BMADSkillCost.WithLabelValues(command, status).Add(res.CostUSD)
			mu.Lock()
			startT, ok := stepStart[command]
			id := stepIDs[command]
			delete(stepStart, command)
			delete(buffers, command)
			delete(stepIDs, command)
			mu.Unlock()
			s.ClearStepCancel(id)
			if ok {
				metrics.BMADSkillDuration.WithLabelValues(command).Observe(time.Since(startT).Seconds())
			}
			preview := res.Text
			if len(preview) > 600 {
				preview = preview[:600] + "…"
			}
			// Mise à jour : on cible la dernière step `running` de la
			// commande + projet + phase. Marche dans 99% des cas ; en
			// concurrence extrême (deux goroutines concurrentes) on
			// pourrait écrire sur la mauvaise ligne mais on bloque
			// déjà les pipelines en parallèle via la map
			// cancellations, donc ce cas n'arrive pas.
			if _, dbErr := s.db().ExecContext(ctx,
				`UPDATE bmad_phase_steps
				 SET finished_at = datetime('now'),
				     status = ?, input_tokens = ?, output_tokens = ?,
				     cost_usd = ?, reply_preview = ?, reply_full = ?, error_text = ?
				 WHERE id = (
				   SELECT id FROM bmad_phase_steps
				   WHERE project_id = ? AND phase = ? AND command = ? AND status = 'running'
				   ORDER BY started_at DESC LIMIT 1
				 )`,
				status, res.InputTokens, res.OutputTokens, res.CostUSD,
				preview, res.Text, errText,
				projectID, phase, command,
			); dbErr != nil {
				slog.Warn("bmad step log finish failed", "project", projectID, "cmd", command, "error", dbErr)
			}
			if res.CostUSD > 0 {
				// Snapshot PRÉ-add pour détecter les franchissements de
				// seuil (80%, 100%) : on doit pouvoir dire "on VIENT DE
				// passer 80%" plutôt que "on est au-dessus de 80%" qui
				// spammerait à chaque step une fois le seuil franchi.
				var prevTotal, cap_ float64
				_ = s.db().QueryRowContext(ctx,
					`SELECT total_cost_usd, COALESCE(cost_cap_usd, 0) FROM projects WHERE id = ?`,
					projectID).Scan(&prevTotal, &cap_)
				_, _ = s.db().ExecContext(ctx,
					`UPDATE projects SET total_cost_usd = total_cost_usd + ?,
					 updated_at = datetime('now') WHERE id = ?`,
					res.CostUSD, projectID)
				total := prevTotal + res.CostUSD

				// Alerte 80% — fire UNE fois quand on franchit le seuil
				// vers le haut. Pas de kill, juste un event pour que le
				// dashboard / Slack avertisse l'opérateur avant le coup
				// de grâce à 100%.
				if cap_ > 0 && prevTotal < 0.8*cap_ && total >= 0.8*cap_ && total < cap_ {
					if s.eventBus != nil {
						_, _ = s.eventBus.Publish(ctx, "project.cost_cap_warning", "api",
							map[string]any{
								"project_id": projectID,
								"total_usd":  total,
								"cap_usd":    cap_,
								"pct":        int(total / cap_ * 100),
							})
					}
				}

				// Plafond coût : si le cumul vient de passer le cap,
				// on annule le run courant. L'observer tourne dans la
				// goroutine de la séquence BMAD, donc cancel() au
				// context du run fait remonter l'erreur au
				// RunSequenceObserved qui stoppe net. Le fail() dans
				// runArchitect/runIteration flippe alors le projet en
				// failed avec stage=cost-cap.
				if cap_ > 0 && total >= cap_ {
					slog.Warn("bmad: cost cap reached, cancelling run",
						"project", projectID, "total_usd", total, "cap_usd", cap_)
					_, _ = s.db().ExecContext(ctx,
						`UPDATE projects SET failure_stage = 'cost-cap',
						 failure_error = 'Plafond coût atteint', status = 'failed',
						 updated_at = datetime('now') WHERE id = ?`,
						projectID)
					if s.eventBus != nil {
						_, _ = s.eventBus.Publish(ctx, "project.cost_cap_reached", "api",
							map[string]any{
								"project_id": projectID,
								"total_usd":  total,
								"cap_usd":    cap_,
							})
					}
					s.cancelRun(projectID)
				}
			}
			if s.eventBus != nil {
				_, _ = s.eventBus.Publish(ctx, "project.bmad_step_finished", "api", map[string]any{
					"project_id":    projectID,
					"phase":         phase,
					"index":         index,
					"total":         total,
					"command":       command,
					"status":        status,
					"cost_usd":      res.CostUSD,
					"input_tokens":  res.InputTokens,
					"output_tokens": res.OutputTokens,
					"error":         errText,
				})
			}
		},
	}
}
