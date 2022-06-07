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
	// attach to port
	Attach(p *tunnel.Port) (error)
	// read a message
	Read() (tunnel.Message, error)
	// write a message
	Write(tunnel.Message) (error)
	// get port
	GetPort() (*tunnel.Port)
	// detach port
	Detach() 
	// Encode
	Encode(*WireMessage) error
	// Decode
	Decode(*WireMessage) error
	// close
	Close() error
}


// base wire
type BaseWire struct {
	// the connected port
	port *tunnel.Port
}


// attach wire to port
func (w *BaseWire) Attach(port *tunnel.Port) (error) {
	w.port = port
	return nil
}

// get port
func (w *BaseWire) GetPort() (*tunnel.Port) {
	return w.port
}

// close port
func (w *BaseWire) Detach() {
	if err := w.port.Close(); err != nil {
		logger.Fatalf("close port error %s", err)
	}
}

// read message from tun
func (w *BaseWire) Read() (tunnel.Message, error) {
	log.Fatal(errors.New("basewire read on implemented"))
	return nil, nil
}

// send message to tun
func (w *BaseWire) Write(tunnel.Message) (error) {
	log.Fatal(errors.New("basewire write on implemented"))
	return nil
}

// Encode
func (w *BaseWire) Encode(msg *WireMessage) error {
	return nil
}

// Decode
func (w *BaseWire) Decode(msg *WireMessage) error {
	return nil
}

// close
func (w *BaseWire) Close() error {
	return nil
}


// wire manager
type WireManager interface {
	// New connection
	Connect(string) error
	// return wire protocol
	Protocol() string
}

type BaseWireManager struct {}


func (m *BaseWireManager) Connect(endpoint string) error {
	logger.Fatalf("Connect() not implemented %+v", m)
	return nil
}

func (m *BaseWireManager) Protocol() string {
	logger.Fatalf("Protocol() not implemented %+v", m)
	return "none"
}

// handle port <-> wire communication
func Communicate(w Wire, port *tunnel.Port) (error) {

	inDone := make(chan bool)
	outDone := make(chan bool)

	w.Attach(port)
	// read wire data and relay it to port
	go func () {
		for {
			msg, err := w.Read()
			if err != nil {
				logger.Printf("read wire error %+v", err)
				w.Detach()
				close(inDone)
				return
			}
			// ignore nil msg
			if msg == nil {
				continue
			}
			// send msg to port
			if err := port.WriteInput(msg); err != nil {
				logger.Printf("send to port %+s error %+v", port.GetAddr(), err)
				w.Detach()
				close(inDone)
				return
			}
		}
	} ()

	// read port data and relay it to wire
	go func () {
		for {
			msg, err := port.ReadOutput()
			if err != nil {
				logger.Printf("read port %s error %+v", port.GetAddr(), err)
				w.Detach()
				close(outDone)
				return
			}
			// send msg to wire
			if err := w.Write(msg); err != nil {
				logger.Printf("send to wire error %+v", err)
				w.Detach()
				close(outDone)
				return
			}
		}
	} ()
	// wait either routine to quit
	select {
	case <- inDone:
	case <- outDone:
	}
	return errors.Errorf("a wire <-> port(%s) communication was lost", port.GetAddr())
}

func RegisterWireManager(w WireManager) error {
	managersLock.Lock()
	defer managersLock.Unlock()
	managers[w.Protocol()] = w
	return nil
}


func Connect(protocol string, endpoint string) error {
	managersLock.Lock()
	manager, ok := managers[protocol]
	managersLock.Unlock()

	if ok {
		if err := manager.Connect(endpoint); err != nil {
			logger.Printf("get new wire failed %+v", err)
			return err
		}
		return nil
	}
	return errors.Errorf("protocol(%s) not supported", protocol)
}
