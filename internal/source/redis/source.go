package redis

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/mingchen/redis-diag/internal/cluster"
	"github.com/mingchen/redis-diag/internal/config"
	"github.com/mingchen/redis-diag/internal/model"
	"github.com/mingchen/redis-diag/internal/source"
)

type Source struct {
	cfg      config.Config
	rdb      goredis.UniversalClient
	addr     string
	baseOpts *goredis.Options

	nodeMu            sync.RWMutex
	mu                sync.Mutex
	lastCommandCalls  map[string]map[string]int64
	lastCommandSample map[string]time.Time
	slowlogSeen       map[string]map[int64]struct{}
	nodeClients       map[string]nodeClient
}

type nodeClient struct {
	addr   string
	client *goredis.Client
}

func New(cfg config.Config) (*Source, error) {
	resolved, err := config.ResolveTarget(cfg)
	if err != nil {
		return nil, err
	}
	opts := &goredis.UniversalOptions{
		Addrs:                 []string{resolved.Addr},
		Username:              resolved.Username,
		Password:              resolved.Password,
		DB:                    resolved.DB,
		TLSConfig:             resolved.TLSConfig,
		ContextTimeoutEnabled: true,
	}
	if cfg.Target.Cluster {
		opts.IsClusterMode = true
	}
	client := goredis.NewUniversalClient(opts)
	baseOpts := &goredis.Options{
		Addr:                  opts.Addrs[0],
		Username:              opts.Username,
		Password:              opts.Password,
		DB:                    opts.DB,
		TLSConfig:             opts.TLSConfig,
		ContextTimeoutEnabled: true,
	}
	return &Source{
		cfg:               cfg,
		rdb:               client,
		addr:              resolved.Display,
		baseOpts:          baseOpts,
		lastCommandCalls:  make(map[string]map[string]int64),
		lastCommandSample: make(map[string]time.Time),
		slowlogSeen:       make(map[string]map[int64]struct{}),
		nodeClients:       make(map[string]nodeClient),
	}, nil
}

func (s *Source) SetTarget(addr string) error {
	s.nodeMu.Lock()
	defer s.nodeMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.rdb != nil {
		_ = s.rdb.Close()
	}

	opts := &goredis.Options{
		Addr:                  addr,
		Username:              s.baseOpts.Username,
		Password:              s.baseOpts.Password,
		DB:                    s.baseOpts.DB,
		TLSConfig:             s.baseOpts.TLSConfig,
		ContextTimeoutEnabled: true,
	}
	s.rdb = goredis.NewUniversalClient(&goredis.UniversalOptions{
		Addrs:    []string{addr},
		Username: opts.Username,
		Password: opts.Password,
		DB:       opts.DB,
	})
	s.addr = addr
	s.lastCommandCalls = make(map[string]map[string]int64)
	s.lastCommandSample = make(map[string]time.Time)
	s.slowlogSeen = make(map[string]map[int64]struct{})
	s.nodeClients = make(map[string]nodeClient)
	return nil
}

func (s *Source) Close() error {
	s.nodeMu.Lock()
	defer s.nodeMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, client := range s.nodeClients {
		if err := client.client.Close(); err != nil {
			log.Printf("error closing node client: %v", err)
		}
	}
	return s.rdb.Close()
}

