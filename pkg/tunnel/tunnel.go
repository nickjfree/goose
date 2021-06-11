package vswitch




type Message interface {

}

//
type Tunnel struct {
	// input mesages
	input chan Message
	// output
	ports map[string]Port
}


func NewTunnel() {
	return &Tunnel{
		input: make (chan Message),
		ports: make (map[string]Port)
	}
}

// add port with address
func (t *Tunnel) AddPort(addr string) (*Port, error) {

}

// remove port
func (t *Tunnel) Remove(addr string) (error) {

}


// main loop
func (t *Tunnel) run() (error) {

}
