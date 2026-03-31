package runtime

import (
	"context"
	"sync"
	"time"

	"github.com/mingchen/redis-diag/internal/cluster"
	"github.com/mingchen/redis-diag/internal/config"
	"github.com/mingchen/redis-diag/internal/poll"
	"github.com/mingchen/redis-diag/internal/source"
	"github.com/mingchen/redis-diag/internal/store"
)

type Engine struct {
	cfg   config.Config
	src   source.Source
	store *store.Store

	scheduler    *poll.Scheduler
	mu           sync.RWMutex
	nodes        []cluster.NodeState
	errors       map[string]map[string]string
	clientSort   string
	clientFilter string
	slowlogMode  string
	wg           sync.WaitGroup
}

func New(cfg config.Config, src source.Source, st *store.Store) *Engine {
	return &Engine{
		cfg:         cfg,
		src:         src,
		store:       st,
		scheduler:   poll.NewScheduler(),
		errors:      make(map[string]map[string]string),
		clientSort:  "idle",
		slowlogMode: "latest",
	}
}

func (e *Engine) Start(ctx context.Context) error {
	nodes, err := e.src.DiscoverNodes(ctx)
	if err != nil {
		return err
	}
	valid := make(map[string]struct{}, len(nodes))
	for _, node := range nodes {
		valid[node.NodeID] = struct{}{}
	}
	e.store.PruneNodes(valid)
	e.mu.Lock()
	e.nodes = nodes
	e.mu.Unlock()

	e.spawnNodeRefresh(ctx)
	e.spawnTicker(ctx, "dashboard", e.cfg.Panels.Dashboard.Interval, e.collectDashboard)
	e.spawnTicker(ctx, "clients", e.cfg.Panels.Clients.Interval, e.collectClients)
	e.spawnTicker(ctx, "slowlog", e.cfg.Panels.Slowlog.Interval, e.collectSlowlog)
	e.spawnTicker(ctx, "replication", e.cfg.Panels.Replication.Interval, e.collectReplication)
	e.spawnTicker(ctx, "keys", e.cfg.Panels.Keys.Interval, e.collectBigKeys)
	e.spawnTicker(ctx, "commands", e.cfg.Panels.Commands.Interval, e.collectCommands)
	return nil
}

func (e *Engine) Nodes() []cluster.NodeState {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]cluster.NodeState, len(e.nodes))
	copy(out, e.nodes)
	return out
}

func (e *Engine) spawnTicker(ctx context.Context, name string, interval time.Duration, collect func(context.Context, cluster.NodeState)) {
	if interval <= 0 {
		interval = time.Second
	}
	e.scheduler.Run(ctx, name, func(runCtx context.Context) {
		e.runCollectionPass(runCtx, collect)
	})

	ticker := time.NewTicker(interval)
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				e.scheduler.Run(ctx, name, func(runCtx context.Context) {
					e.runCollectionPass(runCtx, collect)
				})
			}
		}
	}()
}

func (e *Engine) spawnNodeRefresh(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				nodes, err := e.src.DiscoverNodes(ctx)
				if err != nil {
					e.setError("nodes", "_", err.Error())
					continue
				}
				e.setError("nodes", "_", "")
				valid := make(map[string]struct{}, len(nodes))
				for _, node := range nodes {
					valid[node.NodeID] = struct{}{}
				}
				e.store.PruneNodes(valid)
				e.mu.Lock()
				e.nodes = nodes
				for panel, panelErrors := range e.errors {
					for nodeID := range panelErrors {
						if nodeID == "_" {
							continue
						}
						if _, ok := valid[nodeID]; !ok {
							delete(panelErrors, nodeID)
						}
					}
					e.errors[panel] = panelErrors
				}
				e.mu.Unlock()
			}
		}
	}()
}

func (e *Engine) Wait() {
	e.wg.Wait()
	e.scheduler.Wait()
}

func (e *Engine) runCollectionPass(ctx context.Context, collect func(context.Context, cluster.NodeState)) {
	for _, node := range e.Nodes() {
		if !node.Online {
			continue
		}
		collect(ctx, node)
	}
}

func (e *Engine) collectDashboard(ctx context.Context, node cluster.NodeState) {
	snapshot, err := e.src.FetchDashboard(ctx, source.NodeRef{NodeID: node.NodeID, Addr: node.Addr})
	if err == nil {
		e.setError("dashboard", node.NodeID, "")
		e.store.SetDashboard(snapshot)
		return
	}
	e.setError("dashboard", node.NodeID, err.Error())
}

