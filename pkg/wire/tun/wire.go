package tun

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/songgao/water"
	// "github.com/songgao/water/waterutil"
	"github.com/pkg/errors"

	"goose/pkg/wire"
	"goose/pkg/tunnel"
)

var (
	logger = log.New(os.Stdout, "tunwire: ", log.LstdFlags | log.Lshortfile)
	// manager
	tunWireManager *TunWireManager
)


const TUN_BUFFERSIZE = 2048

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
}

// read message from tun 
func (w *TunWire) Read() (tunnel.Message, error) {

	// payload := make ([]byte, TUN_BUFFERSIZE)
	// n, err := w.ifTun.Read(payload)
	// if err != nil {
	// 	return nil, err
	// }
	// if !waterutil.IsIPv4(payload) {
	// 	// logger.Printf("recv: not ipv4 packet len %d", n)
	// 	return nil, nil
	// }
	// srcIP := waterutil.IPv4Source(payload)
	// dstIP := waterutil.IPv4Destination(payload)
	// proto := waterutil.IPv4Protocol(payload)
	// log the packet
	// logger.Printf("recv: src %s, dst %s, protocol %+v, len %d", srcIP, dstIP, proto, n)
	return nil, nil
}


// send message to tun
func (w *TunWire) Write(msg tunnel.Message) (error) {

	// payload, ok := msg.Payload().([]byte)
	// if !ok {
	// 	logger.Printf("invalid payload format %+v", payload)
	// 	return nil
	// }

	// if !waterutil.IsIPv4(payload) {
	// 	logger.Printf("send: not ipv4 packet len %d", len(payload))
	// 	return nil
	// }
	// // srcIP := waterutil.IPv4Source(payload)
	// // dstIP := waterutil.IPv4Destination(payload)
	// // proto := waterutil.IPv4Protocol(payload)
	// // logger.Printf("send: src %s, dst %s, protocol %+v, len %d", srcIP, dstIP, proto, len(payload))
	// _, err := w.ifTun.Write(payload)
	// if err != nil {
	// 	return errors.Wrap(err, "write tun error")
	// }
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

	seg := strings.Split(endpoint, "/")
	if len(seg) != 3 {
		return errors.Errorf("invalid tun endpoint %s", endpoint)
	}
	name := seg[0]
	address := fmt.Sprintf("%s/%s", seg[1], seg[2])
	
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
