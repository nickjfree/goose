package tunnel


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
	// in
	input chan Message
	// out
	output chan Message
	// client
	client interface{}
}


// get output channel
func (p *Port) Ouput() (chan Message) {
	return p.output
}

// get input channel
func (p *Port) Input() (chan Message) {
	return p.input
}

// send output msg
func (p *Port) SendOutput(msg Message) (error) {
	p.PkgOut += 1
	p.output <- msg
	return nil
}

// send input msg
func (p *Port) SendInput(msg Message) (error) {
	p.PkgIn += 1
	p.input <- msg
	return nil
}

// get address ipv4/mac
func (p *Port) GetAddr() (string) {
	return p.Addr
}
