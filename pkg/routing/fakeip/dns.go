package fakeip

import (
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/pkg/errors"
	"log"
	"net"
	"os"

	"goose/pkg/message"
)

var (
	logger = log.New(os.Stdout, "fakeip: ", log.LstdFlags|log.Lshortfile)
)

// checksum layer
type ChecksumLayer interface {
	SetNetworkLayerForChecksum(gopacket.NetworkLayer) error
}

func (manager *FakeIPManager) FakeDnsResponse(p *message.Packet) error {

	packet := gopacket.NewPacket(p.Data, layers.LayerTypeIPv4, gopacket.Default)

	if ipLayer := packet.Layer(layers.LayerTypeIPv4); ipLayer != nil {
		ip := ipLayer.(*layers.IPv4)
		// must be udp packet
		if udpLayer := packet.Layer(layers.LayerTypeUDP); udpLayer != nil {
			udp := udpLayer.(*layers.UDP)
			// decode dns layer
			if dnsLayer := packet.Layer(layers.LayerTypeDNS); dnsLayer != nil {
				dns := dnsLayer.(*layers.DNS)
				// we only want response packet
				if !dns.QR {
					return nil
				}
				for i, ans := range dns.Answers {
					if ans.Type == layers.DNSTypeA && !ans.IP.Equal(net.IPv4(8, 8, 8, 8)) {
						fakeIP, err := manager.Alloc(string(ans.Name), ans.IP)
						if err != nil {
							return err
						}
						// replace anwser to fake ip
						dns.Answers[i].IP = fakeIP
						// serialize modifyed  packet
						buffer := gopacket.NewSerializeBuffer()
						options := gopacket.SerializeOptions{
							ComputeChecksums: true,
							FixLengths:       true,
						}
						udp.SetNetworkLayerForChecksum(ip)
						if err := gopacket.SerializePacket(buffer, options, packet); err != nil {
							return errors.WithStack(err)
						}
						p.Src = fakeIP
						p.Data = buffer.Bytes()
					}
				}
			}
		}
	}
	return nil
}

// replace fake ip to real ip
func (manager *FakeIPManager) DNAT(p *message.Packet) error {

	packet := gopacket.NewPacket(p.Data, layers.LayerTypeIPv4, gopacket.Default)

	if ipLayer := packet.Layer(layers.LayerTypeIPv4); ipLayer != nil {
		ip := ipLayer.(*layers.IPv4)
		if dst := manager.ToReal(ip.DstIP); dst != nil {
			// set network layers
			for _, layer := range packet.Layers() {
				if checksum, ok := layer.(ChecksumLayer); ok {
					checksum.SetNetworkLayerForChecksum(ip)
				}
			}
			ip.DstIP = *dst
			buffer := gopacket.NewSerializeBuffer()
			options := gopacket.SerializeOptions{
				ComputeChecksums: true,
				FixLengths:       true,
			}
			if err := gopacket.SerializePacket(buffer, options, packet); err != nil {
				return errors.WithStack(err)
			}
			p.Dst = *dst
			p.Data = buffer.Bytes()
		}
	}
	return nil
}

// replace real src ip to fake ip
func (manager *FakeIPManager) SNAT(p *message.Packet) error {

	packet := gopacket.NewPacket(p.Data, layers.LayerTypeIPv4, gopacket.Default)

	if ipLayer := packet.Layer(layers.LayerTypeIPv4); ipLayer != nil {
		ip := ipLayer.(*layers.IPv4)
		if src := manager.ToFake(ip.SrcIP); src != nil {
			// set network layers
			for _, layer := range packet.Layers() {
				if checksum, ok := layer.(ChecksumLayer); ok {
					checksum.SetNetworkLayerForChecksum(ip)
				}
			}
			ip.SrcIP = *src
			buffer := gopacket.NewSerializeBuffer()
			options := gopacket.SerializeOptions{
				ComputeChecksums: true,
				FixLengths:       true,
			}
			if err := gopacket.SerializePacket(buffer, options, packet); err != nil {
				return errors.WithStack(err)
			}
			p.Src = *src
			p.Data = buffer.Bytes()
		}
	}
	return nil
}
