package store

import (
	"sync"

	"github.com/mingchen/redis-diag/internal/model"
)

type Store struct {
	mu sync.RWMutex

	dashboard   map[string]model.DashboardSnapshot
	clients     map[string]model.ClientsSnapshot
	slowlog     map[string]model.SlowlogSnapshot
	replication map[string]model.ReplicationSnapshot
	bigKeys     map[string]model.BigKeysSnapshot
	commands    map[string]model.CommandStatsSnapshot
}

func New() *Store {
	return &Store{
		dashboard:   make(map[string]model.DashboardSnapshot),
		clients:     make(map[string]model.ClientsSnapshot),
		slowlog:     make(map[string]model.SlowlogSnapshot),
		replication: make(map[string]model.ReplicationSnapshot),
		bigKeys:     make(map[string]model.BigKeysSnapshot),
		commands:    make(map[string]model.CommandStatsSnapshot),
	}
}

func (s *Store) SetDashboard(snapshot model.DashboardSnapshot) {
	if snapshot.NodeID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dashboard[snapshot.NodeID] = snapshot
}

func (s *Store) Dashboard(nodeID string) (model.DashboardSnapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot, ok := s.dashboard[nodeID]
	return snapshot, ok
}

func (s *Store) SetClients(snapshot model.ClientsSnapshot) {
	if snapshot.NodeID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[snapshot.NodeID] = snapshot
}

func (s *Store) Clients(nodeID string) (model.ClientsSnapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot, ok := s.clients[nodeID]
	return snapshot, ok
}

func (s *Store) SetSlowlog(snapshot model.SlowlogSnapshot) {
	if snapshot.NodeID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.slowlog[snapshot.NodeID] = snapshot
}

func (s *Store) Slowlog(nodeID string) (model.SlowlogSnapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot, ok := s.slowlog[nodeID]
	return snapshot, ok
}

func (s *Store) SetReplication(snapshot model.ReplicationSnapshot) {
	if snapshot.NodeID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.replication[snapshot.NodeID] = snapshot
}

func (s *Store) Replication(nodeID string) (model.ReplicationSnapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot, ok := s.replication[nodeID]
	return snapshot, ok
}

func (s *Store) SetBigKeys(snapshot model.BigKeysSnapshot) {
	if snapshot.NodeID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bigKeys[snapshot.NodeID] = snapshot
}

func (s *Store) BigKeys(nodeID string) (model.BigKeysSnapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot, ok := s.bigKeys[nodeID]
	return snapshot, ok
}

func (s *Store) SetCommands(snapshot model.CommandStatsSnapshot) {
	if snapshot.NodeID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.commands[snapshot.NodeID] = snapshot
}

func (s *Store) Commands(nodeID string) (model.CommandStatsSnapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot, ok := s.commands[nodeID]
	return snapshot, ok
}

func (s *Store) PruneNodes(valid map[string]struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for nodeID := range s.dashboard {
		if _, ok := valid[nodeID]; !ok {
			delete(s.dashboard, nodeID)
		}
	}
	for nodeID := range s.clients {
		if _, ok := valid[nodeID]; !ok {
			delete(s.clients, nodeID)
		}
	}
	for nodeID := range s.slowlog {
		if _, ok := valid[nodeID]; !ok {
			delete(s.slowlog, nodeID)
		}
	}
	for nodeID := range s.replication {
		if _, ok := valid[nodeID]; !ok {
			delete(s.replication, nodeID)
		}
	}
	for nodeID := range s.bigKeys {
		if _, ok := valid[nodeID]; !ok {
			delete(s.bigKeys, nodeID)
		}
	}
	for nodeID := range s.commands {
		if _, ok := valid[nodeID]; !ok {
			delete(s.commands, nodeID)
		}
	}
}
