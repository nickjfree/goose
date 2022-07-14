package fakeip

import (
	"net"
	"sync"

	"goose/pkg/utils"
)


// fake ip manager
type FakeIPManager struct {
	// network
	network net.IPNet
	// fake ip pool
	pool *utils.IPPool
	// fake to real ip mapping
	f2r *utils.IPMapping
	// real to fake ip mapping
	r2f *utils.IPMapping
	// hosts to skip 
	skipHosts map[string]struct{}
	// lock
	mu sync.Mutex
}

func NewFakeIPManager(network string) *FakeIPManager {
	_, ipNet, err := net.ParseCIDR(network)
	if err != nil {
		logger.Fatal(err)
	}
	m := &FakeIPManager{
		network: *ipNet,
		pool: utils.NewIPPool(*ipNet),
		f2r: utils.NewIPMapping(),
		r2f: utils.NewIPMapping(),
		skipHosts: make(map[string]struct{}),
	}
	return m
}

// alloc fake ip
func (manager *FakeIPManager) Alloc(domain string, real net.IP) (net.IP, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	var fake net.IP
	var err error

	// find fakeip from mapping
	if f := manager.r2f.Get(real); f != nil {
		fake = *f
	} else {
		// alloc new fake ip
		if fake, err = manager.pool.Alloc(); err != nil {
			return nil, err
		}
	}
	// update mapping
	manager.f2r.Put(fake, real)
	manager.r2f.Put(real, fake)
	return fake, nil
}

// get real ip by fake ip
func (manager *FakeIPManager) ToReal(fake net.IP) *net.IP {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	return manager.f2r.Get(fake)
}

// get fake ip by real ip
func (manager *FakeIPManager) ToFake(real net.IP) *net.IP {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	return manager.r2f.Get(real)
}


// dns traffice routing
func (manager *FakeIPManager) DNSRoutings() []net.IPNet {
	return []net.IPNet{
		manager.network,
		net.IPNet{
			IP: net.IPv4(8, 8, 8, 8),
			Mask: net.IPv4Mask(255, 255, 255, 255),
		},
	}
}
