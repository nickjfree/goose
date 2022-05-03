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
)


func main() {

	flag.StringVar(&endpoint, "e", "https://us.nick12.com", "remote endpoint, http url or peerid")
	flag.BoolVar(&isClient, "c", false, "run as client")
	flag.StringVar(&protocol, "p", "http3", "protocol: http/http3/ipfs")
	flag.StringVar(&localAddr, "local", "192.168.100.1/24", "local ipv4 address to set on the tunnel interface")
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
			go func() { wire.ServeIPFS(t) } ()
		default:
			go func() { wire.ServeHTTP3(t) } ()
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
			go func() { wire.ConnectIPFS(endpoint, localAddr, t) } ()
		default:
			go func() { wire.ConnectHTTP3(endpoint, localAddr, t) } ()
		}
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<- c
	// try restore system route
	t.RestoreRoute()
}
