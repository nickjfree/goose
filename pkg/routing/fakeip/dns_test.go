package fakeip

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/nickjfree/goose/pkg/message"
)

func appendDNSName(buf []byte, name string) []byte {
	for _, label := range bytes.Split([]byte(name), []byte(".")) {
		buf = append(buf, byte(len(label)))
		buf = append(buf, label...)
	}
	return append(buf, 0)
}

func appendRR(buf []byte, name string, typ uint16, rdata []byte) []byte {
	buf = appendDNSName(buf, name)
	hdr := make([]byte, 10)
	binary.BigEndian.PutUint16(hdr[0:], typ)
	binary.BigEndian.PutUint16(hdr[2:], 1)
	binary.BigEndian.PutUint32(hdr[4:], 60)
	binary.BigEndian.PutUint16(hdr[8:], uint16(len(rdata)))
	buf = append(buf, hdr...)
	return append(buf, rdata...)
}

// qr marks a response. qtype is the question type. answers and additionals are raw rdata records.
func dnsPacket(t *testing.T, qr bool, qtype uint16, answers, additionals [][2]interface{}) []byte {
	t.Helper()
	payload := make([]byte, 12)
	binary.BigEndian.PutUint16(payload[0:], 0x1234)
	if qr {
		binary.BigEndian.PutUint16(payload[2:], 0x8180)
	} else {
		binary.BigEndian.PutUint16(payload[2:], 0x0100)
	}
	binary.BigEndian.PutUint16(payload[4:], 1)
	binary.BigEndian.PutUint16(payload[6:], uint16(len(answers)))
	binary.BigEndian.PutUint16(payload[10:], uint16(len(additionals)))
	payload = appendDNSName(payload, "example.com")
	q := make([]byte, 4)
	binary.BigEndian.PutUint16(q[0:], qtype)
	binary.BigEndian.PutUint16(q[2:], 1)
	payload = append(payload, q...)
	for _, rr := range answers {
		payload = appendRR(payload, "example.com", rr[0].(uint16), rr[1].([]byte))
	}
	for _, rr := range additionals {
		payload = appendRR(payload, "example.com", rr[0].(uint16), rr[1].([]byte))
	}

	ip := &layers.IPv4{
		Version:  4,
		TTL:      64,
		Protocol: layers.IPProtocolUDP,
		SrcIP:    net.IPv4(8, 8, 8, 8),
		DstIP:    net.IPv4(10, 0, 0, 2),
	}
	udp := &layers.UDP{SrcPort: 53, DstPort: 5353}
	if !qr {
		udp.SrcPort = 5353
		udp.DstPort = 53
	}
	udp.SetNetworkLayerForChecksum(ip)
	buf := gopacket.NewSerializeBuffer()
	err := gopacket.SerializeLayers(buf, gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}, ip, udp, gopacket.Payload(payload))
	if err != nil {
		t.Fatalf("serialize: %s", err)
	}
	return buf.Bytes()
}

// priority 1, owner name as target, ipv4hint
func httpsRData(ip net.IP) []byte {
	return append([]byte{0x00, 0x01, 0x00, 0x00, 0x04, 0x00, 0x04}, ip.To4()...)
}

func answerIP(t *testing.T, data []byte) net.IP {
	t.Helper()
	packet := gopacket.NewPacket(data, layers.LayerTypeIPv4, gopacket.Default)
	dnsLayer := packet.Layer(layers.LayerTypeDNS)
	if dnsLayer == nil {
		t.Fatal("dns layer missing")
	}
	dns := dnsLayer.(*layers.DNS)
	if len(dns.Answers) != 1 || dns.Answers[0].Type != layers.DNSTypeA {
		t.Fatalf("answers = %+v", dns.Answers)
	}
	return dns.Answers[0].IP
}

func TestEgressDropsSVCBAndHTTPSResponses(t *testing.T) {
	manager := NewFakeIPManager("11.0.0.0/16", "", "")
	real := net.IPv4(1, 2, 3, 4).To4()
	hint := net.IPv4(9, 9, 9, 9).To4()

	a := dnsPacket(t, true, 1, [][2]interface{}{{uint16(1), []byte(real)}}, nil)
	packet := &message.Packet{Data: append([]byte(nil), a...)}
	drop, err := manager.Egress(packet)
	if err != nil || drop {
		t.Fatalf("A response drop=%v err=%v", drop, err)
	}
	if got := answerIP(t, packet.Data); !got.Equal(net.IPv4(11, 0, 0, 1)) {
		t.Fatalf("rewritten A = %s, want 11.0.0.1", got)
	}

	cases := []struct {
		name        string
		qtype       uint16
		answers     [][2]interface{}
		additionals [][2]interface{}
	}{
		{"https answer", 65, [][2]interface{}{{uint16(65), httpsRData(hint)}}, nil},
		{"svcb answer", 64, [][2]interface{}{{uint16(64), httpsRData(hint)}}, nil},
		{"https nodata", 65, nil, nil},
		{"a plus https additional", 1, [][2]interface{}{{uint16(1), []byte(real)}}, [][2]interface{}{{uint16(65), httpsRData(hint)}}},
	}
	for _, tc := range cases {
		raw := dnsPacket(t, true, tc.qtype, tc.answers, tc.additionals)
		original := append([]byte(nil), raw...)
		pkt := &message.Packet{Data: raw}
		drop, err := manager.Egress(pkt)
		if err != nil {
			t.Fatalf("%s: %s", tc.name, err)
		}
		if !drop {
			t.Fatalf("%s: response was forwarded", tc.name)
		}
		if !bytes.Equal(pkt.Data, original) {
			t.Fatalf("%s: dropped packet was modified", tc.name)
		}
	}

	query := dnsPacket(t, false, 65, nil, nil)
	queryOriginal := append([]byte(nil), query...)
	queryPacket := &message.Packet{Data: query}
	drop, err = manager.Egress(queryPacket)
	if err != nil || drop {
		t.Fatalf("HTTPS query drop=%v err=%v", drop, err)
	}
	if !bytes.Equal(queryPacket.Data, queryOriginal) {
		t.Fatal("HTTPS query was modified")
	}
}
