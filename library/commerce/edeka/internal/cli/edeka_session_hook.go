package cli

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/edeka/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/edeka/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/edeka/internal/edeka"
)

// The spec-generated market commands (suggest, slots selected/reservation)
// call the placeholder host market.edeka.shop. This hook sends them to the
// selected market and gives them the imported session as the only login
// state: the generated persistent cookie jar is swapped for an in-memory jar
// seeded from the session file, and every Set-Cookie the market answers with
// is merged back into that file. A second cookie copy would go stale as the
// shop rotates its session cookie.
func init() {
	registerClientHook(func(c *client.Client) error {
		if c == nil || c.HTTPClient == nil || cliutil.IsVerifyEnv() {
			return nil
		}
		path, _ := edeka.SessionPath()
		s, err := edeka.LoadSession(path)
		if err != nil {
			s, path = &edeka.Session{}, ""
		}
		host, err := edeka.ResolveMarket("", s)
		if err != nil {
			return nil // the command reports the request error; market finder still works
		}
		next := c.HTTPClient.Transport
		if next == nil {
			next = http.DefaultTransport
		}
		// the rewrite runs first, so the write-back sees the real market host
		c.HTTPClient.Transport = marketRewriteTransport{next: edeka.SessionWriteBack(next, path), host: host}
		jar, err := cookiejar.New(nil)
		if err != nil {
			return err
		}
		u, _ := url.Parse("https://" + edeka.PlaceholderHost + "/")
		jar.SetCookies(u, s.HostCookies(host))
		c.HTTPClient.Jar = jar
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
