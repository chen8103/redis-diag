package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/mingchen/redis-diag/internal/cluster"
	"github.com/mingchen/redis-diag/internal/config"
	"github.com/mingchen/redis-diag/internal/model"
	"github.com/mingchen/redis-diag/internal/source"
	"github.com/mingchen/redis-diag/internal/store"
)

type fakeSource struct{}

func (f fakeSource) DiscoverNodes(ctx context.Context) ([]cluster.NodeState, error) {
	return []cluster.NodeState{{NodeID: "node-a", Addr: "127.0.0.1:6379", Online: true}}, nil
}

func (f fakeSource) FetchDashboard(ctx context.Context, node source.NodeRef) (model.DashboardSnapshot, error) {
	return model.DashboardSnapshot{SnapshotMeta: model.SnapshotMeta{NodeID: node.NodeID, Quality: model.QualityExact}, Role: "master"}, nil
}

func (f fakeSource) FetchClients(ctx context.Context, node source.NodeRef, options source.ClientsOptions) (model.ClientsSnapshot, error) {
	return model.ClientsSnapshot{}, nil
}

func (f fakeSource) FetchSlowlog(ctx context.Context, node source.NodeRef, options source.SlowlogOptions) (model.SlowlogSnapshot, error) {
	return model.SlowlogSnapshot{}, nil
}

func (f fakeSource) FetchReplication(ctx context.Context, node source.NodeRef) (model.ReplicationSnapshot, error) {
	return model.ReplicationSnapshot{SnapshotMeta: model.SnapshotMeta{NodeID: node.NodeID, Quality: model.QualityExact}, Role: "master"}, nil
}

func (f fakeSource) FetchBigKeys(ctx context.Context, node source.NodeRef, options source.BigKeyOptions) (model.BigKeysSnapshot, error) {
	return model.BigKeysSnapshot{}, nil
}

func (f fakeSource) FetchCommandStats(ctx context.Context, node source.NodeRef) (model.CommandStatsSnapshot, error) {
	return model.CommandStatsSnapshot{SnapshotMeta: model.SnapshotMeta{NodeID: node.NodeID, Quality: model.QualityExact}, WarmingUp: true}, nil
}

func TestEngineStartLoadsCoreSnapshots(t *testing.T) {
	cfg := config.Default()
	cfg.Panels.Dashboard.Interval = 20 * time.Millisecond
	cfg.Panels.Replication.Interval = 20 * time.Millisecond
	cfg.Panels.Commands.Interval = 20 * time.Millisecond
	cfg.Normalize()

	st := store.New()
	engine := New(cfg, fakeSource{}, st)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := engine.Start(ctx); err != nil {
		t.Fatalf("start failed: %v", err)
	}

	time.Sleep(60 * time.Millisecond)

	if _, ok := st.Dashboard("node-a"); !ok {
		t.Fatalf("expected dashboard snapshot")
	}
	if _, ok := st.Replication("node-a"); !ok {
		t.Fatalf("expected replication snapshot")
	}
	if _, ok := st.Commands("node-a"); !ok {
		t.Fatalf("expected command stats snapshot")
	}
}
