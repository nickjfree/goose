package main

import (
	// "context"
	"fmt"
	"flag"
	"log"
	"os"
	"os/signal"
	"goose/pkg/tunnel"
	"goose/pkg/routing"
)


var (
	logger = log.New(os.Stdout, "logger: ", log.Lshortfile)
	// run as client
	isClient = false
	// remote http1.1/http3 endpoint or libp2p peerid
	endpoint = ""
	// protocl
	protocol = ""
	// local addr
	localAddr = ""
	// namespace 
	namespace = ""
)


const (
	PROTOCOL_HELP = `
transport protocol.

options: http/http3/ipfs

http: 
	Client and server communicate through an upgraded http1.1 protocol. (HTTP 101 Switching Protocol)
	Can be used with Cloudflare
http3: 
	Client and server communicate through HTTP3 stream
	faster then http1.1 but doesn't support Cloudflare for now
ipfs:
	Client and server communicate through a libp2p stream. 
	With some cool features:
	Server discovery, client can search for random servers to connect through the public IPFS network
	Hole puching service, client and server can both run behind their NAT firewalls. NO PUBLIC IP NEEDED
`

	ENDPOINT_HELP = `
remote server endpoint.

for http/http3 protocols. this should be a http url of the goose server.
for ipfs protocols, this should be a libp2p PeerID. If empty, the client will try to find a random goose server in the network
`

	LOCAL_HELP = `
virtual ip address to use in CIDR format.

local ipv4 address to set on the tunnel interface.
if the error message shows someone else is using the same ip address, please change it to another one
`
)



func main() {

	flag.StringVar(&endpoint, "e", "", ENDPOINT_HELP)
	flag.BoolVar(&isClient, "c", false, "flag. run as client. if not set, it will run as a server")
	flag.StringVar(&protocol, "p", "ipfs", PROTOCOL_HELP)
	flag.StringVar(&localAddr, "local", "192.168.100.2/24", LOCAL_HELP)
	flag.StringVar(&namespace, "n", "", "namespace")
	flag.Parse()

	// set up tun device
	t := tunnel.NewTunSwitch()
	go func() { logger.Printf("tunnel quit: %s", <- t.Start()) } ()

	endpoint := fmt.Sprintf("%s/%s", protocol, endpoint)
	r := routing.NewRouter(localAddr)
	connector, err := routing.NewConnector(r)
	if err != nil {
		logger.Fatal(err)
	}
	// setup tunnel
	tunnel := fmt.Sprintf("tun/%s/%s", "goose", localAddr)
	connector.ConnectEndpoint(tunnel)
	// connect to server
	if isClient {
		connector.ConnectEndpoint(endpoint)
	}
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<- c
}
