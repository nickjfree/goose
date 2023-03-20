package wire

import (
	"log"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/nickjfree/goose/pkg/message"
	"github.com/pkg/errors"
)

var (
	logger = log.New(os.Stdout, "wire: ", log.LstdFlags|log.Lshortfile)

	// wire managers
	managers = map[string]WireManager{}
	// wire managers lock
	managersLock sync.Mutex
	// wire handler, set by connector
	wireHandler func(Wire, bool) error

	// inbound wire channel
	inboundWires = make(chan Wire)
	// boutbound wire channel
	outboundWires = make(chan Wire)
)

// wire interface
type Wire interface {
	// endpoint
	Endpoint() string
	// addr
	Address() net.IP
	// Encode
	Encode(*message.Message) error
	// Decode
	Decode(*message.Message) error
	// close
	Close() error
}

// base wire
type BaseWire struct{}

func (w *BaseWire) Endpoint() string {
	return ""
}

func (w *BaseWire) Address() net.IP {
	return net.IP{}
}

// Encode
func (w *BaseWire) Encode(msg *message.Message) error {
	return nil
}

// Decode
func (w *BaseWire) Decode(msg *message.Message) error {
	return nil
}

// close
func (w *BaseWire) Close() error {
	return nil
}

// wire manager
type WireManager interface {
	// dial new connection
	Dial(string) error
	// return wire protocol
	Protocol() string
}

type BaseWireManager struct {
	In  chan Wire
	Out chan Wire
}

func NewBaseWireManager() BaseWireManager {
	return BaseWireManager{
		In:  inboundWires,
		Out: outboundWires,
	}
}

func (m *BaseWireManager) Connect(endpoint string) error {
	logger.Fatalf("Connect() not implemented %+v", m)
	return nil
}

func (m *BaseWireManager) Protocol() string {
	logger.Fatalf("Protocol() not implemented %+v", m)
	return "none"
}

func RegisterWireManager(w WireManager) error {
	managersLock.Lock()
	defer managersLock.Unlock()
	managers[w.Protocol()] = w
	return nil
}

func Dial(endpoint string) error {

	sep := strings.SplitN(endpoint, "/", 2)
	protocol := sep[0]
	endpoint = sep[1]

	managersLock.Lock()
	manager, ok := managers[protocol]
	managersLock.Unlock()

	if ok {
		if err := manager.Dial(endpoint); err != nil {
			logger.Printf("dial wire(%s) failed %s", endpoint, err)
			return err
		}
		return nil
	}
	return errors.Errorf("protocol(%s) not supported", protocol)
}

func In() <-chan Wire {
	return inboundWires
}

func Out() <-chan Wire {
	return outboundWires
}
