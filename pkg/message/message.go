package message

import (
	"encoding/gob"
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
type Message struct {
	Type int   		
	Payload interface{}
}

// packet of network traffic
type Packet struct {
	Src net.IP
	Dst net.IP
	Data []byte
}

// routing register msg
type Routing struct {
	// type
	Type int
	// peer's provided networks
	Networks []net.IPNet
	// message
	Message string
}


func init() {
	gob.RegisterName("wire.Message", Message{})
	gob.RegisterName("wire.Packet",  Packet{})
	gob.RegisterName("wire.Routing", Routing{})
}