package routing

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"goose/pkg/message"
	"goose/pkg/wire"
)

var (
	logger = log.New(os.Stdout, "routing: ", log.LstdFlags|log.Lshortfile)
)

const (
	// connection concurrenty
	dialConcurrency = 8
	// port send buffer size
	portBufferSize = 2048
	// max retries
	connMaxRetries = 32

	// wire status
	statusUnknown    = 0
	statusConnected  = 1
	statusConnecting = 2
	statusFailed     = 3

	// rtt stats
	rttAlphaMean     = 0.15
	rttAlphaVariance = 0.15
)

// Connector interface
type Connector interface {
	Dial(string)
}

// endpoint state
type epState struct {
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
	epStats map[string]epState
	// wire status lock
	lock sync.Mutex
	// connection requests
	requests chan string
	// router
	router *Router
}

// rtt stats of port
type rttStats struct {
	// start
	start time.Time
	// mean rtt in ms
	mean float32
	// variance
	variance float32
}

// port is a connect session with a node
type Port struct {
	// wire
	w wire.Wire
	// rtt
	rttStats rttStats
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
	// packete in
	pktIn int64
	// packet out
	pktOut int64
}

func NewBaseConnector(r *Router) (Connector, error) {
	c := &BaseConnector{
		epStats:  make(map[string]epState),
		requests: make(chan string, dialConcurrency),
		router:   r,
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

// mark endpint as connected
func (c *BaseConnector) setConnected(endpoint string, w wire.Wire) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	state, ok := c.epStats[endpoint]
	if ok && (state.status == statusConnecting || state.status == statusFailed) {
		state.status = statusConnected
		state.wire = w
		state.failed = 0
		c.epStats[endpoint] = state
		return nil
	}
	if !ok {
		// first connection
		state = epState{
			status: statusConnected,
			failed: 0,
		}
		c.epStats[endpoint] = state
		return nil
	}
	return errors.Errorf("invalid endpoint status %s %+v", endpoint, state)
}

// mark endpint as failed
func (c *BaseConnector) setFailed(endpoint string) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	state, ok := c.epStats[endpoint]
	if ok && state.status == statusConnecting || state.status == statusConnected {
		state.status = statusFailed
		state.failed += 1
		// remove endpoint failed too many times
		if state.failed >= connMaxRetries {
			logger.Printf("endpoint %s failed too many times, remove it", endpoint)
			delete(c.epStats, endpoint)
		} else {
			c.epStats[endpoint] = state
		}
		return nil
	}
	return errors.Errorf("invalid endpoint status %s %+v", endpoint, state)
}

// mark endpint as connecting
func (c *BaseConnector) setConnecting(endpoint string) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	state, ok := c.epStats[endpoint]
	if ok && state.status == statusFailed {
		state.status = statusConnecting
		c.epStats[endpoint] = state
		return nil
	} else if ok {
		return errors.Errorf("invalid endpoint status %s %+v", endpoint, state)
	}
	// first connection
	state = epState{
		status: statusConnecting,
		failed: 0,
	}
	c.epStats[endpoint] = state
	return nil
}

// remove endpoint status
func (c *BaseConnector) setUnknow(endpoint string) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	state, ok := c.epStats[endpoint]
	if ok && state.status == statusConnected {
		delete(c.epStats, endpoint)
	}
	return errors.Errorf("invalid endpoint status %s %+v", endpoint, state)
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
		w:         w,
		router:    c.router,
		output:    make(chan message.Packet, portBufferSize),
		announce:  make(chan message.Routing),
		closeFunc: closeFunc,
		ctx:       ctx,
		rttStats: rttStats{
			start: time.Now(),
		},
	}
	go func() {
		logger.Printf("handle port(%s) output: %s", p, p.handleOutput())
	}()
	return p
}

// run the connector
func (c *BaseConnector) start() error {
	// start background connection handler
	go c.handleConnection()
	// handle connection requests
	for i := 1; i < dialConcurrency; i++ {
		go func() {
			for {
				select {
				case endpoint := <-c.requests:
					if err := c.setConnecting(endpoint); err != nil {
						logger.Printf("%s", err)
						continue
					}
					if err := c.connect(endpoint); err != nil {
						logger.Printf("connection failed %s", err)
						c.setFailed(endpoint)
					}
				case <-c.router.Done():
					return
				}
			}
		}()
	}
	// handle failed connection
	ticker := time.NewTicker(time.Second * 15)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			requests := []string{}
			// find connection to retry
			c.lock.Lock()
			for endpoint, state := range c.epStats {
				if state.status == statusFailed && state.failed < connMaxRetries {
					requests = append(requests, endpoint)
				}
			}
			c.lock.Unlock()
			for i, _ := range requests {
				c.requests <- requests[i]
			}
		case <-c.router.Done():
			return nil
		}
	}
}

