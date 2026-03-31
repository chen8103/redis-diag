package model

import "time"

type Quality string

const (
	QualityExact   Quality = "exact"
	QualitySampled Quality = "sampled"
	QualityPartial Quality = "partial"
	QualityStale   Quality = "stale"
)

func (q Quality) IsTerminalSuccess() bool {
	return q == QualityExact || q == QualitySampled || q == QualityPartial
}

type SnapshotMeta struct {
	NodeID      string
	NodeLabel   string
	CollectedAt time.Time
	Duration    time.Duration
	TTL         time.Duration
	Quality     Quality
	Err         string
}

func (m SnapshotMeta) IsFresh(now time.Time) bool {
	if m.CollectedAt.IsZero() {
		return false
	}
	if m.TTL <= 0 {
		return true
	}
	return now.Sub(m.CollectedAt) <= m.TTL
}

type DashboardSnapshot struct {
	SnapshotMeta
	Version                string
	Role                   string
	ConnectedClients       int64
	UsedMemory             uint64
	UsedMemoryRSS          uint64
	InstantaneousOpsPerSec int64
	HitRate                float64
	EvictedKeys            int64
	ExpiredKeys            int64
	Keyspace               map[string]string
}

type ClientRow struct {
	ID    string
	Addr  string
	Age   int64
	Idle  int64
	DB    int
	Flags string
	Cmd   string
	OMem  int64
	QBuf  int64
}

type ClientsSnapshot struct {
	SnapshotMeta
	Rows             []ClientRow
	ConnectedClients int64
	IPStats          map[string]int64
	Limited          bool
	SortBy           string
	Filter           string
}

type SlowlogEntry struct {
	ID         int64
	Timestamp  time.Time
	DurationUS int64
	Command    string
	ClientAddr string
	ClientName string
}

type SlowlogSnapshot struct {
	SnapshotMeta
	Entries    []SlowlogEntry
	Mode       string
	ResetAware bool
}

type ReplicaState struct {
	Addr   string
	State  string
	Offset int64
	Lag    int64
}

type ReplicationSnapshot struct {
	SnapshotMeta
	Role                   string
	MasterHost             string
	MasterLinkStatus       string
	MasterLastIOSecondsAgo int64
	ConnectedSlaves        int64
	Replicas               []ReplicaState
	MasterReplOffset       int64
}

type BigKeyRow struct {
	Key       string
	Type      string
	Size      int64
	SizeUnit  string
	Source    string
	SampledAt time.Time
}

type BigKeysSnapshot struct {
	SnapshotMeta
	Rows            []BigKeyRow
	TopK            int
	ScanCount       int
	SampledKeys     int
	TimeBudget      time.Duration
	RemainingCursor uint64
}

type CommandStatRow struct {
	Command     string
	CallsTotal  int64
	UsecTotal   int64
	UsecPerCall float64
	QPSDelta    float64
}

type CommandStatsSnapshot struct {
	SnapshotMeta
	Rows      []CommandStatRow
	WarmingUp bool
}

type MonitorEvent struct {
	NodeID     string
	At         time.Time
	DB         int
	ClientAddr string
	Command    string
	Args       []string
	Raw        string
}
