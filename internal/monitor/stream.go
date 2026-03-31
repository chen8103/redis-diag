package monitor

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/mingchen/redis-diag/internal/cluster"
	"github.com/mingchen/redis-diag/internal/config"
	"github.com/mingchen/redis-diag/internal/model"
)

type Stream struct {
	cfg config.Config
	buf *Buffer

	mu      sync.RWMutex
	running bool
	node    cluster.NodeState
	err     string
	cancel  context.CancelFunc
}

func NewStream(cfg config.Config) *Stream {
	return &Stream{
		cfg: cfg,
		buf: NewBuffer(cfg.Monitor.BufferCapacity, cfg.Monitor.Window),
	}
}

func (s *Stream) Start(node cluster.NodeState) {
	s.mu.Lock()
	if s.running {
		if s.cancel != nil {
			s.cancel()
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.running = true
	s.node = node
	s.err = ""
	s.cancel = cancel
	s.buf = NewBuffer(s.cfg.Monitor.BufferCapacity, s.cfg.Monitor.Window)
	s.mu.Unlock()

	go s.run(ctx, node)
}

func (s *Stream) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.running = false
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *Stream) Running() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

func (s *Stream) Snapshot() (cluster.NodeState, []model.MonitorEvent, Aggregate, string, bool) {
	s.mu.RLock()
	node := s.node
	errMsg := s.err
	running := s.running
	s.mu.RUnlock()
	return node, s.buf.EventsInWindow(), s.buf.Aggregate(), errMsg, running
}

func (s *Stream) run(ctx context.Context, node cluster.NodeState) {
	resolved, err := config.ResolveTarget(s.cfg)
	if err != nil {
		s.setError(err)
		return
	}
	conn, err := s.dial(ctx, node.Addr, resolved)
	if err != nil {
		s.setError(err)
		return
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	if err := s.sendAuth(reader, conn); err != nil {
		s.setError(err)
		return
	}
	if err := writeRESP(conn, "MONITOR"); err != nil {
		s.setError(err)
		return
	}
	if err := expectOK(reader); err != nil {
		s.setError(err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			line, err := reader.ReadString('\n')
			if err != nil {
				var netErr net.Error
				if errors.As(err, &netErr) && netErr.Timeout() {
					continue
				}
				s.setError(err)
				return
			}
			event, err := ParseLine(node.NodeID, line)
			if err != nil {
				continue
			}
			if s.cfg.Monitor.MaxArgLen > 0 {
				for i, arg := range event.Args {
					if len(arg) > s.cfg.Monitor.MaxArgLen {
						event.Args[i] = arg[:s.cfg.Monitor.MaxArgLen] + "..."
					}
				}
			}
			if s.cfg.Monitor.MaskArgs {
				for i := range event.Args {
					event.Args[i] = "<redacted>"
				}
			}
			event.Raw = ""
			s.buf.Add(event)
		}
	}
}

func (s *Stream) dial(ctx context.Context, addr string, resolved config.ResolvedTarget) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	if resolved.UseTLS {
		return tls.DialWithDialer(dialer, "tcp", addr, resolved.TLSConfig) //nolint:gosec
	}
	return dialer.DialContext(ctx, "tcp", addr)
}

func (s *Stream) sendAuth(reader *bufio.Reader, conn net.Conn) error {
	resolved, err := config.ResolveTarget(s.cfg)
	if err != nil {
		return err
	}
	if resolved.Username != "" {
		if err := writeRESP(conn, "AUTH", resolved.Username, resolved.Password); err != nil {
			return err
		}
		return expectOK(reader)
	}
	if resolved.Password != "" {
		if err := writeRESP(conn, "AUTH", resolved.Password); err != nil {
			return err
		}
		return expectOK(reader)
	}
	return nil
}

func (s *Stream) setError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = err.Error()
	s.running = false
}

func writeRESP(conn net.Conn, parts ...string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "*%d\r\n", len(parts))
	for _, part := range parts {
		fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(part), part)
	}
	_, err := conn.Write([]byte(b.String()))
	return err
}

func expectOK(reader *bufio.Reader) error {
	line, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "+OK") {
		return fmt.Errorf("redis reply: %s", line)
	}
	return nil
}
