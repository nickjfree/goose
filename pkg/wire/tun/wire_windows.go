//go:build windows
// +build windows

package tun

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/songgao/water"
	"net"
	"reflect"
	"strconv"
	"strings"
	"syscall"

	"github.com/nickjfree/goose/pkg/utils"
	"github.com/nickjfree/goose/pkg/wire"
)

var (
	file_device_unknown  = uint32(0x00000022)
	tap_ioctl_config_tun = (file_device_unknown << 16) | (0 << 14) | (10 << 2) | 0
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

func (w *TunWire) ChangeAddress(addr string) error {
	localIP, _, err := net.ParseCIDR(addr)
	if err != nil {
		return errors.WithStack(err)
	}
	// reconfig the tap-windows driver with the new address
	if err := setTUN(getFd(w.ifTun), addr); err != nil {
		return errors.WithStack(err)
	}
	// check addr is cidr format
	if err := setIPAddress(w.ifTun, addr); err != nil {
		return err
	}
	w.address = localIP
	return nil
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
	logger.Printf("set tunnel mtu to 1000")
	// to make windows use this dnsserver, we must set interface metric to a small value
	args = fmt.Sprintf("interface ipv4 set interface \"%s\" metric=9", iface.Name())
	if out, err := utils.RunCmd("netsh", strings.Split(args, " ")...); err != nil {
		return errors.Wrap(err, string(out))
	}
	logger.Printf("set tunnel metric to 9")
	return nil
}

func getFd(iface *water.Interface) syscall.Handle {
	value := reflect.ValueOf(iface.ReadWriteCloser)
	if value.Kind() != reflect.Ptr {
		panic("Expected a pointer to a struct")
	}

	field := value.Elem().FieldByName("fd")
	if !field.IsValid() {
		panic("Field not found")
	}
	return syscall.Handle(field.Uint())
}

// setTUN is used to configure the IP address in the underlying driver when using TUN
func setTUN(fd syscall.Handle, network string) error {
	var bytesReturned uint32
	rdbbuf := make([]byte, syscall.MAXIMUM_REPARSE_DATA_BUFFER_SIZE)

	localIP, remoteNet, err := net.ParseCIDR(network)
	if err != nil {
		return fmt.Errorf("Failed to parse network CIDR in config, %v", err)
	}
	if localIP.To4() == nil {
		return fmt.Errorf("Provided network(%s) is not a valid IPv4 address", network)
	}
	code2 := make([]byte, 0, 12)
	code2 = append(code2, localIP.To4()[:4]...)
	code2 = append(code2, remoteNet.IP.To4()[:4]...)
	code2 = append(code2, remoteNet.Mask[:4]...)
	if len(code2) != 12 {
		return fmt.Errorf("Provided network(%s) is not valid", network)
	}
	if err := syscall.DeviceIoControl(fd, tap_ioctl_config_tun, &code2[0], uint32(12), &rdbbuf[0], uint32(len(rdbbuf)), &bytesReturned, nil); err != nil {
		return err
	}
	return nil
}
