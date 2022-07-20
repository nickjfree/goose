package main

import (
	// "context"
	"fmt"
	"goose/pkg/options"
	"goose/pkg/routing"
	"log"
	"os"
	"os/signal"
	"strings"
)

var (
	logger = log.New(os.Stdout, "logger: ", log.Lshortfile)
)

func main() {

	opts := []routing.Option{
		// metric
		routing.WithMaxMetric(4),
		// use base connector
		routing.WithConnector(),
	}
	if options.Forward != "" {
		opts = append(opts, routing.WithForward(strings.Split(options.Forward, ",")...))
	}
	if options.Namespace != "" {
		opts = append(opts, routing.WithDiscovery(options.Namespace))
	}

	if options.FakeRange != "" {
		opts = append(opts, routing.WithFakeIP(options.FakeRange))
	}

	r := routing.NewRouter(options.LocalAddr, opts...)

	// connct to tunnel and peers
	tunnel := fmt.Sprintf("tun/%s/%s", "goose", options.LocalAddr)
	r.Dial(tunnel)
	if options.Endpoints != "" {
		addrs := strings.Split(options.Endpoints, ",")
		for _, endpoint := range addrs {
			r.Dial(endpoint)
		}
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	r.Close()
	logger.Printf("Press Ctrl+C again to quit")
	<-c
}
