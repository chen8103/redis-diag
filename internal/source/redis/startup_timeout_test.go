package redis

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/mingchen/redis-diag/internal/config"
)

func TestDiscoverNodesHonorsStartupTimeout(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				time.Sleep(2 * time.Second)
			}(conn)
		}
	}()

	cfg := config.Default()
	cfg.Target.Host = "127.0.0.1"
	cfg.Target.Port = ln.Addr().(*net.TCPAddr).Port
	cfg.StartupTimeout = 50 * time.Millisecond

	src, err := New(cfg)
	if err != nil {
		t.Fatalf("new source failed: %v", err)
	}
	defer src.Close()

	done := make(chan error, 1)
	go func() {
		_, err := src.DiscoverNodes(context.Background())
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("expected startup timeout error")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("discover nodes did not return within expected timeout")
	}
}
