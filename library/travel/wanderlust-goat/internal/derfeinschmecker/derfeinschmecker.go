// Package derfeinschmecker is a Stage-2 stub source. Real implementation deferred per
// the v2 brief — package exists so the regions table can name it without
// breaking the wiring test. Promoting to a real source = (a) replace this
// file with a real Client; (b) remove StubReason from the dispatcher's
// trace output.
package derfeinschmecker

import "github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/sourcetypes"

// StubReason is the user-facing explanation in coverage / status output.
const StubReason = "Der Feinschmecker is paywalled; deferred"

// Client is a stub source; embeds sourcetypes.StubClient.
type Client struct {
	*sourcetypes.StubClient
}

// NewClient returns the stub client.
func NewClient() *Client {
	return &Client{
		StubClient: &sourcetypes.StubClient{
			SlugName:   "derfeinschmecker",
			LocaleCode: "de",
			Reason:     StubReason,
		},
	}
}
