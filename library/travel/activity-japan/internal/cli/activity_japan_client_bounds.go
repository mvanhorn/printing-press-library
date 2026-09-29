package cli

import (
	"errors"
	"io"
	"net/http"

	"github.com/mvanhorn/printing-press-library/library/travel/activity-japan/internal/client"
)

const activityJapanMaxBody = 2 << 20

var errActivityJapanBodyTooLarge = errors.New("Activity Japan response exceeds 2 MiB limit")

type activityJapanBoundedTransport struct{ base http.RoundTripper }

func (t activityJapanBoundedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	r.Body = &activityJapanBoundedBody{ReadCloser: r.Body, remaining: activityJapanMaxBody}
	return r, nil
}

type activityJapanBoundedBody struct {
	io.ReadCloser
	remaining int
}

func (b *activityJapanBoundedBody) Read(p []byte) (int, error) {
	if b.remaining == 0 {
		var probe [1]byte
		n, err := b.ReadCloser.Read(probe[:])
		if n > 0 {
			return 0, errActivityJapanBodyTooLarge
		}
		return 0, err
	}
	if len(p) > b.remaining {
		p = p[:b.remaining]
	}
	n, err := b.ReadCloser.Read(p)
	b.remaining -= n
	return n, err
}
func init() {
	registerClientHook(func(c *client.Client) error {
		base := c.HTTPClient.Transport
		if base == nil {
			base = http.DefaultTransport
		}
		c.HTTPClient.Transport = activityJapanBoundedTransport{base: base}
		// Plan and session observations are always refreshed. Inventory has its own bounded cache.
		c.NoCache = true
		return nil
	})
}
