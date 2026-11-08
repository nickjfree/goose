package fakeip

import (
	"encoding/binary"
	"net"
	"sync"
	"time"

	"github.com/nickjfree/goose/pkg/routing/rule"
	"github.com/nickjfree/goose/pkg/utils"
)

const (
	fakeIPExpire = time.Hour * 24
)

func ipKey(ip net.IP) uint32 {
	ip4 := ip.To4()
	if ip4 == nil {
		return 0
	}
	return binary.BigEndian.Uint32(ip4)
}

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
	f2r map[uint32]*ipTracking
	// real to fake ip mapping
	r2f map[uint32]*ipTracking
	// real ip to the domain name it was resolved from
	r2d map[uint32]string
	// ip trackings
	trackings []*ipTracking
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
		f2r:        make(map[uint32]*ipTracking),
		r2f:        make(map[uint32]*ipTracking),
		r2d:        make(map[uint32]string),
		trackings:  []*ipTracking{},
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
			manager.expire(time.Now())
		}
	}
}

func (manager *FakeIPManager) expire(now time.Time) {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	old := manager.trackings
	manager.trackings = []*ipTracking{}
	for _, item := range old {
		if now.Sub(item.Updated) < fakeIPExpire {
			manager.trackings = append(manager.trackings, item)
		} else {
			// free the expired item
			manager.free_locked(item)
		}
	}

	for name, records := range manager.nameServer {
		kept := records[:0]
		for _, rec := range records {
			if now.Sub(rec.Exp) < 0 {
				kept = append(kept, rec)
			}
		}
		if len(kept) == 0 {
			delete(manager.nameServer, name)
		} else {
			manager.nameServer[name] = kept
		}
	}
}

func (manager *FakeIPManager) free_locked(tracking *ipTracking) {
	realKey := ipKey(tracking.Real)
	delete(manager.r2f, realKey)
	delete(manager.f2r, ipKey(tracking.Fake))
	delete(manager.r2d, realKey)
	manager.pool.Free(tracking.Fake)
}

// alloc fake ip
func (manager *FakeIPManager) alloc(domain string, real net.IP) (net.IP, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	realKey := ipKey(real)
	// remember the domain this ip was resolved from. the newest one wins
	if domain != "" {
		manager.r2d[realKey] = domain
	}
	// find fakeip from mapping
	if item, ok := manager.r2f[realKey]; ok {
		item.Updated = time.Now()
		return item.Fake, nil
	}
	// alloc new fake ip
	fake, err := manager.pool.Alloc()
	if err != nil {
		return nil, err
	}
	item := &ipTracking{
		Real:    real,
		Fake:    fake,
		Updated: time.Now(),
	}
	// update mapping
	manager.f2r[ipKey(fake)] = item
	manager.r2f[realKey] = item
	manager.trackings = append(manager.trackings, item)
	return fake, nil
}

// get real ip by fake ip
func (manager *FakeIPManager) toReal(fake net.IP) *net.IP {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if item, ok := manager.f2r[ipKey(fake)]; ok {
		item.Updated = time.Now()
		return &item.Real
	}
	return nil
}

// get fake ip by real ip
func (manager *FakeIPManager) toFake(real net.IP) *net.IP {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if item, ok := manager.r2f[ipKey(real)]; ok {
		item.Updated = time.Now()
		return &item.Fake
	}
	return nil
}

// get the domain name the ip was resolved from. empty if unknown
func (manager *FakeIPManager) Domain(ip net.IP) string {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	return manager.r2d[ipKey(ip)]
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
