package client

import (
	"github.com/mvanhorn/printing-press-library/library/commerce/shopper/internal/config"
	"testing"
)

func TestShopperHeadersInjected(t *testing.T) {
	cfg := &config.Config{BaseURL: "https://siteapi.shopper.com.br"}
	c := New(cfg, 0, 0)

	required := map[string]string{
		"app-os-x-version": "web:1002",
		"x-store-id":       "1",
		"x-cluster-id":     "1",
	}
	for k, want := range required {
		got := c.Config.Headers[k]
		if got != want {
			t.Errorf("header %q = %q, want %q", k, got, want)
		}
	}
}
