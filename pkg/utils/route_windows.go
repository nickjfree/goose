package utils

import (
	"encoding/binary"
	"net"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"
)

var (
	// win32 api GetBestRoute
	nGetBestRoute uintptr
	// default gateway
	defaultGateway string
	// default interface index
	defaultIfIndex IF_INDEX
	// tap interface luid and index
	tapLuid    uint64
	tapIfIndex IF_INDEX
	routeMu    sync.Mutex

	procConvertInterfaceLuidToIndex = modiphlpapi.NewProc("ConvertInterfaceLuidToIndex")
	procDeleteIpForwardEntry2       = modiphlpapi.NewProc("DeleteIpForwardEntry2")
	procGetIpForwardTable           = modiphlpapi.NewProc("GetIpForwardTable")
	procDeleteIpForwardEntry        = modiphlpapi.NewProc("DeleteIpForwardEntry")
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
		logger.Fatalf("looadlibrary iphlpapi.dll error: %s", err)
	}
	defer syscall.FreeLibrary(iphlp)
	nGetBestRoute = getProcAddr(iphlp, "GetBestRoute")

	if err := initTapInterface(); err != nil {
		logger.Printf("init tap interface warning: %s", err)
	} else {
		if err := cleanTapRoutes(); err != nil {
			logger.Printf("clean tap routes warning: %s", err)
		}
		go cleanTapRoutesOnUp()
	}

	if defaultGateway, err = getDefaultGateway(); err != nil {
		logger.Fatalf("get default gateway error: %s", err)
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

func initTapInterface() error {
	luid, err := tapAdapterLUID(tapComponentID)
	if err != nil {
		return err
	}
	var idx IF_INDEX
	r0, _, _ := procConvertInterfaceLuidToIndex.Call(
		uintptr(unsafe.Pointer(&luid)),
		uintptr(unsafe.Pointer(&idx)),
	)
	runtime.KeepAlive(&luid)
	if r0 != 0 {
		return errors.Wrap(syscall.Errno(r0), "ConvertInterfaceLuidToIndex")
	}
	tapLuid = luid
	tapIfIndex = idx
	return nil
}

func isNextHopOnInterface(luid uint64, nextHop [4]byte, addrs []windows.MibUnicastIpAddressRow) bool {
	if nextHop == [4]byte{0, 0, 0, 0} {
		return true
	}
	nh := binary.BigEndian.Uint32(nextHop[:])
	for i := range addrs {
		if addrs[i].InterfaceLuid != luid {
			continue
		}
		addr := (*windows.RawSockaddrInet4)(unsafe.Pointer(&addrs[i].Address)).Addr
		ip := binary.BigEndian.Uint32(addr[:])
		if addrs[i].OnLinkPrefixLength == 0 || addrs[i].OnLinkPrefixLength > 32 {
			continue
		}
		mask := uint32(0xffffffff) << (32 - addrs[i].OnLinkPrefixLength)
		if (nh & mask) == (ip & mask) {
			return true
		}
	}
	return false
}

// cleanTapRoutes removes all leftover IPv4 routes bound to the TAP interface
// (using GetIpForwardTable2 so routes on disconnected interfaces are also seen)
// as well as orphaned goose 192.168.x.1 routes on other interfaces.
func cleanTapRoutes() error {
	routeMu.Lock()
	defer routeMu.Unlock()

	if tapLuid == 0 {
		return nil
	}

	var addrTable *windows.MibUnicastIpAddressTable
	var addrs []windows.MibUnicastIpAddressRow
	if err := windows.GetUnicastIpAddressTable(windows.AF_INET, &addrTable); err == nil && addrTable != nil {
		defer windows.FreeMibTable(unsafe.Pointer(addrTable))
		addrs = unsafe.Slice(&addrTable.Table[0], addrTable.NumEntries)
	}

	var table *windows.MibIpForwardTable2
	if err := windows.GetIpForwardTable2(windows.AF_INET, &table); err != nil {
		return errors.Wrap(err, "GetIpForwardTable2")
	}
	defer windows.FreeMibTable(unsafe.Pointer(table))

	rows := table.Rows()
	for i := range rows {
		if rows[i].Protocol == windows.MIB_IPPROTO_LOCAL {
			continue
		}
		nextHop := (*windows.RawSockaddrInet4)(unsafe.Pointer(&rows[i].NextHop)).Addr
		isTap := rows[i].InterfaceLuid == tapLuid || rows[i].InterfaceIndex == uint32(tapIfIndex)
		isOrphan := rows[i].Protocol == windows.MIB_IPPROTO_NETMGMT &&
			nextHop[0] == 192 && nextHop[1] == 168 && nextHop[3] == 1 &&
			len(addrs) > 0 && !isNextHopOnInterface(rows[i].InterfaceLuid, nextHop, addrs)

		if isTap || isOrphan {
			dst := (*windows.RawSockaddrInet4)(unsafe.Pointer(&rows[i].DestinationPrefix.Prefix)).Addr
			r0, _, _ := procDeleteIpForwardEntry2.Call(uintptr(unsafe.Pointer(&rows[i])))
			logger.Printf("clean stale route %s/%d -> %s (if=%d, err=%d)",
				net.IP(dst[:]), rows[i].DestinationPrefix.PrefixLength, net.IP(nextHop[:]), rows[i].InterfaceIndex, r0)
		}
	}

	var size uint32
	procGetIpForwardTable.Call(0, uintptr(unsafe.Pointer(&size)), 0)
	if size >= 4 {
		buf := make([]byte, size)
		r0, _, _ := procGetIpForwardTable.Call(
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(unsafe.Pointer(&size)),
			0,
		)
		runtime.KeepAlive(&size)
		if r0 == 0 {
			numEntries := *(*uint32)(unsafe.Pointer(&buf[0]))
			if numEntries > 0 {
				mibRows := unsafe.Slice(
					(*MIB_IPFORWARDROW)(unsafe.Pointer(&buf[4])),
					numEntries,
				)
				for i := range mibRows {
					if mibRows[i].DwForwardIfIndex == tapIfIndex &&
						mibRows[i].ForwardProto != MIB_IPFORWARD_PROTO(windows.MIB_IPPROTO_LOCAL) {
						procDeleteIpForwardEntry.Call(uintptr(unsafe.Pointer(&mibRows[i])))
					}
				}
			}
		}
		runtime.KeepAlive(buf)
	}
	return nil
}

// cleanTapRoutesOnUp waits for the TAP adapter to transition to IfOperStatusUp
// (after water.New opens it) and cleans any routes Windows restores on connect.
func cleanTapRoutesOnUp() {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var ifRow windows.MibIfRow2
		ifRow.InterfaceLuid = tapLuid
		if err := windows.GetIfEntry2Ex(windows.MibIfEntryNormalWithoutStatistics, &ifRow); err == nil {
			if ifRow.OperStatus == windows.IfOperStatusUp {
				_ = cleanTapRoutes()
				time.Sleep(time.Second)
				_ = cleanTapRoutes()
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
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

func SetRoute(network string, gateway string) error {
	routeMu.Lock()
	defer routeMu.Unlock()

	if network == "0.0.0.0/0" {
		// change default route
		if out, err := RunCmd("route", "change", network, gateway); err != nil {
			return errors.Wrap(err, string(out))
		}
	} else if tapIfIndex != 0 && gateway != defaultGateway {
		if out, err := RunCmd("route", "add", network, gateway, "if", strconv.Itoa(int(tapIfIndex))); err != nil {
			return errors.Wrap(err, string(out))
		}
	} else {
		if out, err := RunCmd("route", "add", network, gateway); err != nil {
			return errors.Wrap(err, string(out))
		}
	}
	return nil
}

func RemoveRoute(network string, gateway string) error {
	if out, err := RunCmd("route", "delete", network, gateway); err != nil {
		return errors.Wrap(err, string(out))
	}
	return nil
}

// nat rules
func SetupNAT(tun string) error {
	return nil
}
