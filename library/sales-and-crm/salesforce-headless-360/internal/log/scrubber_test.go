package log

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestRedactKnownSecrets(t *testing.T) {
	input := `Authorization: Bearer abc.def {"access_token":"secret-access-value","refresh_token" : "secret-refresh-value","assertion":"secret-jwt-value"}`
	got := Redact(input)
	for _, secret := range []string{"abc.def", "secret-access-value", "secret-refresh-value", "secret-jwt-value"} {
		if strings.Contains(got, secret) {
			t.Fatalf("secret %q leaked in %q", secret, got)
		}
	}
}

func TestHandlerScrubsMessagesAndAttrs(t *testing.T) {
	var buf bytes.Buffer
	RegisterComplianceFields([]string{"SSN__c"})
	logger := slog.New(NewHandler(slog.NewTextHandler(&buf, nil)))
	logger.Info(`payload {"access_token":"secret-access-value","SSN__c":"123-45-6789"}`,
		"Authorization", "Bearer token-value",
		"refresh_token", "structured-refresh",
		"SSN__c", "123-45-6789",
	)
	out := buf.String()
	for _, secret := range []string{"secret-access-value", "token-value", "structured-refresh", "123-45-6789"} {
		if strings.Contains(out, secret) {
			t.Fatalf("secret %q leaked in %q", secret, out)
		}
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Fatalf("expected redaction marker in %q", out)
	}
}
