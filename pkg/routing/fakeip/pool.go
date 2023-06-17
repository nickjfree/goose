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
		network: *ipNet,
		pool:    pool,
		f2r: utils.NewIPMapping(func(ip net.IP) error {
			pool.Free(ip)
			return nil
		}),
		r2f:        utils.NewIPMapping(nil),
		nameServer: make(map[string][]dnsRecord),
	}
	if script != "" && db != "" {
		m.rule = rule.New(script, db)
		if err := m.rule.Run(); err != nil {
			logger.Fatal(err)
		}
	}
	return m
}

// alloc fake ip
func (manager *FakeIPManager) alloc(domain string, real net.IP) (net.IP, error) {
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
func (manager *FakeIPManager) toReal(fake net.IP) *net.IP {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	return manager.f2r.Get(fake)
}

// get fake ip by real ip
func (manager *FakeIPManager) toFake(real net.IP) *net.IP {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	return manager.r2f.Get(real)
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
