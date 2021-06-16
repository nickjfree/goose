// +build linux

package wire

import (
	"github.com/songgao/water"
)


// create tun device on linux
func NewTunWire(name string, addr string) (Wire, error) {
	// tun config
	config := water.Config{
		DeviceType: water.TUN,
	}
	config.Name = name
	ifTun, err := water.New(config)
	if err != nil {
		logger.Fatal(err)
	}

	return &TunWire{
		BaseWire: BaseWire{},
		ifTun: ifTun,
	}, nil
}
