// +build linux

package tun

import (
	"net"
	"github.com/songgao/water"
	"github.com/pkg/errors"
	
	"goose/pkg/route"
	"goose/pkg/wire"
)


// create tun device on linux
func NewTunWire(name string, addr string) (wire.Wire, error) {
	// tun config
	config := water.Config{
		DeviceType: water.TUN,
	}
	config.Name = name
	ifTun, err := water.New(config)
	if err != nil {
		logger.Fatal(err)
	}
	// check addr is cidr format
	_, _, err = net.ParseCIDR(addr)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	// set ip address to the tunnel interface
	if out, err := route.RunCmd("ip", "addr", "add", addr, "dev", name); err != nil {
		return nil, errors.Wrap(err, string(out))
	}
	// bring the tunnel interface up
	if out, err := route.RunCmd("ip", "link", "set", name, "up"); err != nil {
		return nil, errors.Wrap(err, string(out))
	}
	return &TunWire{
		ifTun: ifTun,
	}, nil
}
