package main

import (
	// "context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"

	"github.com/nickjfree/goose/pkg/options"
	"github.com/nickjfree/goose/pkg/routing"
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
		opts = append(opts, routing.WithFakeIP(options.FakeRange, options.RuleScript, options.GeoipDbFile))
	}

	if options.Name != "" && options.Namespace != "" {
		opts = append(opts, routing.WithName(fmt.Sprintf("%s.%s", options.Name, options.Namespace)))
	}

	r := routing.NewRouter(options.LocalAddr, opts...)

	// create the tun device
	tunnel := fmt.Sprintf("tun/%s/%s", "goose", options.LocalAddr)
	r.Dial(tunnel)
	// create a wireguard listener if enabled
	if options.WireguardConfig != "" {
		wireguard := fmt.Sprintf("wireguard/%s", options.WireguardConfig)
		r.Dial(wireguard)
	}
	// connect to peers
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
