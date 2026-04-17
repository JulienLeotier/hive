package adapter

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
)

// SafeInvoke wraps a.Invoke with panic recovery so a misbehaving adapter
// can't leave a task stuck in "running" forever or crash the workflow engine
// goroutine. A recovered panic is converted into a regular error with the
// stack trace logged for post-mortem.
//
// Why this exists: adapters are effectively untrusted plugins — HTTP-backed,
// subprocess, user-authored. A typo, nil deref, or out-of-bounds slice in
// one adapter call must not take down the orchestrator.
func SafeInvoke(ctx context.Context, a Adapter, task Task) (result TaskResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			slog.Error("adapter panicked",
				"task_id", task.ID,
				"task_type", task.Type,
				"panic", fmt.Sprint(r),
				"stack", string(stack))
			err = fmt.Errorf("adapter panic: %v", r)
			result = TaskResult{
				TaskID: task.ID,
				Status: "failed",
				Error:  err.Error(),
			}
		}
	}()
	return a.Invoke(ctx, task)
}
