package store

import (
	"testing"
	"time"

	"github.com/mingchen/redis-diag/internal/model"
)

func TestStoreKeepsLatestSnapshot(t *testing.T) {
	s := New()
	first := model.DashboardSnapshot{
		SnapshotMeta: model.SnapshotMeta{
			NodeID:      "node-a",
			CollectedAt: time.Now(),
			Quality:     model.QualityExact,
		},
		Role: "master",
	}
	second := model.DashboardSnapshot{
		SnapshotMeta: model.SnapshotMeta{
			NodeID:      "node-a",
			CollectedAt: time.Now().Add(time.Second),
			Quality:     model.QualityExact,
		},
		Role: "replica",
	}

	s.SetDashboard(first)
	s.SetDashboard(second)

	got, ok := s.Dashboard("node-a")
	if !ok {
		t.Fatalf("expected snapshot for node-a")
	}
	if got.Role != "replica" {
		t.Fatalf("expected latest snapshot to win, got role %q", got.Role)
	}
}
