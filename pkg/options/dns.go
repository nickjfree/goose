package options

import (
	"fmt"
	"net"
	"strings"

	"github.com/pkg/errors"
)

var (
	// DNS is comma-separated IPv4 addresses used as resolvers when fake-ip is enabled.
	DNS = "8.8.8.8"
)

// ParseDNSServers splits DNS into validated, de-duplicated IPv4 addresses.
func ParseDNSServers() ([]net.IP, error) {
	if strings.TrimSpace(DNS) == "" {
		return nil, errors.New("dns: at least one resolver is required")
	}
	parts := strings.Split(DNS, ",")
	out := make([]net.IP, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		ip := net.ParseIP(part)
		if ip == nil || ip.To4() == nil {
			return nil, fmt.Errorf("dns: invalid IPv4 address %q", part)
		}
		ip = ip.To4()
		key := ip.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, ip)
	}
	if len(out) == 0 {
		return nil, errors.New("dns: at least one resolver is required")
	}
	return out, nil
}