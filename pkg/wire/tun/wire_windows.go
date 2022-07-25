//go:build windows
// +build windows

package tun

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/songgao/water"
	"net"
	"strconv"
	"strings"

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
			Network:     addr,
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
	//  set mtu to 1200
	args = fmt.Sprintf("interface ipv4 set subinterface \"%s\" mtu=1000 store=active", iface.Name())
	if out, err := utils.RunCmd("netsh", strings.Split(args, " ")...); err != nil {
		return errors.Wrap(err, string(out))
	}
	logger.Printf("set tunnel mtu to 1100")
	// to make windows use this dnsserver, we must set interface metric to a small value
	args = fmt.Sprintf("interface ipv4 set interface \"%s\" metric=9", iface.Name())
	if out, err := utils.RunCmd("netsh", strings.Split(args, " ")...); err != nil {
		return errors.Wrap(err, string(out))
	}
	logger.Printf("set tunnel metric to 9")
	return nil
}
