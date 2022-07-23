package utils

import (
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"
)

const (
	// key expiration time
	keyExpiration = time.Second * 900
)

func ipToUint(ip net.IP) uint32 {
	v := uint32(ip[0]) << 24
	v += uint32(ip[1]) << 16
	v += uint32(ip[2]) << 8
	v += uint32(ip[3])
	return v
}

func uintToIP(v uint32) net.IP {
	return net.IP{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
}

// inc ip
func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

type entry struct {
	value  net.IP
	expire time.Time
}

// ip to ip mapping
type IPMapping struct {
	// data
	data map[uint32]entry
	// mutex
	mux sync.Mutex
	// expire handler
	expireCB func(net.IP) error
}

// create an ip to ip mapping
func NewIPMapping(expire func(net.IP) error) *IPMapping {
	m := &IPMapping{
		data:     make(map[uint32]entry),
		expireCB: expire,
	}
	go m.refresh()
	return m
}

// put value
func (c *IPMapping) Put(ip1, ip2 net.IP) {
	c.mux.Lock()
	defer c.mux.Unlock()

	c.data[ipToUint(ip1)] = entry{
		value:  ip2,
		expire: time.Now().Add(keyExpiration),
	}
}

// get value
func (c *IPMapping) Get(ip net.IP) *net.IP {
	c.mux.Lock()
	defer c.mux.Unlock()

	entry, exists := c.data[ipToUint(ip)]
	if exists && time.Since(entry.expire) < keyExpiration {
		// update expiration time
		entry.expire = time.Now().Add(keyExpiration)
		c.data[ipToUint(ip)] = entry
		return &entry.value
	}
	return nil
}

// delete value
func (c *IPMapping) Delete(ip net.IP) {
	c.mux.Lock()
	defer c.mux.Unlock()
	delete(c.data, ipToUint(ip))
}

// refresh
func (c *IPMapping) refresh() {
	// refresh mapping every 120s
	ticker := time.NewTicker(time.Second * 120)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.mux.Lock()
			for k, v := range c.data {
				if time.Since(v.expire) > keyExpiration {
					delete(c.data, k)
					if c.expireCB != nil {
						c.expireCB(uintToIP(k))
					}
				}
			}
			c.mux.Unlock()
		}
	}
}

// ip pool
type IPPool struct {
	// network range to alloc ip from
	network net.IPNet
	// retired ip
	retired []net.IP
	// max alloced ip
	max net.IP
	// mutex
	mux sync.Mutex
}

// create ip pool
func NewIPPool(network net.IPNet) *IPPool {
	p := &IPPool{ network: network }
	// we will modify ip value, that is why we make a copy of the network.IP
	p.max = uintToIP(ipToUint(network.IP))
	return p
}

// alloc new ip
func (p *IPPool) Alloc() (net.IP, error) {
	p.mux.Lock()
	defer p.mux.Unlock()

	result := net.IPv4(0, 0, 0, 0)
	// if we have retired ip, use it
	if len(p.retired) > 0 {
		result, p.retired = p.retired[len(p.retired)-1], p.retired[:len(p.retired)-1]
		return result, nil
	}
	// alloc new ip
	inc(p.max)
	if !p.network.Contains(p.max) {
		return result, errors.Errorf("no more free ip address to alloc from %s", p.network)
	}
	// deep copy ip data
	result = uintToIP(ipToUint(p.max))
	return result, nil
}

// free ip
func (p *IPPool) Free(ip net.IP) {
	p.mux.Lock()
	defer p.mux.Unlock()
	p.retired = append(p.retired, ip)
}
