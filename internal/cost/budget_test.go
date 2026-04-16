package cost

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetAndListBudgets(t *testing.T) {
	tr := setupTracker(t)
	require.NoError(t, ensureBudgetsTable(tr.db))
	ctx := context.Background()

	require.NoError(t, tr.SetBudget(ctx, "reviewer", 5.0))
	require.NoError(t, tr.SetBudget(ctx, "writer", 10.0))

	budgets, err := tr.ListBudgets(ctx)
	require.NoError(t, err)
	assert.Len(t, budgets, 2)
}

func TestSetBudgetUpserts(t *testing.T) {
	tr := setupTracker(t)
	require.NoError(t, ensureBudgetsTable(tr.db))
	ctx := context.Background()

	require.NoError(t, tr.SetBudget(ctx, "reviewer", 5.0))
	require.NoError(t, tr.SetBudget(ctx, "reviewer", 7.5))

	budgets, err := tr.ListBudgets(ctx)
	require.NoError(t, err)
	require.Len(t, budgets, 1)
	assert.Equal(t, 7.5, budgets[0].DailyLimit)
}

func TestSetBudgetRejectsNegative(t *testing.T) {
	tr := setupTracker(t)
	require.NoError(t, ensureBudgetsTable(tr.db))
	err := tr.SetBudget(context.Background(), "reviewer", -1)
	assert.Error(t, err)
}

func TestEvaluateAlertsFlagsBreach(t *testing.T) {
	tr := setupTracker(t)
	require.NoError(t, ensureBudgetsTable(tr.db))
	ctx := context.Background()

	require.NoError(t, tr.SetBudget(ctx, "reviewer", 0.10))
	require.NoError(t, tr.Record(ctx, "a1", "reviewer", "wf", "t1", 0.08))
	require.NoError(t, tr.Record(ctx, "a1", "reviewer", "wf", "t2", 0.05))

	alerts, err := tr.EvaluateAlerts(ctx)
	require.NoError(t, err)
	require.Len(t, alerts, 1)
	assert.True(t, alerts[0].Breached)
	assert.InDelta(t, 0.13, alerts[0].Spend, 0.001)
}

func TestDeleteBudget(t *testing.T) {
	tr := setupTracker(t)
	require.NoError(t, ensureBudgetsTable(tr.db))
	ctx := context.Background()

	require.NoError(t, tr.SetBudget(ctx, "reviewer", 5.0))
	require.NoError(t, tr.DeleteBudget(ctx, "reviewer"))

	budgets, err := tr.ListBudgets(ctx)
	require.NoError(t, err)
	assert.Empty(t, budgets)
}
