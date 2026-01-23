package common

import (
	"fmt"
	"net"
	"net/url"
)

// ValidateWebhookUrl checks if the provided URL string is valid, uses HTTP/HTTPS,
// and does not point to a private or loopback IP address.
func ValidateWebhookUrl(rawUrl string) error {
	u, err := url.Parse(rawUrl)
	if err != nil {
		return fmt.Errorf("invalid url format: %w", err)
	}

	if u.Scheme != "https" {
		// We allow http for localhost for development and testing
		if u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1") {
			return nil
		}
		return fmt.Errorf("unsupported_protocol: url scheme must be https")
	}

	hostname := u.Hostname()
	if hostname == "" {
		return fmt.Errorf("url must have a hostname")
	}

	// Resolve hostname to IP(s)
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return fmt.Errorf("failed to lookup hostname: %w", err)
	}

	for _, ip := range ips {
		if isPrivateIP(ip) {
			return fmt.Errorf("url resolves to private ip: %s", ip.String())
		}
	}

	return nil
}

func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	// IPv4 Private Ranges
	// 10.0.0.0/8
	// 172.16.0.0/12
	// 192.168.0.0/16
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 10 {
			return true
		}
		if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
			return true
		}
		if ip4[0] == 192 && ip4[1] == 168 {
			return true
		}
		return false
	}

	// IPv6 Private Ranges (Unique Local Address fc00::/7)
	// fd00::/8 is practically what's used.
	if len(ip) == net.IPv6len {
		return (ip[0] & 0xfe) == 0xfc
	}

	return false
}
