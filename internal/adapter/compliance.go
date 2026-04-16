package adapter

import (
	"context"
	"fmt"
	"time"
)

// ComplianceResult aggregates the outcome of running the compliance suite.
type ComplianceResult struct {
	Passed  []string          // names of passing checks
	Failed  map[string]string // name → failure reason
	Skipped map[string]string // name → skip reason
}

// OK reports whether every check passed (Skipped checks don't affect pass).
func (r ComplianceResult) OK() bool { return len(r.Failed) == 0 }

// Summary formats the result as a human-readable one-liner.
func (r ComplianceResult) Summary() string {
	if r.OK() {
		return fmt.Sprintf("PASS — %d/%d checks", len(r.Passed), len(r.Passed)+len(r.Skipped))
	}
	return fmt.Sprintf("FAIL — %d passed, %d failed, %d skipped", len(r.Passed), len(r.Failed), len(r.Skipped))
}

// RunCompliance exercises the Agent Adapter Protocol against any Adapter.
// Story 7.4 AC: "a protocol compliance test suite validates any adapter
// implementation". Story 1.2 references the same suite.
//
// Checks exercised:
//   1. Declare returns a non-empty Name.
//   2. Declare returns at least one TaskType.
//   3. Health returns a status string.
//   4. Invoke returns a TaskResult whose TaskID matches the input.
//   5. Checkpoint + Resume round-trips without error.
//
// Adapters with features the harness can't safely exercise (e.g., a real
// CrewAI subprocess) should wrap themselves in a mock or skip via SkipInvoke.
type ComplianceOptions struct {
	// SampleTask is the Task passed to Invoke. When nil, a trivial ping is used.
	SampleTask *Task
	// SkipInvoke bypasses the Invoke round-trip (use for adapters that need
	// external processes not available in the test environment).
	SkipInvoke bool
	// SkipCheckpoint bypasses Checkpoint/Resume (some adapters are stateless).
	SkipCheckpoint bool
	// Timeout bounds each call. Defaults to 5 seconds.
	Timeout time.Duration
}

// RunCompliance runs the full suite and returns a structured result.
func RunCompliance(a Adapter, opts ComplianceOptions) ComplianceResult {
	result := ComplianceResult{
		Failed:  map[string]string{},
		Skipped: map[string]string{},
	}
	if opts.Timeout == 0 {
		opts.Timeout = 5 * time.Second
	}

	// Declare
	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	caps, err := a.Declare(ctx)
	cancel()
	switch {
	case err != nil:
		result.Failed["declare"] = err.Error()
	case caps.Name == "":
		result.Failed["declare.name"] = "Name is empty"
	case len(caps.TaskTypes) == 0:
		result.Failed["declare.task_types"] = "TaskTypes is empty"
	default:
		result.Passed = append(result.Passed, "declare")
	}

	// Health
	ctx, cancel = context.WithTimeout(context.Background(), opts.Timeout)
	h, err := a.Health(ctx)
	cancel()
	if err != nil {
		result.Failed["health"] = err.Error()
	} else if h.Status == "" {
		result.Failed["health.status"] = "Status is empty"
	} else {
		result.Passed = append(result.Passed, "health")
	}

	// Invoke
	if opts.SkipInvoke {
		result.Skipped["invoke"] = "skipped by caller"
	} else {
		task := opts.SampleTask
		if task == nil {
			task = &Task{ID: "compliance-ping", Type: "ping", Input: map[string]any{"test": true}}
		}
		ctx, cancel = context.WithTimeout(context.Background(), opts.Timeout)
		res, err := a.Invoke(ctx, *task)
		cancel()
		switch {
		case err != nil:
			result.Failed["invoke"] = err.Error()
		case res.TaskID != task.ID:
			result.Failed["invoke.task_id"] = fmt.Sprintf("returned TaskID=%q, want %q", res.TaskID, task.ID)
		default:
			result.Passed = append(result.Passed, "invoke")
		}
	}

	// Checkpoint + Resume round-trip
	if opts.SkipCheckpoint {
		result.Skipped["checkpoint"] = "skipped by caller"
	} else {
		ctx, cancel = context.WithTimeout(context.Background(), opts.Timeout)
		cp, err := a.Checkpoint(ctx)
		cancel()
		if err != nil {
			result.Failed["checkpoint"] = err.Error()
		} else {
			ctx, cancel = context.WithTimeout(context.Background(), opts.Timeout)
			err = a.Resume(ctx, cp)
			cancel()
			if err != nil {
				result.Failed["resume"] = err.Error()
			} else {
				result.Passed = append(result.Passed, "checkpoint", "resume")
			}
		}
	}

	return result
}
