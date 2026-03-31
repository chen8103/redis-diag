package monitor

import (
	"testing"
	"time"

	"github.com/mingchen/redis-diag/internal/model"
)

func TestBufferKeepsLatestEvents(t *testing.T) {
	buf := NewBuffer(2, 5*time.Second)
	buf.Add(model.MonitorEvent{Command: "get"})
	buf.Add(model.MonitorEvent{Command: "set"})
	buf.Add(model.MonitorEvent{Command: "del"})

	events := buf.Events()
	if len(events) != 2 {
		t.Fatalf("expected capacity 2, got %d", len(events))
	}
	if events[0].Command != "set" || events[1].Command != "del" {
		t.Fatalf("unexpected retained commands %+v", events)
	}
}
