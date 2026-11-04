package utils

import (
	"runtime"
	"syscall"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	tapDriverKey    = `SYSTEM\CurrentControlSet\Control\Class\{4D36E972-E325-11CE-BFC1-08002BE10318}`
	tapComponentID  = "tap0901"

	rpcCAuthnWinnt           = 10
	fwpmSessionFlagDynamic   = 0x00000001
	fwpUint8                 = 1
	fwpUint16                = 2
	fwpUint64                = 4
	fwpMatchEqual            = 0
	fwpActionFlagTerminating = 0x00001000
	fwpActionBlock           = 0x00000001 | fwpActionFlagTerminating
	fwpActionPermit          = 0x00000002 | fwpActionFlagTerminating

	dnsPort           = 53
	wfpWeightPermit   = 15
	wfpWeightBlock    = 1
	wfpSubLayerWeight = 0xffff
)

var (
	modiphlpapi = windows.NewLazySystemDLL("iphlpapi.dll")
	modfwpuclnt = windows.NewLazySystemDLL("fwpuclnt.dll")

	procConvertInterfaceGuidToLuid = modiphlpapi.NewProc("ConvertInterfaceGuidToLuid")

	procFwpmEngineOpen0        = modfwpuclnt.NewProc("FwpmEngineOpen0")
	procFwpmEngineClose0       = modfwpuclnt.NewProc("FwpmEngineClose0")
	procFwpmTransactionBegin0  = modfwpuclnt.NewProc("FwpmTransactionBegin0")
	procFwpmTransactionCommit0 = modfwpuclnt.NewProc("FwpmTransactionCommit0")
	procFwpmTransactionAbort0  = modfwpuclnt.NewProc("FwpmTransactionAbort0")
	procFwpmSubLayerAdd0       = modfwpuclnt.NewProc("FwpmSubLayerAdd0")
	procFwpmFilterAdd0         = modfwpuclnt.NewProc("FwpmFilterAdd0")

	// FWPM_LAYER_ALE_AUTH_CONNECT_V4 = {c38d57d1-05a7-4c33-904f-7fbceee60e82}
	fwpmLayerALEAuthConnectV4 = windows.GUID{
		Data1: 0xc38d57d1,
		Data2: 0x05a7,
		Data3: 0x4c33,
		Data4: [8]byte{0x90, 0x4f, 0x7f, 0xbc, 0xee, 0xe6, 0x0e, 0x82},
	}

	// FWPM_LAYER_ALE_AUTH_CONNECT_V6 = {4a72393b-319f-44bc-84c3-ba54dcb3b6b4}
	fwpmLayerALEAuthConnectV6 = windows.GUID{
		Data1: 0x4a72393b,
		Data2: 0x319f,
		Data3: 0x44bc,
		Data4: [8]byte{0x84, 0xc3, 0xba, 0x54, 0xdc, 0xb3, 0xb6, 0xb4},
	}

	// FWPM_CONDITION_IP_REMOTE_PORT = {c35a604d-d22b-4e1a-91b4-68f674ee674b}
	fwpmConditionIPRemotePort = windows.GUID{
		Data1: 0xc35a604d,
		Data2: 0xd22b,
		Data3: 0x4e1a,
		Data4: [8]byte{0x91, 0xb4, 0x68, 0xf6, 0x74, 0xee, 0x67, 0x4b},
	}

	// FWPM_CONDITION_IP_LOCAL_INTERFACE = {4cd62a49-59c3-4969-b7f3-bda5d32890a4}
	fwpmConditionIPLocalInterface = windows.GUID{
		Data1: 0x4cd62a49,
		Data2: 0x59c3,
		Data3: 0x4969,
		Data4: [8]byte{0xb7, 0xf3, 0xbd, 0xa5, 0xd3, 0x28, 0x90, 0xa4},
	}
)

// FWPM_DISPLAY_DATA0
type fwpmDisplayData0 struct {
	name        *uint16
	description *uint16
}

// FWPM_SESSION0
type fwpmSession0 struct {
	sessionKey           windows.GUID
	displayData          fwpmDisplayData0
	flags                uint32
	txnWaitTimeoutInMSec uint32
	processId            uint32
	sid                  uintptr
	username             *uint16
	kernelMode           int32
}

// FWP_BYTE_BLOB
type fwpByteBlob struct {
	size uint32
	data *byte
}

// FWPM_SUBLAYER0
type fwpmSubLayer0 struct {
	subLayerKey  windows.GUID
	displayData  fwpmDisplayData0
	flags        uint16
	providerKey  *windows.GUID
	providerData fwpByteBlob
	weight       uint16
}

// FWP_VALUE0 / FWP_CONDITION_VALUE0
type fwpValue0 struct {
	dataType uint32
	value    uintptr
}

