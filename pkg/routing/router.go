package routing

import (
	"net"
	"sync"
	"strings"
	"time"

	"github.com/pkg/errors"

	"goose/pkg/message"
)


type portRouting struct {
	
	lastUpdated time.Time

	routings []net.IPNet
}

type Router struct {
	// lock
	lock sync.Mutex
	// port routing infos
	portRoutings map[*Port]portRouting
	// provided networks
	provided []net.IPNet
}


func NewRouter() *Router {
	return &Router{
		portRoutings: make(map[*Port]portRouting),
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
		r.lock.Lock()
		defer r.lock.Unlock()
		delete(r.portRoutings, p)
	} ()

	go func() {
		logger.Printf("routing quite for port(%s) %+v", p, r.handleRouting(p))
		r.lock.Lock()
		defer r.lock.Unlock()
		delete(r.portRoutings, p)
	} ()
	return nil
}

// update routing tables for this port
func (r *Router) UpdateRouting(p *Port, routings message.Routing) error {
	logger.Printf("update routings for %+v %+v", p, routings)
	r.lock.Lock()
	defer r.lock.Unlock()
	r.portRoutings[p] = portRouting{
		lastUpdated: time.Now(),
		routings: routings.Routings,
	}
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
				return err
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
			routings := message.Routing{}
			if err := p.AnnouceRouting(&routings); err != nil {
				return err
			}
			// simulate tun wire routing reply
			if strings.HasPrefix(p.String(), "tun") {
				routing := message.Routing{
					Type: message.MessageTypeRouting,
					Routings: r.provided,
				}
				r.UpdateRouting(p, routing)
			}
		}

	}
}