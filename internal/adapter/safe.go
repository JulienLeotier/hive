package adapter

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// SafeInvoke wraps a.Invoke with panic recovery so a misbehaving adapter
// can't leave a task stuck in "running" forever or crash the workflow engine
// goroutine. A recovered panic is converted into a regular error with the
// stack trace logged for post-mortem.
//
// Adapter invocations are wrapped in an "adapter.invoke" span so operators
// can see the full task timeline across adapter types (HTTP, claude-code,
// mcp, crewai, …) in any OTLP backend.
//
// Why this exists: adapters are effectively untrusted plugins — HTTP-backed,
// subprocess, user-authored. A typo, nil deref, or out-of-bounds slice in
// one adapter call must not take down the orchestrator.
func SafeInvoke(ctx context.Context, a Adapter, task Task) (result TaskResult, err error) {
	ctx, span := otel.Tracer("hive/adapter").Start(ctx, "adapter.invoke")
	span.SetAttributes(
		attribute.String("task.id", task.ID),
		attribute.String("task.type", task.Type),
	)
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			slog.Error("adapter panicked",
				"task_id", task.ID,
				"task_type", task.Type,
				"panic", fmt.Sprint(r),
				"stack", string(stack))
			err = fmt.Errorf("adapter panic: %v", r)
			span.RecordError(err)
			span.SetStatus(codes.Error, "panic")
			result = TaskResult{
				TaskID: task.ID,
				Status: "failed",
				Error:  err.Error(),
			}
		}
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else if result.Status == "failed" {
			span.SetStatus(codes.Error, result.Error)
		}
		span.SetAttributes(attribute.String("task.status", result.Status))
		span.End()
	}()
	return a.Invoke(ctx, task)
}
