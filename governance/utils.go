package governance

import (
	"fmt"
	"net"
	"strconv"
)

// GetLocalIP Get local IP address
func GetLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1", err
	}

	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}

	return "127.0.0.1", fmt.Errorf("no valid IP found")
}

// FormatServiceAddress Format service address
func FormatServiceAddress(host string, port int) string {
	return net.JoinHostPort(host, strconv.Itoa(port))
}

// Parse service address
// Format: "127.0.0.1:9002" -> ("127.0.0.1", 9002)
func ParseServiceAddress(address string) (string, int, error) {
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return "", 0, fmt.Errorf("invalid address format: %w", err)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid port: %w", err)
	}

	return host, port, nil
}

// GenerateInstanceID Generate instance ID
func GenerateInstanceID(serviceName, address string, port int) string {
	return fmt.Sprintf("%s-%s-%d", serviceName, address, port)
}

