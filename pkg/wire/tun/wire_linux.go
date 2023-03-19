//go:build linux
// +build linux

package tun

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/songgao/water"
	"net"

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
		logger.Fatalf("%s", errors.WithStack(err))
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
	// set mtu to 1200
	if out, err := utils.RunCmd("ifconfig", name, "mtu", "1000", "up"); err != nil {
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
		ifTun:   ifTun,
		name:    name,
		address: address,
		network: *network,
		gateway: gateway,
	}, nil
}

func (w *TunWire) ChangeAddress(addr string) error {
	// check addr is cidr format
	address, network, err := net.ParseCIDR(addr)
	if err != nil {
		return errors.WithStack(err)
	}
	// delete old ip address of the tunnel interface
	maskLen, _ := network.Mask.Size()
	old := fmt.Sprintf("%s/%d", w.address.String(), maskLen)
	if out, err := utils.RunCmd("ip", "addr", "delete", old, "dev", w.name); err != nil {
		return errors.Wrap(err, string(out))
	}
	// set new address to the tunnel interface
	if out, err := utils.RunCmd("ip", "addr", "add", addr, "dev", w.name); err != nil {
		return errors.Wrap(err, string(out))
	}
	w.address = address
	logger.Printf("set tunnel ip address to %s", addr)
	return nil
}
