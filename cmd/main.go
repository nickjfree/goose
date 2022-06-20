package main

import (
	// "context"
	"fmt"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"goose/pkg/routing"
)


var (
	logger = log.New(os.Stdout, "logger: ", log.Lshortfile)
	// remote http1.1/http3 endpoint or libp2p peerid
	endpoints = ""
	// local addr
	localAddr = ""
	// forward
	forward = ""
	// namespace 
	namespace = ""
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
if the error message shows someone else is using the same ip address, please change it to another one
`
)


func main() {

	flag.StringVar(&endpoints, "e", "", ENDPOINT_HELP)
	flag.StringVar(&localAddr, "l", "192.168.100.2/24", LOCAL_HELP)
	flag.StringVar(&forward, "f", "", "forward networks, comma separated CIDRs")
	flag.StringVar(&namespace, "n", "", "namespace")
	flag.Parse()

	tunnel := fmt.Sprintf("tun/%s/%s", "goose", localAddr)
	// create router
	opts := []routing.Option{
		// metric
		routing.WithMaxMetric(4),
	}
	if forward != "" {
		opts = append(opts, routing.WithForward(strings.Split(forward, ",")...))
	}
	r := routing.NewRouter(localAddr, opts...)
	// create connector
	connector, err := routing.NewConnector(r)
	if err != nil {
		logger.Fatal(err)
	}
	// setup tunnel
	connector.ConnectEndpoint(tunnel)
	// connect to peers
	if endpoints != "" {
		addrs := strings.Split(endpoints, ",")
		for _, endpoint := range addrs {
			connector.ConnectEndpoint(endpoint)
		}
	}
	
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<- c
	r.Close()
	logger.Printf("Press Ctrl+C again to quit")
	<- c
}
