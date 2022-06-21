package utils


import (
	"fmt"
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
		logger.Fatalf("get default gateway error: %+v", err)
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

// set route for server
func SetWireRoute(serverIp string) error {
	if out, err := RunCmd("ip", "route", "add", fmt.Sprintf("%s/32", serverIp), "via", defaultGateway); err != nil {
		return errors.Wrap(err, string(out))
	}
	return nil
}

// restore route for server
func RestoreWireRoute(serverIp string) error {
	if out, err := RunCmd("ip", "route", "delete", fmt.Sprintf("%s/32", serverIp), "via", defaultGateway); err != nil {
		return errors.Wrap(err, string(out))
	}
	return nil
}

// remove default route
func RemoveDefaultRoute() error {
	if out, err := RunCmd("ip", "route", "delete", "0.0.0.0/0", "via", defaultGateway); err != nil {
		return errors.Wrap(err, string(out))
	}
	return nil
}

// restore default route
func RestoreDefaultRoute() error {
	if out, err := RunCmd("ip", "route", "add", "0.0.0.0/0", "via", defaultGateway); err != nil {
		return errors.Wrap(err, string(out))
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
	// running as a router
	if out, err := RunCmd("iptables", "-t", "nat", "-A", "POSTROUTING", "-o", defaultInterface, "-j", "MASQUERADE"); err != nil {
		return errors.Wrap(err, string(out))
	}
	return nil
}
