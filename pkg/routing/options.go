package routing

import (
	"github.com/pkg/errors"
	"net"

	"github.com/nickjfree/goose/pkg/routing/discovery"
	"github.com/nickjfree/goose/pkg/routing/fakeip"
	"github.com/nickjfree/goose/pkg/utils"
)

// router option
type Option func(r *Router) error

// max metric allowd for this rouer
func WithMaxMetric(metric int) Option {
	return func(r *Router) error {
		r.maxMetric = metric
		return nil
	}
}

func WithConnector() Option {
	return func(r *Router) error {
		// create connector
		c, err := NewBaseConnector(r)
		if err != nil {
			return err
		}
		r.Connector = c
		return nil
	}
}

// forward cidrs
func WithForward(forwardCIDRs ...string) Option {
	return func(r *Router) error {
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
			if err := utils.SetupNAT("goose"); err != nil {
				return err
			}
		}
		r.forwardCIDRs = forwardCIDRs
		return nil
	}
}

// discovery
func WithDiscovery(namespace string) Option {
	return func(r *Router) error {
		pf := discovery.NewPeerFinder(namespace)
		// relace id with the peerID
		r.id = pf.ID().String()
		go func() {
			for peer := range pf.Peers() {
				r.Dial(peer)
			}
		}()
		return nil
	}
}

// dns fake ip
func WithFakeIP(network, script, db string) Option {
	return func(r *Router) error {
		r.fakeIP = fakeip.NewFakeIPManager(network, script, db)
		return nil
	}
}

// name
func WithName(name string) Option {
	return func(r *Router) error {
		r.name = name
		return nil
	}
}
