package main

import (
	// "context"
	"log"
	"os"
	"os/signal"
	// "time"
	"goose/pkg/tunnel"
	"goose/pkg/wire"
)


var (
	logger = log.New(os.Stdout, "logger: ", log.Lshortfile)
)

// test tun wire
func TestTunWire() {

	t := tunnel.NewTunSwitch()
	_, err := t.AddPort("192.168.0.1", false)
	if err != nil {
		logger.Fatal(err)
	}
	local := t.GetPort("0.0.0.0")

	tun, err := wire.NewTunWire("tun1")
	if err != nil {
		logger.Fatal(err)
	}

	go func() { logger.Printf("tunnel quit: %s", <- t.Start()) } ()
	go func() { logger.Printf("wire quit: %s", wire.Communicate(tun, local, 0)) } ()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<- c
}


func main() {
	TestTunWire()
}
