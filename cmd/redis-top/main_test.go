package main

import (
	"testing"
	"time"
)

func TestParseConfigFromArgsSupportsKeysAndMonitorFlags(t *testing.T) {
	cfg, showVersion, err := parseConfigFromArgs([]string{
		"--startup-timeout", "4s",
		"--keys-topk", "50",
		"--keys-scan-count", "500",
		"--keys-sample-max", "800",
		"--keys-interval", "20s",
		"--keys-timeout", "2s",
		"--monitor-buffer", "4096",
		"--monitor-ui-refresh", "750ms",
		"--monitor-window", "10s",
		"--monitor-mask-args=false",
		"--monitor-max-arg-len", "64",
		"--monitor-selected-node-only=false",
	})
	if err != nil {
		t.Fatalf("parseConfigFromArgs failed: %v", err)
	}
	if showVersion {
		t.Fatalf("expected showVersion false")
	}
	if cfg.StartupTimeout != 4*time.Second {
		t.Fatalf("unexpected startup timeout: %s", cfg.StartupTimeout)
	}
	if cfg.Keys.TopK != 50 || cfg.Keys.ScanCount != 500 || cfg.Keys.SampleKeysMax != 800 {
		t.Fatalf("unexpected keys config: %+v", cfg.Keys)
	}
	if cfg.Panels.Keys.Interval != 20*time.Second || cfg.Panels.Keys.Timeout != 2*time.Second {
		t.Fatalf("unexpected keys panel config: %+v", cfg.Panels.Keys)
	}
	if cfg.Monitor.BufferCapacity != 4096 {
		t.Fatalf("unexpected monitor buffer: %d", cfg.Monitor.BufferCapacity)
	}
	if cfg.Monitor.UIRefresh != 750*time.Millisecond || cfg.Monitor.Window != 10*time.Second {
		t.Fatalf("unexpected monitor timing config: %+v", cfg.Monitor)
	}
	if cfg.Monitor.MaskArgs {
		t.Fatalf("expected monitor masking false")
	}
	if cfg.Monitor.MaxArgLen != 64 {
		t.Fatalf("unexpected max arg len: %d", cfg.Monitor.MaxArgLen)
	}
	if cfg.Monitor.SelectedNodeOnly {
		t.Fatalf("expected selected-node-only false")
	}
}

func TestParseConfigFromArgsVersionFlag(t *testing.T) {
	_, showVersion, err := parseConfigFromArgs([]string{"--version"})
	if err != nil {
		t.Fatalf("parseConfigFromArgs failed: %v", err)
	}
	if !showVersion {
		t.Fatalf("expected showVersion true")
	}
}
