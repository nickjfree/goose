package wireguard

import (
	"fmt"
	"github.com/nickjfree/goose/pkg/message"
	"github.com/nickjfree/goose/pkg/wire"
	"github.com/pkg/errors"
	"github.com/songgao/water/waterutil"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"log"
	"net"
	"os"
	"sync/atomic"
	"time"
)

const (
	tun_buffer_size  = 1024
	error_tun_closed = "TunDevice %s closed"
)

var (
	logger = log.New(os.Stdout, "wireguardwire: ", log.LstdFlags|log.Lshortfile)
	// wireguard manager
	wgWireManager *WGWireManager
)

// register wireguard wire manager
func init() {
	wgWireManager = newWGWireManager()
	wire.RegisterWireManager(wgWireManager)
}

// wireguard-wire manager
type WGWireManager struct {
	wire.BaseWireManager
}

func newWGWireManager() *WGWireManager {
	return &WGWireManager{
		BaseWireManager: wire.NewBaseWireManager(),
	}
}

// create a wireguard server then register it as a goose wire
func (m *WGWireManager) Dial(endpoint string) error {

	// create a goose tun device
	w, err := NewTunDevice()
	if err != nil {
		return errors.WithStack(err)
	}
	// create wireguard device
	dev := device.NewDevice(w, conn.NewDefaultBind(), device.NewLogger(device.LogLevelError, ""))
	// set configuration from file
	config, err := convertToConfigProtocol(endpoint)
	if err != nil {
		return errors.WithStack(err)
	}
	logger.Printf("setting up wireguard:\n%s", config.Protocol)
	if err := dev.IpcSet(config.Protocol); err != nil {
		return errors.WithStack(err)
	}
	if err := dev.Up(); err != nil {
		return errors.WithStack(err)
	}
	// set dev
	w.dev = dev
	// set config
	w.config = config
	m.Out <- w
	return nil
}

func (m *WGWireManager) Protocol() string {
	return "wireguard"
}

// tun device to interface with goose
type TunDevice struct {
	// base
	wire.BaseWire
	// config
	config *Config
	// address
	address net.IP
	// wireguard device
	dev *device.Device
	// event chan
	events chan tun.Event
	// output chan
	outBuffer chan []byte
	// input chan
	inBuffer chan []byte
	// update routing flag
	updateRouting chan struct{}
	// close state
	closed atomic.Bool
	done   chan struct{}
}

func NewTunDevice() (*TunDevice, error) {
	t := &TunDevice{
		outBuffer:     make(chan []byte, tun_buffer_size),
		inBuffer:      make(chan []byte, tun_buffer_size),
		events:        make(chan tun.Event),
		updateRouting: make(chan struct{}),
		address:       net.ParseIP("0.0.0.0"),
		done:          make(chan struct{}),
	}
	go t.loop()
	return t, nil
}

func (t *TunDevice) Endpoint() string {
	return fmt.Sprintf("wireguard/%s/%d", t.address.String(), t.config.ListenPort)
}

func (t *TunDevice) Address() net.IP {
	return t.address
}

// Encode
func (t *TunDevice) Encode(msg *message.Message) error {

	switch msg.Type {
	case message.MessageTypePacket:
		packet, ok := msg.Payload.(message.Packet)
		if !ok {
			return errors.Errorf("got invalid packet struct %s", msg.Payload)
		}
		if !waterutil.IsIPv4(packet.Data) {
			logger.Printf("sent: not ipv4 packet len %d", len(packet.Data))
			return nil
		}
		if err := t.writePacket(msg); err != nil {
			return err
		}
	case message.MessageTypeRouting:
		// wireaquard handles routing itself
		return nil
	}
	return nil
}

// Decode
func (t *TunDevice) Decode(msg *message.Message) error {
	// read packet
	if err := t.readPacket(msg); err != nil {
		return err
	}
	return nil
}

func (t *TunDevice) readPacket(msg *message.Message) error {
	for {
		select {
		case buff, ok := <-t.inBuffer:
			if !ok {
				return errors.Errorf(error_tun_closed, t.Endpoint())
			}
			if !waterutil.IsIPv4(buff) {
				continue
			} else {
				msg.Type = message.MessageTypePacket
				msg.Payload = message.Packet{
					Src:  waterutil.IPv4Source(buff),
					Dst:  waterutil.IPv4Destination(buff),
					TTL:  message.PacketTTL,
					Data: buff,
				}
				return nil
			}
		case _, ok := <-t.updateRouting:
			if !ok {
				return errors.Errorf(error_tun_closed, t.Endpoint())
			}
			routing := message.Routing{
				Type:     message.MessageTypeRouting,
				Routings: []message.RoutingEntry{},
			}
			for _, network := range t.config.AllowedIPs {
				routing.Routings = append(routing.Routings, message.RoutingEntry{
					Network: network,
					// local net, metric is always 0
					Metric: 0,
					Rtt:    0,
					Origin: "",
					Name:   "",
				})
			}
			msg.Payload = routing
			msg.Type = message.MessageTypeRouting
			return nil
		}
	}
}

func (t *TunDevice) writePacket(msg *message.Message) error {
	packet, ok := msg.Payload.(message.Packet)
	if !ok {
		return errors.Errorf("got invalid packet struct %s", msg.Payload)
	}
	if !waterutil.IsIPv4(packet.Data) {
		logger.Printf("sent: not ipv4 packet len %d", len(packet.Data))
		return nil
	}
	select {
	case <-t.done:
		return errors.Errorf(error_tun_closed, t.Endpoint())
	default:
		t.outBuffer <- packet.Data
	}
	return nil
}

func (t *TunDevice) loop() {

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-t.done:
			logger.Printf(error_tun_closed, t.Endpoint())
			return
		case <-ticker.C:
			select {
			case t.updateRouting <- struct{}{}:
			default:
			}
		}
	}
}

func (t *TunDevice) Close() error {
	if t.closed.CompareAndSwap(false, true) {
		close(t.outBuffer)
		close(t.inBuffer)
		close(t.updateRouting)
		close(t.events)
		close(t.done)
		t.dev.Close()
	}
	return nil
}

func (t *TunDevice) Name() (string, error) {
	return "goose", nil
}

func (t *TunDevice) File() *os.File {
	return nil
}

func (t *TunDevice) MTU() (int, error) {
	return 1000, nil
}

func (t *TunDevice) Events() <-chan tun.Event {
	return t.events
}

func (t *TunDevice) BatchSize() int {
	return 1
}

func (t *TunDevice) Read(bufs [][]byte, sizes []int, offset int) (n int, err error) {
	if sizes == nil || len(sizes) == 0 {
		return 0, errors.Errorf("error: empty sizes")
	}
	if bufs == nil && len(bufs) == 0 {
		return 0, errors.Errorf("error: empty bufs")
	}
	buf, ok := <-t.outBuffer
	if !ok {
		return 0, errors.Errorf(error_tun_closed, t.Endpoint())
	}
	size := copy(bufs[0][offset:], buf)
	sizes[0] = size
	return 1, nil
}

func (t *TunDevice) Write(bufs [][]byte, offset int) (int, error) {
	if bufs == nil || len(bufs) == 0 {
		return 0, errors.Errorf("error: empty bufs")
	}
	n := len(bufs[0][offset:])
	buf := make([]byte, n)
	copy(buf, bufs[0][offset:])
	select {
	case <-t.done:
		return 0, errors.Errorf(error_tun_closed, t.Endpoint())
	default:
		t.inBuffer <- buf
	}
	return 1, nil
}
