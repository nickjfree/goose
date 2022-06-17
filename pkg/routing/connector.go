package routing


import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/pkg/errors"
	"goose/pkg/wire"
	"goose/pkg/message"
)


var (
	logger = log.New(os.Stdout, "routing: ", log.LstdFlags | log.Lshortfile)
)

const (
	// connection concurrenty
	dialConcurrency = 8
	// port send buffer size
	portBufferSize = 2048
)

// wire connector
type Connector struct {
	// connected wire
	connected map[string]wire.Wire
	// failed
	failed map[string]int
	// wire status lock
	lock sync.Mutex
	// connection requests
	requests chan string
	// router
	router *Router
}

// port is a connect session with a node
type Port struct {
	// wire
	w wire.Wire
	// router
	router *Router
	// output queue
	output chan message.Packet
	// routing
	announce chan message.Routing
	// close func
	closeFunc func() error
	// close
	close sync.Once
	// context
	ctx context.Context
}


func NewConnector(router *Router) (*Connector, error) {
	c := &Connector{
		connected: make(map[string]wire.Wire),
		failed: make(map[string]int),
		requests: make(chan string, dialConcurrency),
		router: router,
	}
	go c.Start()
	return c, nil
}


// connect to endpoint
func (c *Connector) ConnectEndpoint(endpoint string) {
	c.requests <- endpoint
}


func (c *Connector) CloseEndpoint(endpoint string, reconnect bool) {
	c.lock.Lock()
	delete(c.connected, endpoint)
	c.lock.Unlock()
	if reconnect {
		c.requests <- endpoint
	}
}


// connect to endpoint
func (c *Connector) newPort(w wire.Wire, reconnect bool) *Port {

	ctx, cancel := context.WithCancel(context.Background())
	
	closeFunc := func() error {
		cancel()
		c.CloseEndpoint(w.Endpoint(), reconnect)
		return nil
	}

	p := &Port{
		w: w,
		router: c.router,
		output: make(chan message.Packet, portBufferSize),
		announce: make(chan message.Routing),
		closeFunc: closeFunc,
		ctx: ctx,
	}
	go func() {
		logger.Printf("handle port(%s) output: %+v", p, p.handleOutput())
	} ()
	return p
}


// run the connector
func (c *Connector) Start() error {
	// start background connection handler
	go c.handleConnection()
	// handle connection requests
	for i := 1; i < dialConcurrency; i++ {
		go func () {
			for {
				endpoint := <- c.requests
				if err := c.connect(endpoint); err != nil {
					logger.Printf("connection failed %+v", err)
					c.lock.Lock()
					c.failed[endpoint] += 1
					c.lock.Unlock()
				}
			}
		} ()
	}
	// handle failed connection
	ticker := time.NewTicker(time.Second * 15)
	defer ticker.Stop()
	for {
		select {
		case <- ticker.C:			
			requests := []string{}
			// find connection to retry
			c.lock.Lock()
			for endpoint, failCount := range c.failed {
				if failCount > 0 {
					requests = append(requests, endpoint)
				}
			}
			c.lock.Unlock()		
			for i, _ := range requests {
				c.requests <- requests[i]
			}
		}
	}
	return nil
}


// connect the wire
func (c *Connector) connect(endpoint string) error {

	c.lock.Lock()
	_, ok := c.connected[endpoint]	
	c.lock.Unlock()
	
	if ok {
		// wire already connected
		return errors.Errorf("ignore already connected wire %s", endpoint)
	}
	// connecto the wire
	if err := wire.Dial(endpoint); err != nil {
		return err
	}
	return nil
}

// handle new wire
func (c *Connector) handleNewWire(w wire.Wire, reconnect bool) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	endpoint := w.Endpoint()
	_, ok := c.connected[endpoint]
	if ok {
		// close it
		w.Close()
		return errors.Errorf("ignore already connected wire %s", endpoint)
	}
	// add wire to router
	port := c.newPort(w, reconnect)
	if err := c.router.RegisterPort(port); err != nil {
		port.Close()
	}
	c.connected[endpoint] = w
	delete(c.failed, endpoint)
	logger.Printf("connected to %s", endpoint)
	return nil
}

// handle connections
func (c *Connector) handleConnection() error {

	for {
		var err error
		select {
		case w := <- wire.In():
			err = c.handleNewWire(w, false)
		case w := <- wire.Out():
			err = c.handleNewWire(w, true)
		}
		if err != nil {
			logger.Printf("handle connection %+v", err)
		}
	}
}


func (p *Port) ReadPacket(packet *message.Packet) error {
	
	for {
		msg := message.Message{}
		if err := p.w.Decode(&msg); err != nil {
			return err
		}
		switch msg.Type {
		case message.MessageTypePacket:
			if p, ok := msg.Payload.(message.Packet); ok {
				*packet = p
				return nil
			} else {
				return errors.Errorf("invalid packet %+v", msg)
			}
		case message.MessageTypeRouting:
			if routing, ok := msg.Payload.(message.Routing); ok {
				// handle routiong info
				p.router.UpdateRouting(p, routing)
			} else {
				return errors.Errorf("invalid routing message %+v", msg)
			}
		}
	}
	return nil
}


// send packet to target wire
func (p *Port) WritePacket(packet *message.Packet) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second * 1)
	defer cancel()
	select {
	case p.output <- *packet:
	case <- ctx.Done():
		return errors.Errorf("port(%s) dead", p.w.Endpoint())
	}
	return nil
}

// send routing info to peers
func (p *Port) AnnouceRouting(routings *message.Routing) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second * 1)
	defer cancel()
	select {
	case p.announce <- *routings:
	case <- ctx.Done():
		return errors.Errorf("port(%s) dead", p.w.Endpoint())
	}
	return nil
}

// close port
func (p *Port) Close() error {
	var err error

	p.close.Do(func() {
		err = p.closeFunc()
	})
	return err
}

func (p *Port) String() string {
	return fmt.Sprintf("%s@%s", p.w.Endpoint(), p.w.Address().String())
}


// close port
func (p *Port) handleOutput() error {
	var err error
	// close wire when done
	defer p.w.Close()
	for {
		select {
		case packet := <- p.output:
			msg := message.Message{
				Type: message.MessageTypePacket,
				Payload: packet,
			}
			if err := p.w.Encode(&msg); err != nil {
				return err
			}
		case routings := <- p.announce:
			msg := message.Message{
				Type: message.MessageTypeRouting,
				Payload: routings,
			}
			if err := p.w.Encode(&msg); err != nil {
				return err
			}
		case <- p.ctx.Done():
			return errors.Errorf("port(%s) closed", p.w.Endpoint())
		}
	}
	return err
}
