package browser

import "errors"

var ErrNoSession = errors.New("no user-controlled browser session is available; pass a fixture file or configure a browser debug endpoint")

// SessionConfig describes how profilepress may attach to a user-controlled browser.
type SessionConfig struct {
	CDPURL string
}

func (c SessionConfig) Validate() error {
	if c.CDPURL == "" {
		return ErrNoSession
	}
	return nil
}
