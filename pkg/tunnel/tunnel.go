package tunnel


import (
	"errors"
	"log"
	"os"
	"time"
)

var (
	logger = log.New(os.Stdout, "logger: ", log.Lshortfile)
)


type Message interface {

}

//
type Tunnel struct {
	// input mesages
	input chan Message
	// output
	ports map[string]*Port
	// quit
	q chan bool
	// error
	e chan error
}

func NewTunnel() (*Tunnel) {
	return &Tunnel{
		input: make (chan Message),
		ports: make (map[string]*Port),
		q: make (chan bool),
	}
}

// add port with address
func (t *Tunnel) AddPort(addr string) (*Port, error) {
	p, _ := t.ports[addr]
	if p != nil {
		return p, nil
	}
	p = &Port{
		Addr: addr,
		input: t.input,
		output: make(chan Message),
	}
	t.ports[addr] = p
	return p, nil
}

// remove port
func (t *Tunnel) Remove(addr string) (error) {
	return nil
}

// close tunnel
func (t *Tunnel) Close() (error) {
	t.q <- true
	return nil
}


func (t *Tunnel) Start() (chan error) {
	go func() {
		t.e <- t.run()
	} ()
	return t.e
}


// main loop
func (t *Tunnel) run() (error) {

	for {
		select {
			// handle inbound messages
			case msg := <- t.input:
				logger.Printf("%+v", msg)
			case <- t.q:
				// quit
				return errors.New("quit")
			case <- time.Tick(time.Second * 30):
				logger.Println("tick")
		}
	}
	return nil
}
