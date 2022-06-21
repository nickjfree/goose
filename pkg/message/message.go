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
	// ttl
	PacketTTL = 32
)

// wire message
type Message struct {
	Type int   		
	Payload interface{}
}

// packet of network traffic
type Packet struct {
	// dst
	Dst net.IP
	// src
	Src net.IP
	// ttl
	TTL int
	// the real ip packet
	Data []byte
}


// routing entry 
type RoutingEntry struct {
	// network
	Network net.IPNet
	// metric
	Metric int
}

// routing register msg
type Routing struct {
	// type
	Type int
	// peer's provided routings
	Routings []RoutingEntry
	// message
	Message string
}


func init() {
	gob.RegisterName("M", Message{})
	gob.RegisterName("P", Packet{})
	gob.RegisterName("R", Routing{})
}