package wireguard

import (
	"bufio"
	"encoding/base64"
	"encoding/hex"
	"github.com/pkg/errors"
	"net"
	"os"
	"strconv"
	"strings"
)

func base64ToHex(base64Str string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(base64Str)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(decoded), nil
}

type Config struct {
	ListenPort int
	AllowedIPs []net.IPNet
	Protocol   string
}

// private_key=988cfb7e9b9531ce29d52ddedc3e2c89f92ad5f1ad833449b3f92072e6004561
// listen_port=58120
// public_key=CdjruGQqzRC5zUUQEPNjXRPlbmj5t/C0VzF+g93wGkM=
// allowed_ip=192.168.4.28/32
// persistent_keepalive_interval=25
func convertToConfigProtocol(configFile string) (*Config, error) {
	result := strings.Builder{}
	cfg := Config{}
	// read file
	config, err := os.Open(configFile)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	// parse configFile content
	scanner := bufio.NewScanner(config)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "=") {
			kv := strings.SplitN(line, " = ", 2)
			if len(kv) == 2 {
				key, value := kv[0], kv[1]

				switch key {
				case "PrivateKey":
					hexValue, err := base64ToHex(value)
					if err != nil {
						return nil, errors.WithStack(err)
					}
					result.WriteString("private_key=" + hexValue + "\n")
				case "PublicKey":
					hexValue, err := base64ToHex(value)
					if err != nil {
						return nil, errors.WithStack(err)
					}
					result.WriteString("public_key=" + hexValue + "\n")
				case "ListenPort":
					cfg.ListenPort, _ = strconv.Atoi(value)
					result.WriteString("listen_port=" + value + "\n")
				case "AllowedIPs":
					cidrs := strings.Split(value, ",")
					for _, item := range cidrs {
						cidr := strings.TrimSpace(item)
						_, network, err := net.ParseCIDR(cidr)
						if err != nil {
							return nil, errors.WithStack(err)
						}
						cfg.AllowedIPs = append(cfg.AllowedIPs, *network)
						result.WriteString("allowed_ip=" + cidr + "\n")
					}
				case "Endpoint":
					result.WriteString("endpoint=" + value + "\n")
				case "PersistentKeepalive":
					result.WriteString("persistent_keepalive_interval=" + value + "\n")
				}
			}
		}
	}
	cfg.Protocol = result.String()
	return &cfg, nil
}
