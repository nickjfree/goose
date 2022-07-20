package tun

import (
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/songgao/water"
	"github.com/songgao/water/waterutil"

	"goose/pkg/message"
	"goose/pkg/utils"
	"goose/pkg/wire"
)

var (
	logger = log.New(os.Stdout, "tunwire: ", log.LstdFlags|log.Lshortfile)
	// manager
	tunWireManager *TunWireManager
)

const (
	// max receive buffer size
	tunBuffSize = 2048
	// ignored routing
	defaultRouting = "0.0.0.0/0"
)

// register ipfs wire manager
func init() {
	tunWireManager = newTunWireManager()
	wire.RegisterWireManager(tunWireManager)
}

// tun device
type TunWire struct {
	// base
	wire.BaseWire
	// tun interface
	ifTun *water.Interface
	// name
	name string
	// address
	address net.IP
	// gateway
	gateway net.IP
	// local network
	network net.IPNet
	// current routings
	routings []net.IPNet
}

func (w *TunWire) Endpoint() string {
	maskSize, _ := w.network.Mask.Size()
	return fmt.Sprintf("tun/%s/%s/%d", w.name, w.address.String(), maskSize)
}

func (w *TunWire) Address() net.IP {
	return w.address
}

// Encode
func (w *TunWire) Encode(msg *message.Message) error {

	switch msg.Type {

	case message.MessageTypePacket:
		if err := w.writePacket(msg); err != nil {
			return err
		}
	case message.MessageTypeRouting:
		if routing, ok := msg.Payload.(message.Routing); ok {
			if routing.Type == message.RoutingRegisterFailed {
				return errors.Errorf("register routing failed %s", routing.Message)
			}
			return w.setupHostRouting(routing.Routings)
		} else {
			return errors.Errorf("invalid routing message %+v", msg)
		}
	}
	return nil
}

// Decode
func (w *TunWire) Decode(msg *message.Message) error {

	if err := w.readPacket(msg); err != nil {
		return err
	}
	return nil
}

func (w *TunWire) Close() error {
	w.ifTun.Close()
	// clear all routings
	if err := w.setupHostRouting([]message.RoutingEntry{}); err != nil {
		logger.Printf("clear routings failed: %+v", err)
		return err
	}
	return nil
}

func (w *TunWire) readPacket(msg *message.Message) error {
	buff := make([]byte, tunBuffSize)
	for {
		n, err := w.ifTun.Read(buff)
		if err != nil {
			return errors.WithStack(err)
		}
		if !waterutil.IsIPv4(buff) {
			// logger.Printf("recv: ignore none ipv4 packet len %d", n)
			continue
		} else {
			msg.Type = message.MessageTypePacket
			msg.Payload = message.Packet{
				Src:  waterutil.IPv4Source(buff),
				Dst:  waterutil.IPv4Destination(buff),
				TTL:  message.PacketTTL,
				Data: buff[0:n],
			}
			return nil
		}
	}
}

func (w *TunWire) writePacket(msg *message.Message) error {
	packet, ok := msg.Payload.(message.Packet)
	if !ok {
		return errors.Errorf("got invalid packet struct %+v", msg.Payload)
	}
	if !waterutil.IsIPv4(packet.Data) {
		logger.Printf("sent: not ipv4 packet len %d", len(packet.Data))
		return nil
	}
	if _, err := w.ifTun.Write(packet.Data); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (w *TunWire) setupHostRouting(routings []message.RoutingEntry) error {
	// route host traffic to this tun interface

	add := []net.IPNet{}
	remove := []net.IPNet{}
	newRoutings := []net.IPNet{}
	// routings to add
	for _, routing := range routings {
		// ignore defult routing
		netString := routing.Network.String()
		newRoutings = append(newRoutings, routing.Network)
		skip := false
		for _, exists := range w.routings {
			// skip already exists routings
			if netString == exists.String() {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		add = append(add, routing.Network)
	}
	//routings to remove
	for _, exists := range w.routings {
		// ignore defult routing
		netString := exists.String()

		skip := false
		for _, routing := range routings {
			// skip updated routings
			if netString == routing.Network.String() {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		remove = append(remove, exists)
	}
	if err := setRouting(add, remove, w.gateway.String()); err != nil {
		return err
	}
	// update routings
	w.routings = newRoutings
	return nil
}

func setRouting(add, remove []net.IPNet, gateway string) error {
	for _, network := range add {
		netString := network.String()
		if err := utils.RouteTable.SetRoute(netString, gateway); err != nil {
			return err
		}
	}
	for _, network := range remove {
		netString := network.String()
		if netString == defaultRouting {
			// restore traffic
			if err := utils.RouteTable.SetRoute(defaultRouting, ""); err != nil {
				return err
			}
			continue
		}
		if err := utils.RouteTable.RemoveRoute(netString); err != nil {
			return err
		}
	}
	return nil
}

// Tun-wire manager
type TunWireManager struct {
	wire.BaseWireManager
}

func newTunWireManager() *TunWireManager {
	return &TunWireManager{
		BaseWireManager: wire.NewBaseWireManager(),
	}
}

func (m *TunWireManager) Dial(endpoint string) error {

	seg := strings.SplitN(endpoint, "/", 2)
	if len(seg) != 2 {
		return errors.Errorf("invalid tun endpoint %s", endpoint)
	}
	name := seg[0]
	address := seg[1]

	w, err := NewTunWire(name, address)
	if err != nil {
		return err
	}
	m.Out <- w
	return nil
}

func (m *TunWireManager) Protocol() string {
	return "tun"
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// gen a default gateway from cidr address
func defaultGateway(cidr string) (net.IP, error) {
	var gateway net.IP
	address, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return gateway, errors.WithStack(err)
	}
	gateway = network.IP
	inc(gateway)
	if gateway.Equal(address) {
		return gateway, errors.Errorf("%s is reserved for gateway", address.String())
	}
	return gateway, nil
}
