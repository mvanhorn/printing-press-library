// Copyright 2026 Chirantan Rajhans and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: SUBSTACK_PUBLICATION validation. Not generator-emitted.

package config

import (
	"fmt"
	"strings"
)

// SetPublication validates and stores the publication subdomain used to fill
// {publication} in publication-scoped Substack endpoints.
func (c *Config) SetPublication(subdomain string) error {
	subdomain = strings.TrimSpace(subdomain)
	if subdomain == "" {
		return nil
	}
	if !validPublicationLabel(subdomain) {
		return fmt.Errorf("invalid publication subdomain %q: use a single DNS label such as mypub", subdomain)
	}
	if c.TemplateVars == nil {
		c.TemplateVars = map[string]string{}
	}
	c.TemplateVars["publication"] = subdomain
	return nil
}

// validPublicationLabel reports whether s is a single DNS label safe to
// substitute into the {publication}.substack.com Creator host. The publication
// is always one subdomain label; rejecting punctuation (/, @, :, ., %, …) stops
// a crafted SUBSTACK_PUBLICATION from steering the authenticated Creator
// request off-platform — e.g. "x.evil.com/" would otherwise yield host
// "x.evil.com" after buildURL substitutes it into the base URL.
func validPublicationLabel(s string) bool {
	if s == "" || len(s) > 63 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '-' && i != 0 && i != len(s)-1:
		default:
			return false
		}
	}
	return true
}
