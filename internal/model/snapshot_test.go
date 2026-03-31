package model

import (
	"testing"
	"time"
)

func TestSnapshotMetaIsFresh(t *testing.T) {
	base := time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)
	meta := SnapshotMeta{
		CollectedAt: base,
		TTL:         2 * time.Second,
	}

	if !meta.IsFresh(base.Add(time.Second)) {
		t.Fatalf("expected snapshot to be fresh within ttl")
	}
	if meta.IsFresh(base.Add(3 * time.Second)) {
		t.Fatalf("expected snapshot to become stale after ttl")
	}
}

func TestQualityHelpers(t *testing.T) {
	if !QualityExact.IsTerminalSuccess() {
		t.Fatalf("exact should be treated as successful quality")
	}
	if QualityStale.IsTerminalSuccess() {
		t.Fatalf("stale should not be treated as successful quality")
	}
}
