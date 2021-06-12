package wire


import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"
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
	Read(context.Context) (tunnel.Message, error)
	// write a message
	Write(context.Context, tunnel.Message) (error)
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
func (w *BaseWire) Read(ctx context.Context) (tunnel.Message, error) {
	log.Fatal(errors.New("basewire read on implemented"))
	return nil, nil
}


// send message to tun
func (w *BaseWire) Write(context.Context, tunnel.Message) (error) {
	log.Fatal(errors.New("basewire write on implemented"))
	return nil
}


func Communicate(w Wire, port *tunnel.Port, idleTimeout int) (error) {

	inDone := make(chan error)
	outDone := make(chan error)

	w.Attach(port)
	// read wire data
	go func () {
		for {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second * time.Duration(idleTimeout))
			defer cancel()
			msg, err := w.Read(ctx)
			if err != nil {
				logger.Printf("read wire %+v error %s", w, err)
				w.Detach()
				inDone <- err
				return 
			}
			// send msg to port
			if err = port.SendInput(context.Background(), msg); err != nil {
				logger.Printf("send to port %+v error %s", port, err)
				w.Detach()
				inDone <- err
				return
			}
		}
	} ()

	// read port data
	go func () {
		for {
			msg, err := port.ReadOutput(context.Background())
			if err != nil {
				logger.Printf("read port %+v error %s", port, err)
				w.Detach()
				outDone <- err
				return
			}
			// send msg to port
			ctx, cancel := context.WithTimeout(context.Background(), time.Second * time.Duration(idleTimeout))
			defer cancel()
			if err = w.Write(ctx, msg); err != nil {
				logger.Printf("send to wire %+v error %s", w, err)
				w.Detach()
				outDone <- err
				return
			}
		}
	} ()
	// wait for routines to quit
	logger.Printf("In quit: %s, Out quit: %s", <- inDone, <- outDone)

	return errors.New(fmt.Sprintf("wire %+v quit", w))
}