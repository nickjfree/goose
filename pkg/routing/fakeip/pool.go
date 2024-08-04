package fakeip

import (
	"net"
	"sync"
	"time"

	"github.com/nickjfree/goose/pkg/routing/rule"
	"github.com/nickjfree/goose/pkg/utils"
)

// dns record A
type dnsRecord struct {
	IP  net.IP
	Exp time.Time
}

type ipTracking struct {
	Real    net.IP
	Fake    net.IP
	Updated time.Time
}

// fake ip manager
type FakeIPManager struct {
	// network
	network net.IPNet
	// fake ip pool
	pool *utils.IPPool
	// fake to real ip mapping
	f2r map[string]*net.IP
	// real to fake ip mapping
	r2f map[string]*net.IP
	// ip trackings
	trackings []ipTracking
	// fakeip rule
	rule *rule.Rule
	// fale name server cache
	nameServer map[string][]dnsRecord
	// lock
	mu sync.Mutex
}

func NewFakeIPManager(network, script, db string) *FakeIPManager {
	_, ipNet, err := net.ParseCIDR(network)
	if err != nil {
		logger.Fatal(err)
	}

	pool := utils.NewIPPool(*ipNet)

	m := &FakeIPManager{
		network:    *ipNet,
		pool:       pool,
		f2r:        make(map[string]*net.IP),
		r2f:        make(map[string]*net.IP),
		trackings:  []ipTracking{},
		nameServer: make(map[string][]dnsRecord),
	}
	if script != "" && db != "" {
		m.rule = rule.New(script, db)
		if err := m.rule.Run(); err != nil {
			logger.Fatal(err)
		}
	}
	go m.run()
	return m
}

// run the expiring loop
func (manager *FakeIPManager) run() {

	ticker := time.NewTicker(time.Second * 120)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			manager.mu.Lock()
			old := manager.trackings
			manager.trackings = []ipTracking{}
			for _, item := range old {
				if now.Sub(item.Updated) < time.Second*900 {
					manager.trackings = append(manager.trackings, item)
				} else {
					// free the expired item
					manager.free_locked(&item)
				}
			}
			manager.mu.Unlock()
		}
	}
}

func (manager *FakeIPManager) free_locked(tracking *ipTracking) {
	delete(manager.r2f, string(tracking.Real.To4()))
	delete(manager.f2r, string(tracking.Fake.To4()))
}

// alloc fake ip
func (manager *FakeIPManager) alloc(domain string, real net.IP) (net.IP, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	var fake net.IP
	var err error

	// find fakeip from mapping
	if f, ok := manager.r2f[string(real.To4())]; ok {
		fake = *f
		return fake, nil
	} else {
		// alloc new fake ip
		if fake, err = manager.pool.Alloc(); err != nil {
			return nil, err
		}
		// update mapping
		manager.f2r[string(fake.To4())] = &real
		manager.r2f[string(real.To4())] = &fake
		return fake, nil
	}
}

// get real ip by fake ip
func (manager *FakeIPManager) toReal(fake net.IP) *net.IP {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	return manager.f2r[string(fake.To4())]
}

// get fake ip by real ip
func (manager *FakeIPManager) toFake(real net.IP) *net.IP {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	return manager.r2f[string(real.To4())]
}

// dns traffice routing
func (manager *FakeIPManager) DNSRoutings() []net.IPNet {
	return []net.IPNet{
		manager.network,
		{
			IP:   net.IPv4(8, 8, 8, 8),
			Mask: net.IPv4Mask(255, 255, 255, 255),
		},
	}
}

// register custome dns record to fakeip manager
func (manager *FakeIPManager) SetNameRecord(name string, ip net.IP) {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	// expire time for the record
	exp := time.Now().Add(3 * time.Minute)

	answers, ok := manager.nameServer[name]
	if ok {
		found := false
		for i := range answers {
			if answers[i].IP.Equal(ip) {
				answers[i].Exp = exp
				found = true
				break
			}
		}
		if !found {
			answers = append(answers, dnsRecord{
				IP:  ip,
				Exp: exp,
			})
		}
	} else {
		answers = []dnsRecord{
			{
				IP:  ip,
				Exp: exp,
			},
		}
	}
	manager.nameServer[name] = answers
}

func (manager *FakeIPManager) GetNameRecord(name string) []net.IP {
	current := time.Now()
	manager.mu.Lock()
	defer manager.mu.Unlock()

	result := []net.IP{}
	if answers, ok := manager.nameServer[name]; ok {
		for _, ans := range answers {
			if current.Sub(ans.Exp) < 0 {
				result = append(result, ans.IP)
			}
		}
	}
	return result
}
