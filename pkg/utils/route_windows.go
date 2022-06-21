package utils


import (
	"fmt"
	"net"
	"syscall"
	"time"
	"unsafe"

	"github.com/pkg/errors"
)

var (
	// win32 api GetBestRoute
	nGetBestRoute uintptr
	// default gateway
	defaultGateway string
	// default interface index
	defaultIfIndex IF_INDEX
)

type (
	DWORD               uint32
	ULONG               uint32
	NET_IFINDEX         ULONG
	IF_INDEX            NET_IFINDEX
	NL_ROUTE_PROTOCOL   int32
	MIB_IPFORWARD_PROTO NL_ROUTE_PROTOCOL
	MIB_IPFORWARD_TYPE  int32
)

func init() {
	iphlp, err := syscall.LoadLibrary("iphlpapi.dll")
	if err != nil {
		logger.Fatalf("looadlibrary iphlpapi.dll error: %+v", err)
	}
	defer syscall.FreeLibrary(iphlp)
	nGetBestRoute = getProcAddr(iphlp, "GetBestRoute")

	if defaultGateway, err = getDefaultGateway(); err != nil {
		logger.Fatalf("get default gateway error: %+v", err)
	}
	logger.Printf("system gateway is %s, at interface %d", defaultGateway, defaultIfIndex)
}

func getProcAddr(lib syscall.Handle, name string) uintptr {
	addr, err := syscall.GetProcAddress(lib, name)
	if err != nil {
		panic(name + " " + err.Error())
	}
	return addr
}

type MIB_IPFORWARDROW struct {
	DwForwardDest      DWORD
	DwForwardMask      DWORD
	DwForwardPolicy    DWORD
	DwForwardNextHop   DWORD
	DwForwardIfIndex   IF_INDEX
	ForwardType        MIB_IPFORWARD_TYPE
	ForwardProto       MIB_IPFORWARD_PROTO
	DwForwardAge       DWORD
	DwForwardNextHopAS DWORD
	DwForwardMetric1   DWORD
	DwForwardMetric2   DWORD
	DwForwardMetric3   DWORD
	DwForwardMetric4   DWORD
	DwForwardMetric5   DWORD
}

func dwordIP(d DWORD) (ip net.IP) {
	ip = make(net.IP, net.IPv4len)
	ip[0] = byte(d & 0xff)
	ip[1] = byte((d >> 8) & 0xff)
	ip[2] = byte((d >> 16) & 0xff)
	ip[3] = byte((d >> 24) & 0xff)
	return
}

func ipDword(ip net.IP) (d DWORD) {
	ip = ip.To4()
	d |= DWORD(ip[0]) << 0
	d |= DWORD(ip[1]) << 8
	d |= DWORD(ip[2]) << 16
	d |= DWORD(ip[3]) << 24
	return
}

// find system default gateway
func getDefaultGateway() (string, error) {
	var row MIB_IPFORWARDROW
	_, _, err := syscall.Syscall(nGetBestRoute, 3,
		uintptr(ipDword(net.ParseIP("8.8.8.8"))),
		uintptr(ipDword(net.ParseIP("0.0.0.0"))),
		uintptr(unsafe.Pointer(&row)))
	if err != syscall.Errno(0) {
		return "", err
	}
	// record default interface index
	defaultIfIndex = row.DwForwardIfIndex
	return dwordIP(row.DwForwardNextHop).String(), nil
}


// wait tunnel status up
func waitTunnelUp(tunnelGateway string) error {
	for {	
		var row MIB_IPFORWARDROW
		_, _, err := syscall.Syscall(nGetBestRoute, 3,
			uintptr(ipDword(net.ParseIP(tunnelGateway))),
			uintptr(ipDword(net.ParseIP("0.0.0.0"))),
			uintptr(unsafe.Pointer(&row)))
		if err != syscall.Errno(0) {
			return err
		}
		if row.DwForwardIfIndex != defaultIfIndex {
			// tunnelGateway is at tap interface now
			return nil
		} 
		// continue wait
		time.Sleep(5 * time.Second)
	}
}

// set route for server
func SetWireRoute(serverIp string) error {
	if out, err := RunCmd("route", "add", fmt.Sprintf("%s/32", serverIp), defaultGateway); err != nil {
		return errors.Wrap(err, string(out))
	}
	return nil
}

func SetRoute(network string, gateway string) error {
	if out, err := RunCmd("route", "add", network, gateway); err != nil {
		return errors.Wrap(err, string(out))
	}
	return nil
}

func RemoveRoute(network string, gateway string) error {
	if out, err := RunCmd("route", "delete", network, gateway); err != nil {
		return errors.Wrap(err, string(out))
	}
	return nil
}

// restore route for server
func RestoreWireRoute(serverIp string) error {
	if out, err := RunCmd("route", "delete", fmt.Sprintf("%s/32", serverIp), defaultGateway); err != nil {
		return errors.Wrap(err, string(out))
	}
	return nil
}

// restore default route
func RestoreDefaultRoute() error {
	if out, err := RunCmd("route", "change", "0.0.0.0/0", defaultGateway); err != nil {
		return errors.Wrap(err, string(out))
	}
	return nil
}

// nat rules
func SetupNAT() error {
	return errors.Errorf("can not forward packets on windows")
}