package workflow

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// evalContext holds the data a condition can inspect.
//
//	upstream.<task>.<field>   → value from a prior task's output JSON
//	upstream.<task>           → whole object
//	result.<field>            → alias for "last completed task's output" (most recent)
type evalContext struct {
	Upstream map[string]map[string]any
}

// buildEvalContext extracts upstream results into a context the evaluator can read.
func buildEvalContext(result *RunResult, deps []string) evalContext {
	ctx := evalContext{Upstream: map[string]map[string]any{}}
	if result == nil {
		return ctx
	}
	result.mu.Lock()
	defer result.mu.Unlock()
	for _, dep := range deps {
		t, ok := result.TaskResults[dep]
		if !ok || t.Output == "" {
			continue
		}
		var parsed map[string]any
		if err := json.Unmarshal([]byte(t.Output), &parsed); err == nil {
			ctx.Upstream[dep] = parsed
		}
	}
	return ctx
}

// EvaluateCondition reports whether a task's condition is satisfied.
// An empty condition is always true.
//
// Supported syntax:
//
//	<path> <op> <literal>
//
// where <path> is dotted (e.g. upstream.review.score), <op> is one of
// == != > >= < <= contains, and <literal> is either a number, a bool, or a
// double-quoted string. This is intentionally small — enough for routing
// decisions without embedding a full expression language.
func EvaluateCondition(cond string, ctx evalContext) (bool, error) {
	cond = strings.TrimSpace(cond)
	if cond == "" {
		return true, nil
	}

	// Match <path> <op> <literal>
	re := regexp.MustCompile(`^(\S+)\s+(==|!=|>=|<=|>|<|contains)\s+(.+)$`)
	m := re.FindStringSubmatch(cond)
	if m == nil {
		return false, fmt.Errorf("unparseable condition %q", cond)
	}

	path, op, litStr := m[1], m[2], strings.TrimSpace(m[3])

	lhs, found := lookup(ctx, path)
	if !found {
		// Missing value: only equality checks against null/empty succeed.
		return op == "==" && (litStr == "null" || litStr == `""`), nil
	}

	return compare(lhs, op, litStr)
}

func lookup(ctx evalContext, path string) (any, bool) {
	parts := strings.Split(path, ".")
	if len(parts) < 2 || parts[0] != "upstream" {
		return nil, false
	}
	taskName := parts[1]
	obj, ok := ctx.Upstream[taskName]
	if !ok {
		return nil, false
	}
	var current any = obj
	for _, p := range parts[2:] {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = m[p]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func compare(lhs any, op, litStr string) (bool, error) {
	// Numeric comparison
	if lhsNum, lhsOk := toFloat(lhs); lhsOk {
		if rhsNum, err := strconv.ParseFloat(litStr, 64); err == nil {
			switch op {
			case "==":
				return lhsNum == rhsNum, nil
			case "!=":
				return lhsNum != rhsNum, nil
			case ">":
				return lhsNum > rhsNum, nil
			case ">=":
				return lhsNum >= rhsNum, nil
			case "<":
				return lhsNum < rhsNum, nil
			case "<=":
				return lhsNum <= rhsNum, nil
			}
		}
	}

	// String comparison — accept "quoted" literals
	lhsStr := fmt.Sprintf("%v", lhs)
	rhsStr := litStr
	if strings.HasPrefix(litStr, `"`) && strings.HasSuffix(litStr, `"`) {
		rhsStr = litStr[1 : len(litStr)-1]
	}
	// Bool literal shortcuts
	if litStr == "true" || litStr == "false" {
		if b, ok := lhs.(bool); ok {
			want := litStr == "true"
			switch op {
			case "==":
				return b == want, nil
			case "!=":
				return b != want, nil
			}
		}
	}
	switch op {
	case "==":
		return lhsStr == rhsStr, nil
	case "!=":
		return lhsStr != rhsStr, nil
	case "contains":
		return strings.Contains(lhsStr, rhsStr), nil
	}
	return false, fmt.Errorf("operator %q not applicable to %v", op, lhs)
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(n, 64)
		return f, err == nil
	}
	return 0, false
}
