package config

import (
	"testing"
	"time"
)

func TestDefaults(t *testing.T) {
	cfg := Default()

	if cfg.Target.Host != "127.0.0.1" {
		t.Fatalf("expected default host 127.0.0.1, got %q", cfg.Target.Host)
	}
	if cfg.Target.Port != 6379 {
		t.Fatalf("expected default port 6379, got %d", cfg.Target.Port)
	}
	if cfg.StartupTimeout != 3*time.Second {
		t.Fatalf("expected startup timeout 3s, got %s", cfg.StartupTimeout)
	}
	if cfg.Refresh != time.Second {
		t.Fatalf("expected refresh 1s, got %s", cfg.Refresh)
	}
	if cfg.ClientsLimit != 20 {
		t.Fatalf("expected clients limit 20, got %d", cfg.ClientsLimit)
	}
	if cfg.Keys.TopK != 20 {
		t.Fatalf("expected keys topk 20, got %d", cfg.Keys.TopK)
	}
	if cfg.Keys.ScanCount != 200 {
		t.Fatalf("expected keys scan count 200, got %d", cfg.Keys.ScanCount)
	}
	if cfg.Keys.SampleKeysMax != 200 {
		t.Fatalf("expected keys sample max 200, got %d", cfg.Keys.SampleKeysMax)
	}
	if cfg.Monitor.BufferCapacity != 2000 {
		t.Fatalf("expected monitor buffer 2000, got %d", cfg.Monitor.BufferCapacity)
	}
	if !cfg.Monitor.MaskArgs {
		t.Fatalf("expected monitor masking enabled by default")
	}
	if !cfg.Monitor.SelectedNodeOnly {
		t.Fatalf("expected monitor selected-node-only by default")
	}
}

func TestNormalizeCopiesGlobalRefreshIntoPanels(t *testing.T) {
	cfg := Default()
	cfg.Refresh = 2 * time.Second
	cfg.Panels.Dashboard.Interval = 0
	cfg.Panels.Commands.Interval = 0

	cfg.Normalize()

	if cfg.Panels.Dashboard.Interval != 2*time.Second {
		t.Fatalf("expected dashboard interval 2s, got %s", cfg.Panels.Dashboard.Interval)
	}
	if cfg.Panels.Commands.Interval != 2*time.Second {
		t.Fatalf("expected commands interval 2s, got %s", cfg.Panels.Commands.Interval)
	}
	if cfg.Panels.Keys.Interval <= cfg.Refresh {
		t.Fatalf("expected keys interval to stay slower than global refresh, got %s", cfg.Panels.Keys.Interval)
	}
}

func TestNormalizeFillsKeysAndMonitorDefaults(t *testing.T) {
	cfg := Default()
	cfg.Keys.TopK = 0
	cfg.Keys.ScanCount = 0
	cfg.Keys.SampleKeysMax = 0
	cfg.StartupTimeout = 0
	cfg.Monitor.BufferCapacity = 0
	cfg.Monitor.UIRefresh = 0
	cfg.Monitor.Window = 0
	cfg.Monitor.MaxArgLen = 0

	cfg.Normalize()

	if cfg.Keys.TopK != 20 || cfg.Keys.ScanCount != 200 || cfg.Keys.SampleKeysMax != 200 {
		t.Fatalf("expected keys defaults to be restored, got %+v", cfg.Keys)
	}
	if cfg.StartupTimeout != 3*time.Second {
		t.Fatalf("expected startup timeout default, got %s", cfg.StartupTimeout)
	}
	if cfg.Monitor.BufferCapacity != 2000 {
		t.Fatalf("expected monitor buffer default, got %d", cfg.Monitor.BufferCapacity)
	}
	if cfg.Monitor.UIRefresh != 300*time.Millisecond {
		t.Fatalf("expected monitor ui refresh default, got %s", cfg.Monitor.UIRefresh)
	}
	if cfg.Monitor.Window != 5*time.Second {
		t.Fatalf("expected monitor window default, got %s", cfg.Monitor.Window)
	}
	if cfg.Monitor.MaxArgLen != 128 {
		t.Fatalf("expected monitor arg len default, got %d", cfg.Monitor.MaxArgLen)
	}
}
