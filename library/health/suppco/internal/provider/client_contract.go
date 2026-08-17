package provider

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/health/suppco/internal/client"
)

const APIOrigin = "https://api.supp.co"

// ConfigureClient makes provider reads stateless and refuses credentials on a
// different origin. A loopback origin is allowed only for PrintingPress mock
// verification; live dogfood does not use that mode.
func ConfigureClient(c *client.Client, allowLoopbackTestOrigin bool) error {
	if c == nil || c.HTTPClient == nil {
		return errors.New("SuppCo client is not initialized")
	}
	if c.Config != nil && c.Config.SkipTLSVerify {
		return errors.New("refusing SuppCo client with TLS verification disabled")
	}
	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return fmt.Errorf("parse SuppCo base URL: %w", err)
	}
	if !validBaseOrigin(base, allowLoopbackTestOrigin) {
		return fmt.Errorf("refusing non-canonical SuppCo API origin %q; expected %s", c.BaseURL, APIOrigin)
	}
	if c.Config == nil {
		return errors.New("SuppCo bearer token is required")
	}
	authScheme, authValue, hasAuthValue := strings.Cut(strings.TrimSpace(c.Config.AuthHeader()), " ")
	if !hasAuthValue || !strings.EqualFold(authScheme, "Bearer") || strings.TrimSpace(authValue) == "" {
		return errors.New("SuppCo bearer token is required")
	}
	for name := range c.Config.Headers {
		if strings.EqualFold(name, "Authorization") {
			return errors.New("SuppCo provider requests do not accept generic Authorization headers; use a bearer token")
		}
		if strings.EqualFold(name, "Cookie") {
			return errors.New("SuppCo provider requests do not accept saved Cookie headers")
		}
	}

	c.NoCache = true
	// SuppCo authentication is bearer-only. Ignore Set-Cookie responses so a
	// long-lived CLI or MCP process never accumulates or replays browser state.
	c.HTTPClient.Jar = nil
	previousRedirect := c.HTTPClient.CheckRedirect
	c.HTTPClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		if req == nil || len(via) == 0 || via[0] == nil || via[0].URL == nil || req.URL == nil {
			return errors.New("refusing redirect without an origin")
		}
		if req.URL.Scheme != via[0].URL.Scheme || req.URL.Host != via[0].URL.Host {
			return fmt.Errorf("refusing cross-origin redirect from %s://%s to %s://%s", via[0].URL.Scheme, via[0].URL.Host, req.URL.Scheme, req.URL.Host)
		}
		if previousRedirect != nil {
			return previousRedirect(req, via)
		}
		return nil
	}
	return nil
}

func validBaseOrigin(base *url.URL, allowLoopback bool) bool {
	if base == nil || base.User != nil || base.RawQuery != "" || base.Fragment != "" || strings.TrimRight(base.Path, "/") != "" {
		return false
	}
	if base.Scheme == "https" && base.Host == "api.supp.co" {
		return true
	}
	if !allowLoopback || base.Scheme != "http" && base.Scheme != "https" {
		return false
	}
	host := base.Hostname()
	ip := net.ParseIP(host)
	return host == "localhost" || ip != nil && ip.IsLoopback()
}
