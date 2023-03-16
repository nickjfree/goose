package routing

import (
	"fmt"
	"net"
	"runtime"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/yl2chen/cidranger"

	"goose/pkg/message"
	"goose/pkg/routing/fakeip"
)

const (
	// idle timeout
	idleTimeout = time.Second * 300
	// routing interval
	routingInterval = time.Second * 30
	// routing entry expire time
	routingExpire = time.Second * 90
	// default routing
	defaultRouting = "0.0.0.0/0"
)

// routing entry
type routingEntry struct {
	// network
	network net.IPNet
	// port
	port *Port
	// metric
	metric int
	// rtt
	rtt int
	// origin
	origin string
	// last updated
	updatedAt time.Time
}

func (entry *routingEntry) Network() net.IPNet {
	return entry.network
}

func (entry *routingEntry) String() string {
	return fmt.Sprintf("%s -> %s metric %d %s rtt %dms", entry.network.String(), entry.port, entry.metric%10, entry.origin, entry.rtt)
}

type portState struct {
	updatedAt time.Time
	// routing entries
	routings []routingEntry
}

// router
type Router struct {
	// connector
	Connector
	// id
	id string
	// lock
	lock sync.Mutex
	// port routing infos
	portStats map[*Port]portState
	// forward networks
	forwardCIDRs []string
	// provided networks from local networks
	localNets []net.IPNet
	// route table
	routeTable cidranger.Ranger
	// max metric allowed
	maxMetric int
	// fake ip manager
	fakeIP *fakeip.FakeIPManager
	// closed
	closed chan struct{}
}

func NewRouter(localcidr string, opts ...Option) *Router {
	localNets := []net.IPNet{}
	// ipaddress
	address, _, err := net.ParseCIDR(localcidr)
	if err != nil {
		logger.Fatal(err)
	}
	// local ip/32
	localNets = append(localNets, net.IPNet{
		IP:   address,
		Mask: net.CIDRMask(32, 32),
	})
	r := &Router{
		id:         uuid.New().String(),
		portStats:  make(map[*Port]portState),
		routeTable: cidranger.NewPCTrieRanger(),
		localNets:  localNets,
		closed:     make(chan struct{}),
	}
	for _, opt := range opts {
		if err := opt(r); err != nil {
			logger.Fatal(err)
		}
	}
	go r.background()
	return r
}

func (r *Router) RegisterPort(p *Port) error {
	// handle the port
	r.lock.Lock()
	defer r.lock.Unlock()
	r.portStats[p] = portState{
		updatedAt: time.Now(),
	}

	go func() {
		logger.Printf("traffic quite for port(%s) %s", p, r.handleTraffic(p))
		r.clearRouting(p)
	}()

	go func() {
		logger.Printf("routing quite for port(%s) %s", p, r.handleRouting(p))
		r.clearRouting(p)
	}()
	return nil
}

// update single routing entry
func (r *Router) updateEntry(myEntry, peerEntry *routingEntry) error {

	// if a address with same ip but diffrent origin is detected
	mask, _ := peerEntry.network.Mask.Size()
	if myEntry.origin != "" && peerEntry.origin != "" &&
		myEntry.port != peerEntry.port &&
		myEntry.origin != peerEntry.origin && mask == 32 {
		return errors.Errorf("conflicting address %s", peerEntry.network.String())
	}

	// update routings for entries with smaller metric
	if myEntry.metric > peerEntry.metric {
		myEntry.port = peerEntry.port
		myEntry.metric = peerEntry.metric
		myEntry.rtt = peerEntry.rtt
		myEntry.origin = peerEntry.origin
		myEntry.updatedAt = time.Now()
		return nil
	}
	// same metric, select the one with smaller rtt
	if myEntry.metric == peerEntry.metric {

		if myEntry.port == peerEntry.port ||
			(myEntry.port != peerEntry.port && peerEntry.port.Faster(myEntry.rtt-peerEntry.port.Rtt())) {
			myEntry.port = peerEntry.port
			myEntry.metric = peerEntry.metric
			myEntry.rtt = peerEntry.rtt
			myEntry.origin = peerEntry.origin
			myEntry.updatedAt = time.Now()
		}
	}

	// TODO: metric + rtt

	return nil
}

// handle conflict routing response
func (r *Router) handleConflict(p *Port, routing message.Routing) error {
	// just redirect the message to target port

	for _, entry := range routing.Routings {

		target, err := r.FindDestPort(entry.Network.IP)
		if err != nil {
			return err
		}
		if entry.Metric < 0 {
			continue
		}
		entry.Metric = entry.Metric - 1
		msg := message.Routing{
			Type:     message.RoutingRegisterAck,
			Routings: []message.RoutingEntry{entry},
			Message:  "conflict",
		}
		logger.Printf("get a conflict message %+v", entry)
		if err := target.AnnouceRouting(&msg); err != nil {
			target.Close()
		}
	}
	return nil
}

