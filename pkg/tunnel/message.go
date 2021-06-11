package tunnel

// tun message
type TunMessage struct {
	// dst
	dst string 
	// src
	src string
	// payload, ipv4 package
	payload []byte
	// source port
	port *Port
}

// alloc new tun message
func NewTunMessage(dst string, src string, payload []byte, port *Port) *TunMessage {
	return &TunMessage{
		dst: dst,
		src: src,
		payload: payload,
		port: port,
	}
}


func (m *TunMessage) GetPort() *Port {
	return m.port
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

func (m *TunMessage) SetPort(p *Port) {
	m.port = p
}