// FWPM_FILTER_CONDITION0
type fwpmFilterCondition0 struct {
	fieldKey       windows.GUID
	matchType      uint32
	conditionValue fwpValue0
}

// FWPM_ACTION0
type fwpmAction0 struct {
	actionType uint32
	filterType windows.GUID
}

// FWPM_FILTER0
type fwpmFilter0 struct {
	filterKey           windows.GUID
	displayData         fwpmDisplayData0
	flags               uint32
	providerKey         *windows.GUID
	providerData        fwpByteBlob
	layerKey            windows.GUID
	subLayerKey         windows.GUID
	weight              fwpValue0
	numFilterConditions uint32
	filterCondition     *fwpmFilterCondition0
	action              fwpmAction0
	// union { UINT64 rawContext; GUID providerContextKey; } requires 8-byte alignment and 16-byte size
	rawContext      [2]uint64
	reserved        *windows.GUID
	filterId        uint64
	effectiveWeight fwpValue0
}

// tapAdapterLUID finds the TAP adapter with the given ComponentId in the
// Windows registry and converts its NetCfgInstanceId GUID to a 64-bit NET_LUID.
func tapAdapterLUID(componentID string) (uint64, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, tapDriverKey, registry.READ)
	if err != nil {
		return 0, errors.Wrap(err, "open tap adapter registry key")
	}
	defer k.Close()

	keys, err := k.ReadSubKeyNames(-1)
	if err != nil {
		return 0, errors.WithStack(err)
	}

	var instanceID string
	for _, sub := range keys {
		subKey, err := registry.OpenKey(registry.LOCAL_MACHINE, tapDriverKey+"\\"+sub, registry.READ)
		if err != nil {
			continue
		}
		comp, _, err := subKey.GetStringValue("ComponentId")
		if err != nil || comp != componentID {
			subKey.Close()
			continue
		}
		id, _, err := subKey.GetStringValue("NetCfgInstanceId")
		subKey.Close()
		if err != nil {
			continue
		}
		instanceID = id
		break
	}
	if instanceID == "" {
		return 0, errors.Errorf("tap adapter with ComponentId %s not found in registry", componentID)
	}

	guid, err := windows.GUIDFromString(instanceID)
	if err != nil {
		return 0, errors.Wrapf(err, "parse tap NetCfgInstanceId %s", instanceID)
	}

	var luid uint64
	r0, _, _ := procConvertInterfaceGuidToLuid.Call(
		uintptr(unsafe.Pointer(&guid)),
		uintptr(unsafe.Pointer(&luid)),
	)
	runtime.KeepAlive(&guid)
	if r0 != 0 {
		return 0, errors.Wrapf(syscall.Errno(r0), "ConvertInterfaceGuidToLuid(%s)", instanceID)
	}
	return luid, nil
}

// BlockOutsideDNS installs dynamic WFP filters that permit DNS (port 53) on the
// Goose TAP interface and block DNS on all other IPv4 and IPv6 interfaces.
// Because the session uses FWPM_SESSION_FLAG_DYNAMIC, Windows automatically
// removes the filters when the process exits.
func BlockOutsideDNS() error {
	luid, err := tapAdapterLUID(tapComponentID)
	if err != nil {
		return err
	}
	if err := setupWFPDNSFilter(luid); err != nil {
		return err
	}
	logger.Printf("block dns on all interfaces except tap (luid 0x%x)", luid)
	if out, err := RunCmd("ipconfig", "/flushdns"); err != nil {
		logger.Printf("flush dns cache failed: %s (%s)", err, string(out))
	}
	return nil
}

