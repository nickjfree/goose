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

}

// get input channel
func (p *Port) Input() (chan Message) {

}

// get address ipv4/mac
func (p *Port) GetAddr() (string) {
	return addr
}
