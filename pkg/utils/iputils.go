package utils

import (
	"math/rand"
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

// Generate a random IP address within the network
func RandomIP(network net.IPNet) net.IP {
	ip := make(net.IP, len(network.IP))
	rand.Read(ip)
	for i := 0; i < len(ip); i++ {
		ip[i] &= ^network.Mask[i]
		ip[i] |= network.IP[i]
	}
	return ip
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
	p := &IPPool{network: network}
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