// update routing tables for this port
func (r *Router) UpdateRouting(p *Port, routing message.Routing) error {

	// handle routing ack. to get the peer rtt
	if routing.Type == message.RoutingRegisterAck {
		p.EndRttTiming()
		// handle conflict routing message if needed
		if err := r.handleConflict(p, routing); err != nil {
			return err
		}
		return nil
	}
	// conflict entries to reply to peers
	conflictEntries := []message.RoutingEntry{}
	// log the peer provided networks
	err := func() error {
		r.lock.Lock()
		defer r.lock.Unlock()
		for _, entry := range routing.Routings {
			peerEntry := routingEntry{
				network: entry.Network,
				port:    p,
				// inc distance
				metric:    entry.Metric + 1,
				rtt:       p.Rtt() + entry.Rtt,
				origin:    entry.Origin,
				updatedAt: time.Now(),
			}
			// routings reach max hops
			if peerEntry.metric >= r.maxMetric {
				continue
			}
			// find the same network
			containing, err := r.routeTable.CoveredNetworks(peerEntry.network)
			if err != nil {
				return errors.WithStack(err)
			}
			matched := false
			for _, e := range containing {
				if myEntry, ok := e.(*routingEntry); ok {
					n := myEntry.Network()
					// duplicated network
					if n.String() == peerEntry.network.String() {
						matched = true
						// only update routings for entries with smaller metric
						if err := r.updateEntry(myEntry, &peerEntry); err != nil {
							conflictEntries = append(conflictEntries, entry)
						}
						break
					}
				}
			}
			// not matched, new routing info
			if !matched {
				if err := r.routeTable.Insert(&peerEntry); err != nil {
					return errors.WithStack(err)
				}
			}
		}
		if state, ok := r.portStats[p]; ok {
			state.updatedAt = time.Now()
			r.portStats[p] = state
		}
		return nil
	}()

	// if there is error in routing update for this port. close this port
	if err != nil {
		defer p.Close()
		msg := message.Routing{
			Type:     message.RoutingRegisterFailed,
			Routings: []message.RoutingEntry{},
			Message:  fmt.Sprintf("peer closed with error: %s", err),
		}
		// send error message
		if err := p.AnnouceRouting(&msg); err != nil {
			return err
		}
		return err
	} else {
		// ack peers
		msg := message.Routing{
			Type:     message.RoutingRegisterAck,
			Routings: conflictEntries,
			Message:  "ack",
		}
		// send ack message
		if err := p.AnnouceRouting(&msg); err != nil {
			p.Close()
			return err
		}
	}
	return nil
}

// find dest port
func (r *Router) FindDestPort(dst net.IP) (*Port, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	containingNetworks, err := r.routeTable.ContainingNetworks(dst)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	var targetEntry *routingEntry
	maskLen := -1
	for _, e := range containingNetworks {
		if entry, ok := e.(*routingEntry); ok {
			n, _ := entry.network.Mask.Size()
			if n > maskLen {
				targetEntry = entry
				maskLen = n
			}
		}
	}
	if targetEntry != nil {
		return targetEntry.port, nil
	}
	return nil, nil
}

// Close the router
func (r *Router) Close() {
	close(r.closed)
}

func (r *Router) Done() <-chan struct{} {
	return r.closed
}

func (r *Router) handleTraffic(p *Port) error {

	defer p.Close()
	for {
		select {
		case <-r.closed:
			return nil
		default:
			packet := message.Packet{}
			if err := p.ReadPacket(&packet); err != nil {
				return err
			}
			// check packet ttl
			packet.TTL -= 1
			if packet.TTL <= 0 {
				continue
			}
			// replace fake ip to src ip for ingress traffic from tunnel
			if r.fakeIP != nil && p.IsTunnel() {
				if err := r.fakeIP.DNAT(&packet); err != nil {
					return err
				}
			}
			// routing
			target, err := r.FindDestPort(packet.Dst)
			if err != nil {
				return err
			}
			if target != nil {
				// fake ip enabled, should do some modifying for egress traffic from tunnel
				if r.fakeIP != nil && target.IsTunnel() {
					// modify dns response
					if err := r.fakeIP.FakeDnsResponse(&packet); err != nil {
						return err
					}
					// replace src ip to fake ip
					if err := r.fakeIP.SNAT(&packet); err != nil {
						return err
					}
				}
				if err := target.WritePacket(&packet); err != nil {
					// target port too slow or dead. we should close it. or it will slowdown everyone
					logger.Printf("error relaying packet to port(%s). it is too slow. close port: %s", target, err)
					target.Close()
				}
			} else {
				// TODO: record not routed dst ip
				// TODO: dst ip as peer discovery keys
				// logger.Printf("Send packet %s, no destination\n", packet)
			}
		}
	}
}

