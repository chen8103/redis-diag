package poll

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestSchedulerSkipsOverlappingRuns(t *testing.T) {
	var runs atomic.Int32
	s := NewScheduler()

	s.Run(context.Background(), "dashboard", func(ctx context.Context) {
		runs.Add(1)
		time.Sleep(25 * time.Millisecond)
	})
	s.Run(context.Background(), "dashboard", func(ctx context.Context) {
		runs.Add(1)
	})

	time.Sleep(40 * time.Millisecond)

	if got := runs.Load(); got != 2 {
		t.Fatalf("expected overlapping run to be coalesced into one extra pass, got %d runs", got)
	}
}
