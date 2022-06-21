package routing

import (
	"fmt"
	"net"
	"runtime"
	"sync"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/yl2chen/cidranger"

	"goose/pkg/message"
	"goose/pkg/utils"
)

const (
	// idle timeout
	idleTimeout = time.Second * 300
	// routing interval
	routingInterval = time.Second * 30
	// routing entry expire time
	routingExpire =  time.Second * 90
)



// routing entry
type routingEntry struct {
	// network
	network net.IPNet
	// port
	port *Port
	// metric
	metric int
	// last updated
	updatedAt time.Time
}


func (entry *routingEntry) Network() net.IPNet {
	return entry.network
}

func (entry *routingEntry) String() string {
	return fmt.Sprintf("%s -> %s metric %d", entry.network.String(), entry.port, entry.metric)
}


type portState struct {

	updatedAt time.Time
	// routing entries
	routings []routingEntry
}


// router
type Router struct {
	// lock
	lock sync.Mutex
	// port routing infos
	portStats map[*Port]portState
	// forward networks
	forwardCIDRs []string
	// provided networks form local networks
	localNets []net.IPNet
	// route table
	routeTable cidranger.Ranger
	// max metric allowed
	maxMetric int
	// closed
	closed chan struct{}
}


// router option
type Option func (r *Router) error

// max metric allowd for this rouer
func WithMaxMetric(metric int) func (r *Router) error {
	return func (r *Router) error {
		r.maxMetric = metric
		return nil
	}
}

// forward cidrs
func WithForward(forwardCIDRs ...string) func (r *Router) error {
	return func (r *Router) error {
		// append local forward nets
		for _, cidr := range forwardCIDRs {
			_, network, err := net.ParseCIDR(cidr)
			if err != nil {
				return errors.WithStack(err)
			}
			r.localNets = append(r.localNets, *network)
		}
		// set up nat
		if len(forwardCIDRs) > 0 {
			if err := utils.SetupNAT(); err != nil {
				return err
			}
		}
		// forward network must use default gateway
		for _, cidr := range forwardCIDRs {
			if err := utils.SetForwardRoute(cidr); err != nil {
				return err
			}
		}
		r.forwardCIDRs = forwardCIDRs
		return nil
	}
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
		IP: address,
		Mask: net.CIDRMask(32, 32),
	})
	r := &Router{
		portStats: make(map[*Port]portState),
		routeTable: cidranger.NewPCTrieRanger(),
		localNets: localNets,
		closed: make(chan struct{}),
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
		logger.Printf("traffic quite for port(%s) %+v", p, r.handleTraffic(p))
		r.clearRouting(p)
	} ()

	go func() {
		logger.Printf("routing quite for port(%s) %+v", p, r.handleRouting(p))
		r.clearRouting(p)
	} ()
	return nil
}

// update routing tables for this port
func (r *Router) UpdateRouting(p *Port, routing message.Routing) error {
	// log the peer provided networks
	err := func() error {
		r.lock.Lock()
		defer r.lock.Unlock()

		for _, entry := range routing.Routings {
			peerEntry := routingEntry{
				network: entry.Network,
				port: p,
				// inc distance
				metric: entry.Metric + 1,
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
			for _, entry := range containing {
				if myEntry, ok := entry.(*routingEntry); ok {
					n := myEntry.Network()
					// duplicated network
					if n.String() == peerEntry.network.String() {
						matched = true
						// only update routings for entries with smaller metric
						if myEntry.metric > peerEntry.metric {
							myEntry.port = peerEntry.port
							myEntry.metric = peerEntry.metric
							myEntry.updatedAt = time.Now()
						}
						if myEntry.metric == peerEntry.metric && myEntry.port == peerEntry.port {
							myEntry.updatedAt = time.Now()
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
	} ()

	// if there is error in routing update for this port. close this port
	if err != nil {
		defer p.Close()
		msg := message.Routing{ 
			Type: message.RoutingRegisterFailed,
			Routings: []message.RoutingEntry{},
			Message: fmt.Sprintf("peer closed with error: %s", err),
		}
		// send error message
		if err := p.AnnouceRouting(&msg); err != nil {
			return err
		}
		return err
	}
	return nil
}

// find dest port
func (r *Router) FindDestPort(packet message.Packet) (*Port, error) {
	dst := packet.Dst
	r.lock.Lock()
	defer r.lock.Unlock()

	containingNetworks, err := r.routeTable.ContainingNetworks(dst)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	var targetEntry	*routingEntry
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
	// remove forward routings
	for _, cidr := range r.forwardCIDRs {
		if err := utils.RestoreForwardRoute(cidr); err != nil {
			logger.Printf("remove route %s failed %+v", cidr, err)
		}
	}
}

func (r *Router) Done() <- chan struct{} {
	return r.closed
}

func (r *Router) handleTraffic(p *Port) error {

	defer p.Close()	
	for {
		select {
		case <- r.closed:
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
			// routing 
			target, err := r.FindDestPort(packet)
			if err != nil {
				return err
			}
			if target != nil {
				if err := target.WritePacket(&packet); err != nil {
					// target port too slow or dead. we should close it. or it will slowdown everyone
					logger.Printf("error relaying packet to port(%s). it is too slow. close port: %+v", p, err)
					target.Close()
					return nil
				}
			} else {
				// TODO: record not routed dst ip
				// TODO: dst ip as peer discovery keys
				// logger.Printf("Send packet %+v, no destination\n", packet)
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
		case <- ticker.C:
			// check routing status
			r.lock.Lock()
			state, ok := r.portStats[p]
			r.lock.Unlock()
			if ok {
				diff := time.Now().Sub(state.updatedAt)
				if diff > time.Second * 300 {
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
			msg := message.Routing{ Routings: routings }
			// send routings
			if err := p.AnnouceRouting(&msg); err != nil {
				return err
			}
			// tunnel port is not a real node
			// fake it, make it looks like the message is from the tunnel port
			if strings.HasPrefix(p.String(), "tun") {

				routing := message.Routing{
					Type: message.MessageTypeRouting,
					Routings: []message.RoutingEntry{},
				}
				for _, network := range r.localNets {
					routing.Routings = append(routing.Routings, message.RoutingEntry{
						Network: network,
						// local net, metric is always
						Metric: 0,
					})
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
			routings = append(routings, message.RoutingEntry{
				Network: entry.network,
				Metric: entry.metric,
			})
		}
	}
	return routings, nil
}

func (r *Router) clearRouting(p *Port) error {
	// remove port routing
	r.lock.Lock()
	defer r.lock.Unlock()
	delete(r.portStats, p)
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
		case <- ticker.C:
			// infos 
			logger.Println("goroutines:", runtime.NumGoroutine())
			// refresh
			if err := r.refreshRoutings(); err != nil {
				logger.Printf("refresh routing failed with: %+v", err)
			}
		case <- r.closed:
			logger.Println("router closed")
			return
		}
	}
}