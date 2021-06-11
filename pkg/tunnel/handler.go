package tunnel

import (
	"context"
	// "errors"
	// "log"
	"time"
)

// tun mode handler
func Tun(t *Tunnel, msg Message) (bool, error) {

	// src := msg.GetSrc()
	dst := msg.GetDst()
	dstPort := t.GetPort(dst)
	if dstPort == nil {
		// fallback to local port
		dstPort = t.GetLocalPort()
	}
	if dstPort != nil {
		// relay msg to dstPort
		ctx, cancel := context.WithTimeout(context.Background(), 1 * time.Second)
		defer cancel()
		if err := dstPort.SendOutput(ctx, msg); err != nil {
			// dstPort error. close it
			logger.Printf("warning dstPort(%s) dead, force closed", dstPort.GetAddr())
			dstPort.Close()
		}
	} else {
		logger.Printf("no dst port found for addr %s", dst)
	}
	return true, nil
}


// tap mode handler
func Tap(t *Tunnel, msg Message) (bool, error) {
	return false, nil
}