func (s *Source) DiscoverNodes(ctx context.Context) ([]cluster.NodeState, error) {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.StartupTimeout)
	defer cancel()
	meta, err := s.capabilities(ctx)
	if err != nil {
		return nil, err
	}
	if s.cfg.Target.Cluster {
		raw, err := s.clusterNodes(ctx)
		if err != nil {
			return nil, err
		}
		nodes := parseClusterNodes(raw)
		s.nodeMu.Lock()
		valid := make(map[string]cluster.NodeState, len(nodes))
		for i := range nodes {
			nodes[i].Capabilities = meta
			nodes[i].LastSuccessAt = time.Now()
			valid[nodes[i].NodeID] = nodes[i]
			s.ensureNodeClientLocked(nodes[i])
		}
		for nodeID, client := range s.nodeClients {
			if _, ok := valid[nodeID]; !ok {
				_ = client.client.Close()
				delete(s.nodeClients, nodeID)
				delete(s.slowlogSeen, nodeID)
				delete(s.lastCommandCalls, nodeID)
				delete(s.lastCommandSample, nodeID)
			}
		}
		s.nodeMu.Unlock()
		return nodes, nil
	}
	node := cluster.NodeState{
		NodeID:        "standalone",
		Addr:          s.addr,
		Role:          "unknown",
		Online:        true,
		LastSeenAt:    time.Now(),
		LastSuccessAt: time.Now(),
		Capabilities:  meta,
	}
	return []cluster.NodeState{node}, nil
}

func (s *Source) FetchDashboard(ctx context.Context, node source.NodeRef) (model.DashboardSnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.Panels.Dashboard.Timeout)
	defer cancel()
	start := time.Now()
	var raw string
	err := s.withNodeClient(node, func(client goredis.Cmdable) error {
		var err error
		raw, err = client.Info(ctx).Result()
		return err
	})
	if err != nil {
		return model.DashboardSnapshot{}, err
	}
	info := parseInfo(raw)
	hits := parseInt64(info["keyspace_hits"])
	misses := parseInt64(info["keyspace_misses"])

	keyspace := make(map[string]string)
	for key, value := range info {
		if strings.HasPrefix(key, "db") {
			keyspace[key] = value
		}
	}

	return model.DashboardSnapshot{
		SnapshotMeta: model.SnapshotMeta{
			NodeID:      node.NodeID,
			NodeLabel:   node.Addr,
			CollectedAt: time.Now(),
			Duration:    time.Since(start),
			TTL:         s.cfg.Panels.Dashboard.Interval * 2,
			Quality:     model.QualityExact,
		},
		Version:                info["redis_version"],
		Role:                   info["role"],
		Uptime:                 parseInt64(info["uptime_in_seconds"]),
		ConnectedClients:       parseInt64(info["connected_clients"]),
		UsedMemory:             parseUint64(info["used_memory"]),
		UsedMemoryRSS:          parseUint64(info["used_memory_rss"]),
		InstantaneousOpsPerSec: parseInt64(info["instantaneous_ops_per_sec"]),
		HitRate:                computeHitRate(hits, misses),
		EvictedKeys:            parseInt64(info["evicted_keys"]),
		ExpiredKeys:            parseInt64(info["expired_keys"]),
		Keyspace:               keyspace,
	}, nil
}

func (s *Source) FetchClients(ctx context.Context, node source.NodeRef, options source.ClientsOptions) (model.ClientsSnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.Panels.Clients.Timeout)
	defer cancel()
	start := time.Now()
	var raw string
	err := s.withNodeClient(node, func(client goredis.Cmdable) error {
		var err error
		raw, err = client.ClientList(ctx).Result()
		return err
	})
	if err != nil {
		return model.ClientsSnapshot{}, err
	}
	sortBy := options.SortBy
	if sortBy == "" {
		sortBy = "idle"
	}
	rows, ipStats := parseClientList(raw, sortBy, options.Filter, options.Limit)
	connected := int64(0)
	if trimmed := strings.TrimSpace(raw); trimmed != "" {
		connected = int64(strings.Count(trimmed, "\n") + 1)
	}
	return model.ClientsSnapshot{
		SnapshotMeta: model.SnapshotMeta{
			NodeID:      node.NodeID,
			NodeLabel:   node.Addr,
			CollectedAt: time.Now(),
			Duration:    time.Since(start),
			TTL:         s.cfg.Panels.Clients.Interval * 2,
			Quality:     model.QualityExact,
		},
		Rows:             rows,
		ConnectedClients: connected,
		IPStats:          ipStats,
		Limited:          options.Limit > 0 && len(rows) >= options.Limit,
		SortBy:           sortBy,
		Filter:           options.Filter,
	}, nil
}

