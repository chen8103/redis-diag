package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/mingchen/redis-diag/internal/config"
	"github.com/mingchen/redis-diag/internal/monitor"
	"github.com/mingchen/redis-diag/internal/runtime"
	redisSource "github.com/mingchen/redis-diag/internal/source/redis"
	"github.com/mingchen/redis-diag/internal/store"
	"github.com/mingchen/redis-diag/internal/ui"
)

var version = "dev"

func main() {
	cfg, showVersion, err := parseConfigFromArgs(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
	if showVersion {
		fmt.Fprintln(os.Stdout, version)
		return
	}

	src, err := redisSource.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	st := store.New()
	engine := runtime.New(cfg, src, st)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := engine.Start(ctx); err != nil {
		log.Fatal(err)
	}

	mon := monitor.NewStream(cfg)
	app := ui.New(cfg, engine, st, mon)
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
	cancel()
	engine.Wait()
	if err := src.Close(); err != nil {
		log.Fatal(err)
	}
}

func parseConfigFromArgs(args []string) (config.Config, bool, error) {
	cfg := config.Default()
	showVersion := false
	fs := flag.NewFlagSet("redis-top", flag.ContinueOnError)

	fs.StringVar(&cfg.Target.Host, "host", cfg.Target.Host, "redis host")
	fs.IntVar(&cfg.Target.Port, "port", cfg.Target.Port, "redis port")
	fs.StringVar(&cfg.Target.URI, "uri", cfg.Target.URI, "redis uri")
	fs.StringVar(&cfg.Target.Username, "username", cfg.Target.Username, "redis acl username")
	fs.StringVar(&cfg.Target.Password, "password", cfg.Target.Password, "redis password")
	fs.StringVar(&cfg.Target.Host, "h", cfg.Target.Host, "redis host (short)")
	fs.IntVar(&cfg.Target.Port, "P", cfg.Target.Port, "redis port (short)")
	fs.StringVar(&cfg.Target.Password, "p", cfg.Target.Password, "redis password (short)")
	fs.BoolVar(&cfg.Target.TLS, "tls", cfg.Target.TLS, "enable tls")
	fs.BoolVar(&cfg.Target.Insecure, "insecure", cfg.Target.Insecure, "skip tls verification")
	fs.BoolVar(&cfg.Target.Cluster, "cluster", cfg.Target.Cluster, "force cluster mode")
	fs.DurationVar(&cfg.StartupTimeout, "startup-timeout", cfg.StartupTimeout, "startup connection timeout")
	fs.DurationVar(&cfg.Refresh, "refresh", cfg.Refresh, "global refresh interval")
	fs.IntVar(&cfg.ClientsLimit, "clients-limit", cfg.ClientsLimit, "max clients shown")
	fs.IntVar(&cfg.SlowlogLimit, "slowlog-limit", cfg.SlowlogLimit, "max slowlog rows shown")

	fs.DurationVar(&cfg.Panels.Keys.Interval, "keys-interval", cfg.Panels.Keys.Interval, "big keys refresh interval")
	fs.DurationVar(&cfg.Panels.Keys.Timeout, "keys-timeout", cfg.Panels.Keys.Timeout, "big keys scan timeout budget")
	fs.IntVar(&cfg.Keys.TopK, "keys-topk", cfg.Keys.TopK, "number of big keys to display")
	fs.IntVar(&cfg.Keys.ScanCount, "keys-scan-count", cfg.Keys.ScanCount, "SCAN count per iteration for big keys sampling")
	fs.IntVar(&cfg.Keys.SampleKeysMax, "keys-sample-max", cfg.Keys.SampleKeysMax, "maximum sampled keys per big keys refresh")

	fs.BoolVar(&cfg.Monitor.Enabled, "monitor", cfg.Monitor.Enabled, "open with monitor enabled")
	fs.IntVar(&cfg.Monitor.BufferCapacity, "monitor-buffer", cfg.Monitor.BufferCapacity, "monitor ring buffer capacity")
	fs.DurationVar(&cfg.Monitor.UIRefresh, "monitor-ui-refresh", cfg.Monitor.UIRefresh, "monitor ui refresh interval")
	fs.DurationVar(&cfg.Monitor.Window, "monitor-window", cfg.Monitor.Window, "monitor aggregate window")
	fs.BoolVar(&cfg.Monitor.MaskArgs, "monitor-mask-args", cfg.Monitor.MaskArgs, "mask monitor arguments")
	fs.IntVar(&cfg.Monitor.MaxArgLen, "monitor-max-arg-len", cfg.Monitor.MaxArgLen, "maximum displayed monitor argument length")
	fs.BoolVar(&cfg.Monitor.SelectedNodeOnly, "monitor-selected-node-only", cfg.Monitor.SelectedNodeOnly, "restrict monitor to selected node")

	fs.BoolVar(&showVersion, "version", false, "print version and exit")
	if err := fs.Parse(args); err != nil {
		return config.Config{}, false, err
	}

	cfg.Normalize()
	return cfg, showVersion, nil
}
