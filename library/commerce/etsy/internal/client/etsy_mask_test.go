package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/config"
)

func TestEtsyCredentialErrorMask(t *testing.T) {
	const key = "synthetic-key-abc123"
	const secret = "synthetic-secret-xyz789"
	for _, composed := range []bool{false, true} {
		name := "environment"
		if composed {
			name = "precomposed"
		}
		t.Run(name, func(t *testing.T) {
			isolateEtsyQuota(t)
			t.Setenv("ETSY_SHARED_SECRET", "")
			apiKey := key + ":" + secret
			if !composed {
				apiKey = key
				t.Setenv("ETSY_SHARED_SECRET", secret)
			}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("echo " + key + ":" + secret + " separately " + secret))
			}))
			defer srv.Close()
			c := New(&config.Config{BaseURL: srv.URL, EtsyApiKey: apiKey}, time.Second, 0)
			c.HTTPClient = srv.Client()
			_, err := c.Get(context.Background(), "/mask", nil)
			if err == nil {
				t.Fatal("expected API error")
			}
			if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), key+":"+secret) {
				t.Fatalf("credential leaked: %v", err)
			}
		})
	}
}
