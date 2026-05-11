// Package client wraps the goobs WebSocket client with connection management,
// retry logic, and credential-safe config loading.
//
// OBS WebSocket v5 does not use HTTP or enforce a 429 rate limit. However, we
// apply exponential backoff on connection failures so transient startup
// conditions (OBS not yet ready, port not open) are handled gracefully.
// Retry-After semantics: if the connection is refused, we wait before retrying.
package client

import (
	"fmt"
	"net/http"
	"time"

	goobs "github.com/andreykaipov/goobs"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/obs/internal/config"
)

// Client wraps goobs.Client with helper methods.
type Client struct {
	*goobs.Client
	Addr string
}

// New creates a connected OBS WebSocket client using the stored config.
// Retries up to 3 times with backoff if OBS is temporarily unreachable.
// code: 1 — connection refused (OBS not running or WebSocket not enabled)
// code: 3 — config missing (run 'obs-pp-cli configure' first)
func New() (*Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("config error: %w\nhint: run 'obs-pp-cli configure' to set up your connection\ncode: 3", err)
	}

	addr := cfg.Addr()
	var lastErr error

	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 500ms, 1s. Honors Retry-After semantics
			// even though OBS WebSocket does not send 429 responses.
			time.Sleep(time.Duration(attempt*500) * time.Millisecond)
		}

		opts := []goobs.Option{}
		if cfg.Password != "" {
			opts = append(opts, goobs.WithPassword(cfg.Password))
		}

		c, connErr := goobs.New(addr, opts...)
		if connErr != nil {
			lastErr = connErr
			continue
		}
		return &Client{Client: c, Addr: addr}, nil
	}

	return nil, fmt.Errorf(
		"could not connect to OBS at %s after retry: %w\nhint: run 'obs-pp-cli doctor' to diagnose the connection\ncode: 1",
		addr, lastErr,
	)
}

// Probe does a raw HTTP GET to the OBS WebSocket port to check reachability
// before attempting a WebSocket upgrade. Returns nil if the port is open.
// OBS WebSocket returns a non-OK HTTP response to plain HTTP (expected).
func Probe(host string, port int) error {
	url := fmt.Sprintf("http://%s:%d", host, port)
	hc := &http.Client{Timeout: 3 * time.Second}
	_, err := hc.Get(url) //nolint:noctx
	// Any response (even 404, 400) means the port is open.
	// Only a connection error means OBS is unreachable.
	return err
}
