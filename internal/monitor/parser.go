package monitor

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mingchen/redis-diag/internal/model"
)

func ParseLine(nodeID, raw string) (model.MonitorEvent, error) {
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "+"))
	firstSpace := strings.IndexByte(raw, ' ')
	if firstSpace <= 0 {
		return model.MonitorEvent{}, fmt.Errorf("invalid monitor line")
	}
	tsRaw := raw[:firstSpace]
	rest := strings.TrimSpace(raw[firstSpace+1:])
	open := strings.Index(rest, "[")
	close := strings.Index(rest, "]")
	if open < 0 || close < 0 || close <= open {
		return model.MonitorEvent{}, fmt.Errorf("missing monitor metadata")
	}
	meta := strings.Fields(rest[open+1 : close])
	if len(meta) < 2 {
		return model.MonitorEvent{}, fmt.Errorf("invalid monitor metadata")
	}
	db, _ := strconv.Atoi(meta[0])
	args := parseQuoted(strings.TrimSpace(rest[close+1:]))
	command := ""
	if len(args) > 0 {
		command = strings.ToLower(args[0])
		args = args[1:]
	}
	seconds, _ := strconv.ParseFloat(tsRaw, 64)
	at := time.Unix(int64(seconds), int64((seconds-float64(int64(seconds)))*float64(time.Second)))
	return model.MonitorEvent{
		NodeID:     nodeID,
		At:         at,
		DB:         db,
		ClientAddr: meta[1],
		Command:    command,
		Args:       args,
		Raw:        raw,
	}, nil
}

func parseQuoted(raw string) []string {
	out := make([]string, 0)
	for len(raw) > 0 {
		raw = strings.TrimSpace(raw)
		if raw == "" || raw[0] != '"' {
			break
		}
		raw = raw[1:]
		var current strings.Builder
		escaped := false
		quoteFound := false
		for i := 0; i < len(raw); i++ {
			ch := raw[i]
			if escaped {
				current.WriteByte(ch)
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				out = append(out, current.String())
				raw = raw[i+1:]
				quoteFound = true
				break
			}
			current.WriteByte(ch)
		}
		if !quoteFound {
			break
		}
	}
	return out
}
