// +build windows

package wire

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"goose/pkg/route"
	"github.com/songgao/water"
	"github.com/pkg/errors"
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
	// set ipaddress
	if err := setIPAddress(ifTun, addr); err != nil {
		return nil, errors.Wrap(err, "")
	}
	return &TunWire{
		BaseWire: BaseWire{},
		ifTun: ifTun,
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
		return errors.Wrap(err, "")
	}
	args := fmt.Sprintf("interface ip set address name=\"%s\" static %s %s none", 
		iface.Name(), 
		localIP.String(), 
		maskString(ipNet.Mask),
	)
	if out, err := route.RunCmd("netsh", strings.Split(args, " ")...); err != nil {
		return errors.Wrap(err, string(out))
	}
	logger.Printf("set tunnel ip address to %s", addr)
	// set tunnel dns server to 8.8.8.8
	args = fmt.Sprintf("interface ip set dnsservers name=\"%s\" static 8.8.8.8 primary", 
		iface.Name(), 
		localIP.String(), 
		maskString(ipNet.Mask),
	)
	if out, err := route.RunCmd("netsh", strings.Split(args, " ")...); err != nil {
		return errors.Wrap(err, string(out))
	}
	logger.Printf("set tunnel dnsservers to 8.8.8.8")
	return nil
}
