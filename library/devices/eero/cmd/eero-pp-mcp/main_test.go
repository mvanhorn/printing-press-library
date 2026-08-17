package main

import "testing"

func TestLoopbackAddress(t *testing.T) {
	for _, address := range []string{"127.0.0.1:7777", "[::1]:7777", "localhost:7777"} {
		if !isLoopbackAddress(address) {
			t.Errorf("%s should be loopback", address)
		}
	}
	for _, address := range []string{"0.0.0.0:7777", ":7777", "192.168.1.20:7777"} {
		if isLoopbackAddress(address) {
			t.Errorf("%s should not be loopback", address)
		}
	}
}
