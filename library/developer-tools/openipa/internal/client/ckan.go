// Hand-written addition: CKAN client for IPA open data portal — preserve on regeneration.

package client

import (
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/openipa/internal/cliutil"
)

const ckanBaseURL = "https://indicepa.gov.it/ipa-dati"

// NewCKAN returns a Client pre-configured for the IPA CKAN open-data portal.
// No AUTH_ID is needed; Config is nil so authHeader() returns "".
func NewCKAN(timeout time.Duration) *Client {
	return &Client{
		BaseURL:    ckanBaseURL,
		HTTPClient: newHTTPClient(timeout, nil),
		limiter:    cliutil.NewAdaptiveLimiter(2),
	}
}
