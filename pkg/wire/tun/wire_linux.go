// +build linux

package tun

import (
	"net"
	"github.com/songgao/water"
	"github.com/pkg/errors"
	
	"goose/pkg/utils"
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
		logger.Fatalf("%+v", errors.WithStack(err))
	}
	// check addr is cidr format
	address, network, err := net.ParseCIDR(addr)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	// set ip address to the tunnel interface
	if out, err := utils.RunCmd("ip", "addr", "add", addr, "dev", name); err != nil {
		return nil, errors.Wrap(err, string(out))
	}
	// bring the tunnel interface up
	if out, err := utils.RunCmd("ip", "link", "set", name, "up"); err != nil {
		return nil, errors.Wrap(err, string(out))
	}
	gateway, err := defaultGateway(addr)
	if err != nil {
		ifTun.Close()
		return nil, err
	}
	return &TunWire{
		ifTun: ifTun,
		name: name,
		address: address,
		network: *network,
		gateway: gateway,
	}, nil
}

func setRouting(add, remove []net.IPNet, gateway string) error {
	for _, network := range add {
		netString := network.String()
		if out, err := utils.RunCmd("ip", "route", "add", netString, "via", gateway); err != nil {
			return errors.Wrap(err, string(out))
		}
	}
	for _, network := range remove {
		netString := network.String()
		if out, err := utils.RunCmd("ip", "route", "delete", netString, "via", gateway); err != nil {
			return errors.Wrap(err, string(out))
		}
	}
	return nil
}