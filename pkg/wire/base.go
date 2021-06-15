package wire


import (
	"log"
	"os"
	"github.com/pkg/errors"
	"goose/pkg/tunnel"
)


var (
	logger = log.New(os.Stdout, "wire: ", log.Lshortfile)
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


func Communicate(w Wire, port *tunnel.Port) (error) {

	inDone := make(chan error)
	outDone := make(chan error)

	w.Attach(port)
	// read wire data
	go func () {
		for {
			msg, err := w.Read()
			if err != nil {
				logger.Printf("read wire %+v error %+v", w, err)
				w.Detach()
				inDone <- err
				return
			}
			// send msg to port
			if err := port.WriteInput(msg); err != nil {
				logger.Printf("send to port %+v error %+v", port, err)
				w.Detach()
				inDone <- err
				return
			}
		}
	} ()

	// read port data
	go func () {
		for {
			msg, err := port.ReadOutput()
			if err != nil {
				logger.Printf("read port %+v error %s", port, err)
				w.Detach()
				outDone <- err
				return
			}
			// send msg to port
			if err := w.Write(msg); err != nil {
				logger.Printf("send to wire %+v error %s", w, err)
				w.Detach()
				outDone <- err
				return
			}
		}
	} ()
	// wait either routine to quit
	select {
	case err := <- inDone:
		logger.Printf("In quit: %+v", err)
	case err := <- outDone:
		logger.Printf("Out quit: %+v", err)
	}
	return errors.Errorf("wire %+v quit", w)
}