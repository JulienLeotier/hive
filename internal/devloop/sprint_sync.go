package devloop

import (
	"context"
	"database/sql"
	"log/slog"
	"strconv"
	"strings"

	"github.com/JulienLeotier/hive/internal/bmad"
)

// syncSprintStatus lit BMAD's sprint-status.yaml et propage le statut
// de CHAQUE story vers la DB Hive. Utilisé après chaque invocation
// BMAD (dev-story / code-review) pour rattraper le cas où BMAD a
// touché plusieurs stories en parallèle dans une seule skill (que
// Hive croyait cibler une seule story).
//
// Mapping BMAD key "0-3-auth-..." → Hive story :
//   - epicIdx = 0 → epic_id = "epc_<projectID>_0"
//   - storyIdx = 3 (1-based) → ordering = 2 (0-based)
//
// Mapping status BMAD → Hive :
//   - backlog / ready-for-dev → pending
//   - in-progress           → dev
//   - review                → review
//   - ready-for-done / done → done
//
// N'écrase PAS un statut terminal en DB Hive (done, blocked) : si
// l'opérateur a explicitement bloqué une story, BMAD ne peut pas la
// débloquer via ce sync — il faut passer par l'UI retry.
func syncSprintStatus(ctx context.Context, db *sql.DB, projectID, workdir string) {
	sprint, err := bmad.ReadSprintStatus(workdir)
	if err != nil || sprint == nil {
		return
	}
	updated := 0
	for bmadKey, bmadStatus := range sprint.DevelopmentStatus {
		epicIdx, storyOrdering, ok := parseBMADKey(bmadKey)
		if !ok {
			continue
		}
		hiveStatus := mapBMADStatus(bmadStatus)
		epicID := "epc_" + projectID + "_" + strconv.Itoa(epicIdx)

		res, err := db.ExecContext(ctx,
			`UPDATE stories SET status = ?, updated_at = datetime('now')
			 WHERE epic_id = ? AND ordering = ?
			   AND status NOT IN ('done', 'blocked')
			   AND status != ?`,
			hiveStatus, epicID, storyOrdering, hiveStatus)
		if err != nil {
			continue
		}
		if n, _ := res.RowsAffected(); n > 0 {
			updated++
			slog.Info("devloop sync: story status",
				"project", projectID, "bmad_key", bmadKey,
				"new_status", hiveStatus)
		}
	}
	if updated > 0 {
		slog.Info("devloop sync: applied", "project", projectID, "stories_updated", updated)
	}
}

// parseBMADKey extrait epicIdx (0-based) et storyOrdering (0-based)
// d'une clé BMAD "A-B-slug-...". BMAD indexe les stories à 1 ; Hive
// à 0 — on soustrait 1 pour matcher.
func parseBMADKey(key string) (epicIdx, storyOrdering int, ok bool) {
	parts := strings.SplitN(key, "-", 3)
	if len(parts) < 2 {
		return 0, 0, false
	}
	epicIdx, err1 := strconv.Atoi(parts[0])
	storyIdx1, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || storyIdx1 < 1 {
		return 0, 0, false
	}
	return epicIdx, storyIdx1 - 1, true
}

// mapBMADStatus convertit un statut BMAD vers le statut Hive
// équivalent. Statuts inconnus → pending (conservative).
func mapBMADStatus(bmadStatus string) string {
	switch bmadStatus {
	case "ready-for-done", "done", "approved":
		return "done"
	case "in-progress":
		return "dev"
	case "review":
		return "review"
	case "blocked":
		return "blocked"
	default: // backlog, ready-for-dev, "", etc.
		return "pending"
	}
}
