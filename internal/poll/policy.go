package poll

import (
	"time"

	"github.com/mingchen/redis-diag/internal/config"
)

type PanelPolicy struct {
	Interval time.Duration
	Timeout  time.Duration
}

type Policy struct {
	Dashboard     PanelPolicy
	Clients       PanelPolicy
	Slowlog       PanelPolicy
	Replication   PanelPolicy
	Keys          PanelPolicy
	Commands      PanelPolicy
	NodeRefresh   PanelPolicy
	MonitorUI     PanelPolicy
	MaxConcurrency int
}

func DefaultPolicy() Policy {
	return FromConfig(config.Default())
}

func FromConfig(cfg config.Config) Policy {
	cfg.Normalize()
	return Policy{
		Dashboard:      PanelPolicy{Interval: cfg.Panels.Dashboard.Interval, Timeout: cfg.Panels.Dashboard.Timeout},
		Clients:        PanelPolicy{Interval: cfg.Panels.Clients.Interval, Timeout: cfg.Panels.Clients.Timeout},
		Slowlog:        PanelPolicy{Interval: cfg.Panels.Slowlog.Interval, Timeout: cfg.Panels.Slowlog.Timeout},
		Replication:    PanelPolicy{Interval: cfg.Panels.Replication.Interval, Timeout: cfg.Panels.Replication.Timeout},
		Keys:           PanelPolicy{Interval: cfg.Panels.Keys.Interval, Timeout: cfg.Panels.Keys.Timeout},
		Commands:       PanelPolicy{Interval: cfg.Panels.Commands.Interval, Timeout: cfg.Panels.Commands.Timeout},
		NodeRefresh:    PanelPolicy{Interval: 10 * time.Second, Timeout: time.Second},
		MonitorUI:      PanelPolicy{Interval: cfg.Monitor.UIRefresh, Timeout: cfg.Panels.MonitorUI.Timeout},
		MaxConcurrency: 8,
	}
}

type TickState struct {
	Running bool
}

func ShouldSkip(state TickState) bool {
	return state.Running
}
