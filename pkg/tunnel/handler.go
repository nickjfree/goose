package tunnel

import (
)

// tun mode handler
func Tun(t *Tunnel, msg Message) (bool, error) {

	// src := msg.GetSrc()
	dst := msg.GetDst()
	dstPort := t.GetPort(dst)
	if dstPort == nil {
		// fallback to fallback port
		dstPort = t.GetFallbackPort()
	}
	if dstPort != nil {
		// relay msg to dstPort
		if err := dstPort.WriteOutput(msg); err != nil {
			// dstPort error. close it
			logger.Printf("warning dstPort(%s) dead err %+v, force closed", dstPort.GetAddr(), err)
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
