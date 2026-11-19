package options

import (
	"net"
	"testing"
)

func TestParseDNSServers(t *testing.T) {
	old := DNS
	defer func() { DNS = old }()

	DNS = "1.1.1.1, 8.8.8.8 ,1.1.1.1"
	got, err := ParseDNSServers()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !got[0].Equal(net.IPv4(1, 1, 1, 1)) || !got[1].Equal(net.IPv4(8, 8, 8, 8)) {
		t.Fatalf("got %v", got)
	}

	DNS = "not-an-ip"
	if _, err := ParseDNSServers(); err == nil {
		t.Fatal("expected error for invalid ip")
	}
}