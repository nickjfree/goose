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
	// max retries 
	connMaxRetries = 32

	// wire status
	statusUnknown = 0
	statusConnected = 1
	statusConnecting = 2
	statusFailed = 3
	
)


// Connector interface
type Connector interface {
	Dial(string)
}


// endpoint state
type endpointState struct {
	// wire
	wire wire.Wire
	// status
	status int 
	// failed times	
	failed int
}

// wire connector
type BaseConnector struct {
	// endpoint stats
	endpointStats map[string]endpointState
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


func NewBaseConnector(r *Router) (Connector, error) {
	c := &BaseConnector{
		endpointStats: make(map[string]endpointState),
		requests: make(chan string, dialConcurrency),
		router: r,
	}
	go c.start()
	return c, nil
}


// connect to endpoint
func (c *BaseConnector) Dial(endpoint string) {
	c.requests <- endpoint
}


func (c *BaseConnector) remove(endpoint string, reconnect bool) {
	if reconnect {
		c.setFailed(endpoint)
	} else {
		c.setUnknow(endpoint)
	}
}

func (c *BaseConnector) isConnected(endpoint string) bool {
	c.lock.Lock()
	defer c.lock.Unlock()
	if state, ok := c.endpointStats[endpoint]; ok {
		return state.status == statusConnected
	}
	return false
}

func (c *BaseConnector) isConnecting(endpoint string) bool {
	c.lock.Lock()
	defer c.lock.Unlock()
	if state, ok := c.endpointStats[endpoint]; ok {
		return state.status == statusConnecting
	}
	return false
}

func (c *BaseConnector) isFailed(endpoint string) bool {
	c.lock.Lock()
	defer c.lock.Unlock()
	if state, ok := c.endpointStats[endpoint]; ok {
		return state.status == statusFailed
	}
	return false
}


func (c *BaseConnector) setConnected(endpoint string, w wire.Wire) {
	c.lock.Lock()
	defer c.lock.Unlock()
	state := endpointState{
		wire: w,
		status: statusConnected,
		failed: 0,
	}
	c.endpointStats[endpoint] = state
}

func (c *BaseConnector) setFailed(endpoint string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	state, ok := c.endpointStats[endpoint]
	if ok {
		state.status = statusFailed
		state.failed += 1
	} else {
		state = endpointState{
			wire: nil,
			status: statusFailed,
			failed: 1,
		}
	}
	c.endpointStats[endpoint] = state
}

func (c *BaseConnector) setConnecting(endpoint string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	state, ok := c.endpointStats[endpoint]
	if ok {
		state.status = statusConnecting
	} else {
		state = endpointState{
			wire: nil,
			status: statusConnecting,
			failed: 0,
		}
	}
	c.endpointStats[endpoint] = state
}

func (c *BaseConnector) setUnknow(endpoint string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	delete(c.endpointStats, endpoint)
}


// connect to endpoint
func (c *BaseConnector) newPort(w wire.Wire, reconnect bool) *Port {

	ctx, cancel := context.WithCancel(context.Background())
	
	closeFunc := func() error {
		cancel()
		c.remove(w.Endpoint(), reconnect)
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
func (c *BaseConnector) start() error {
	// start background connection handler
	go c.handleConnection()
	// handle connection requests
	for i := 1; i < dialConcurrency; i++ {
		go func () {
			for {
				select {
				case endpoint := <- c.requests:
					if c.isConnecting(endpoint) {
						// already connecting
						continue
					}
					if err := c.connect(endpoint); err != nil {
						c.setFailed(endpoint)
						logger.Printf("connection failed %+v", err)
					}
				case <- c.router.Done():
					return
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
			for endpoint, state := range c.endpointStats {
				if state.status == statusFailed && state.failed < connMaxRetries {
					requests = append(requests, endpoint)
				}
			}
			c.lock.Unlock()		
			for i, _ := range requests {
				c.requests <- requests[i]
			}
		case <- c.router.Done():
			return nil
		}
	}
	return nil
}


// connect the wire
func (c *BaseConnector) connect(endpoint string) error {

	if c.isConnected(endpoint) || c.isConnecting(endpoint) {
		// wire already connected
		return nil
	}
	c.setConnecting(endpoint)
	// connecto the wire
	if err := wire.Dial(endpoint); err != nil {
		return err
	}
	return nil
}

// handle new wire
func (c *BaseConnector) handleNewWire(w wire.Wire, reconnect bool) error {
	
	endpoint := w.Endpoint()
	// find exists wiew connection
	if c.isConnected(endpoint) {
		// close it
		w.Close()
		return errors.Errorf("ignore already connected wire %s", endpoint)
	}

	// add wire to router
	port := c.newPort(w, reconnect)
	if err := c.router.RegisterPort(port); err != nil {
		port.Close()
		return err
	}
	// set up route for wire
	if err := w.SetRoute(); err != nil {
		port.Close()
		return err
	}
	// mark this wire as connected
	c.setConnected(endpoint, w)
	logger.Printf("connected to %s", endpoint)
	return nil
}

// handle connections
func (c *BaseConnector) handleConnection() error {

	for {
		var err error
		select {
		case w := <- wire.In():
			err = c.handleNewWire(w, false)
		case w := <- wire.Out():
			err = c.handleNewWire(w, true)
		case <- c.router.Done():
			return nil
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
