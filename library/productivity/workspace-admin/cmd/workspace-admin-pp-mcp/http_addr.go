package main

import (
	"fmt"
	"net"
)

// HTTP has no built-in client authentication or TLS, so it must stay local.
// Remote deployments must place an authenticated TLS proxy in front of it.
func loopbackHTTPAddr(addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("invalid HTTP listen address: %w", err)
	}
	if host == "localhost" {
		host = "127.0.0.1"
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return "", fmt.Errorf("HTTP MCP requires a loopback IP address; use an authenticated TLS proxy for remote access")
	}
	return net.JoinHostPort(ip.String(), port), nil
}