func (s *Source) FetchSlowlog(ctx context.Context, node source.NodeRef, options source.SlowlogOptions) (model.SlowlogSnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.Panels.Slowlog.Timeout)
	defer cancel()
	start := time.Now()
	limit := int64(options.Limit)
	if limit <= 0 {
		limit = 20
	}
	var items []goredis.SlowLog
	err := s.withNodeClient(node, func(client goredis.Cmdable) error {
		var err error
		items, err = client.SlowLogGet(ctx, limit).Result()
		return err
	})
	if err != nil {
		return model.SlowlogSnapshot{}, err
	}
	entries := make([]model.SlowlogEntry, 0, len(items))
	for _, item := range items {
		entries = append(entries, model.SlowlogEntry{
			ID:         item.ID,
			Timestamp:  item.Time,
			DurationUS: int64(item.Duration / time.Microsecond),
			Command:    formatSlowlogCommand(item.Args, s.cfg.Monitor.MaskArgs),
			ClientAddr: item.ClientAddr,
			ClientName: item.ClientName,
		})
	}

	mode := options.Mode
	if mode == "" {
		mode = "latest"
	}
	s.mu.Lock()
	rows, seen := buildSlowlogEntries(entries, s.slowlogSeen[node.NodeID], mode)
	s.slowlogSeen[node.NodeID] = seen
	s.mu.Unlock()

	return model.SlowlogSnapshot{
		SnapshotMeta: model.SnapshotMeta{
			NodeID:      node.NodeID,
			NodeLabel:   node.Addr,
			CollectedAt: time.Now(),
			Duration:    time.Since(start),
			TTL:         s.cfg.Panels.Slowlog.Interval * 2,
			Quality:     model.QualityExact,
		},
		Entries:    rows,
		Mode:       mode,
		ResetAware: true,
	}, nil
}

func (s *Source) FetchReplication(ctx context.Context, node source.NodeRef) (model.ReplicationSnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.Panels.Replication.Timeout)
	defer cancel()
	start := time.Now()
	var raw string
	err := s.withNodeClient(node, func(client goredis.Cmdable) error {
		var err error
		raw, err = client.Info(ctx, "replication").Result()
		return err
	})
	if err != nil {
		return model.ReplicationSnapshot{}, err
	}
	info := parseInfo(raw)
	snapshot := model.ReplicationSnapshot{
		SnapshotMeta: model.SnapshotMeta{
			NodeID:      node.NodeID,
			NodeLabel:   node.Addr,
			CollectedAt: time.Now(),
			Duration:    time.Since(start),
			TTL:         s.cfg.Panels.Replication.Interval * 2,
			Quality:     model.QualityExact,
		},
		Role:                       info["role"],
		MasterHost:                 info["master_host"],
		MasterLinkStatus:           info["master_link_status"],
		MasterLastIOSecondsAgo:     parseInt64(info["master_last_io_seconds_ago"]),
		MasterLinkDownSinceSeconds: parseInt64(info["master_link_down_since_seconds"]),
		ConnectedSlaves:            parseInt64(info["connected_slaves"]),
		MasterReplOffset:           parseInt64(info["master_repl_offset"]),
	}

	for key, value := range info {
		if !strings.HasPrefix(key, "slave") {
			continue
		}
		fields := parseMetricKV(value)
		snapshot.Replicas = append(snapshot.Replicas, model.ReplicaState{
			Addr:   fmt.Sprintf("%s:%s", fields["ip"], fields["port"]),
			State:  fields["state"],
			Offset: parseInt64(fields["offset"]),
			Lag:    parseInt64(fields["lag"]),
		})
	}
	return snapshot, nil
}

