package main

import (
	"fmt"
	"net"
	"strings"
)

// researchListener enforces loopback-only operation. The server has no
// authentication or TLS, so merely displaying a warning is insufficient.
func researchListener(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil || port == "" {
		return fmt.Errorf("invalid -listen address (expected loopback host:port): %q", addr)
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("refusing non-loopback -listen address %q: this research API has no authentication or TLS", addr)
	}
	return nil
}
