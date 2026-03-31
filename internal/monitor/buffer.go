package monitor

import (
	"sort"
	"sync"
	"time"

	"github.com/mingchen/redis-diag/internal/model"
)

type Aggregate struct {
	ByCommand map[string]int
	ByClient  map[string]int
	Reads     int
	Writes    int
}

type Buffer struct {
	mu       sync.RWMutex
	capacity int
	window   time.Duration
	events   []model.MonitorEvent
	head     int
	count    int
}

func NewBuffer(capacity int, window time.Duration) *Buffer {
	if capacity <= 0 {
		capacity = 2000
	}
	if window <= 0 {
		window = 5 * time.Second
	}
	return &Buffer{
		capacity: capacity,
		window:   window,
		events:   make([]model.MonitorEvent, capacity),
		head:     0,
		count:    0,
	}
}

func (b *Buffer) Add(event model.MonitorEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events[b.head] = event
	b.head = (b.head + 1) % b.capacity
	if b.count < b.capacity {
		b.count++
	}
}

func (b *Buffer) Events() []model.MonitorEvent {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]model.MonitorEvent, b.count)
	for i := 0; i < b.count; i++ {
		idx := (b.head - b.count + i + b.capacity) % b.capacity
		out[i] = b.events[idx]
	}
	return out
}

func (b *Buffer) EventsInWindow() []model.MonitorEvent {
	b.mu.RLock()
	defer b.mu.RUnlock()
	now := time.Now()
	out := make([]model.MonitorEvent, 0, b.count)
	for i := 0; i < b.count; i++ {
		idx := (b.head - b.count + i + b.capacity) % b.capacity
		if now.Sub(b.events[idx].At) <= b.window {
			out = append(out, b.events[idx])
		}
	}
	return out
}

func (b *Buffer) Aggregate() Aggregate {
	b.mu.RLock()
	defer b.mu.RUnlock()
	now := time.Now()
	out := Aggregate{
		ByCommand: make(map[string]int),
		ByClient:  make(map[string]int),
	}
	for i := 0; i < b.count; i++ {
		idx := (b.head - b.count + i + b.capacity) % b.capacity
		event := b.events[idx]
		if now.Sub(event.At) > b.window {
			continue
		}
		out.ByCommand[event.Command]++
		out.ByClient[event.ClientAddr]++
		switch event.Command {
		case "get", "mget", "exists", "ttl", "pttl", "hget", "hmget", "scard", "zrange":
			out.Reads++
		default:
			out.Writes++
		}
	}
	return out
}

func topCounts(input map[string]int, limit int) []string {
	type pair struct {
		key   string
		count int
	}
	pairs := make([]pair, 0, len(input))
	for key, count := range input {
		pairs = append(pairs, pair{key: key, count: count})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].count == pairs[j].count {
			return pairs[i].key < pairs[j].key
		}
		return pairs[i].count > pairs[j].count
	})
	if limit > 0 && len(pairs) > limit {
		pairs = pairs[:limit]
	}
	out := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		out = append(out, pair.key)
	}
	return out
}
