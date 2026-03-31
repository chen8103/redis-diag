package monitor

import "testing"

func TestParseMonitorLine(t *testing.T) {
	line := `1339518083.107412 [0 127.0.0.1:60866] "keys" "*"`
	event, err := ParseLine("node-a", line)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if event.DB != 0 {
		t.Fatalf("expected db 0, got %d", event.DB)
	}
	if event.ClientAddr != "127.0.0.1:60866" {
		t.Fatalf("unexpected addr %q", event.ClientAddr)
	}
	if event.Command != "keys" {
		t.Fatalf("unexpected command %q", event.Command)
	}
	if len(event.Args) != 1 || event.Args[0] != "*" {
		t.Fatalf("unexpected args %+v", event.Args)
	}
}