func (s *Source) FetchBigKeys(ctx context.Context, node source.NodeRef, options source.BigKeyOptions) (model.BigKeysSnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.Panels.Keys.Timeout)
	defer cancel()
	start := time.Now()
	topK := options.TopK
	if topK <= 0 {
		topK = 20
	}
	scanCount := int64(options.ScanCount)
	if scanCount <= 0 {
		scanCount = 200
	}
	sampleMax := options.SampleKeysMax
	if sampleMax <= 0 {
		sampleMax = 200
	}

	var (
		cursor     uint64
		sampled    int
		candidates []model.BigKeyRow
	)
	deadline := time.Now().Add(s.cfg.Panels.Keys.Timeout)
	for !budgetExpired(deadline) && sampled < sampleMax {
		var keys []string
		var next uint64
		err := s.withNodeClient(node, func(client goredis.Cmdable) error {
			var err error
			keys, next, err = client.Scan(ctx, cursor, "*", scanCount).Result()
			return err
		})
		if err != nil {
			return model.BigKeysSnapshot{}, err
		}
		cursor = next
		for _, key := range keys {
			if budgetExpired(deadline) || sampled >= sampleMax {
				break
			}
			var row model.BigKeyRow
			var ok bool
			err := s.withNodeClient(node, func(client goredis.Cmdable) error {
				row, ok = s.estimateBigKey(ctx, client, key)
				return nil
			})
			if err != nil {
				return model.BigKeysSnapshot{}, err
			}
			if !ok {
				continue
			}
			row.SampledAt = time.Now()
			candidates = append(candidates, row)
			sampled++
		}
		if cursor == 0 {
			break
		}
	}
	rows := keepTopBigKeys(candidates, topK)
	return model.BigKeysSnapshot{
		SnapshotMeta: model.SnapshotMeta{
			NodeID:      node.NodeID,
			NodeLabel:   node.Addr,
			CollectedAt: time.Now(),
			Duration:    time.Since(start),
			TTL:         s.cfg.Panels.Keys.Interval * 2,
			Quality:     model.QualitySampled,
		},
		Rows:            rows,
		TopK:            topK,
		ScanCount:       int(scanCount),
		SampledKeys:     sampled,
		TimeBudget:      s.cfg.Panels.Keys.Timeout,
		RemainingCursor: cursor,
	}, nil
}

func (s *Source) FetchCommandStats(ctx context.Context, node source.NodeRef) (model.CommandStatsSnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.Panels.Commands.Timeout)
	defer cancel()
	start := time.Now()
	var raw string
	err := s.withNodeClient(node, func(client goredis.Cmdable) error {
		var err error
		raw, err = client.Info(ctx, "commandstats").Result()
		return err
	})
	if err != nil {
		return model.CommandStatsSnapshot{}, err
	}

	s.mu.Lock()
	prev := s.lastCommandCalls[node.NodeID]
	if prev == nil {
		prev = make(map[string]int64)
	}
	lastAt := s.lastCommandSample[node.NodeID]
	elapsed := time.Duration(0)
	if !lastAt.IsZero() {
		elapsed = time.Since(lastAt)
	}
	rows := parseCommandStats(raw, prev, elapsed)
	next := make(map[string]int64, len(rows))
	for _, row := range rows {
		next[row.Command] = row.CallsTotal
	}
	s.lastCommandCalls[node.NodeID] = next
	s.lastCommandSample[node.NodeID] = time.Now()
	s.mu.Unlock()

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].QPSDelta == rows[j].QPSDelta {
			return rows[i].CallsTotal > rows[j].CallsTotal
		}
		return rows[i].QPSDelta > rows[j].QPSDelta
	})

	return model.CommandStatsSnapshot{
		SnapshotMeta: model.SnapshotMeta{
			NodeID:      node.NodeID,
			NodeLabel:   node.Addr,
			CollectedAt: time.Now(),
			Duration:    time.Since(start),
			TTL:         s.cfg.Panels.Commands.Interval * 2,
			Quality:     model.QualityExact,
		},
		Rows:      rows,
		WarmingUp: elapsed == 0,
	}, nil
}

