package routing

import (
	"fmt"
	"math"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/nickjfree/goose/pkg/message"
	"github.com/yl2chen/cidranger"
)

const (
	// traffic stats entry expire time
	statsExpire = time.Second * 300
	// max ip addresses to track. a port scan should not grow the map forever
	statsMaxEntries = 4096
	// max width of the port column
	statsNameWidth = 28
)

// traffic counters of a single ip address
type ipStat struct {
	// bytes sent to this ip
	up atomic.Int64
	// bytes received from this ip
	down atomic.Int64
	// counters as of the previous render. only touched by the render goroutine
	lastUp   int64
	lastDown int64
	// the port last carrying this ip's traffic
	port atomic.Pointer[Port]
	// last updated, unix nano
	updatedAt atomic.Int64
}

// a single line of the traffic table, rates in bits per second
type trafficRow struct {
	ip   net.IP
	up   int64
	down int64
	port *Port
}

// per ip traffic counters
type TrafficStats struct {
	// lock protects the map itself, the counters are updated atomically
	lock sync.RWMutex
	// counters by ip address. the key is the 4 byte ipv4 address
	stats map[string]*ipStat
	// time of the previous render, the rates are measured against it
	renderedAt time.Time
	// how many talkers to show
	topN int
}

func newTrafficStats(topN int) *TrafficStats {
	return &TrafficStats{
		stats:      make(map[string]*ipStat),
		renderedAt: time.Now(),
		topN:       topN,
	}
}

// account a relayed packet. in is the port it came from, out the port it goes to
func (s *TrafficStats) Record(packet *message.Packet, in, out *Port) {
	// stats are disabled
	if s == nil {
		return
	}
	n := int64(len(packet.Data))
	now := time.Now().UnixNano()
	// we sent n bytes towards dst, out through the target port
	if stat := s.entry(packet.Dst); stat != nil {
		stat.up.Add(n)
		stat.port.Store(out)
		stat.updatedAt.Store(now)
	}
	// we got n bytes from src, in through the source port
	if stat := s.entry(packet.Src); stat != nil {
		stat.down.Add(n)
		stat.port.Store(in)
		stat.updatedAt.Store(now)
	}
}

// get the counters of an ip, creating them on first sight
func (s *TrafficStats) entry(ip net.IP) *ipStat {
	key := string(ip.To4())
	// not an ipv4 address
	if key == "" {
		return nil
	}
	s.lock.RLock()
	stat, ok := s.stats[key]
	s.lock.RUnlock()
	if ok {
		return stat
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	// someone may have created it while we were taking the write lock
	if stat, ok := s.stats[key]; ok {
		return stat
	}
	if len(s.stats) >= statsMaxEntries {
		return nil
	}
	stat = &ipStat{}
	s.stats[key] = stat
	return stat
}

// convert the counters to rates, drop the idle ips and sort by total rate
func (s *TrafficStats) snapshot() []trafficRow {

	now := time.Now()
	s.lock.Lock()
	defer s.lock.Unlock()

	elapsed := now.Sub(s.renderedAt).Seconds()
	s.renderedAt = now
	if elapsed <= 0 {
		return nil
	}
	rows := []trafficRow{}
	for key, stat := range s.stats {
		if now.Sub(time.Unix(0, stat.updatedAt.Load())) > statsExpire {
			// entry expired, stop tracking this ip
			delete(s.stats, key)
			continue
		}
		up, down := stat.up.Load(), stat.down.Load()
		row := trafficRow{
			ip:   net.IP(key),
			up:   int64(math.Round(float64(up-stat.lastUp) * 8 / elapsed)),
			down: int64(math.Round(float64(down-stat.lastDown) * 8 / elapsed)),
			port: stat.port.Load(),
		}
		stat.lastUp, stat.lastDown = up, down
		// no traffic during the last interval
		if row.up == 0 && row.down == 0 {
			continue
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].up+rows[i].down > rows[j].up+rows[j].down
	})
	return rows
}

