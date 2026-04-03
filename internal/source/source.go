package source

import (
	"context"

	"github.com/mingchen/redis-diag/internal/cluster"
	"github.com/mingchen/redis-diag/internal/model"
)

type NodeRef struct {
	NodeID string
	Addr   string
}

type ClientsOptions struct {
	Limit  int
	SortBy string
	Filter string
}

type SlowlogOptions struct {
	Limit int
	Mode  string
}

type BigKeyOptions struct {
	TopK          int
	ScanCount     int
	SampleKeysMax int
}

type Source interface {
	DiscoverNodes(ctx context.Context) ([]cluster.NodeState, error)
	FetchDashboard(ctx context.Context, node NodeRef) (model.DashboardSnapshot, error)
	FetchClients(ctx context.Context, node NodeRef, options ClientsOptions) (model.ClientsSnapshot, error)
	FetchSlowlog(ctx context.Context, node NodeRef, options SlowlogOptions) (model.SlowlogSnapshot, error)
	FetchReplication(ctx context.Context, node NodeRef) (model.ReplicationSnapshot, error)
	FetchBigKeys(ctx context.Context, node NodeRef, options BigKeyOptions) (model.BigKeysSnapshot, error)
	FetchCommandStats(ctx context.Context, node NodeRef) (model.CommandStatsSnapshot, error)
	SetTarget(addr string) error
}
