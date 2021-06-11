package main

import (
	"context"
	"log"
	"os"
	"time"
	"goose/pkg/tunnel"
)


var (
	logger = log.New(os.Stdout, "logger: ", log.Lshortfile)
)


func main() {
	t := tunnel.NewTunSwitch()
	t.AddPort("192.168.0.1", false)
	local := t.GetPort("0.0.0.0")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1 * time.Second)
		defer cancel()
		if err := local.SendInput(
			ctx, 
			tunnel.NewTunMessage("192.168.0.1", "192.168.0.2", []byte("abcdefg"), local),
		); err != nil {
			logger.Print(err)
		}
	} ()
	logger.Printf("quit: %s", <- t.Start())
}
