package redis

import (
	"testing"
	"time"

	"github.com/mingchen/redis-diag/internal/model"
)

func TestParseClientListSortFilterAndLimit(t *testing.T) {
	raw := "id=1 addr=10.0.0.1:5000 age=10 idle=3 db=0 flags=N cmd=get omem=1 qbuf=2\n" +
		"id=2 addr=10.0.0.2:5001 age=5 idle=1 db=0 flags=N cmd=set omem=4 qbuf=3\n" +
		"id=3 addr=10.0.0.3:5002 age=20 idle=9 db=0 flags=N cmd=ping omem=2 qbuf=1\n"

	rows, _ := parseClientList(raw, "idle", "10.0.0.2", 10)
	if len(rows) != 1 {
		t.Fatalf("expected one filtered row, got %d", len(rows))
	}
	if rows[0].ID != "2" {
		t.Fatalf("expected client id 2, got %q", rows[0].ID)
	}

	rows, _ = parseClientList(raw, "omem", "", 2)
	if len(rows) != 2 {
		t.Fatalf("expected two limited rows, got %d", len(rows))
	}
	if rows[0].ID != "2" {
		t.Fatalf("expected highest omem client first, got %q", rows[0].ID)
	}
}

func TestBuildSlowlogSnapshotSupportsLatestAndNew(t *testing.T) {
	entries := []model.SlowlogEntry{
		{ID: 1, Timestamp: time.Unix(1, 0), DurationUS: 10, Command: "get a"},
		{ID: 2, Timestamp: time.Unix(2, 0), DurationUS: 20, Command: "set b"},
	}

	latest, seen := buildSlowlogEntries(entries, nil, "latest")
	if len(latest) != 2 {
		t.Fatalf("expected all latest rows, got %d", len(latest))
	}
	if len(seen) != 2 {
		t.Fatalf("expected seen set to track two ids, got %d", len(seen))
	}

	newRows, seen := buildSlowlogEntries(entries, seen, "new")
	if len(newRows) != 0 {
		t.Fatalf("expected no new rows on second pass, got %d", len(newRows))
	}

	more := append(entries, model.SlowlogEntry{ID: 3, Timestamp: time.Unix(3, 0), DurationUS: 30, Command: "del c"})
	newRows, _ = buildSlowlogEntries(more, seen, "new")
	if len(newRows) != 1 || newRows[0].ID != 3 {
		t.Fatalf("expected only newly seen id 3, got %+v", newRows)
	}
}
