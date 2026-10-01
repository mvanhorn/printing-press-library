package pocket

import "github.com/mvanhorn/printing-press-library/library/travel/pocket-concierge/internal/client"

// Client combines public domain queries with the single bounded HTTP transport.
type Client struct{ *client.Client }
type Error = client.Error

const Endpoint = client.Endpoint

func New(cacheDir string, refresh, noCache bool) *Client {
	return &Client{client.New(cacheDir, refresh, noCache)}
}
func Fail(code, message string) error { return client.Fail(code, message) }
func ExitCode(err error) int          { return client.ExitCode(err) }
func ErrorJSON(err error) any         { return client.ErrorJSON(err) }
