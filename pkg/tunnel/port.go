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

// disable port
func (p *Port) Disable() bool {
	if p.Disabled {
		return false
	}
	p.Disabled = true
	return true
}

// close the port
func (p *Port) Close() error {
	if ok := p.Disable(); ok {
		// close the output channel
		p.lock.Lock()
		close(p.output)
		p.lock.Unlock()
	}
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
	// p.lock.Lock()
	// defer p.lock.Unlock()
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

// read output
func (p *Port) ReadOutput(ctx context.Context) (Message, error) {
	select {
	case <- ctx.Done():
		// dead port
		return nil, errors.New(fmt.Sprintf("dead port: %s(outbound)", p.Addr))
	case msg, ok := <- p.output:
		if !ok {
			return nil, errors.New(fmt.Sprintf("port closed %s(outbound)", p.Addr))
		}
		return msg, nil
	}
}



// get address ipv4/mac
func (p *Port) GetAddr() (string) {
	return p.Addr
}
