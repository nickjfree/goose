package utils

import (
	"testing"
	"unsafe"
)

// verify that the WFP Go struct layouts match the Windows SDK 64-bit C ABI
func TestWFPStructLayout(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) == 8 {
		if got := unsafe.Sizeof(fwpmDisplayData0{}); got != 16 {
			t.Errorf("fwpmDisplayData0 size = %d, want 16", got)
		}
		if got := unsafe.Sizeof(fwpmSession0{}); got != 72 {
			t.Errorf("fwpmSession0 size = %d, want 72", got)
		}
		if got := unsafe.Sizeof(fwpByteBlob{}); got != 16 {
			t.Errorf("fwpByteBlob size = %d, want 16", got)
		}
		if got := unsafe.Sizeof(fwpmSubLayer0{}); got != 72 {
			t.Errorf("fwpmSubLayer0 size = %d, want 72", got)
		}
		if got := unsafe.Sizeof(fwpValue0{}); got != 16 {
			t.Errorf("fwpValue0 size = %d, want 16", got)
		}
		if got := unsafe.Sizeof(fwpmFilterCondition0{}); got != 40 {
			t.Errorf("fwpmFilterCondition0 size = %d, want 40", got)
		}
		if got := unsafe.Sizeof(fwpmAction0{}); got != 20 {
			t.Errorf("fwpmAction0 size = %d, want 20", got)
		}
		if got := unsafe.Sizeof(fwpmFilter0{}); got != 200 {
			t.Errorf("fwpmFilter0 size = %d, want 200", got)
		}
		var f fwpmFilter0
		if got := unsafe.Offsetof(f.rawContext); got != 152 {
			t.Errorf("fwpmFilter0.rawContext offset = %d, want 152", got)
		}
		if got := unsafe.Offsetof(f.filterId); got != 176 {
			t.Errorf("fwpmFilter0.filterId offset = %d, want 176", got)
		}
	}
}

func TestBuildDNSFilterConditions(t *testing.T) {
	luid := uint64(0x0006000000000001)
	permitConds := buildPermitTAPConditions(&luid)
	if len(permitConds) != 2 {
		t.Fatalf("permit conditions count = %d, want 2", len(permitConds))
	}
	if permitConds[0].fieldKey != fwpmConditionIPRemotePort || permitConds[0].conditionValue.value != dnsPort {
		t.Errorf("unexpected permit port condition: %+v", permitConds[0])
	}
	if permitConds[1].fieldKey != fwpmConditionIPLocalInterface || permitConds[1].conditionValue.dataType != fwpUint64 {
		t.Errorf("unexpected permit interface condition: %+v", permitConds[1])
	}
	if permitConds[1].conditionValue.value != uintptr(unsafe.Pointer(&luid)) {
		t.Errorf("permit condition luid pointer = 0x%x, want 0x%x", permitConds[1].conditionValue.value, uintptr(unsafe.Pointer(&luid)))
	}

	blockConds := buildBlockDNSConditions()
	if len(blockConds) != 1 {
		t.Fatalf("block conditions count = %d, want 1", len(blockConds))
	}
	if blockConds[0].fieldKey != fwpmConditionIPRemotePort || blockConds[0].conditionValue.value != dnsPort {
		t.Errorf("unexpected block port condition: %+v", blockConds[0])
	}
}
