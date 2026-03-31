package config

import "time"

type Target struct {
	Host     string
	Port     int
	URI      string
	Username string
	Password string
	TLS      bool
	Insecure bool
	Cluster  bool
}

type PanelConfig struct {
	Interval time.Duration
	Timeout  time.Duration
}

type Panels struct {
	Dashboard   PanelConfig
	Clients     PanelConfig
	Slowlog     PanelConfig
	Replication PanelConfig
	Keys        PanelConfig
	Commands    PanelConfig
	MonitorUI   PanelConfig
}

type MonitorConfig struct {
	Enabled          bool
	BufferCapacity   int
	UIRefresh        time.Duration
	Window           time.Duration
	MaskArgs         bool
	MaxArgLen        int
	SelectedNodeOnly bool
}

type KeysConfig struct {
	TopK          int
	ScanCount     int
	SampleKeysMax int
}

type Config struct {
	Target       Target
	StartupTimeout time.Duration
	Refresh      time.Duration
	ClientsLimit int
	SlowlogLimit int
	Keys         KeysConfig
	Panels       Panels
	Monitor      MonitorConfig
}

func Default() Config {
	cfg := Config{
		Target: Target{
			Host: "127.0.0.1",
			Port: 6379,
		},
		StartupTimeout: 3 * time.Second,
		Refresh:      time.Second,
		ClientsLimit: 20,
		SlowlogLimit: 20,
		Keys: KeysConfig{
			TopK:          20,
			ScanCount:     200,
			SampleKeysMax: 200,
		},
		Panels: Panels{
			Dashboard:   PanelConfig{Timeout: 500 * time.Millisecond},
			Clients:     PanelConfig{Interval: time.Second, Timeout: 700 * time.Millisecond},
			Slowlog:     PanelConfig{Interval: 2 * time.Second, Timeout: 700 * time.Millisecond},
			Replication: PanelConfig{Interval: 2 * time.Second, Timeout: 500 * time.Millisecond},
			Keys:        PanelConfig{Interval: 15 * time.Second, Timeout: 1500 * time.Millisecond},
			Commands:    PanelConfig{Timeout: 700 * time.Millisecond},
			MonitorUI:   PanelConfig{Interval: 300 * time.Millisecond, Timeout: 100 * time.Millisecond},
		},
		Monitor: MonitorConfig{
			BufferCapacity:   2000,
			UIRefresh:        300 * time.Millisecond,
			Window:           5 * time.Second,
			MaskArgs:         true,
			MaxArgLen:        128,
			SelectedNodeOnly: true,
		},
	}
	cfg.Normalize()
	return cfg
}

func (c *Config) Normalize() {
	if c.Refresh <= 0 {
		c.Refresh = time.Second
	}
	if c.StartupTimeout <= 0 {
		c.StartupTimeout = 3 * time.Second
	}
	copyGlobal := func(panel *PanelConfig) {
		if panel.Interval <= 0 {
			panel.Interval = c.Refresh
		}
	}
	copyGlobal(&c.Panels.Dashboard)
	copyGlobal(&c.Panels.Commands)

	if c.Panels.Clients.Interval <= 0 {
		c.Panels.Clients.Interval = c.Refresh
	}
	if c.Panels.Slowlog.Interval <= 0 {
		c.Panels.Slowlog.Interval = 2 * time.Second
	}
	if c.Panels.Replication.Interval <= 0 {
		c.Panels.Replication.Interval = 2 * time.Second
	}
	if c.Panels.Keys.Interval <= 0 {
		c.Panels.Keys.Interval = 15 * time.Second
	}
	if c.Panels.Keys.Timeout <= 0 {
		c.Panels.Keys.Timeout = 1500 * time.Millisecond
	}
	if c.ClientsLimit <= 0 {
		c.ClientsLimit = 20
	}
	if c.SlowlogLimit <= 0 {
		c.SlowlogLimit = 20
	}
	if c.Keys.TopK <= 0 {
		c.Keys.TopK = 20
	}
	if c.Keys.ScanCount <= 0 {
		c.Keys.ScanCount = 200
	}
	if c.Keys.SampleKeysMax <= 0 {
		c.Keys.SampleKeysMax = 200
	}
	if c.Monitor.BufferCapacity <= 0 {
		c.Monitor.BufferCapacity = 2000
	}
	if c.Monitor.UIRefresh <= 0 {
		c.Monitor.UIRefresh = 300 * time.Millisecond
	}
	if c.Monitor.Window <= 0 {
		c.Monitor.Window = 5 * time.Second
	}
	if c.Monitor.MaxArgLen <= 0 {
		c.Monitor.MaxArgLen = 128
	}
}