func setupWFPDNSFilter(tapLuid uint64) error {
	namePtr, err := windows.UTF16PtrFromString("Goose DNS Filter")
	if err != nil {
		return errors.WithStack(err)
	}
	descPtr, err := windows.UTF16PtrFromString("Block DNS requests outside the Goose TAP interface")
	if err != nil {
		return errors.WithStack(err)
	}
	display := fwpmDisplayData0{
		name:        namePtr,
		description: descPtr,
	}

	session := fwpmSession0{
		displayData: display,
		flags:       fwpmSessionFlagDynamic,
	}

	var engine windows.Handle
	r0, _, _ := procFwpmEngineOpen0.Call(
		0,
		uintptr(rpcCAuthnWinnt),
		0,
		uintptr(unsafe.Pointer(&session)),
		uintptr(unsafe.Pointer(&engine)),
	)
	runtime.KeepAlive(&session)
	if r0 != 0 {
		return errors.Wrap(syscall.Errno(r0), "FwpmEngineOpen0")
	}

	r0, _, _ = procFwpmTransactionBegin0.Call(uintptr(engine), 0)
	if r0 != 0 {
		procFwpmEngineClose0.Call(uintptr(engine))
		return errors.Wrap(syscall.Errno(r0), "FwpmTransactionBegin0")
	}

	abortAndClose := func(cause error) error {
		procFwpmTransactionAbort0.Call(uintptr(engine))
		procFwpmEngineClose0.Call(uintptr(engine))
		return cause
	}

	subLayerKey, err := windows.GenerateGUID()
	if err != nil {
		return abortAndClose(errors.WithStack(err))
	}
	subLayer := fwpmSubLayer0{
		subLayerKey: subLayerKey,
		displayData: display,
		weight:      wfpSubLayerWeight,
	}
	r0, _, _ = procFwpmSubLayerAdd0.Call(
		uintptr(engine),
		uintptr(unsafe.Pointer(&subLayer)),
		0,
	)
	runtime.KeepAlive(&subLayer)
	if r0 != 0 {
		return abortAndClose(errors.Wrap(syscall.Errno(r0), "FwpmSubLayerAdd0"))
	}

	// 1. Permit IPv4 DNS requests exiting through the Goose TAP interface (higher weight).
	permitTapConds := buildPermitTAPConditions(&tapLuid)
	if err := addWFPFilter(engine, subLayerKey, fwpmLayerALEAuthConnectV4, display, wfpWeightPermit, fwpActionPermit, permitTapConds); err != nil {
		return abortAndClose(err)
	}
	runtime.KeepAlive(&tapLuid)

	// 2. Block IPv4 DNS requests on all other interfaces (lower weight).
	blockConds := buildBlockDNSConditions()
	if err := addWFPFilter(engine, subLayerKey, fwpmLayerALEAuthConnectV4, display, wfpWeightBlock, fwpActionBlock, blockConds); err != nil {
		return abortAndClose(err)
	}

	// 3. Block all IPv6 DNS requests so Windows cannot bypass IPv4 fake-IP via IPv6 DNS.
	if err := addWFPFilter(engine, subLayerKey, fwpmLayerALEAuthConnectV6, display, wfpWeightBlock, fwpActionBlock, blockConds); err != nil {
		return abortAndClose(err)
	}

	r0, _, _ = procFwpmTransactionCommit0.Call(uintptr(engine))
	runtime.KeepAlive(namePtr)
	runtime.KeepAlive(descPtr)
	if r0 != 0 {
		return abortAndClose(errors.Wrap(syscall.Errno(r0), "FwpmTransactionCommit0"))
	}

	return nil
}

func buildPermitTAPConditions(tapLuid *uint64) []fwpmFilterCondition0 {
	return []fwpmFilterCondition0{
		{
			fieldKey:  fwpmConditionIPRemotePort,
			matchType: fwpMatchEqual,
			conditionValue: fwpValue0{
				dataType: fwpUint16,
				value:    uintptr(dnsPort),
			},
		},
		{
			fieldKey:  fwpmConditionIPLocalInterface,
			matchType: fwpMatchEqual,
			conditionValue: fwpValue0{
				dataType: fwpUint64,
				value:    uintptr(unsafe.Pointer(tapLuid)),
			},
		},
	}
}

func buildBlockDNSConditions() []fwpmFilterCondition0 {
	return []fwpmFilterCondition0{
		{
			fieldKey:  fwpmConditionIPRemotePort,
			matchType: fwpMatchEqual,
			conditionValue: fwpValue0{
				dataType: fwpUint16,
				value:    uintptr(dnsPort),
			},
		},
	}
}

func addWFPFilter(
	engine windows.Handle,
	subLayerKey windows.GUID,
	layerKey windows.GUID,
	display fwpmDisplayData0,
	weight uint8,
	action uint32,
	conditions []fwpmFilterCondition0,
) error {
	filter := fwpmFilter0{
		displayData: display,
		layerKey:    layerKey,
		subLayerKey: subLayerKey,
		weight: fwpValue0{
			dataType: fwpUint8,
			value:    uintptr(weight),
		},
		numFilterConditions: uint32(len(conditions)),
		filterCondition:     &conditions[0],
		action: fwpmAction0{
			actionType: action,
		},
	}
	var filterID uint64
	r0, _, _ := procFwpmFilterAdd0.Call(
		uintptr(engine),
		uintptr(unsafe.Pointer(&filter)),
		0,
		uintptr(unsafe.Pointer(&filterID)),
	)
	runtime.KeepAlive(&filter)
	runtime.KeepAlive(conditions)
	if r0 != 0 {
		return errors.Wrap(syscall.Errno(r0), "FwpmFilterAdd0")
	}
	return nil
}
