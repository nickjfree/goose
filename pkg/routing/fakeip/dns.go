package fakeip

import (
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/pkg/errors"
	"log"
	"net"
	"os"

	"github.com/nickjfree/goose/pkg/message"
)

var (
	logger = log.New(os.Stdout, "fakeip: ", log.LstdFlags|log.Lshortfile)
)

// checksum layer
type ChecksumLayer interface {
	SetNetworkLayerForChecksum(gopacket.NetworkLayer) error
}

func (manager *FakeIPManager) fakeDnsResponse(p *message.Packet) error {

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
				// modify the dns answers
				for i, ans := range dns.Answers {
					if ans.Type == layers.DNSTypeA && !ans.IP.Equal(net.IPv4(8, 8, 8, 8)) && !ans.IP.Equal(net.IPv4(8, 8, 4, 4)) {
						// skip matched name or ip
						if manager.rule != nil {
							if manager.rule.MatchDomain(string(ans.Name)) {
								return nil
							}
							if manager.rule.MatchDomain(ans.IP.String()) {
								return nil
							}
						}
						fakeIP, err := manager.alloc(string(ans.Name), ans.IP)
						if err != nil {
							return err
						}
						// replace anwser to fake ip
						dns.Answers[i].IP = fakeIP
					}
				}
				// handle NXDomain
				// we apply custome dns records here as a fallback, if there are any.
				if dns.ResponseCode == layers.DNSResponseCodeNXDomain {
					if len(dns.Questions) == 1 {
						ips := manager.GetNameRecord(string(dns.Questions[0].Name))
						if len(ips) > 0 {
							for _, ip := range ips {
								aRecord := layers.DNSResourceRecord{
									Name:  dns.Questions[0].Name, // Name of the domain
									Type:  layers.DNSTypeA,       // Type A (IPv4 address)
									Class: layers.DNSClassIN,     // Class IN (Internet)
									TTL:   180,                   // Time to Live
									IP:    ip,                    // IPv4 address
								}
								// Add the A record to the answer section
								dns.Answers = append(dns.Answers, aRecord)
							}
							dns.ANCount = uint16(len(ips))
							dns.ResponseCode = layers.DNSResponseCodeNoErr
						}
					}
				}
				// serialize the modifyed packet
				buffer := gopacket.NewSerializeBuffer()
				options := gopacket.SerializeOptions{
					ComputeChecksums: true,
					FixLengths:       true,
				}
				udp.SetNetworkLayerForChecksum(ip)
				if err := gopacket.SerializePacket(buffer, options, packet); err != nil {
					return errors.WithStack(err)
				}
				p.Data = buffer.Bytes()
			}
		}
	}
	return nil
}

// replace fake ip to real ip
func (manager *FakeIPManager) dNAT(p *message.Packet) error {

	packet := gopacket.NewPacket(p.Data, layers.LayerTypeIPv4, gopacket.Default)

	if ipLayer := packet.Layer(layers.LayerTypeIPv4); ipLayer != nil {
		ip := ipLayer.(*layers.IPv4)
		if dst := manager.toReal(ip.DstIP); dst != nil {
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
func (manager *FakeIPManager) sNAT(p *message.Packet) error {

	packet := gopacket.NewPacket(p.Data, layers.LayerTypeIPv4, gopacket.Default)

	if ipLayer := packet.Layer(layers.LayerTypeIPv4); ipLayer != nil {
		ip := ipLayer.(*layers.IPv4)
		if src := manager.toFake(ip.SrcIP); src != nil {
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

// replace dst ipaddress
func (manager *FakeIPManager) Ingress(p *message.Packet) (bool, error) {
	return false, manager.dNAT(p)
}

// replace src ipaddress
func (manager *FakeIPManager) Egress(p *message.Packet) (bool, error) {
	drop := false
	if err := manager.fakeDnsResponse(p); err != nil {
		return drop, err
	}
	// replace src ip to fake ip
	if err := manager.sNAT(p); err != nil {
		return drop, err
	}
	return drop, nil
}
