package options

import (
	"flag"
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
)

func init() {
	flag.StringVar(&Endpoints, "e", "", ENDPOINT_HELP)
	flag.StringVar(&LocalAddr, "l", "192.168.100.2/24", LOCAL_HELP)
	flag.StringVar(&Forward, "f", "", "forward networks, comma separated CIDRs")
	flag.StringVar(&Namespace, "n", "", "namespace")
	flag.StringVar(&FakeRange, "p", "", "fake ip range")
	flag.Parse()
}