package wire

import (
	"net"
)

const (
	
	// traffic data
	MessageTypePacket = 0
	// routing info
	MessageTypeRouting = 1
	// routing 
	RoutingRegister = 0
	RoutingRegisterOK = 1
	RoutingRegisterFailed = 2
)

// wire message
type WireMessage struct {
	Type int   		
	Payload interface{}
}

// packet of network traffic
type Package struct {
	Src net.IP
	Dst net.IP
	Data []byte
}

// routing register msg
type RoutingMessage struct {
	// type
	Type int
	// opposite peer's local networks
	Routings []net.IPNet
}
