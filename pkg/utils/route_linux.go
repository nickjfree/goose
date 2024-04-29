package utils

import (
	"github.com/pkg/errors"
	"regexp"
	"strings"
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

// ensure iptables rule
func iptablesEnsureRule(table, chain string, rule ...string) error {
	cmd := []string{"-t", table, "-C", chain}
	cmd = append(cmd, rule...)
	// check rule exists
	for {
		if _, err := RunCmd("iptables", cmd...); err != nil {
			// if something went wrong with the command
			if !strings.Contains(err.Error(), "Bad rule") {
				return err
			}
			// change to add
			cmd[2] = "-A"
			continue
		}
		return nil
	}
}

// ensure iptables chain
func iptablesEnsureChain(table, chain string) error {
	cmd := []string{"-t", table, "-L", chain}
	// check chain exists
	for {
		if _, err := RunCmd("iptables", cmd...); err != nil {
			// if something went wrong with the command
			if !strings.Contains(err.Error(), chain) {
				return err
			}
			// change to add
			cmd[2] = "-N"
			continue
		}
		return nil
	}
}

type Rule struct {
	Table string
	Chain string
	Rule  []string
}

// set up iptables rules when running as a router
func SetupNAT(tun string) error {
	// enabled ip forward
	if out, err := RunCmd("sysctl", "-w", "net.ipv4.ip_forward=1"); err != nil {
		return errors.Wrap(err, string(out))
	}
	if out, err := RunCmd("sysctl", "-p"); err != nil {
		return errors.Wrap(err, string(out))
	}

	mssClamp := []Rule{
		{
			Table: "mangle",
			Chain: "GOOSE-FORWARD",
			Rule:  []string{"-p", "tcp", "--tcp-flags", "SYN,RST", "SYN", "-i", tun, "-j", "TCPMSS", "--set-mss", "940"},
		},
		{
			Table: "mangle",
			Chain: "GOOSE-FORWARD",
			Rule:  []string{"-p", "tcp", "--tcp-flags", "SYN,RST", "SYN", "-o", tun, "-j", "TCPMSS", "--set-mss", "940"},
		},
	}

	// only to masquerate for packates from the tun interface
	markMASQ := []Rule{
		{
			Table: "mangle",
			Chain: "GOOSE-FORWARD",
			Rule:  []string{"-i", tun, "-j", "MARK", "--set-xmark", "0x0200/0x0200"},
		},
	}

	// block DoH, so we can intercept DNS responses
	blockDoH := []Rule{
		// block DoH
		{
			Table: "filter",
			Chain: "GOOSE-FORWARD",
			Rule:  []string{"-i", tun, "-p", "tcp", "--dport", "443", "-d", "8.8.8.8", "-j", "DROP"},
		},
		{
			Table: "filter",
			Chain: "GOOSE-FORWARD",
			Rule:  []string{"-i", tun, "-p", "tcp", "--dport", "443", "-d", "8.8.4.4", "-j", "DROP"},
		},
		{
			Table: "filter",
			Chain: "GOOSE-FORWARD",
			Rule:  []string{"-i", tun, "-p", "tcp", "--dport", "53", "-j", "DROP"},
		},
		{
			Table: "filter",
			Chain: "GOOSE-FORWARD",
			Rule:  []string{"-i", tun, "-p", "tcp", "--dport", "853", "-j", "DROP"},
		},
		{
			Table: "filter",
			Chain: "GOOSE-FORWARD",
			Rule:  []string{"-i", tun, "-p", "tcp", "--dport", "443", "-d", "8.8.4.4", "-j", "DROP"},
		},
	}

	// masq
	masq := []Rule{
		{
			Table: "nat",
			Chain: "GOOSE-MASQ",
			Rule:  []string{"-m", "mark", "--mark", "0x0200/0x0200", "-j", "MASQUERADE"},
		},
	}

	// system rule
	system := []Rule{
		{
			Table: "filter",
			Chain: "FORWARD",
			Rule:  []string{"-j", "GOOSE-FORWARD"},
		},
		{
			Table: "mangle",
			Chain: "FORWARD",
			Rule:  []string{"-j", "GOOSE-FORWARD"},
		},
		{
			Table: "nat",
			Chain: "POSTROUTING",
			Rule:  []string{"-j", "GOOSE-MASQ"},
		},
	}

	// ensure all rules exists
	for _, rules := range [][]Rule{mssClamp, markMASQ, blockDoH, masq, system} {
		for _, rule := range rules {
			//  ensure chain
			if err := iptablesEnsureChain(rule.Table, rule.Chain); err != nil {
				return err
			}
			// ensure rule
			if err := iptablesEnsureRule(rule.Table, rule.Chain, rule.Rule...); err != nil {
				return err
			}
		}
	}

	return nil
}
