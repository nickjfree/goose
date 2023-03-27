package utils

import (
	"regexp"

	"github.com/pkg/errors"
)

var (
	// default gateway
	defaultGateway string
	// default interface
	defaultInterface string
)

func init() {
	var err error
	if defaultGateway, err = getDefaultGateway(); err != nil {
		logger.Fatalf("get default gateway error: %s", err)
	}
	logger.Printf("system gateway is %s, at interface %s", defaultGateway, defaultInterface)
}

// find system default gateway
func getDefaultGateway() (string, error) {
	// a simple solution. may not work well
	out, err := RunCmd("ip", "route", "get", "8.8.8.8")
	if err != nil {
		return "", errors.Wrap(err, string(out))
	}
	re := regexp.MustCompile(`.*via\s(.*)\sdev\s(.*?)\s`)
	matches := re.FindStringSubmatch(string(out))
	if len(matches) != 3 {
		return "", errors.Errorf("ip route output format not supported %s", out)
	}
	defaultInterface = matches[2]
	return matches[1], nil
}

func SetRoute(network string, gateway string) error {
	if out, err := RunCmd("ip", "route", "replace", network, "via", gateway); err != nil {
		return errors.Wrap(err, string(out))
	}
	return nil
}

func RemoveRoute(network string, gateway string) error {
	if out, err := RunCmd("ip", "route", "delete", network, "via", gateway); err != nil {
		logger.Printf("error remove route %s %s", string(out), err)
		return nil
	}
	return nil
}

// nat rules
func SetupNAT() error {
	// enabled ip forward
	if out, err := RunCmd("sysctl", "-w", "net.ipv4.ip_forward=1"); err != nil {
		return errors.Wrap(err, string(out))
	}
	if out, err := RunCmd("sysctl", "-p"); err != nil {
		return errors.Wrap(err, string(out))
	}
	//tcp mss clamp
	if out, err := RunCmd("iptables", "-t", "mangle", "-A", "FORWARD", "-p", "tcp",
		"--tcp-flags", "SYN,RST", "SYN", "-o", defaultInterface, "-j", "TCPMSS", "--set-mss", "940"); err != nil {
		return errors.Wrap(err, string(out))
	}
	// block DoH
	// iptables -A FORWARD -p tcp --dport 443 -d 8.8.8.8 -j DROP
	if out, err := RunCmd("iptables", "-A", "FORWARD", "-p", "tcp", "--dport", "443", "-d", "8.8.8.8", "-j", "DROP"); err != nil {
		return errors.Wrap(err, string(out))
	}
	// iptables -A FORWARD -p tcp --dport 443 -d 8.8.8.8 -j DROP
	if out, err := RunCmd("iptables", "-A", "FORWARD", "-p", "tcp", "--dport", "443", "-d", "8.8.4.4", "-j", "DROP"); err != nil {
		return errors.Wrap(err, string(out))
	}
	// iptables -A FORWARD -p tcp --dport 853 -j DROP
	if out, err := RunCmd("iptables", "-A", "FORWARD", "-p", "tcp", "--dport", "853", "-j", "DROP"); err != nil {
		return errors.Wrap(err, string(out))
	}
	// running as a router
	if out, err := RunCmd("iptables", "-t", "nat", "-A", "POSTROUTING", "-o", defaultInterface, "-j", "MASQUERADE"); err != nil {
		return errors.Wrap(err, string(out))
	}
	return nil
}
