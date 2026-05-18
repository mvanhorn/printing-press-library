// Bridge between Cobra command handlers and the hand-written OmniLogic
// client + store. The generator emits a generic client expecting REST+JSON;
// OmniLogic is XML-RPC-over-HTTP with two-stage auth, so we bypass the
// generated client entirely and call the omnilogic package directly.

package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/hayward-omnilogic/internal/omnilogic"
	"github.com/mvanhorn/printing-press-library/library/devices/hayward-omnilogic/internal/store"
)

const (
	envUser = "HAYWARD_USER"
	envPW   = "HAYWARD_PW"
)

// envCredentials reads the OmniLogic credentials from the environment.
// Returns empty strings when unset — callers decide whether to surface that
// as an error (most commands) or carry on (doctor).
func envCredentials() (email, password string) {
	return strings.TrimSpace(os.Getenv(envUser)), os.Getenv(envPW)
}

// newOmnilogicClient builds a client from the user's environment. It does
// NOT trigger a login; the first operation (or doctor) lazily authenticates.
func newOmnilogicClient(timeout time.Duration) *omnilogic.Client {
	email, pw := envCredentials()
	return omnilogic.New(email, pw, timeout)
}

// requireCreds returns ErrMissingCredentials when either env var is unset.
// Used by every command that performs a real cloud call.
func requireCreds() error {
	email, pw := envCredentials()
	if email == "" || pw == "" {
		return omnilogic.ErrMissingCredentials
	}
	return nil
}

// requireCredsUnlessDryRun is the verifier-friendly variant: in dry-run mode
// it returns nil (so structural verification probes succeed without
// credentials), otherwise it behaves like requireCreds. Use this in mutation
// commands so verify's --dry-run probe still exits 0 even when no env vars
// are set in the verify subprocess.
func requireCredsUnlessDryRun(flags *rootFlags) error {
	if flags != nil && flags.dryRun {
		return nil
	}
	return requireCreds()
}

// openStore opens the local SQLite store. Best effort: returns a store on
// success or an explanatory error.
func openStore() (*store.Store, error) {
	return store.Open("")
}

// resolveSite picks the site to operate on. With a hint, that exact site
// must exist. Without, the only registered site is used. The site list is
// loaded from cache first; falls back to a live GetSiteList.
func resolveSite(c *omnilogic.Client, s *store.Store, hint int) (omnilogic.Site, error) {
	if s != nil {
		sites, err := s.ListSites()
		if err == nil && len(sites) > 0 {
			site, err := omnilogic.ResolveSite(sites, hint)
			if err == nil {
				return site, nil
			}
		}
	}
	sites, err := c.GetSiteList()
	if err != nil {
		return omnilogic.Site{}, fmt.Errorf("listing sites: %w", err)
	}
	if s != nil {
		_ = s.UpsertSites(sites)
	}
	return omnilogic.ResolveSite(sites, hint)
}

// resolveMspConfig returns the live MSP config for a site, falling back to
// the latest cached snapshot if the cloud call fails.
func resolveMspConfig(c *omnilogic.Client, s *store.Store, siteID int) (*omnilogic.MspConfig, error) {
	if c != nil {
		cfg, err := c.GetMspConfig(siteID)
		if err == nil {
			if s != nil {
				_ = s.UpsertMspConfig(cfg)
			}
			return cfg, nil
		}
		if s == nil {
			return nil, err
		}
	}
	if s == nil {
		return nil, errors.New("no client and no store available")
	}
	cfg, err := s.LatestMspConfig(siteID)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("no cached MSP config for site %d; run 'sync' first", siteID)
	}
	return cfg, nil
}

// classifyOmnilogicError adapts our package's errors to the CLI's typed
// exit-code envelope. Unauthenticated/missing-creds maps to exit 4; API
// errors carry HTTP status into the exit-code class.
func classifyOmnilogicError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, omnilogic.ErrMissingCredentials) {
		return &cliError{
			code: 4,
			err:  fmt.Errorf("%s and %s must be set in your environment.\nExport them or add them to your shell profile:\n  export %s='your-email@example.com'\n  export %s='your-password'", envUser, envPW, envUser, envPW),
		}
	}
	var authErr *omnilogic.AuthError
	if errors.As(err, &authErr) {
		hint := ""
		if authErr.StatusCode == 401 {
			hint = "\nHint: post-Oct-2025 the API expects an EMAIL in HAYWARD_USER, not a username."
		}
		return &cliError{
			code: 4,
			err:  fmt.Errorf("Hayward auth failed (HTTP %d).%s\nDetails: %s", authErr.StatusCode, hint, omnilogic.Truncate(authErr.Body, 200)),
		}
	}
	var apiErr *omnilogic.APIError
	if errors.As(err, &apiErr) {
		code := 5
		if apiErr.StatusCode >= 500 {
			code = 6
		}
		return &cliError{
			code: code,
			err:  fmt.Errorf("%s", apiErr.Error()),
		}
	}
	return err
}
