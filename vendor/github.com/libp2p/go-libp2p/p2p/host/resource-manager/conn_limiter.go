package rcmgr

import (
	"net/netip"
	"sync"
)

type ConnLimitPerCIDR struct {
	// How many leading 1 bits in the mask
	BitMask   int
	ConnCount int
}

// 8 for now so that it matches the number of concurrent dials we may do
// in swarm_dial.go. With future smart dialing work we should bring this
// down
var defaultMaxConcurrentConns = 8

var defaultIP4Limit = ConnLimitPerCIDR{
	ConnCount: defaultMaxConcurrentConns,
	BitMask:   32,
}
var defaultIP6Limits = []ConnLimitPerCIDR{
	{
		ConnCount: defaultMaxConcurrentConns,
		BitMask:   56,
	},
	{
		ConnCount: 8 * defaultMaxConcurrentConns,
		BitMask:   48,
	},
}

func WithLimitPeersPerCIDR(ipv4 []ConnLimitPerCIDR, ipv6 []ConnLimitPerCIDR) Option {
	return func(rm *resourceManager) error {
		if ipv4 != nil {
			rm.connLimiter.connLimitPerCIDRIP4 = ipv4
		}
		if ipv6 != nil {
			rm.connLimiter.connLimitPerCIDRIP6 = ipv6
		}
		return nil
	}
}

type connLimiter struct {
	mu                  sync.Mutex
	connLimitPerCIDRIP4 []ConnLimitPerCIDR
	connLimitPerCIDRIP6 []ConnLimitPerCIDR
	ip4connsPerLimit    []map[string]int
	ip6connsPerLimit    []map[string]int
}

func newConnLimiter() *connLimiter {
	return &connLimiter{
		connLimitPerCIDRIP4: []ConnLimitPerCIDR{defaultIP4Limit},
		connLimitPerCIDRIP6: defaultIP6Limits,
	}
}

// addConn adds a connection for the given IP address. It returns true if the connection is allowed.
func (cl *connLimiter) addConn(ip netip.Addr) bool {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	limits := cl.connLimitPerCIDRIP4
	countsPerLimit := cl.ip4connsPerLimit
	isIP6 := ip.Is6()
	if isIP6 {
		limits = cl.connLimitPerCIDRIP6
		countsPerLimit = cl.ip6connsPerLimit
	}

	if len(countsPerLimit) == 0 && len(limits) > 0 {
		countsPerLimit = make([]map[string]int, len(limits))
		if isIP6 {
			cl.ip6connsPerLimit = countsPerLimit
		} else {
			cl.ip4connsPerLimit = countsPerLimit
		}
	}

	for i, limit := range limits {
		prefix, err := ip.Prefix(limit.BitMask)
		if err != nil {
			return false
		}
		masked := prefix.String()
		counts, ok := countsPerLimit[i][masked]
		if !ok {
			if countsPerLimit[i] == nil {
				countsPerLimit[i] = make(map[string]int)
			}
			countsPerLimit[i][masked] = 0
		}
		if counts+1 > limit.ConnCount {
			return false
		}
	}

	// All limit checks passed, now we update the counts
	for i, limit := range limits {
		prefix, _ := ip.Prefix(limit.BitMask)
		masked := prefix.String()
		countsPerLimit[i][masked]++
	}

	return true
}

func (cl *connLimiter) rmConn(ip netip.Addr) {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	limits := cl.connLimitPerCIDRIP4
	countsPerLimit := cl.ip4connsPerLimit
	isIP6 := ip.Is6()
	if isIP6 {
		limits = cl.connLimitPerCIDRIP6
		countsPerLimit = cl.ip6connsPerLimit
	}

	for i, limit := range limits {
		prefix, err := ip.Prefix(limit.BitMask)
		if err != nil {
			// Unexpected since we should have seen this IP before in addConn
			log.Errorf("unexpected error getting prefix: %v", err)
			continue
		}
		masked := prefix.String()
		counts, ok := countsPerLimit[i][masked]
		if !ok || counts == 0 {
			// Unexpected, but don't panic
			log.Errorf("unexpected conn count for %s ok=%v count=%v", masked, ok, counts)
			continue
		}
		countsPerLimit[i][masked]--
		if countsPerLimit[i][masked] == 0 {
			delete(countsPerLimit[i], masked)
		}
	}
}
