package ui

import (
	"bufio"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/mingchen/redis-diag/internal/cluster"
	"github.com/mingchen/redis-diag/internal/config"
	"github.com/mingchen/redis-diag/internal/monitor"
	engineruntime "github.com/mingchen/redis-diag/internal/runtime"
	"github.com/mingchen/redis-diag/internal/store"
)

type App struct {
	cfg           config.Config
	engine        *engineruntime.Engine
	monitor       *monitor.Stream
	store         *store.Store
	tui           *tview.Application
	root          tview.Primitive
	status        *tview.TextView
	header        *tview.TextView
	nav           *tview.TextView
	body          *tview.TextView
	selectedPanel int
	selectedNode  int
	clientsViewIP bool
	done          chan struct{}
	refreshCh     chan struct{}
}

func New(cfg config.Config, engine *engineruntime.Engine, st *store.Store, mon *monitor.Stream) *App {
	app := &App{
		cfg:           cfg,
		engine:        engine,
		monitor:       mon,
		store:         st,
		tui:           tview.NewApplication(),
		status:        tview.NewTextView().SetDynamicColors(true),
		header:        tview.NewTextView().SetDynamicColors(true),
		nav:           tview.NewTextView().SetDynamicColors(true),
		body:          tview.NewTextView().SetDynamicColors(true),
		selectedPanel: 1,
		selectedNode:  0,
		done:          make(chan struct{}),
		refreshCh:     make(chan struct{}, 1),
	}
	app.header.SetTextAlign(tview.AlignLeft)
	app.nav.SetTextAlign(tview.AlignLeft)
	app.body.SetBorder(true).SetTitle(" Dashboard ")
	app.status.SetTextAlign(tview.AlignLeft)
	app.status.SetBorder(true)

	app.header.SetText(fmt.Sprintf("Target: %s   Cluster: %t   Refresh: %s", formatTarget(cfg), cfg.Target.Cluster, cfg.Refresh))
	app.nav.SetText("[yellow][1]Dashboard [2]Clients [3]Slowlog [4]Replication [5]Keys [6]Commands [7]Monitor[-]")
	app.status.SetText("[green]1/2/3/4/5/6/7[-] Panels   [green]s[-] Sort   [green]f[-] Filter   [green]i[-] IP View   [green]m[-] Mode/Monitor   [green]q[-] Quit")

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(app.header, 1, 0, false).
		AddItem(app.nav, 1, 0, false).
		AddItem(app.body, 0, 1, true).
		AddItem(app.status, 3, 0, false)
	app.root = layout
	app.tui.SetRoot(app.root, true)
	app.tui.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		ch := event.Rune()
		if ch != 0 {
			switch ch {
			case 'q', 'Q':
				close(app.done)
				app.tui.Stop()
				return nil
			case '1', '2', '3', '4', '5', '6', '7':
				panel := int(ch - '0')
				if panel >= 1 && panel <= 7 {
					app.selectedPanel = panel
					app.refresh()
				}
				return nil
			case '\t':
				app.cycleNode()
				return nil
			case 'r':
				app.refresh()
				return nil
			case 's':
				if app.selectedPanel == 2 {
					app.cycleClientSort()
				}
				return nil
			case 'f':
				if app.selectedPanel == 2 {
					app.promptClientFilter()
				}
				return nil
			case 'i':
				if app.selectedPanel == 2 {
					app.clientsViewIP = !app.clientsViewIP
					app.refresh()
				}
				return nil
			case 'm':
				if app.selectedPanel == 3 {
					app.engine.ToggleSlowlogMode()
					app.refresh()
				} else if app.selectedPanel == 7 {
					app.toggleMonitor()
				}
				return nil
			}
		}
		if event.Key() == tcell.KeyCtrlC {
			close(app.done)
			app.tui.Stop()
			return nil
		}
		return event
	})
	return app
}

func (a *App) Run() error {
	a.renderCurrentView()

	refreshTicker := time.NewTicker(a.cfg.Monitor.UIRefresh)
	defer refreshTicker.Stop()

	if a.cfg.Monitor.Enabled {
		a.startMonitor()
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-a.done:
				return
			case <-refreshTicker.C:
				a.tui.QueueUpdateDraw(func() {
					a.renderCurrentView()
				})
			case <-a.refreshCh:
				a.tui.QueueUpdateDraw(func() {
					a.renderCurrentView()
				})
			}
		}
	}()

	err := a.tui.Run()
	select {
	case <-a.done:
	default:
		close(a.done)
	}
	wg.Wait()
	a.monitor.Stop()
	return err
}