func (s *Source) capabilities(ctx context.Context) (cluster.CapabilitySet, error) {
	raw, err := s.rdb.Info(ctx, "server").Result()
	if err != nil {
		return cluster.CapabilitySet{}, err
	}
	info := parseInfo(raw)
	return cluster.CapabilitySet{
		RedisVersion:         info["redis_version"],
		SupportsRole:         cluster.CapabilityUnknown,
		SupportsSlowlog:      cluster.CapabilityUnknown,
		SupportsCommandStats: cluster.CapabilityUnknown,
		SupportsMemoryUsage:  cluster.CapabilityUnknown,
		SupportsCluster:      cluster.CapabilityUnknown,
	}, nil
}

func (s *Source) clusterNodes(ctx context.Context) (string, error) {
	type clusterNodeGetter interface {
		ClusterNodes(ctx context.Context) *goredis.StringCmd
	}
	if client, ok := s.rdb.(clusterNodeGetter); ok {
		return client.ClusterNodes(ctx).Result()
	}
	return s.rdb.Do(ctx, "CLUSTER", "NODES").Text()
}

func (s *Source) ensureNodeClientLocked(node cluster.NodeState) nodeClient {
	if client, ok := s.nodeClients[node.NodeID]; ok {
		if client.addr == node.Addr {
			return client
		}
		_ = client.client.Close()
		delete(s.nodeClients, node.NodeID)
	}
	opts := *s.baseOpts
	opts.Addr = node.Addr
	client := nodeClient{
		addr:   node.Addr,
		client: goredis.NewClient(&opts),
	}
	s.nodeClients[node.NodeID] = client
	return client
}

func (s *Source) withNodeClient(node source.NodeRef, fn func(client goredis.Cmdable) error) error {
	if !s.cfg.Target.Cluster || node.NodeID == "" || node.NodeID == "standalone" {
		return fn(s.rdb)
	}
	s.nodeMu.Lock()
	defer s.nodeMu.Unlock()
	return fn(s.ensureNodeClientLocked(cluster.NodeState{NodeID: node.NodeID, Addr: node.Addr}).client)
}

func formatSlowlogCommand(args []string, mask bool) string {
	if len(args) == 0 {
		return ""
	}
	if !mask {
		return strings.Join(args, " ")
	}
	return args[0] + " <redacted>"
}

func (s *Source) estimateBigKey(ctx context.Context, client goredis.Cmdable, key string) (model.BigKeyRow, bool) {
	row := model.BigKeyRow{Key: key, SizeUnit: "bytes"}
	if size, err := client.MemoryUsage(ctx, key).Result(); err == nil && size > 0 {
		row.Size = size
		row.Type = "memory"
		row.Source = "MEMORY USAGE"
		return row, true
	}

	keyType, err := client.Type(ctx, key).Result()
	if err != nil {
		return model.BigKeyRow{}, false
	}
	row.Type = keyType
	row.Source = keyType
	switch keyType {
	case "string":
		size, err := client.StrLen(ctx, key).Result()
		if err != nil {
			return model.BigKeyRow{}, false
		}
		row.Size = size
	case "hash":
		size, err := client.HLen(ctx, key).Result()
		if err != nil {
			return model.BigKeyRow{}, false
		}
		row.Size = size
		row.SizeUnit = "fields"
	case "list":
		size, err := client.LLen(ctx, key).Result()
		if err != nil {
			return model.BigKeyRow{}, false
		}
		row.Size = size
		row.SizeUnit = "items"
	case "set":
		size, err := client.SCard(ctx, key).Result()
		if err != nil {
			return model.BigKeyRow{}, false
		}
		row.Size = size
		row.SizeUnit = "members"
	case "zset":
		size, err := client.ZCard(ctx, key).Result()
		if err != nil {
			return model.BigKeyRow{}, false
		}
		row.Size = size
		row.SizeUnit = "members"
	case "stream":
		size, err := client.XLen(ctx, key).Result()
		if err != nil {
			return model.BigKeyRow{}, false
		}
		row.Size = size
		row.SizeUnit = "entries"
	default:
		return model.BigKeyRow{}, false
	}
	return row, row.Size > 0
}
