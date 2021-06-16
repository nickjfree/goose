// +build windows

package wire

import (
	"github.com/songgao/water"
)

// create tun device on windows
func NewTunWire(name string, addr string) (Wire, error) {
	// tun config, set 
	config := water.Config{
		DeviceType: water.TUN,
		PlatformSpecificParams: water.PlatformSpecificParams{
			ComponentID: "tap0901",
			Network: addr,
		},
	}
	ifTun, err := water.New(config)
	if err != nil {
		logger.Fatal(err)
	}
	return &TunWire{
		BaseWire: BaseWire{},
		ifTun: ifTun,
	}, nil
}
