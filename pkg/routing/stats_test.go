package routing

import (
	"math"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/nickjfree/goose/pkg/message"
	"github.com/nickjfree/goose/pkg/wire"
	"github.com/yl2chen/cidranger"
)

func packet(src, dst string, size int) *message.Packet {
	return &message.Packet{
		Src:  net.ParseIP(src),
		Dst:  net.ParseIP(dst),
		Data: make([]byte, size),
	}
}

// the rates are measured against the wall clock, allow for a little jitter
func approx(t *testing.T, what string, got, want int64) {
	t.Helper()
	if math.Abs(float64(got-want)) > float64(want)/100+1 {
		t.Errorf("%s: got %d bps, want about %d bps", what, got, want)
	}
}

func find(rows []trafficRow, ip string) *trafficRow {
	for i := range rows {
		if rows[i].ip.Equal(net.ParseIP(ip)) {
			return &rows[i]
		}
	}
	return nil
}

// traffic towards an ip is counted as upload, traffic from it as download
func TestRecordDirections(t *testing.T) {

	s := newTrafficStats(10)
	// 1000 bytes from us to the peer, 4000 bytes back
	s.Record(packet("192.168.32.1", "8.8.8.8", 1000), nil, nil)
	s.Record(packet("8.8.8.8", "192.168.32.1", 4000), nil, nil)

	// measure over a known interval, so the rates are predictable
	s.renderedAt = time.Now().Add(-time.Second * 10)
	rows := s.snapshot()

	peer := find(rows, "8.8.8.8")
	if peer == nil {
		t.Fatalf("no counters for the peer, got %+v", rows)
	}
	// 1000 bytes in 10s = 800 bps up, 4000 bytes = 3200 bps down
	approx(t, "peer up", peer.up, 800)
	approx(t, "peer down", peer.down, 3200)
	// the local side is the mirror image
	local := find(rows, "192.168.32.1")
	if local == nil {
		t.Fatalf("no counters for the local address, got %+v", rows)
	}
	approx(t, "local up", local.up, 3200)
	approx(t, "local down", local.down, 800)
}

// rates are measured per interval, not accumulated over the life of the process
func TestRatesAreDeltas(t *testing.T) {

	s := newTrafficStats(10)
	s.Record(packet("10.0.0.1", "10.0.0.2", 1000), nil, nil)
	s.renderedAt = time.Now().Add(-time.Second)
	s.snapshot()

	// same amount of traffic again during the next interval
	s.Record(packet("10.0.0.1", "10.0.0.2", 1000), nil, nil)
	s.renderedAt = time.Now().Add(-time.Second)
	rows := s.snapshot()

	row := find(rows, "10.0.0.2")
	if row == nil {
		t.Fatalf("no counters for the destination, got %+v", rows)
	}
	// still 8000 bps, not 16000. the rate must not accumulate
	approx(t, "second interval", row.up, 8000)
	// an idle interval reports nothing at all
	s.renderedAt = time.Now().Add(-time.Second)
	if rows := s.snapshot(); len(rows) != 0 {
		t.Errorf("idle interval reported %+v, want no rows", rows)
	}
}

// rows are ordered by total rate, the busiest talker first
func TestSnapshotIsSorted(t *testing.T) {

	s := newTrafficStats(10)
	s.Record(packet("10.0.0.1", "1.1.1.1", 100), nil, nil)
	s.Record(packet("10.0.0.1", "2.2.2.2", 5000), nil, nil)
	s.Record(packet("10.0.0.1", "3.3.3.3", 900), nil, nil)

	s.renderedAt = time.Now().Add(-time.Second)
	rows := s.snapshot()
	// the local source ip carries the sum of all three, so it comes first
	want := []string{"10.0.0.1", "2.2.2.2", "3.3.3.3", "1.1.1.1"}
	if len(rows) != len(want) {
		t.Fatalf("got %d rows, want %d: %+v", len(rows), len(want), rows)
	}
	for i, ip := range want {
		if !rows[i].ip.Equal(net.ParseIP(ip)) {
			t.Errorf("row %d: got %s, want %s", i, rows[i].ip, ip)
		}
	}
}

// idle ips are forgotten, so the map does not grow forever
func TestExpireIdleEntries(t *testing.T) {

	s := newTrafficStats(10)
	s.Record(packet("10.0.0.1", "9.9.9.9", 1000), nil, nil)

	stat := s.entry(net.ParseIP("9.9.9.9"))
	stat.updatedAt.Store(time.Now().Add(-statsExpire - time.Second).UnixNano())

	s.renderedAt = time.Now().Add(-time.Second)
	s.snapshot()
	if _, ok := s.stats[string(net.ParseIP("9.9.9.9").To4())]; ok {
		t.Errorf("expired entry is still tracked")
	}
	if _, ok := s.stats[string(net.ParseIP("10.0.0.1").To4())]; !ok {
		t.Errorf("active entry was dropped")
	}
}

// a scan of many destinations must not grow the map without bound
func TestMaxEntries(t *testing.T) {

	s := newTrafficStats(10)
	for i := 0; i < statsMaxEntries+100; i++ {
		ip := net.IPv4(10, byte(i>>16), byte(i>>8), byte(i))
		s.Record(&message.Packet{Src: ip, Dst: ip, Data: make([]byte, 10)}, nil, nil)
	}
	if len(s.stats) > statsMaxEntries {
		t.Errorf("tracking %d ips, want at most %d", len(s.stats), statsMaxEntries)
	}
}

