package redis

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mingchen/redis-diag/internal/cluster"
	"github.com/mingchen/redis-diag/internal/model"
)

func parseInfo(raw string) map[string]string {
	out := make(map[string]string)
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		out[key] = value
	}
	return out
}

func parseCommandStats(raw string, previous map[string]int64, elapsed time.Duration) []model.CommandStatRow {
	rows := make([]model.CommandStatRow, 0)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || !strings.HasPrefix(line, "cmdstat_") {
			continue
		}
		key, payload, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		command := strings.TrimPrefix(key, "cmdstat_")
		fields := parseMetricKV(payload)
		calls := parseInt64(fields["calls"])
		usec := parseInt64(fields["usec"])
		usecPerCall, _ := strconv.ParseFloat(fields["usec_per_call"], 64)

		row := model.CommandStatRow{
			Command:     command,
			CallsTotal:  calls,
			UsecTotal:   usec,
			UsecPerCall: usecPerCall,
		}
		if elapsed > 0 {
			if prev, ok := previous[command]; ok && calls >= prev {
				row.QPSDelta = float64(calls-prev) / elapsed.Seconds()
			}
		}
		rows = append(rows, row)
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].CallsTotal == rows[j].CallsTotal {
			return rows[i].Command < rows[j].Command
		}
		return rows[i].CallsTotal > rows[j].CallsTotal
	})
	return rows
}

func parseMetricKV(raw string) map[string]string {
	out := make(map[string]string)
	for _, part := range strings.Split(raw, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		out[key] = value
	}
	return out
}

func parseInt64(raw string) int64 {
	value, _ := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	return value
}

func parseUint64(raw string) uint64 {
	value, _ := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
	return value
}

func computeHitRate(hits, misses int64) float64 {
	total := hits + misses
	if total <= 0 {
		return 0
	}
	return float64(hits) / float64(total)
}

func parseClientList(raw, sortBy, filter string, limit int) ([]model.ClientRow, map[string]int64) {
	rows := make([]model.ClientRow, 0)
	ipStats := make(map[string]int64)
	filter = strings.ToLower(strings.TrimSpace(filter))
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := make(map[string]string)
		for _, part := range strings.Split(line, " ") {
			key, value, ok := strings.Cut(part, "=")
			if !ok {
				continue
			}
			fields[key] = value
		}
		addr := fields["addr"]
		if ip, _, ok := strings.Cut(addr, ":"); ok {
			ipStats[ip]++
		}
		row := model.ClientRow{
			ID:    fields["id"],
			Addr:  addr,
			Age:   parseInt64(fields["age"]),
			Idle:  parseInt64(fields["idle"]),
			DB:    int(parseInt64(fields["db"])),
			Flags: fields["flags"],
			Cmd:   fields["cmd"],
			OMem:  parseInt64(fields["omem"]),
			QBuf:  parseInt64(fields["qbuf"]),
		}
		if filter != "" {
			haystack := strings.ToLower(fmt.Sprintf("%s %s %s", row.Addr, row.Cmd, row.Flags))
			if !strings.Contains(haystack, filter) {
				continue
			}
		}
		rows = append(rows, row)
	}

	sort.Slice(rows, func(i, j int) bool {
		switch sortBy {
		case "age":
			return rows[i].Age > rows[j].Age
		case "cmd":
			if rows[i].Cmd == rows[j].Cmd {
				return rows[i].Idle < rows[j].Idle
			}
			if rows[i].Cmd == "" {
				return false
			}
			if rows[j].Cmd == "" {
				return true
			}
			return rows[i].Cmd < rows[j].Cmd
		case "omem":
			return rows[i].OMem > rows[j].OMem
		default:
			return rows[i].Idle < rows[j].Idle
		}
	})
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, ipStats
}

func buildSlowlogEntries(entries []model.SlowlogEntry, seen map[int64]struct{}, mode string) ([]model.SlowlogEntry, map[int64]struct{}) {
	if seen == nil {
		seen = make(map[int64]struct{})
	}
	rows := make([]model.SlowlogEntry, 0, len(entries))
	nextSeen := make(map[int64]struct{}, len(entries))
	for _, entry := range entries {
		_, known := seen[entry.ID]
		nextSeen[entry.ID] = struct{}{}
		if mode == "new" && known {
			continue
		}
		rows = append(rows, entry)
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].ID > rows[j].ID
	})
	return rows, nextSeen
}

func parseClusterNodes(raw string) []cluster.NodeState {
	nodes := make([]cluster.NodeState, 0)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 8 {
			continue
		}
		nodeID := parts[0]
		addr := parts[1]
		if hostPort, _, ok := strings.Cut(addr, "@"); ok {
			addr = hostPort
		}
		flags := strings.Split(parts[2], ",")
		role := "master"
		if slicesContains(flags, "slave") {
			role = "replica"
		}
		online := !slicesContains(flags, "fail")
		if len(parts) > 7 && parts[7] == "connected" {
			online = online && true
		} else {
			online = false
		}
		masterID := parts[3]
		if masterID == "-" {
			masterID = ""
		}
		nodes = append(nodes, cluster.NodeState{
			NodeID:     nodeID,
			Addr:       addr,
			Role:       role,
			MasterID:   masterID,
			Online:     online,
			LastSeenAt: time.Now(),
		})
	}
	return nodes
}

func slicesContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func keepTopBigKeys(rows []model.BigKeyRow, topK int) []model.BigKeyRow {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Size == rows[j].Size {
			return rows[i].Key < rows[j].Key
		}
		return rows[i].Size > rows[j].Size
	})
	if topK > 0 && len(rows) > topK {
		return rows[:topK]
	}
	return rows
}

func budgetExpired(deadline time.Time) bool {
	return !deadline.IsZero() && time.Now().After(deadline)
}
