package tunnel


// message
type Message interface {
	// get src addr
	GetSrc() string
	// get dst
	GetDst() string
	// payload
	Payload() interface{}
}


// tun message
type TunMessage struct {
	// dst
	dst string 
	// src
	src string
	// payload, ipv4 package
	payload []byte
}

// alloc new tun message
func NewTunMessage(dst string, src string, payload []byte) *TunMessage {
	return &TunMessage{
		dst: dst,
		src: src,
		payload: payload,
	}
}


func (m *TunMessage) GetSrc() string {
	return m.src
}

func (m *TunMessage) GetDst() string {
	return m.dst
}

func (m *TunMessage) Payload() interface{} {
	return m.payload
}
