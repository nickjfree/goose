package options

import (
	"flag"
)


const (
	ENDPOINT_HELP = `
comma separated remote endpoints.

for http/http3 protocols. this should be a http url of the goose server.
for ipfs protocols, this should be a libp2p PeerID. If empty, the client will try to find a random goose server in the network
`

	LOCAL_HELP = `
virtual ip address to use in CIDR format.
local ipv4 address to set on the tunnel interface.
`
)


var (
	// remote http1.1/http3 endpoint or libp2p peerid
	Endpoints = ""
	// local addr
	LocalAddr = ""
	// forward
	Forward = ""
	// namespace 
	Namespace = ""
	// keep default route
	KeepDefaultRoute = false
)

func init() {
	flag.StringVar(&Endpoints, "e", "", ENDPOINT_HELP)
	flag.StringVar(&LocalAddr, "l", "192.168.100.2/24", LOCAL_HELP)
	flag.StringVar(&Forward, "f", "", "forward networks, comma separated CIDRs")
	flag.StringVar(&Namespace, "n", "", "namespace")
	flag.BoolVar(&KeepDefaultRoute, "d", false, "keep default route")
	flag.Parse()
}