package poll

import (
	"testing"
	"time"

	"github.com/mingchen/redis-diag/internal/config"
)

func TestPolicyDefaults(t *testing.T) {
	policy := DefaultPolicy()

	if policy.Dashboard.Interval != time.Second {
		t.Fatalf("expected dashboard interval 1s, got %s", policy.Dashboard.Interval)
	}
	if policy.Keys.Interval <= policy.Dashboard.Interval {
		t.Fatalf("expected keys interval to be slower than dashboard")
	}
	if policy.MaxConcurrency < 1 {
		t.Fatalf("expected max concurrency to be >= 1")
	}
}

func TestPolicyFromConfig(t *testing.T) {
	cfg := config.Default()
	cfg.Refresh = 3 * time.Second
	cfg.Panels.Dashboard.Interval = 0
	cfg.Panels.Commands.Interval = 0
	cfg.Normalize()

	policy := FromConfig(cfg)

	if policy.Dashboard.Interval != 3*time.Second {
		t.Fatalf("expected dashboard interval from config, got %s", policy.Dashboard.Interval)
	}
	if policy.Commands.Interval != 3*time.Second {
		t.Fatalf("expected commands interval from config, got %s", policy.Commands.Interval)
	}
}

func TestShouldSkipWhenJobStillRunning(t *testing.T) {
	state := TickState{Running: true}
	if !ShouldSkip(state) {
		t.Fatalf("expected running job to be skipped")
	}
}
