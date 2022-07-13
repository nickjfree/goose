package fakeip

import (
	"net"
	"sync"
	"time"

	"goose/pkg/utils"
)

// fake ip entry
type FakeEntry struct {
	// domain name
	Name string
	// fake ip
	Fake  net.IP
	// real ip
	Real net.IP
	// expire
	Expire time.Time
}

// fake ip manager
type FakeIPManager struct {
	// network
	network net.IPNet
	// fake ip pool
	pool *utils.IPPool
	// domain entry
	domains map[string]FakeEntry
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
		domains: make(map[string]FakeEntry),
		f2r: utils.NewIPMapping(),
		r2f: utils.NewIPMapping(),
		skipHosts: make(map[string]struct{}),
	}
	go m.refresh()
	return m
}


// get fakeip by domain
func (manager *FakeIPManager) Network() net.IPNet {
	return manager.network
}

// get fakeip by domain
func (manager *FakeIPManager) GetFakeByDomain(domain string, real net.IP) (net.IP, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	entry, ok := manager.domains[domain]
	if ok {
		entry.Real = real
	} else {
		// alloc new ip from fake pool
		fake, err := manager.pool.Alloc()
		if err != nil {
			return fake, err
		}
		entry = FakeEntry{
			Real: real,
			Fake: fake,
			Name: domain,
		}
	}
	// update expire time
	entry.Expire = time.Now().Add(time.Second * 300)
	manager.domains[domain] = entry
	// update mapping
	manager.f2r.Put(entry.Fake, entry.Real)
	manager.r2f.Put(entry.Real, entry.Fake)
	return entry.Fake, nil
}

// get real ip by fake ip
func (manager *FakeIPManager) GetRealByIP(fake net.IP) *net.IP {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	return manager.f2r.Get(fake)
}

// get fake ip by real ip
func (manager *FakeIPManager) GetFakeByIP(real net.IP) *net.IP {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	return manager.r2f.Get(real)
}

// clear expired hosts
func (manager *FakeIPManager) refresh() {
	// refresh mapping every 120s
	ticker := time.NewTicker(time.Second * 120)
	defer ticker.Stop()

	for {
		select {
		case <- ticker.C:
			manager.mu.Lock()
			for k, v := range manager.domains {
				logger.Printf("host: %s Fake: %s Real %s", v.Name, v.Fake, v.Real)
				if time.Since(v.Expire) > time.Second * 300 {
					// delete host entry and free fake ip
					delete(manager.domains, k)
					manager.pool.Free(v.Fake)
				}
			}
			manager.mu.Unlock()
		}
	}
}
