package routing

import (
	"fmt"
	"net"
	"sync"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/yl2chen/cidranger"

	"goose/pkg/message"
)


// routing entry
type routingEntry struct {
	// network
	network net.IPNet
	// port
	port *Port
}


func (entry *routingEntry) Network() net.IPNet {
	return entry.network
}

type portRouting struct {

	lastUpdated time.Time
	// routing entries
	routings []routingEntry
}


type Router struct {
	// lock
	lock sync.Mutex
	// port routing infos
	portRoutings map[*Port]portRouting
	// provided networks form tunnel
	localNets []net.IPNet
	// all provided networks
	allNets []net.IPNet
	// route table
	routeTable cidranger.Ranger
}


func NewRouter(ipcidr string, localCIDRs ...string) *Router {
	localNets := []net.IPNet{}
	// ipaddress
	address, _, err := net.ParseCIDR(ipcidr)
	if err != nil {
		logger.Fatal(err)
	}
	// local ip/32
	localNets = append(localNets, net.IPNet{
		IP: address,
		Mask: net.CIDRMask(32, 32),
	})
	// append local nets
	for _, cidr := range localCIDRs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			logger.Fatalf("%+v", errors.WithStack(err))
		}
		localNets = append(localNets, *network)
	}
	return &Router{
		portRoutings: make(map[*Port]portRouting),
		routeTable: cidranger.NewPCTrieRanger(),
		localNets: localNets,
	}
}

func (r *Router) RegisterPort(p *Port) error {
	// handle the port
	r.lock.Lock()
	defer r.lock.Unlock()
	r.portRoutings[p] = portRouting{
		lastUpdated: time.Now(),
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
	cidrs := []string{}
	for _, net := range routing.Networks {
		cidrs = append(cidrs, net.String())
	}
	logger.Printf("peer routing: %s(%s) provides %+v", p, routing.Message, cidrs)
	r.lock.Lock()
	defer r.lock.Unlock()

	err := func() error {
		entries := []routingEntry{}
		for _, network := range routing.Networks {
			entry := routingEntry{
				network: network,
				port: p,
			}
			containing, err := r.routeTable.CoveredNetworks(network)
			if err != nil {
				return errors.WithStack(err)
			}
			for _, entry := range containing {
				e, _ := entry.(*routingEntry)
				n := e.Network()
				// another port already has this routing
				if n.String() == network.String() && e.port != p {
					// routing already exists
					return errors.Errorf("routing %s already exists for %s", network, p)
				}
			}
			if err := r.routeTable.Insert(&entry); err != nil {
				return errors.WithStack(err)
			}
			entries = append(entries, entry)
		}

		r.portRoutings[p] = portRouting{
			lastUpdated: time.Now(),
			routings: entries,
		}
		return nil
	} ()
	if err != nil {
		defer p.Close()
		msg := message.Routing{ 
			Type: message.RoutingRegisterFailed,
			Networks: []net.IPNet{},
			Message: fmt.Sprintf("peer closed with error: %s", err),
		}
		// get peer routings
		if err := p.AnnouceRouting(&msg); err != nil {
			return err
		}
		return err
	}
	// merge to provide networks
	return nil
}

// find dest port
func (r *Router) FindDestPort(packet message.Packet) (*Port, error) {
	return nil, nil
}

func (r *Router) handleTraffic(p *Port) error {

	defer p.Close()	
	for {
		packet := message.Packet{}
		if err := p.ReadPacket(&packet); err != nil {
			return err
		}
		// routing 
		target, err := r.FindDestPort(packet)
		if err != nil {
			return err
		}
		if target != nil {
			if err := target.WritePacket(&packet); err != nil {
				// target port too slow or dead. we should close it. or it will slowdown everyone
				logger.Printf("error relaying packet to port(%s). too slow. close it: %+v", p, err)
				target.Close()
				return nil
			}
		} else {
			// TODO: record not routed dst ip 
		}
	}
}

// annouce routings to peers
func (r *Router) handleRouting(p *Port) error {
	defer p.Close()
	// annouce routing everty 10 seconds
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()

	for {
		select {
		case <- ticker.C:
			// check routing status
			r.lock.Lock()
			pr, ok := r.portRoutings[p]
			r.lock.Unlock()
			if ok {
				diff := time.Now().Sub(pr.lastUpdated)
				if diff > time.Second * 300 {
					return errors.Errorf("port(%s) idle closed", p)
				}
			} else {
				return errors.Errorf("port(%s) has no routing infos", p)
			}
			// TODO: get some real routings
			routings, err := r.getRoutingsForPort(p)
			if err != nil {
				return err
			}
			msg := message.Routing{ Networks: routings }
			// set peer routings
			if err := p.AnnouceRouting(&msg); err != nil {
				return err
			}
			// simulate tun wire routing reply
			if strings.HasPrefix(p.String(), "tun") {
				routing := message.Routing{
					Type: message.MessageTypeRouting,
					Networks: r.localNets,
				}
				r.UpdateRouting(p, routing)
			}
		}
	}
}


// get routings to send to the port
func (r *Router) getRoutingsForPort(p *Port) ([]net.IPNet, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	networks := []net.IPNet{}
	// merged provided networks

	// for port, portRoutings := range r.portRoutings {

	// }


	return networks, nil
}

func (r *Router) clearRouting(p *Port) error {
	// remove port routing
	r.lock.Lock()
	defer r.lock.Unlock()
	delete(r.portRoutings, p)
	// update routing tables
	portRouting, ok := r.portRoutings[p]
	if ok {
		// remove 
		for _, entry := range portRouting.routings {
			network := entry.Network()
			if _, err := r.routeTable.Remove(network); err != nil {
				return errors.WithStack(err)
			}
		}
	}
	return nil
}
