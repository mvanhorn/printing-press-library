package client

import (
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/pointhound/internal/config"
)

func TestAuthHeaderExpiredRefreshTokenReturnsActionableError(t *testing.T) {
	c := &Client{
		Config: &config.Config{
			AccessToken:  "stale-token",
			RefreshToken: "refresh-token",
			TokenExpiry:  time.Now().Add(-time.Hour),
		},
	}

	_, err := c.authHeader()
	if err == nil {
		t.Fatalf("expected expired refresh token to return an error")
	}
	if !strings.Contains(err.Error(), "auth login --chrome") {
		t.Fatalf("expected auth login hint, got %q", err.Error())
	}
}
