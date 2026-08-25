// Copyright 2026 smoochy and contributors. Licensed under Apache-2.0. See LICENSE.

// HAND-AUTHORED. Guards the password-redaction layer described in synology.go:
// DSM carries the account password in the login request's query string, so any
// text quoting that URL must never reach a terminal, a log, or an error string.

package client

import (
	"errors"
	"strings"
	"testing"
)

func TestScrubDSMSecretsRedactsCredentialParams(t *testing.T) {
	const password = "S3cr3t!pass"
	cases := []string{
		`Get "https://nas.example.com:5001/webapi/entry.cgi?api=SYNO.API.Auth&method=login&account=admin&passwd=` + password + `": dial tcp: timeout`,
		"DSM login: passwd=" + password + "&otp_code=123456",
		"password=" + password,
	}
	for _, in := range cases {
		got := scrubDSMSecrets(in)
		if strings.Contains(got, password) {
			t.Fatalf("password survived scrubbing:\n  in:  %s\n  got: %s", in, got)
		}
		if !strings.Contains(got, "=***") {
			t.Fatalf("expected a redaction marker in %q", got)
		}
	}
}

func TestScrubDSMSecretsLeavesInnocentTextAlone(t *testing.T) {
	const in = "GET /webapi/entry.cgi?api=SYNO.Core.System&method=info&account=admin"
	if got := scrubDSMSecrets(in); got != in {
		t.Fatalf("scrubbing altered text with no credential:\n  in:  %s\n  got: %s", in, got)
	}
}

func TestScrubDSMSecretErrorKeepsCleanErrorsIdentical(t *testing.T) {
	clean := errors.New("connection refused")
	if got := scrubDSMSecretError(clean); got != clean {
		t.Fatalf("clean error was rewritten: %v", got)
	}
	if got := scrubDSMSecretError(errors.New("passwd=hunter2")); strings.Contains(got.Error(), "hunter2") {
		t.Fatalf("password survived error scrubbing: %v", got)
	}
}

func TestIsDSMSecretParam(t *testing.T) {
	for _, key := range []string{"passwd", "PASSWD", " otp_code ", "password"} {
		if !isDSMSecretParam(key) {
			t.Fatalf("%q should be treated as a secret parameter", key)
		}
	}
	for _, key := range []string{"account", "device_id", "api"} {
		if isDSMSecretParam(key) {
			t.Fatalf("%q should not be treated as a secret parameter", key)
		}
	}
}
