// Package dianping is a Stage-2 stub source. Real implementation deferred per
// the v2 brief — package exists so the regions table can name it without
// breaking the wiring test. Promoting to a real source = (a) replace this
// file with a real Client; (b) remove StubReason from the dispatcher's
// trace output.
package dianping

import "github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/sourcetypes"

// StubReason is the user-facing explanation in coverage / status output.
const StubReason = "Dianping requires Chinese-IP and app key; deferred"

// Client is a stub source; embeds sourcetypes.StubClient.
type Client struct {
	*sourcetypes.StubClient
}

// NewClient returns the stub client.
func NewClient() *Client {
	return &Client{
		StubClient: &sourcetypes.StubClient{
			SlugName:   "dianping",
			LocaleCode: "zh",
			Reason:     StubReason,
		},
	}
}
