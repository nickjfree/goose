// +build windows

package tun

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"github.com/songgao/water"
	"github.com/pkg/errors"

	"goose/pkg/utils"
	"goose/pkg/wire"
)

// create tun device on windows
func NewTunWire(name string, addr string) (wire.Wire, error) {
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
	address, network, err := net.ParseCIDR(addr)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	// set ipaddress
	if err := setIPAddress(ifTun, addr); err != nil {
		return nil, errors.Wrap(err, "")
	}
	gateway, err := defaultGateway(addr)
	if err != nil {
		return nil, err
	}
	return &TunWire{
		ifTun: ifTun,
		address: address,
		network: *network,
		gateway: gateway,
	}, nil
}



func maskString(mask net.IPMask) string {
	var s []string
    for _, i := range mask {
        s = append(s, strconv.Itoa(int(i)))
    }
    return strings.Join(s, ".")
}

// set ipaddress
func setIPAddress(iface *water.Interface, addr string) error {

	localIP, ipNet, err := net.ParseCIDR(addr)
	if err != nil {
		return errors.WithStack(err)
	}
	args := fmt.Sprintf("interface ip set address name=\"%s\" static %s %s none", 
		iface.Name(), 
		localIP.String(), 
		maskString(ipNet.Mask),
	)
	if out, err := utils.RunCmd("netsh", strings.Split(args, " ")...); err != nil {
		return errors.Wrap(err, string(out))
	}
	logger.Printf("set tunnel ip address to %s", addr)
	// set tunnel dns server to 8.8.8.8
	args = fmt.Sprintf("interface ip set dnsservers name=\"%s\" static 8.8.8.8 primary", 
		iface.Name(),
	)
	if out, err := utils.RunCmd("netsh", strings.Split(args, " ")...); err != nil {
		return errors.Wrap(err, string(out))
	}
	logger.Printf("set tunnel dnsservers to 8.8.8.8")
	return nil
}


func setRouting(add, remove []net.IPNet, gateway string) error {
	for _, network := range add {
		netString := network.String()
		if out, err := utils.RunCmd("route", "add", netString, gateway); err != nil {
			return errors.Wrap(err, string(out))
		}
	}
	for _, network := range remove {
		netString := network.String()
		if out, err := utils.RunCmd("route", "delete", netString, gateway); err != nil {
			return errors.Wrap(err, string(out))
		}
	}
	return nil
}