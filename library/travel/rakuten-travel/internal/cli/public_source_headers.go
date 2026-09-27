// Anonymous Rakuten document routes require HTML content negotiation. This
// preserved hook applies the declared source header without editing the
// generated client or any persisted user/shared configuration.
package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/rakuten-travel/internal/client"
	"net/url"
	"strings"
)

const publicSourceHTMLAccept = "text/html,application/xhtml+xml"

func configurePublicSourceHeaders(c *client.Client) error {
	if c == nil || c.Config == nil {
		return nil
	}
	base, err := url.Parse(c.BaseURL)
	if err != nil || base.Scheme != "https" {
		return nil
	}
	switch base.Hostname() {
	case "travel.rakuten.co.jp", "hotel.travel.rakuten.co.jp", "kw.travel.rakuten.co.jp":
	default:
		return nil
	}
	// Required source representation overrides conflicting JSON negotiation.
	// Copy both the struct and map so invocation setup cannot mutate a caller's
	// config snapshot. Config.Headers also participates in client cache identity.
	config := *c.Config
	config.Headers = make(map[string]string, len(c.Config.Headers)+1)
	for name, value := range c.Config.Headers {
		if !strings.EqualFold(name, "Accept") {
			config.Headers[name] = value
		}
	}
	config.Headers["Accept"] = publicSourceHTMLAccept
	c.Config = &config
	return nil
}

func init() { registerClientHook(configurePublicSourceHeaders) }
