package main

import (
	// "context"
	"flag"
	"log"
	"os"
	"os/signal"
	"goose/pkg/tunnel"
	"goose/pkg/wire"
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
	flag.StringVar(&localAddr, "local", "192.168.100.1/24", LOCAL_HELP)
	flag.StringVar(&namespace, "n", "", "namespace")
	flag.Parse()

	// set up tun device
	t := tunnel.NewTunSwitch()
	go func() { logger.Printf("tunnel quit: %s", <- t.Start()) } ()


	// server
	if !isClient {
		go func() { wire.ServeTun(t, localAddr, true) } ()
		// choose wire protocol
		switch protocol {
		case "http3":
			go func() { wire.ServeHTTP3(t) } ()
		case "http":
			go func() { wire.ServeHTTP(t) } ()
		case "ipfs":
			go func() { wire.ServeIPFS(t, namespace) } ()
		default:
			go func() { wire.ServeIPFS(t, namespace) } ()
		}
	} else {
		// client mode
		go func() { wire.ServeTun(t, localAddr, false) } ()
		// choose wire protocol
		switch protocol {
		case "http3":
			go func() { wire.ConnectHTTP3(endpoint, localAddr, t) } ()
		case "http":
			go func() { wire.ConnectHTTP(endpoint, localAddr, t) } ()
		case "ipfs":
			go func() { wire.ConnectIPFS(endpoint, localAddr, namespace, t) } ()
		default:
			go func() { wire.ConnectIPFS(endpoint, localAddr, namespace, t) } ()
		}
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<- c
	// try restore system route
	t.RestoreRoute()
}
