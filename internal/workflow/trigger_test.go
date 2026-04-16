package workflow

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseScheduleDurations(t *testing.T) {
	d, err := parseSchedule("30s")
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, d)

	d, err = parseSchedule("5m")
	require.NoError(t, err)
	assert.Equal(t, 5*time.Minute, d)
}

func TestParseScheduleCronEveryN(t *testing.T) {
	d, err := parseSchedule("*/15 * * * *")
	require.NoError(t, err)
	assert.Equal(t, 15*time.Minute, d)
}

func TestParseScheduleRejectsJunk(t *testing.T) {
	_, err := parseSchedule("not-a-schedule")
	assert.Error(t, err)
	_, err = parseSchedule("5 4 * * *")
	assert.Error(t, err, "we only support */N, not specific times")
}

func TestManualTriggerFiresWorkflow(t *testing.T) {
	var fired int32
	m := NewTriggerManager(func(ctx context.Context, cfg *Config, p TriggerPayload) error {
		atomic.AddInt32(&fired, 1)
		assert.Equal(t, "manual", p.Source)
		return nil
	})
	require.NoError(t, m.Register(context.Background(), &Config{
		Name:  "wf-1",
		Tasks: []TaskDef{{Name: "t1", Type: "x"}},
	}))
	require.NoError(t, m.FireManual(context.Background(), "wf-1", nil))
	assert.Equal(t, int32(1), atomic.LoadInt32(&fired))
}

func TestWebhookTriggerDispatchesByPath(t *testing.T) {
	var got string
	m := NewTriggerManager(func(ctx context.Context, cfg *Config, p TriggerPayload) error {
		got = cfg.Name
		assert.Equal(t, "webhook", p.Source)
		return nil
	})
	require.NoError(t, m.Register(context.Background(), &Config{
		Name:    "incoming",
		Tasks:   []TaskDef{{Name: "t", Type: "x"}},
		Trigger: &TriggerDef{Type: "webhook", Webhook: "/hooks/pr"},
	}))

	require.NoError(t, m.FireWebhook(context.Background(), "/hooks/pr", map[string]any{"repo": "x"}))
	assert.Equal(t, "incoming", got)

	err := m.FireWebhook(context.Background(), "/hooks/unknown", nil)
	assert.Error(t, err)
}

func TestScheduleTriggerFiresPeriodically(t *testing.T) {
	var calls int32
	m := NewTriggerManager(func(ctx context.Context, cfg *Config, p TriggerPayload) error {
		atomic.AddInt32(&calls, 1)
		return nil
	})
	defer m.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, m.Register(ctx, &Config{
		Name:    "periodic",
		Tasks:   []TaskDef{{Name: "t", Type: "x"}},
		Trigger: &TriggerDef{Type: "schedule", Schedule: "40ms"},
	}))

	time.Sleep(150 * time.Millisecond)
	assert.GreaterOrEqual(t, atomic.LoadInt32(&calls), int32(2))
}
