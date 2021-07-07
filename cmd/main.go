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
	"goose/pkg/route"
)


var (
	logger = log.New(os.Stdout, "logger: ", log.Lshortfile)
	// run as client
	isClient = false
	// remote http3 endpint
	httpEndpoint = ""
	// protocl
	protocol = ""
	// local addr
	localAddr = ""
)


func main() {

	flag.StringVar(&httpEndpoint, "e", "https://us.nick12.com", "remote http endpoint")
	flag.BoolVar(&isClient, "c", false, "run as client")
	flag.StringVar(&protocol, "p", "http3", "protocol: http/http3")
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
			go func() { wire.ConnectHTTP3(httpEndpoint, localAddr, t) } ()
		case "http":
			go func() { wire.ConnectHTTP(httpEndpoint, localAddr, t) } ()
		case "tcp":
			return
		case "udp":
			return
		default:
			go func() { wire.ConnectHTTP3(httpEndpoint, localAddr, t) } ()
		}
	}

	out, _ := route.RunCmd("ping", "www.baidu.com", "-c", "4")
	logger.Printf("%s", string(out))

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<- c
}
