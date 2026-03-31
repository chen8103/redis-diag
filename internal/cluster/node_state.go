package cluster

import "time"

type CapabilityStatus string

const (
	CapabilitySupported         CapabilityStatus = "supported"
	CapabilityUnsupported       CapabilityStatus = "unsupported"
	CapabilityUnknown           CapabilityStatus = "unknown"
	CapabilityTemporarilyFailed CapabilityStatus = "temporarily_failed"
)

type CapabilitySet struct {
	RedisVersion         string
	SupportsRole         CapabilityStatus
	SupportsSlowlog      CapabilityStatus
	SupportsCommandStats CapabilityStatus
	SupportsMemoryUsage  CapabilityStatus
	SupportsCluster      CapabilityStatus
}

type NodeState struct {
	NodeID        string
	Addr          string
	Role          string
	MasterID      string
	Online        bool
	LastSeenAt    time.Time
	LastSuccessAt time.Time
	Capabilities  CapabilitySet
}