// connect the wire
func (c *BaseConnector) connect(endpoint string) error {
	// connecto the wire
	if err := wire.Dial(endpoint); err != nil {
		return err
	}
	return nil
}

// handle new wire
func (c *BaseConnector) handleNewWire(w wire.Wire, reconnect bool) error {

	endpoint := w.Endpoint()

	if err := c.setConnected(endpoint, w); err != nil {
		w.Close()
		return errors.Errorf("ignore already connected wire %s %s", endpoint, err)
	}

	// add wire to router
	port := c.newPort(w, reconnect)
	if err := c.router.RegisterPort(port); err != nil {
		port.Close()
		return err
	}
	logger.Printf("new wire connection: %s", endpoint)
	return nil
}

// handle connections
func (c *BaseConnector) handleConnection() error {

	for {
		var err error
		select {
		case w := <-wire.In():
			err = c.handleNewWire(w, false)
		case w := <-wire.Out():
			err = c.handleNewWire(w, true)
		case <-c.router.Done():
			return nil
		}
		if err != nil {
			logger.Printf("handle connection %s", err)
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
			if pkt, ok := msg.Payload.(message.Packet); ok {
				*packet = pkt
				p.pktIn = p.pktIn + 1
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
}

// send packet to target wire
func (p *Port) WritePacket(packet *message.Packet) error {
	select {
	case p.output <- *packet:
	default:
		// throttle
		timer := time.NewTimer(time.Second * 30)
		defer timer.Stop()
		select {
		case p.output <- *packet:
		case <-timer.C:
			return errors.Errorf("port(%s) dead", p.w.Endpoint())
		}
	}
	return nil
}

// send routing info to peers
func (p *Port) AnnouceRouting(routings *message.Routing) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	select {
	case p.announce <- *routings:
	case <-ctx.Done():
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

func (p *Port) Address() net.IP {
	return p.w.Address()
}

func (p *Port) PeerID() string {
	peer := p.w.Endpoint()
	if strings.HasPrefix(peer, "ipfs") {
		peer = strings.Split(peer, "@")[0]
		peer = strings.Split(peer, "/")[1]
		return peer
	}
	return ""
}

func (p *Port) IsTunnel() bool {
	return strings.HasPrefix(p.String(), "tun")
}

func (p *Port) BeginRttTiming() {
	p.rttStats.start = time.Now()
}

func (p *Port) EndRttTiming() {
	rtt := float32(time.Now().Sub(p.rttStats.start).Milliseconds())
	rttVariance := (rtt - p.rttStats.mean) * (rtt - p.rttStats.mean)
	p.rttStats.mean = p.rttStats.mean*(1-rttAlphaMean) + rtt*rttAlphaMean
	p.rttStats.variance = rttVariance*(1-rttAlphaVariance) + rttVariance*rttAlphaVariance
}

// return true, if port has smaller rtt. law of large number
func (p *Port) Faster(base int) bool {
	delta := float32(base) - p.rttStats.mean
	if delta > 0 && delta*delta > 9*p.rttStats.variance {
		return true
	}
	return false
}

func (p *Port) Rtt() int {
	return int(p.rttStats.mean)
}

// close port
func (p *Port) handleOutput() error {
	// close wire when done
	defer p.w.Close()
	for {
		select {
		case packet := <-p.output:
			msg := message.Message{
				Type:    message.MessageTypePacket,
				Payload: packet,
			}
			if err := p.w.Encode(&msg); err != nil {
				return err
			}
			p.pktOut = p.pktOut + 1
		case routings := <-p.announce:
			msg := message.Message{
				Type:    message.MessageTypeRouting,
				Payload: routings,
			}
			if err := p.w.Encode(&msg); err != nil {
				return err
			}
		case <-p.ctx.Done():
			return errors.Errorf("port(%s) closed", p.w.Endpoint())
		}
	}
}
