package options

import (
	"flag"
	"fmt"
	"math/rand"
)

const (
	ENDPOINT_HELP = `
comma separated remote endpoints.
eg. ipfs/QmVCVa7RfutQDjvUYTejMyVLMMF5xYAM1mEddDVwMmdLf4,ipfs/QmYXWTQ1jTZ3ZEXssCyBHMh4H4HqLPez5dhpqkZbSJjh7r
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
	// fake ip range
	FakeRange = ""
	// rule script path
	RuleScript  = ""
	GeoipDbFile = ""
	// name
	Name = ""
	// wireguard
	WireguardConfig = ""
	// bootstraps
	Bootstraps = ""
	// private
	Private = false
)

func init() {

	defaultLocalAddr := fmt.Sprintf("192.168.%d.%d/24", rand.Intn(255), rand.Intn(255))

	flag.StringVar(&Endpoints, "e", "", ENDPOINT_HELP)
	flag.StringVar(&LocalAddr, "l", defaultLocalAddr, LOCAL_HELP)
	flag.StringVar(&Forward, "f", "", "forward networks, comma separated CIDRs")
	flag.StringVar(&Namespace, "n", "", "namespace")
	flag.StringVar(&FakeRange, "p", "", "fake ip range")
	flag.StringVar(&RuleScript, "r", "", "rule script")
	flag.StringVar(&GeoipDbFile, "g", "", "geoip db file")
	flag.StringVar(&Name, "name", "", "domain name to use, namespace must be set")
	flag.StringVar(&WireguardConfig, "wg", "", "wireguard config file")
	flag.StringVar(&Bootstraps, "b", "", "bootstraps")
	flag.BoolVar(&Private, "private", false, "private network")
	flag.Parse()
}
