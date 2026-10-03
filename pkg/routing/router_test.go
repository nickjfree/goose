package routing

import (
	"strings"
	"testing"

	"github.com/nickjfree/goose/pkg/routing/fakeip"
)

func announced(t *testing.T, r *Router, p *Port) string {
	t.Helper()
	routings, err := r.getRoutingsForPort(p)
	if err != nil {
		t.Fatalf("getRoutingsForPort: %s", err)
	}
	parts := make([]string, 0, len(routings))
	for _, entry := range routings {
		parts = append(parts, entry.Network.String())
	}
	return strings.Join(parts, ",")
}

func countNetwork(list, network string) int {
	if list == "" {
		return 0
	}
	n := 0
	for _, item := range strings.Split(list, ",") {
		if item == network {
			n++
		}
	}
	return n
}

// fake-ip host routes stay on the tunnel announce with or without 0.0.0.0/0
func TestFakeIPRoutingsStayWithoutDefaultRoute(t *testing.T) {

	r := testRouter(t)
	r.fakeIP = fakeip.NewFakeIPManager("11.0.0.0/16", "", "")
	tunnel := testPort("tun/goose")
	exit := testPort("ipfs/QmExit")
	peer := testPort("ipfs/QmPeer")

	got := announced(t, r, tunnel)
	if countNetwork(got, "11.0.0.0/16") != 1 || countNetwork(got, "8.8.8.8/32") != 1 {
		t.Errorf("empty table: got %s, want 11.0.0.0/16 and 8.8.8.8/32 once", got)
	}

	addRoute(t, r, "0.0.0.0/0", exit, 1, "", "")
	addRoute(t, r, "10.1.1.0/24", exit, 1, "", "")
	got = announced(t, r, tunnel)
	if countNetwork(got, "11.0.0.0/16") != 1 || countNetwork(got, "8.8.8.8/32") != 1 {
		t.Errorf("with default route: got %s, want fake range and 8.8.8.8/32 once", got)
	}
	if countNetwork(got, "0.0.0.0/0") != 0 {
		t.Errorf("tunnel announce included 0.0.0.0/0: %s", got)
	}
	if countNetwork(got, "10.1.1.0/24") != 1 {
		t.Errorf("tunnel announce dropped 10.1.1.0/24: %s", got)
	}

	got = announced(t, r, peer)
	if countNetwork(got, "0.0.0.0/0") != 1 || countNetwork(got, "10.1.1.0/24") != 1 {
		t.Errorf("peer announce: got %s, want 0.0.0.0/0 and 10.1.1.0/24", got)
	}
	if countNetwork(got, "8.8.8.8/32") != 0 || countNetwork(got, "11.0.0.0/16") != 0 {
		t.Errorf("fake-ip prefixes leaked to a peer: %s", got)
	}
}
