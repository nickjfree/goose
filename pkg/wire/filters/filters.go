package filters

import (
	"github.com/nickjfree/goose/pkg/message"
	"github.com/nickjfree/goose/pkg/wire"
	"github.com/pkg/errors"
)

// wire filter
type Filter struct {
	// the underlying wire
	wire.Wire
	// packet middlewares
	middlewares []Middleware
}

// packet middleware
type Middleware interface {
	// modify ingress packet
	Ingress(*message.Packet) (bool, error)
	// modify egress packet
	Egress(*message.Packet) (bool, error)
}

func WrapFilter(w wire.Wire) *Filter {
	return &Filter{
		Wire:        w,
		middlewares: []Middleware{},
	}
}

func (filter *Filter) AddMiddleware(m Middleware) {
	filter.middlewares = append(filter.middlewares, m)
}

func (filter *Filter) Encode(msg *message.Message) error {
	if msg.Type == message.MessageTypePacket {
		packet, ok := msg.Payload.(message.Packet)
		if !ok {
			return errors.Errorf("got invalid packet struct %s", msg.Payload)
		}
		for _, mid := range filter.middlewares {
			drop, err := mid.Egress(&packet)
			if err != nil {
				return err
			}
			if drop {
				return nil
			}
		}
		msg.Payload = packet
	}
	return filter.Wire.Encode(msg)
}

func (filter *Filter) Decode(msg *message.Message) error {
	if err := filter.Wire.Decode(msg); err != nil {
		return err
	}
	if msg.Type == message.MessageTypePacket {
		packet, ok := msg.Payload.(message.Packet)
		if !ok {
			return errors.Errorf("got invalid packet struct %s", msg.Payload)
		}
		for _, mid := range filter.middlewares {
			drop, err := mid.Ingress(&packet)
			if err != nil {
				return err
			}
			if drop {
				return nil
			}
		}
		msg.Payload = packet
	}
	return nil
}
