package wire


import (
	"log"
	"os"
	"sync"
	"github.com/pkg/errors"
	"goose/pkg/tunnel"
)


var (
	logger = log.New(os.Stdout, "wire: ", log.LstdFlags | log.Lshortfile)

	// wire managers
	managers = map[string]WireManager{}
	// wire managers lock
	managersLock sync.Mutex	
	// wire handler, set by connector
	wireHandler func (Wire, bool) error

	// inbound wire channel
	inboundWires = make(chan Wire)
	// boutbound wire channel
	outboundWires = make(chan Wire)
)

// wire interface
type Wire interface {
	// addr
	Addr() string
	// Encode
	Encode(*Message) error
	// Decode
	Decode(*Message) error
	// close
	Close() error
}


// base wire
type BaseWire struct {
	// the connected port
	port *tunnel.Port
}


func (w *BaseWire) Addr() string {
	return ""
}

// Encode
func (w *BaseWire) Encode(msg *Message) error {
	return nil
}

// Decode
func (w *BaseWire) Decode(msg *Message) error {
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
	In chan Wire
	Out chan Wire
}

func NewBaseWireManager() BaseWireManager {
	return BaseWireManager{
		In: inboundWires,
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


func Dial(protocol string, endpoint string) error {
	managersLock.Lock()
	manager, ok := managers[protocol]
	managersLock.Unlock()

	if ok {
		if err := manager.Dial(endpoint); err != nil {
			logger.Printf("dial wire(%s) failed %+v", endpoint, err)
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