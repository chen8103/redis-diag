package redis

import (
	"testing"
	"time"
)

func TestParseInfoSections(t *testing.T) {
	raw := "# Server\r\nredis_version:7.2.4\r\n# Stats\r\ninstantaneous_ops_per_sec:12\r\nkeyspace_hits:10\r\nkeyspace_misses:5\r\n"
	info := parseInfo(raw)

	if info["redis_version"] != "7.2.4" {
		t.Fatalf("expected redis_version, got %q", info["redis_version"])
	}
	if info["instantaneous_ops_per_sec"] != "12" {
		t.Fatalf("expected ops/sec, got %q", info["instantaneous_ops_per_sec"])
	}
}

func TestParseCommandStatsAndDelta(t *testing.T) {
	raw := "cmdstat_get:calls=100,usec=50,usec_per_call=0.50\r\ncmdstat_set:calls=40,usec=80,usec_per_call=2.00\r\n"
	rows := parseCommandStats(raw, map[string]int64{
		"get": 80,
		"set": 35,
	}, time.Second)

	if len(rows) != 2 {
		t.Fatalf("expected two command rows, got %d", len(rows))
	}
	if rows[0].Command != "get" {
		t.Fatalf("expected get to sort first by calls, got %q", rows[0].Command)
	}
	if rows[0].QPSDelta <= 0 {
		t.Fatalf("expected positive delta qps, got %f", rows[0].QPSDelta)
	}
}
