package redis

import (
	"testing"
	"time"

	"github.com/mingchen/redis-diag/internal/model"
)

func TestKeepTopBigKeys(t *testing.T) {
	rows := []model.BigKeyRow{
		{Key: "a", Size: 10},
		{Key: "b", Size: 30},
		{Key: "c", Size: 20},
	}

	top := keepTopBigKeys(rows, 2)
	if len(top) != 2 {
		t.Fatalf("expected top2 rows, got %d", len(top))
	}
	if top[0].Key != "b" || top[1].Key != "c" {
		t.Fatalf("unexpected ordering: %+v", top)
	}
}

func TestScanBudgetStopsWhenExpired(t *testing.T) {
	deadline := time.Now().Add(-time.Millisecond)
	if !budgetExpired(deadline) {
		t.Fatalf("expected expired budget")
	}
}