func (a *App) refresh() {
	select {
	case a.refreshCh <- struct{}{}:
	default:
	}
}

func (a *App) renderCurrentView() {
	switch a.selectedPanel {
	case 4:
		a.body.SetTitle(" Replication ")
		a.body.SetText(a.renderReplication())
	case 6:
		a.body.SetTitle(" Commands ")
		a.body.SetText(a.renderCommands())
	case 2:
		a.body.SetTitle(" Clients ")
		a.body.SetText(a.renderClients())
	case 3:
		a.body.SetTitle(" Slowlog ")
		a.body.SetText(a.renderSlowlog())
	case 5:
		a.body.SetTitle(" Keys ")
		a.body.SetText(a.renderBigKeys())
	case 7:
		a.body.SetTitle(" Monitor ")
		a.body.SetText(a.renderMonitor())
	default:
		a.body.SetTitle(" Dashboard ")
		a.body.SetText(a.renderDashboard())
	}
}

func (a *App) renderDashboard() string {
	nodes := a.engine.Nodes()
	if len(nodes) == 0 {
		return "Connecting..."
	}
	var b strings.Builder
	for _, node := range nodes {
		snapshot, ok := a.store.Dashboard(node.NodeID)
		fmt.Fprintf(&b, "[yellow]%s[-]\n", node.Addr)
		if errMsg := a.engine.PanelError("dashboard", node.NodeID); errMsg != "" {
			fmt.Fprintf(&b, "  [red]error[-]=%s\n\n", errMsg)
			continue
		}
		if !ok {
			b.WriteString("  waiting for dashboard snapshot...\n\n")
			continue
		}
		fmt.Fprintf(&b, "  role=%s version=%s quality=%s collected=%s\n", snapshot.Role, snapshot.Version, snapshot.Quality, snapshot.CollectedAt.Format(time.RFC3339))
		fmt.Fprintf(&b, "  clients=%d ops/s=%d used_memory=%d rss=%d hit_rate=%.2f%%\n", snapshot.ConnectedClients, snapshot.InstantaneousOpsPerSec, snapshot.UsedMemory, snapshot.UsedMemoryRSS, snapshot.HitRate*100)
		fmt.Fprintf(&b, "  expired=%d evicted=%d\n", snapshot.ExpiredKeys, snapshot.EvictedKeys)
		if len(snapshot.Keyspace) > 0 {
			keys := make([]string, 0, len(snapshot.Keyspace))
			for key := range snapshot.Keyspace {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				fmt.Fprintf(&b, "  %s=%s\n", key, snapshot.Keyspace[key])
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (a *App) renderReplication() string {
	nodes := a.engine.Nodes()
	if len(nodes) == 0 {
		return "Connecting..."
	}
	var b strings.Builder
	for _, node := range nodes {
		snapshot, ok := a.store.Replication(node.NodeID)
		fmt.Fprintf(&b, "[yellow]%s[-]\n", node.Addr)
		if errMsg := a.engine.PanelError("replication", node.NodeID); errMsg != "" {
			fmt.Fprintf(&b, "  [red]error[-]=%s\n\n", errMsg)
			continue
		}
		if !ok {
			b.WriteString("  waiting for replication snapshot...\n\n")
			continue
		}
		fmt.Fprintf(&b, "  role=%s master=%s link=%s last_io=%ds quality=%s\n", snapshot.Role, snapshot.MasterHost, snapshot.MasterLinkStatus, snapshot.MasterLastIOSecondsAgo, snapshot.Quality)
		fmt.Fprintf(&b, "  connected_slaves=%d repl_offset=%d\n", snapshot.ConnectedSlaves, snapshot.MasterReplOffset)
		for _, replica := range snapshot.Replicas {
			fmt.Fprintf(&b, "  replica=%s state=%s offset=%d lag=%d\n", replica.Addr, replica.State, replica.Offset, replica.Lag)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (a *App) renderCommands() string {
	nodes := a.engine.Nodes()
	if len(nodes) == 0 {
		return "Connecting..."
	}
	var b strings.Builder
	b.WriteString("[red]🔴[-] Critical   [orange]🟠[-] Medium   [yellow]🟡[-] Low\n\n")
	for _, node := range nodes {
		snapshot, ok := a.store.Commands(node.NodeID)
		fmt.Fprintf(&b, "[yellow]%s[-]\n", node.Addr)
		if errMsg := a.engine.PanelError("commands", node.NodeID); errMsg != "" {
			fmt.Fprintf(&b, "  [red]error[-]=%s\n\n", errMsg)
			continue
		}
		if !ok {
			b.WriteString("  waiting for command stats snapshot...\n\n")
			continue
		}
		if snapshot.WarmingUp {
			b.WriteString("  warming up delta baseline...\n")
		}
		rows := snapshot.Rows
		if len(rows) > 10 {
			rows = slices.Clone(rows[:10])
		}
		for _, row := range rows {
			color := commandRiskColor(row.Command)
			fmt.Fprintf(&b, "  %s%-16s[-] calls=%-8d qps=%-8.2f usec/call=%.2f\n", color, row.Command, row.CallsTotal, row.QPSDelta, row.UsecPerCall)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func commandRiskColor(cmd string) string {
	switch {
	case cmd == "keys" || cmd == "flushall" || cmd == "flushdb":
		return "[red]"
	case cmd == "sort" || cmd == "hgetall" || cmd == "smembers" || cmd == "sunion" || cmd == "sinter" || cmd == "sdiff" || cmd == "lrange" || cmd == "getrange" || cmd == "substr":
		return "[orange]"
	case cmd == "scan":
		return "[yellow]"
	default:
		return "[green]"
	}
}

func (a *App) renderClients() string {
	nodes := a.engine.Nodes()
	if len(nodes) == 0 {
		return "Connecting..."
	}
	var b strings.Builder
	viewMode := "clients"
	if a.clientsViewIP {
		viewMode = "ip"
	}
	fmt.Fprintf(&b, "sort=%s filter=%q view=%s\n\n", a.engine.ClientSort(), a.engine.ClientFilter(), viewMode)
	for _, node := range nodes {
		snapshot, ok := a.store.Clients(node.NodeID)
		fmt.Fprintf(&b, "[yellow]%s[-]\n", node.Addr)
		if errMsg := a.engine.PanelError("clients", node.NodeID); errMsg != "" {
			fmt.Fprintf(&b, "  [red]error[-]=%s\n\n", errMsg)
			continue
		}
		if !ok {
			b.WriteString("  waiting for clients snapshot...\n\n")
			continue
		}
		fmt.Fprintf(&b, "  connected=%d shown=%d limited=%t quality=%s\n", snapshot.ConnectedClients, len(snapshot.Rows), snapshot.Limited, snapshot.Quality)
		if a.clientsViewIP {
			if len(snapshot.IPStats) > 0 {
				fmt.Fprintf(&b, "  [yellow]IP Overview:[-]\n")
				sortedIPs := make([]string, 0, len(snapshot.IPStats))
				for ip := range snapshot.IPStats {
					sortedIPs = append(sortedIPs, ip)
				}
				sort.Slice(sortedIPs, func(i, j int) bool {
					return snapshot.IPStats[sortedIPs[i]] > snapshot.IPStats[sortedIPs[j]]
				})
				for _, ip := range sortedIPs {
					fmt.Fprintf(&b, "    %-22s %d\n", ip+":", snapshot.IPStats[ip])
				}
			}
		} else {
			fmt.Fprintf(&b, "  [yellow]Clients:[-]\n")
			for _, row := range snapshot.Rows {
				color := commandRiskColor(row.Cmd)
				fmt.Fprintf(&b, "    id=%-6s addr=%-22s idle=%-6d age=%-6d cmd=%s%-12s[-] omem=%d\n", row.ID, row.Addr, row.Idle, row.Age, color, row.Cmd, row.OMem)
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (a *App) renderSlowlog() string {
	nodes := a.engine.Nodes()
	if len(nodes) == 0 {
		return "Connecting..."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "mode=%s\n\n", a.engine.SlowlogMode())
	for _, node := range nodes {
		snapshot, ok := a.store.Slowlog(node.NodeID)
		fmt.Fprintf(&b, "[yellow]%s[-]\n", node.Addr)
		if errMsg := a.engine.PanelError("slowlog", node.NodeID); errMsg != "" {
			fmt.Fprintf(&b, "  [red]error[-]=%s\n\n", errMsg)
			continue
		}
		if !ok {
			b.WriteString("  waiting for slowlog snapshot...\n\n")
			continue
		}
		for _, entry := range snapshot.Entries {
			fmt.Fprintf(&b, "  id=%-6d duration_us=%-8d at=%s cmd=%s\n", entry.ID, entry.DurationUS, entry.Timestamp.Format(time.RFC3339), entry.Command)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (a *App) renderBigKeys() string {
	nodes := a.engine.Nodes()
	if len(nodes) == 0 {
		return "Connecting..."
	}
	var b strings.Builder
	for _, node := range nodes {
		snapshot, ok := a.store.BigKeys(node.NodeID)
		fmt.Fprintf(&b, "[yellow]%s[-]\n", node.Addr)
		if errMsg := a.engine.PanelError("keys", node.NodeID); errMsg != "" {
			fmt.Fprintf(&b, "  [red]error[-]=%s\n\n", errMsg)
			continue
		}
		if !ok {
			b.WriteString("  waiting for sampled big keys...\n\n")
			continue
		}
		fmt.Fprintf(&b, "  quality=%s sampled=%d topk=%d cursor=%d budget=%s\n", snapshot.Quality, snapshot.SampledKeys, snapshot.TopK, snapshot.RemainingCursor, snapshot.TimeBudget)
		for _, row := range snapshot.Rows {
			fmt.Fprintf(&b, "  %-32s type=%-8s size=%-8d unit=%-8s via=%s\n", row.Key, row.Type, row.Size, row.SizeUnit, row.Source)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (a *App) renderMonitor() string {
	node, events, aggregate, errMsg, running := a.monitor.Snapshot()
	var b strings.Builder
	fmt.Fprintf(&b, "running=%t node=%s\n", running, node.Addr)
	if errMsg != "" {
		fmt.Fprintf(&b, "[red]error[-]=%s\n", errMsg)
	}
	fmt.Fprintf(&b, "window_reads=%d window_writes=%d\n\n", aggregate.Reads, aggregate.Writes)
	for _, event := range events {
		fmt.Fprintf(&b, "%s db=%d addr=%s cmd=%s args=%s\n", event.At.Format(time.RFC3339), event.DB, event.ClientAddr, event.Command, strings.Join(event.Args, " "))
	}
	return b.String()
}

func (a *App) cycleClientSort() {
	current := a.engine.ClientSort()
	options := []string{"idle", "age", "cmd", "omem"}
	for idx, option := range options {
		if option == current {
			a.engine.SetClientSort(options[(idx+1)%len(options)])
			a.refresh()
			return
		}
	}
	a.engine.SetClientSort(options[0])
	a.refresh()
}

func (a *App) promptClientFilter() {
	var value string
	a.tui.Suspend(func() {
		fmt.Print("Enter client filter (addr/cmd substring, blank clears): ")
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil && len(line) == 0 {
			value = ""
			return
		}
		value = strings.TrimSpace(line)
	})
	a.engine.SetClientFilter(value)
	a.refresh()
}

func (a *App) toggleMonitor() {
	if a.monitor.Running() {
		a.monitor.Stop()
		a.refresh()
		return
	}
	a.startMonitor()
	a.refresh()
}

func (a *App) startMonitor() {
	if a.cfg.Monitor.SelectedNodeOnly {
		if node, ok := a.currentNode(); ok {
			a.monitor.Start(node)
		}
		return
	}
	for _, node := range a.engine.Nodes() {
		if node.Online {
			a.monitor.Start(node)
			return
		}
	}
}

func (a *App) cycleNode() {
	nodes := a.engine.Nodes()
	if len(nodes) == 0 {
		return
	}
	a.selectedNode = (a.selectedNode + 1) % len(nodes)
	a.refresh()
}

func (a *App) currentNode() (cluster.NodeState, bool) {
	nodes := a.engine.Nodes()
	if len(nodes) == 0 {
		return cluster.NodeState{}, false
	}
	if a.selectedNode >= len(nodes) {
		a.selectedNode = 0
	}
	return nodes[a.selectedNode], true
}

func formatTarget(cfg config.Config) string {
	resolved, err := config.ResolveTarget(cfg)
	if err == nil {
		return resolved.Display
	}
	return fmt.Sprintf("%s:%d", cfg.Target.Host, cfg.Target.Port)
}
