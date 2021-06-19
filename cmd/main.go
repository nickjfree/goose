package main

import (
	// "context"
	"flag"
	"log"
	"os"
	"os/signal"
	// "time"
	"goose/pkg/tunnel"
	"goose/pkg/wire"
)


var (
	logger = log.New(os.Stdout, "logger: ", log.Lshortfile)
	// run as client
	isClient = false
	// remote http3 endpint
	http3Endpoint = ""
	// protocl
	protocol = ""
	// local addr
	localAddr = ""
)


func main() {

	flag.StringVar(&http3Endpoint, "http3", "https://poa.nick12.com:8081", "remote http3 endpoint")
	flag.BoolVar(&isClient, "c", false, "run as client")
	flag.StringVar(&protocol, "p", "http3", " protocol")
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
		case "tcp":
			return
		case "udp":			
			return
		default:
			go func() { wire.ServeHTTP3(t) } ()
		}	
	} else {
		// client mode
		go func() { wire.ServeTun(t, localAddr, false) } ()
		// choose wire protocol
		switch protocol {
		case "http3":
			go func() { wire.ConnectHTTP3(http3Endpoint, localAddr, t) } ()
		case "tcp":
			return
		case "udp":
			return
		default:
			go func() { wire.ConnectHTTP3(http3Endpoint, localAddr, t) } ()
		}	
	}


	

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<- c
}
