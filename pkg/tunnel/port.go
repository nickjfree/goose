package tunnel


import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// port
type Port struct {
	// addr
	Addr string
	// pgkIn
	PkgIn int
	// pkgOut
	PkgOut int
	// disabled
	Disabled bool
	// isLocal
	IsLocal bool
	// in
	input chan Message
	// out
	output chan Message
	// port lock
	lock sync.Mutex
}

// get output channel
func (p *Port) Ouput() (<-chan Message) {
	return p.output
}

// disable port
func (p *Port) Disable() {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.Disabled = true
}

// close the port
func (p *Port) Close() error {
	p.Disable()
	// close the output channel
	close(p.output)
	return nil
}

// send output msg
func (p *Port) SendOutput(ctx context.Context, msg Message) (error) {
	// lock the port when sending output
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.Disabled {
		return errors.New("port disabled")
	}
	select {
	case <- ctx.Done():
		// dead peer
		return errors.New(fmt.Sprintf("dead peer: %s(outbound)", p.Addr))
	case p.output <- msg:
		p.PkgOut += 1
		return nil
	}
	return nil
}

// send input msg
func (p *Port) SendInput(ctx context.Context, msg Message) (error) {
	// lock the port when sending input
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.Disabled {
		return errors.New("port disabled")
	}
	select {
	case <- ctx.Done():
		// busy tunnel
		return errors.New(fmt.Sprintf("tunnel busy: %s(inbound)", p.Addr))
	case p.input <- msg:
		p.PkgIn += 1
		return nil
	}
}

// get address ipv4/mac
func (p *Port) GetAddr() (string) {
	return p.Addr
}
