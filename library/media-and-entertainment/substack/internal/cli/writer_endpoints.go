// Copyright 2026 Chirantan Rajhans and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack/internal/client"
)

const substackGlobalAPIBase = "https://substack.com/api/v1"
const dryRunPublicationIDPlaceholder = "<resolved-at-runtime>"

func globalAPIPath(path string) string {
	return substackGlobalAPIBase + path
}

func globalAPIPathWithParams(path string, params map[string]string) string {
	if len(params) == 0 {
		return globalAPIPath(path)
	}
	q := url.Values{}
	for k, v := range params {
		if v != "" {
			q.Set(k, v)
		}
	}
	if encoded := q.Encode(); encoded != "" {
		return globalAPIPath(path) + "?" + encoded
	}
	return globalAPIPath(path)
}

func writerPublicationID(ctx context.Context, c *client.Client, flags *rootFlags) (string, error) {
	if id := strings.TrimSpace(flags.publicationID); id != "" {
		if !validPublicationID(id) {
			return "", fmt.Errorf("invalid --publication-id %q: use the numeric Substack publication_id", id)
		}
		return id, nil
	}
	if id := strings.TrimSpace(os.Getenv("SUBSTACK_PUBLICATION_ID")); id != "" {
		if !validPublicationID(id) {
			return "", fmt.Errorf("invalid SUBSTACK_PUBLICATION_ID %q: use the numeric Substack publication_id", id)
		}
		return id, nil
	}
	if c != nil && c.Config != nil {
		if id, ok := firstPublicationID(c.Config.TemplateVars["publication_id"]); ok {
			return id, nil
		}
	}
	if flags.dryRun {
		return dryRunPublicationIDPlaceholder, nil
	}
	id, err := resolveWriterPublicationIDFromProfile(ctx, c, flags.subdomain)
	if err != nil {
		return "", err
	}
	return id, nil
}

func firstPublicationID(candidates ...string) (string, bool) {
	for _, candidate := range candidates {
		id := strings.TrimSpace(candidate)
		if id == "" {
			continue
		}
		if !validPublicationID(id) {
			continue
		}
		return id, true
	}
	return "", false
}

func validPublicationID(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func resolveWriterPublicationIDFromProfile(ctx context.Context, c *client.Client, subdomain string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("could not resolve publication_id: client is nil")
	}
	raw, err := c.Get(ctx, "/user/profile/self", nil)
	if err != nil {
		return "", fmt.Errorf("resolving publication_id via /user/profile/self: %w", err)
	}
	id, err := publicationIDFromProfile(raw, subdomain)
	if err != nil {
		return "", err
	}
	return id, nil
}

func publicationIDFromProfile(raw []byte, subdomain string) (string, error) {
	var profile map[string]any
	if err := json.Unmarshal(raw, &profile); err != nil {
		return "", fmt.Errorf("parsing /user/profile/self response: %w", err)
	}
	wantSubdomain := strings.TrimSpace(strings.ToLower(subdomain))
	if pub, ok := profile["primaryPublication"].(map[string]any); ok {
		id := jsonNumberString(pub["id"])
		if id != "" && publicationMatchesSubdomain(pub, wantSubdomain) {
			return id, nil
		}
	}
	for _, key := range []string{"publications", "publicationUsers"} {
		items, ok := profile[key].([]any)
		if !ok {
			continue
		}
		for _, item := range items {
			pub, ok := item.(map[string]any)
			if !ok {
				continue
			}
			id := jsonNumberString(pub["publication_id"])
			if id == "" {
				id = jsonNumberString(pub["id"])
			}
			if id != "" && publicationMatchesSubdomain(pub, wantSubdomain) {
				return id, nil
			}
		}
	}
	if wantSubdomain == "" {
		return "", fmt.Errorf("could not infer publication_id from /user/profile/self; pass --publication-id or set SUBSTACK_PUBLICATION_ID")
	}
	return "", fmt.Errorf("could not infer publication_id for subdomain %q from /user/profile/self; pass --publication-id or set SUBSTACK_PUBLICATION_ID", subdomain)
}

func publicationMatchesSubdomain(pub map[string]any, wantSubdomain string) bool {
	if wantSubdomain == "" {
		return true
	}
	for _, key := range []string{"subdomain", "custom_domain"} {
		got := strings.TrimSpace(strings.ToLower(jsonNumberString(pub[key])))
		if got == "" {
			continue
		}
		if got == wantSubdomain || strings.TrimSuffix(got, ".substack.com") == wantSubdomain {
			return true
		}
	}
	return false
}

func jsonNumberString(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case float64:
		if x == float64(int64(x)) {
			return fmt.Sprintf("%d", int64(x))
		}
	}
	return ""
}
