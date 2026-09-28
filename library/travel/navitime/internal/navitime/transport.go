package navitime

import (
	"github.com/enetx/surf"
	"net/http"
	"time"
)

// NewHTTPClient supplies the verified Firefox transport without cookies/auth.
// Callers set their own redirect policy; the provider refuses redirects.
func NewHTTPClient(timeout time.Duration) (*http.Client, error) {
	c, err := surf.NewClient().Builder().Impersonate().Firefox().Timeout(timeout).Build().Result()
	if err != nil {
		return nil, err
	}
	std := c.Std()
	std.Timeout = timeout
	std.Jar = nil
	return std, nil
}