// ipv6 and malformed addresses are ignored rather than counted under a bad key
func TestNonIPv4Ignored(t *testing.T) {

	s := newTrafficStats(10)
	s.Record(&message.Packet{
		Src:  net.ParseIP("2001:4860:4860::8888"),
		Dst:  net.ParseIP("8.8.4.4"),
		Data: make([]byte, 100),
	}, nil, nil)

	if len(s.stats) != 1 {
		t.Errorf("tracking %d ips, want only the ipv4 one", len(s.stats))
	}
}

func TestFormatBps(t *testing.T) {

	cases := []struct {
		bps  int64
		want string
	}{
		{0, "0 bps"},
		{999, "999 bps"},
		{1000, "1.00 Kbps"},
		{1500, "1.50 Kbps"},
		{12400000, "12.40 Mbps"},
		{2500000000, "2.50 Gbps"},
	}
	for _, c := range cases {
		if got := formatBps(c.bps); got != c.want {
			t.Errorf("formatBps(%d) = %s, want %s", c.bps, got, c.want)
		}
	}
}

func TestTruncate(t *testing.T) {

	if got := truncate("short", 10); got != "short" {
		t.Errorf("got %s, want short", got)
	}
	if got := truncate("a-very-long-peer-name", 10); got != "a-very-..." {
		t.Errorf("got %s, want a-very-...", got)
	}
}

// a wire that is only good for giving a port an endpoint
type stubWire struct {
	wire.BaseWire
	endpoint string
}

func (w *stubWire) Endpoint() string {
	return w.endpoint
}

func testPort(endpoint string) *Port {
	return &Port{w: &stubWire{endpoint: endpoint}}
}

func testRouter(t *testing.T) *Router {
	t.Helper()
	return &Router{routeTable: cidranger.NewPCTrieRanger()}
}

func addRoute(t *testing.T, r *Router, cidr string, p *Port, metric int, origin, name string) {
	t.Helper()
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatalf("bad cidr %s: %s", cidr, err)
	}
	entry := &routingEntry{
		network: *network,
		port:    p,
		metric:  metric,
		origin:  origin,
		name:    name,
	}
	if err := r.routeTable.Insert(entry); err != nil {
		t.Fatalf("insert %s: %s", cidr, err)
	}
}

// several nodes can share the lowest metric behind one port. the name of the
// node the port is actually connected to must win over its nameless neighbours
func TestPortNamePrefersTheConnectedPeer(t *testing.T) {

	r := testRouter(t)
	peer := testPort("ipfs/QmT8huihz7rir54HJVXfVA6LvKWhHh93svoHp7KHv2MbBq")
	// wireguard clients behind the peer, same metric, no name and no origin
	addRoute(t, r, "192.168.4.28/32", peer, 2, "", "")
	addRoute(t, r, "192.168.4.29/32", peer, 2, "", "")
	addRoute(t, r, "192.168.4.30/32", peer, 2, "", "")
	// the peer itself, announced with its libp2p id as the origin
	addRoute(t, r, "192.168.101.3/32", peer, 2, "QmT8huihz7rir54HJVXfVA6LvKWhHh93svoHp7KHv2MbBq", "t.nj,jp,us")
	// a named node two hops away
	addRoute(t, r, "10.67.13.233/32", peer, 3, "QmSomeOtherNodeIdentity", "far.away")

	names, _ := r.portNames()
	if got := portLabel(peer, names); got != "t.nj,jp,us" {
		t.Errorf("got %s, want t.nj,jp,us", got)
	}
}

// a peer without a name of its own still beats the raw endpoint
func TestPortNameFallsBackToNamedNeighbour(t *testing.T) {

	r := testRouter(t)
	peer := testPort("ipfs/QmUnnamedPeerIdentity")
	addRoute(t, r, "192.168.4.28/32", peer, 2, "", "")
	addRoute(t, r, "192.168.101.9/32", peer, 3, "QmSomeOtherNodeIdentity", "far.away")

	names, _ := r.portNames()
	if got := portLabel(peer, names); got != "far.away" {
		t.Errorf("got %s, want far.away", got)
	}
}

// with nothing named at all, the endpoint is all we can show
func TestPortNameFallsBackToEndpoint(t *testing.T) {

	r := testRouter(t)
	peer := testPort("ipfs/QmT8huihz7rir54HJVXfVA6LvKWhHh93svoHp7KHv2MbBq")
	addRoute(t, r, "192.168.4.28/32", peer, 2, "", "")

	names, _ := r.portNames()
	got := portLabel(peer, names)
	if !strings.HasPrefix(got, "ipfs/QmT8huihz") || len(got) > statsNameWidth {
		t.Errorf("got %s, want a truncated endpoint", got)
	}
}

// our own tunnel addresses are reported so they can be kept out of the table
func TestPortNamesCollectsLocalAddresses(t *testing.T) {

	r := testRouter(t)
	tunnel := testPort("tun/goose/192.168.32.166/24")
	peer := testPort("ipfs/QmT8huihz7rir54HJVXfVA6LvKWhHh93svoHp7KHv2MbBq")
	addRoute(t, r, "192.168.32.166/32", tunnel, 1, "QmOurOwnIdentity", "me.nj")
	addRoute(t, r, "192.168.101.3/32", peer, 2, "QmT8huihz7rir54HJVXfVA6LvKWhHh93svoHp7KHv2MbBq", "t.nj,jp,us")

	names, locals := r.portNames()
	if !locals[string(net.ParseIP("192.168.32.166").To4())] {
		t.Errorf("our own tunnel address was not reported as local")
	}
	if locals[string(net.ParseIP("192.168.101.3").To4())] {
		t.Errorf("a peer address was reported as local")
	}
	// the tunnel is labelled by its interface, not by our own node name
	if got := portLabel(tunnel, names); got != "tun/goose" {
		t.Errorf("got %s, want tun/goose", got)
	}
}
