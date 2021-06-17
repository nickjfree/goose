package wire


import (
	"log"
	"os"
	"github.com/pkg/errors"
	"goose/pkg/tunnel"
)


var (
	logger = log.New(os.Stdout, "wire: ", log.LstdFlags | log.Lshortfile)
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