func (e *Engine) collectReplication(ctx context.Context, node cluster.NodeState) {
	snapshot, err := e.src.FetchReplication(ctx, source.NodeRef{NodeID: node.NodeID, Addr: node.Addr})
	if err == nil {
		e.setError("replication", node.NodeID, "")
		e.store.SetReplication(snapshot)
		return
	}
	e.setError("replication", node.NodeID, err.Error())
}

func (e *Engine) collectCommands(ctx context.Context, node cluster.NodeState) {
	snapshot, err := e.src.FetchCommandStats(ctx, source.NodeRef{NodeID: node.NodeID, Addr: node.Addr})
	if err == nil {
		e.setError("commands", node.NodeID, "")
		e.store.SetCommands(snapshot)
		return
	}
	e.setError("commands", node.NodeID, err.Error())
}

func (e *Engine) PanelError(panel, nodeID string) string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if panelErrors, ok := e.errors[panel]; ok {
		return panelErrors[nodeID]
	}
	return ""
}

func (e *Engine) setError(panel, nodeID, message string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	panelErrors := e.errors[panel]
	if panelErrors == nil {
		panelErrors = make(map[string]string)
		e.errors[panel] = panelErrors
	}
	if message == "" {
		delete(panelErrors, nodeID)
		return
	}
	panelErrors[nodeID] = message
}

func (e *Engine) SetClientSort(sortBy string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.clientSort = sortBy
}

func (e *Engine) ClientSort() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.clientSort
}

func (e *Engine) SetClientFilter(filter string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.clientFilter = filter
}

func (e *Engine) ClientFilter() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.clientFilter
}

func (e *Engine) ToggleSlowlogMode() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.slowlogMode == "new" {
		e.slowlogMode = "latest"
	} else {
		e.slowlogMode = "new"
	}
	return e.slowlogMode
}

func (e *Engine) SlowlogMode() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.slowlogMode
}

func (e *Engine) collectClients(ctx context.Context, node cluster.NodeState) {
	if snapshot, ok := e.store.Dashboard(node.NodeID); ok {
		if snapshot.ConnectedClients > 5000 {
			if latest, ok := e.store.Clients(node.NodeID); ok &&
				time.Since(latest.CollectedAt) < 5*time.Second &&
				latest.SortBy == e.ClientSort() &&
				latest.Filter == e.ClientFilter() {
				return
			}
		} else if snapshot.ConnectedClients > 1000 {
			if latest, ok := e.store.Clients(node.NodeID); ok &&
				time.Since(latest.CollectedAt) < 3*time.Second &&
				latest.SortBy == e.ClientSort() &&
				latest.Filter == e.ClientFilter() {
				return
			}
		}
	}
	snapshot, err := e.src.FetchClients(ctx, source.NodeRef{NodeID: node.NodeID, Addr: node.Addr}, source.ClientsOptions{
		Limit:  e.cfg.ClientsLimit,
		SortBy: e.ClientSort(),
		Filter: e.ClientFilter(),
	})
	if err == nil {
		e.setError("clients", node.NodeID, "")
		e.store.SetClients(snapshot)
		return
	}
	e.setError("clients", node.NodeID, err.Error())
}

func (e *Engine) collectSlowlog(ctx context.Context, node cluster.NodeState) {
	snapshot, err := e.src.FetchSlowlog(ctx, source.NodeRef{NodeID: node.NodeID, Addr: node.Addr}, source.SlowlogOptions{
		Limit: e.cfg.SlowlogLimit,
		Mode:  e.SlowlogMode(),
	})
	if err == nil {
		e.setError("slowlog", node.NodeID, "")
		e.store.SetSlowlog(snapshot)
		return
	}
	e.setError("slowlog", node.NodeID, err.Error())
}

func (e *Engine) collectBigKeys(ctx context.Context, node cluster.NodeState) {
	snapshot, err := e.src.FetchBigKeys(ctx, source.NodeRef{NodeID: node.NodeID, Addr: node.Addr}, source.BigKeyOptions{
		TopK:          e.cfg.Keys.TopK,
		ScanCount:     e.cfg.Keys.ScanCount,
		SampleKeysMax: e.cfg.Keys.SampleKeysMax,
	})
	if err == nil {
		e.setError("keys", node.NodeID, "")
		e.store.SetBigKeys(snapshot)
		return
	}
	e.setError("keys", node.NodeID, err.Error())
}
