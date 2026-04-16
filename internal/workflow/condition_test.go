package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ctxWith(data map[string]map[string]any) evalContext {
	return evalContext{Upstream: data}
}

func TestEvaluateEmptyCondition(t *testing.T) {
	ok, err := EvaluateCondition("", ctxWith(nil))
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestEvaluateNumericGT(t *testing.T) {
	ctx := ctxWith(map[string]map[string]any{"review": {"score": 0.85}})
	ok, err := EvaluateCondition("upstream.review.score > 0.8", ctx)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = EvaluateCondition("upstream.review.score > 0.9", ctx)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestEvaluateNumericRange(t *testing.T) {
	ctx := ctxWith(map[string]map[string]any{"lint": {"errors": 3.0}})
	for _, c := range []struct {
		cond string
		want bool
	}{
		{"upstream.lint.errors >= 3", true},
		{"upstream.lint.errors <= 3", true},
		{"upstream.lint.errors < 3", false},
		{"upstream.lint.errors != 0", true},
	} {
		ok, err := EvaluateCondition(c.cond, ctx)
		require.NoError(t, err)
		assert.Equal(t, c.want, ok, c.cond)
	}
}

func TestEvaluateStringEquality(t *testing.T) {
	ctx := ctxWith(map[string]map[string]any{"build": {"result": "success"}})
	ok, err := EvaluateCondition(`upstream.build.result == "success"`, ctx)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = EvaluateCondition(`upstream.build.result != "failure"`, ctx)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestEvaluateContains(t *testing.T) {
	ctx := ctxWith(map[string]map[string]any{"scan": {"summary": "2 critical issues found"}})
	ok, err := EvaluateCondition(`upstream.scan.summary contains "critical"`, ctx)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestEvaluateBool(t *testing.T) {
	ctx := ctxWith(map[string]map[string]any{"approve": {"approved": true}})
	ok, err := EvaluateCondition("upstream.approve.approved == true", ctx)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestEvaluateMissingPathDefaultsFalse(t *testing.T) {
	ctx := ctxWith(map[string]map[string]any{"review": {}})
	ok, err := EvaluateCondition("upstream.review.score > 0.8", ctx)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestEvaluateBadCondition(t *testing.T) {
	_, err := EvaluateCondition("this is not a condition", ctxWith(nil))
	assert.Error(t, err)
}
