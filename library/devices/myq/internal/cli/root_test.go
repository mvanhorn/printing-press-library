package cli

import (
	"testing"
)

func TestNewClientReadsClientSecretFromEnvironment(t *testing.T) {
	t.Setenv("MYQ_CLIENT_SECRET", "secret-123")

	c, err := newClient(&rootFlags{
		username: "user@example.com",
		password: "password-123",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := c.ClientSecret; got != "secret-123" {
		t.Fatalf("ClientSecret = %q, want %q", got, "secret-123")
	}
}
