package route


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
func setServerRoute(serverIp string) error {
	if out, err := RunCmd("route", "add", "-host", fmt.Sprintf("%s/32", serverIp), "gw", defaultGateway); err != nil {
		return errors.Wrap(err, string(out))
	}
	return nil
}

// set traffic route
func setTrafficRoute(tunnelGateway string) error {
	// set 0.0.0.0 to use tunnelGateway
	if out, err := RunCmd("route", "add", "-net", "0.0.0.0/0", "gw", tunnelGateway); err != nil {
		return errors.Wrap(err, string(out))
	}
	// clear old traffic route
	if out, err := RunCmd("route", "delete", "-net", "0.0.0.0/0", "gw", defaultGateway); err != nil {
		return errors.Wrap(err, string(out))
	}
	return nil
}

// restore network
func RestoreRoute(tunnelGateway string, serverIp string) error {
	// set 0.0.0.0 to use virtual gateway
	if out, err := RunCmd("route", "add", "-net", "0.0.0.0/0", "gw", defaultGateway); err != nil {
		return errors.Wrap(err, string(out))
	}
	// clear old traffic route
	if out, err := RunCmd("route", "delete", "-net", "0.0.0.0/0", "gw", tunnelGateway); err != nil {
		return errors.Wrap(err, string(out))
	}
	// remove server route
	if out, err := RunCmd("route", "delete", "-host", fmt.Sprintf("%s/32", serverIp), "gw", defaultGateway); err != nil {
		return errors.Wrap(err, string(out))
	}
	return nil
}

// setup route
func SetupRoute(tunnelGateway string, serverIp string) error {
	if err := setServerRoute(serverIp); err != nil {
		return err
	}
	if err := setTrafficRoute(tunnelGateway); err != nil {
		return err
	}
	return nil
}