// annouce routings to peers
func (r *Router) handleRouting(p *Port) error {
	defer p.Close()
	// annouce routing everty 10 seconds
	ticker := time.NewTicker(routingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.closed:
			return nil
		case <-ticker.C:
			// check routing status
			r.lock.Lock()
			state, ok := r.portStats[p]
			r.lock.Unlock()
			if ok {
				diff := time.Now().Sub(state.updatedAt)
				if diff > time.Second*300 {
					return errors.Errorf("port(%s) idle closed", p)
				}
			} else {
				return errors.Errorf("port(%s) has no stat infos", p)
			}
			// TODO: get some real routings
			routings, err := r.getRoutingsForPort(p)
			if err != nil {
				return err
			}
			msg := message.Routing{Routings: routings}
			// send routings
			p.BeginRttTiming()
			if err := p.AnnouceRouting(&msg); err != nil {
				return err
			}
			// tunnel port is not a real node
			// fake it, make it looks like the message is from the tunnel port
			if p.IsTunnel() {
				routing := message.Routing{
					Type:     message.MessageTypeRouting,
					Routings: []message.RoutingEntry{},
				}
				// the first localnet is the tunnel address
				origin := r.id
				for _, network := range r.localNets {
					routing.Routings = append(routing.Routings, message.RoutingEntry{
						Network: network,
						// local net, metric is always
						Metric: 0,
						Rtt:    0,
						Origin: origin,
					})
					origin = ""
				}
				r.UpdateRouting(p, routing)
			}
		}
	}
}

// get routings to send to the port
func (r *Router) getRoutingsForPort(p *Port) ([]message.RoutingEntry, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	routings := []message.RoutingEntry{}

	// send my routing tables to peer
	all, err := r.routeTable.CoveredNetworks(*cidranger.AllIPv4)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	for _, e := range all {
		if entry, ok := e.(*routingEntry); ok {
			// split horizon
			if entry.port == p {
				continue
			}
			// not tunnel
			if !p.IsTunnel() {
				routings = append(routings, message.RoutingEntry{
					Network: entry.network,
					Metric:  entry.metric,
					Rtt:     entry.rtt,
					Origin:  entry.origin,
				})
				continue
			}
			// tunnel
			// route none default traffics
			if entry.network.String() != defaultRouting {
				routings = append(routings, message.RoutingEntry{
					Network: entry.network,
					Metric:  entry.metric,
					Rtt:     entry.rtt,
				})
			} else if r.fakeIP != nil {
				// if fakeip is enabled, route dns traffics to the tunnel
				for _, network := range r.fakeIP.DNSRoutings() {
					routings = append(routings, message.RoutingEntry{
						Network: network,
						Metric:  entry.metric,
						Rtt:     entry.rtt,
					})
				}
			}
		}
	}
	return routings, nil
}

func (r *Router) clearRouting(p *Port) error {
	// remove port routing
	r.lock.Lock()
	defer r.lock.Unlock()
	delete(r.portStats, p)
	all, err := r.routeTable.CoveredNetworks(*cidranger.AllIPv4)
	if err != nil {
		return errors.WithStack(err)
	}
	for _, e := range all {
		if entry, ok := e.(*routingEntry); ok {
			if entry.port == p {
				// entry expired, remove the routing
				if _, err := r.routeTable.Remove(entry.Network()); err != nil {
					return errors.WithStack(err)
				}
			}
		}
	}
	return nil
}

// refresh routing tables
func (r *Router) refreshRoutings() error {

	now := time.Now()
	r.lock.Lock()
	defer r.lock.Unlock()

	all, err := r.routeTable.CoveredNetworks(*cidranger.AllIPv4)
	if err != nil {
		return errors.WithStack(err)
	}
	for _, e := range all {
		if entry, ok := e.(*routingEntry); ok {
			logger.Printf("routing: %s\n", entry)
			if now.Sub(entry.updatedAt) > routingExpire {
				// entry expired, remove the routing
				if _, err := r.routeTable.Remove(entry.Network()); err != nil {
					return errors.WithStack(err)
				}
			} else {
				// todo
			}
		}
	}
	return nil
}

// do background works
// refresh routing table
func (r *Router) background() {

	ticker := time.NewTicker(routingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// infos
			logger.Println("goroutines:", runtime.NumGoroutine())
			// refresh
			if err := r.refreshRoutings(); err != nil {
				logger.Printf("refresh routing failed with: %s", err)
			}
		case <-r.closed:
			logger.Println("router closed")
			return
		}
	}
}
