package message

import (
	"bytes"
	"encoding/gob"
	"net"

	"github.com/pkg/errors"
)

const (

	// traffic data
	MessageTypePacket = 0
	// routing info
	MessageTypeRouting = 1
	// routing
	RoutingRegisterFailed = 2
	RoutingRegisterAck    = 3
	// ttl
	PacketTTL = 32
)

// wire message
type Message struct {
	Type    int
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

// encode to bytes
func (m *Message) Encode() ([]byte, error) {

	b := bytes.Buffer{}
	enc := gob.NewEncoder(&b)
	if err := enc.Encode(m); err != nil {
		return nil, errors.WithStack(err)
	}
	return b.Bytes(), nil
}

// decode from bytes
func (m *Message) Decode(buf []byte) error {
	b := bytes.NewBuffer(buf)

	dec := gob.NewDecoder(b)
	if err := dec.Decode(m); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

// split routing message into multiple small messages
func (m *Message) Split() ([]Message, error) {

	if m.Type != MessageTypeRouting {
		return nil, errors.Errorf("can only split routing messages")
	}
	msgs := []Message{}

	routingMessage, ok := m.Payload.(Routing)
	if !ok {
		return nil, errors.Errorf("bad message %+v", m)
	}

	fragment := []RoutingEntry{}
	for i, _ := range routingMessage.Routings {

		fragment = append(fragment, routingMessage.Routings[i])
		if len(fragment) >= 32 {
			msg := Message{
				Type: MessageTypeRouting,
				Payload: Routing{
					Type:     routingMessage.Type,
					Routings: fragment,
				},
			}
			msgs = append(msgs, msg)
			fragment = []RoutingEntry{}
		}
	}
	if len(fragment) > 0 {
		msg := Message{
			Type: MessageTypeRouting,
			Payload: Routing{
				Type:     routingMessage.Type,
				Routings: fragment,
			},
		}
		msgs = append(msgs, msg)
	}
	return msgs, nil
}