// how well an entry names the node at the far end of its port. higher is better
func nameScore(entry *routingEntry) int {
	// a node announces its own address with its libp2p id as the origin, so this
	// entry belongs to the node the port is connected to, not one behind it
	if entry.origin != "" && entry.origin == entry.port.PeerID() {
		return 3
	}
	if entry.name != "" {
		return 2
	}
	if entry.origin != "" {
		return 1
	}
	// wireguard clients and other nameless nodes reachable through the port
	return 0
}

// collect the friendly name of every port, and our own tunnel addresses
func (r *Router) portNames() (map[*Port]string, map[string]bool) {

	names := map[*Port]string{}
	locals := map[string]bool{}

	r.lock.Lock()
	defer r.lock.Unlock()

	all, err := r.routeTable.CoveredNetworks(*cidranger.AllIPv4)
	if err != nil {
		logger.Printf("get routings failed with: %s", err)
		return names, locals
	}
	// a node publishes its name on its own /32 routing entry, pick the entry that
	// best describes the node at the other end of each port
	best := map[*Port]*routingEntry{}
	for _, e := range all {
		entry, ok := e.(*routingEntry)
		if !ok {
			continue
		}
		if mask, _ := entry.network.Mask.Size(); mask != 32 {
			continue
		}
		if entry.port.IsTunnel() {
			// our own address, not a remote talker
			locals[string(entry.network.IP.To4())] = true
			continue
		}
		score := nameScore(entry)
		// nameless nodes behind the peer say nothing about the port itself
		if score == 0 {
			continue
		}
		current, ok := best[entry.port]
		if !ok {
			best[entry.port] = entry
			continue
		}
		// the closer node wins, but only between equally good names. several
		// nodes can share the lowest metric behind a single port
		currentScore := nameScore(current)
		if score > currentScore || (score == currentScore && entry.metric < current.metric) {
			best[entry.port] = entry
		}
	}
	for port, entry := range best {
		names[port] = entry.Name()
	}
	return names, locals
}

// name to show for a port. peer names come from the routing table, fall back
// to the endpoint itself
func portLabel(p *Port, names map[*Port]string) string {
	if p == nil {
		return ""
	}
	if p.IsTunnel() {
		// tun/<name>/<address>/<mask>, the interface name is enough
		if seg := strings.Split(p.w.Endpoint(), "/"); len(seg) >= 2 {
			return strings.Join(seg[:2], "/")
		}
	}
	if name, ok := names[p]; ok && name != "" {
		return truncate(name, statsNameWidth)
	}
	return truncate(p.String(), statsNameWidth)
}

// render the top talkers table
func (r *Router) renderTraffic() {

	rows := r.stats.snapshot()
	if len(rows) == 0 {
		return
	}
	names, locals := r.portNames()
	// the domain column is only useful when fakeip resolves the names for us
	withDomain := r.fakeIP != nil

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	header := table.Row{"IP"}
	if withDomain {
		header = append(header, "Domain")
	}
	header = append(header, "Download", "Upload", "Port")
	t.AppendHeader(header)
	t.SetColumnConfigs([]table.ColumnConfig{
		{Name: "Download", Align: text.AlignRight},
		{Name: "Upload", Align: text.AlignRight},
	})

	count := 0
	for _, row := range rows {
		if count >= r.stats.topN {
			break
		}
		// our own tunnel address just mirrors the totals of everyone else
		if locals[string(row.ip.To4())] {
			continue
		}
		line := table.Row{row.ip.String()}
		if withDomain {
			// domains are shown in full, they are the point of the table
			line = append(line, r.fakeIP.Domain(row.ip))
		}
		line = append(line, formatBps(row.down), formatBps(row.up), portLabel(row.port, names))
		t.AppendRow(line)
		count++
	}
	if count > 0 {
		t.Render()
	}
}

// format a rate in bits per second
func formatBps(bps int64) string {
	switch {
	case bps >= 1000*1000*1000:
		return fmt.Sprintf("%.2f Gbps", float64(bps)/(1000*1000*1000))
	case bps >= 1000*1000:
		return fmt.Sprintf("%.2f Mbps", float64(bps)/(1000*1000))
	case bps >= 1000:
		return fmt.Sprintf("%.2f Kbps", float64(bps)/1000)
	}
	return fmt.Sprintf("%d bps", bps)
}

// cut a long name down to size
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
