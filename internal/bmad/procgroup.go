package bmad

import (
	"os/exec"
	"syscall"
)

// configureProcessGroup fait deux choses pour que la mort d'un ctx
// tue AUSSI les descendants du process claude CLI :
//
//  1. Setpgid: true → le process claude devient leader de son propre
//     groupe de process. Tous ses descendants héritent du même pgid.
//  2. cmd.Cancel → quand ctx.Done() fire, Go appelle cette func au
//     lieu du default "SIGKILL sur le pid seul". On envoie SIGKILL
//     au groupe entier (-pgid) — claude + node + python + bash + etc.
//
// Sans ce setup, claude reçoit SIGKILL mais ses children (tool_use
// lance souvent des node/python/bash) se font reparenter à init et
// continuent à tourner. Observé en prod : après un air hot-reload,
// plusieurs node + python orphelins continuaient à consommer CPU +
// tokens jusqu'à ce que je les kill -9 à la main.
//
// Portable macOS + Linux (BSD/Unix). Windows ne reçoit pas cette
// feature (pas de setpgid), fallback sur le default exec.CommandContext.
func configureProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		// Kill le groupe entier. Negative pid = all processes in pgid.
		// Tolère ESRCH (process déjà mort) — pas d'erreur remontée.
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		return nil
	}
}
