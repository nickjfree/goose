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
)


func main() {

	flag.StringVar(&http3Endpoint, "http3", "https://poa.nick12.com:8081", "remote http3 endpoint")
	flag.BoolVar(&isClient, "c", false, "run as client")
	flag.StringVar(&protocol, "p", "http3", " protocol")
	flag.Parse()

	// set up tun device
	t := tunnel.NewTunSwitch()
	go func() { logger.Printf("tunnel quit: %s", <- t.Start()) } ()

	// server
	if !isClient {
		go func() { wire.ServeTun(t, "0.0.0.0", true) } ()
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
		go func() { wire.ServeTun(t, "192.168.1.1", false) } ()
		switch protocol {
		case "http3":
			go func() { wire.ConnectHTTP3(http3Endpoint, t) } ()
		case "tcp":
			return
		case "udp":			
			return
		default:
			go func() { wire.ConnectHTTP3(http3Endpoint, t) } ()
		}	
	}


	

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<- c
}
