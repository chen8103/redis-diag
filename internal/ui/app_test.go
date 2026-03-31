package ui

import (
	"context"
	"testing"

	"github.com/mingchen/redis-diag/internal/cluster"
	"github.com/mingchen/redis-diag/internal/config"
	"github.com/mingchen/redis-diag/internal/model"
	"github.com/mingchen/redis-diag/internal/monitor"
	engineruntime "github.com/mingchen/redis-diag/internal/runtime"
	"github.com/mingchen/redis-diag/internal/source"
	"github.com/mingchen/redis-diag/internal/store"
)

type nilSource struct{}

func (nilSource) DiscoverNodes(context.Context) ([]cluster.NodeState, error) {
	return []cluster.NodeState{{NodeID: "node-a", Addr: "127.0.0.1:6379", Online: true}}, nil
}

func (nilSource) FetchDashboard(context.Context, source.NodeRef) (model.DashboardSnapshot, error) {
	return model.DashboardSnapshot{}, nil
}

func (nilSource) FetchClients(context.Context, source.NodeRef, source.ClientsOptions) (model.ClientsSnapshot, error) {
	return model.ClientsSnapshot{}, nil
}

func (nilSource) FetchSlowlog(context.Context, source.NodeRef, source.SlowlogOptions) (model.SlowlogSnapshot, error) {
	return model.SlowlogSnapshot{}, nil
}

func (nilSource) FetchReplication(context.Context, source.NodeRef) (model.ReplicationSnapshot, error) {
	return model.ReplicationSnapshot{}, nil
}

func (nilSource) FetchBigKeys(context.Context, source.NodeRef, source.BigKeyOptions) (model.BigKeysSnapshot, error) {
	return model.BigKeysSnapshot{}, nil
}

func (nilSource) FetchCommandStats(context.Context, source.NodeRef) (model.CommandStatsSnapshot, error) {
	return model.CommandStatsSnapshot{}, nil
}

func TestInitialRenderDoesNotNeedApplicationRunLoop(t *testing.T) {
	cfg := config.Default()
	st := store.New()
	engine := engineruntime.New(cfg, nilSource{}, st)
	app := New(cfg, engine, st, monitor.NewStream(cfg))

	app.renderCurrentView()

	if app.body.GetTitle() == "" {
		t.Fatalf("expected body title to be initialized")
	}
}
