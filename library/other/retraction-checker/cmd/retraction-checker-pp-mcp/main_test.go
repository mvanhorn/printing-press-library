// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

package main

import "testing"

func TestIsLoopbackBind(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{addr: "127.0.0.1:7777", want: true},
		{addr: "127.0.0.2:7777", want: true},
		{addr: "[::1]:7777", want: true},
		{addr: "localhost:7777", want: false},
		{addr: "LOCALHOST.:7777", want: false},
		{addr: "api.localhost:7777", want: false},
		{addr: "127.0.0.1.example:7777", want: false},
		{addr: ":7777", want: false},
		{addr: "0.0.0.0:7777", want: false},
		{addr: "[::]:7777", want: false},
		{addr: "192.0.2.1:7777", want: false},
		{addr: "not-an-address", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.addr, func(t *testing.T) {
			if got := isLoopbackBind(tc.addr); got != tc.want {
				t.Fatalf("isLoopbackBind(%q) = %v, want %v", tc.addr, got, tc.want)
			}
		})
	}
}
