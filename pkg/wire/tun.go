package wire

import (
	"goose/pkg/tunnel"
	"github.com/songgao/water"
	"github.com/songgao/water/waterutil"
)


const TUN_BUFFERSIZE = 2048

// tun device
type TunWire struct {
	// base
	BaseWire
	// tun interface
	ifTun *water.Interface
}


func NewTunWire(name string) (Wire, error) {
	// tun config
	config := water.Config{
		DeviceType: water.TUN,
	}
	config.Name = name
	ifTun, err := water.New(config)
	if err != nil {
		logger.Fatal(err)
	}

	return &TunWire{
		BaseWire: BaseWire{},
		ifTun: ifTun,
	}, nil
}

// read message from tun 
func (w *TunWire) Read() (tunnel.Message, error) {

	payload := make ([]byte, TUN_BUFFERSIZE)
	n, err := w.ifTun.Read(payload)
	if err != nil {
		return nil, err
	}
	if !waterutil.IsIPv4(payload) {
		logger.Printf("recv: not ipv4 packet len %d", n)
		return tunnel.NewTunMessage("", "", payload), nil
	}

	srcIP := waterutil.IPv4Source(payload)
	dstIP := waterutil.IPv4Destination(payload)
	proto := waterutil.IPv4Protocol(payload)
	// log the packet
	logger.Printf("recv: src %s, dst %s, protocol %+v, len %d", srcIP, dstIP, proto, n)
	return tunnel.NewTunMessage(dstIP.String(), srcIP.String(), payload[:n]), nil
}


// send message to tun
func (w *TunWire) Write(msg tunnel.Message) (error) {

	payload, ok := msg.Payload().([]byte)
	if !ok {
		logger.Printf("invalid payload format %+v", payload)
		return nil
	}
	n, err := w.ifTun.Write(payload)
	if err != nil {
		return err
	}
	if !waterutil.IsIPv4(payload) {
		logger.Printf("send: not ipv4 packet len %d", n)
		return nil
	}

	srcIP := waterutil.IPv4Source(payload)
	dstIP := waterutil.IPv4Destination(payload)
	proto := waterutil.IPv4Protocol(payload)

	logger.Printf("send: src %s, dst %s, protocol %+v, len %d", srcIP, dstIP, proto, n)
	return nil
}


// attach tun deivce to wire
func ServeTun(t *tunnel.Tunnel, localAddr string, fallback bool) {
	local, err := t.AddPort(localAddr, fallback)
	if err != nil {
		logger.Fatal(err)
	}
	tun, err := NewTunWire("goose")
	if err != nil {
		logger.Fatal(err)
	}
	logger.Printf("wire quit: %s", Communicate(tun, local))
}