package cli

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/edeka/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/edeka/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/edeka/internal/edeka"
)

// The spec-generated market commands (suggest, slots selected/reservation)
// call the placeholder host market.edeka.shop. This hook sends them to the
// selected market and gives them the imported session cookies, instead of a
// second cookie copy in the generated credential store (which would go stale
// as the shop rotates its session cookie).
func init() {
	registerClientHook(func(c *client.Client) error {
		if c == nil || c.HTTPClient == nil || cliutil.IsVerifyEnv() {
			return nil
		}
		path, _ := edeka.SessionPath()
		s, err := edeka.LoadSession(path)
		if err != nil {
			s = &edeka.Session{}
		}
		host, err := edeka.ResolveMarket("", s)
		if err != nil {
			return nil // the command reports the request error; market finder still works
		}
		next := c.HTTPClient.Transport
		if next == nil {
			next = http.DefaultTransport
		}
		c.HTTPClient.Transport = marketRewriteTransport{next: next, host: host}
		if c.HTTPClient.Jar != nil {
			u, _ := url.Parse("https://" + edeka.PlaceholderHost + "/")
			c.HTTPClient.Jar.SetCookies(u, s.HostCookies(host))
		}
		return nil
	})
}

// marketRewriteTransport replaces the placeholder host with the market host.
type marketRewriteTransport struct {
	next http.RoundTripper
	host string
}

func (t marketRewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.EqualFold(req.URL.Hostname(), edeka.PlaceholderHost) {
		r := req.Clone(req.Context())
		r.URL.Host = t.host
		r.Host = t.host
		return t.next.RoundTrip(r)
	}
	return t.next.RoundTrip(req)
}
