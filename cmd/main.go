package main

import (
	"log"
	"os"
	"goose/pkg/tunnel"
)


var (
	logger = log.New(os.Stdout, "logger: ", log.Lshortfile)
)



type TestMsg struct {
	Src string
	Dst string
}

func main() {
	t := tunnel.NewTunnel()
	p, err := t.AddPort("0.0.0.0")
	if err != nil {
		logger.Printf("error %s", err)
	}
	go func() {
		p.SendInput(TestMsg{"192.168.0.1", "192.168.0.2"})
	} ()
	logger.Printf("quit: %s", <- t.Start())
}
