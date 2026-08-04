package client

import (
	"bytes"
	"fmt"
	"net/http"
)

// ListSites returns sites visible to the signed-in user.
// Server administrators see all sites; site users typically see only their current site.
// If Query Sites is forbidden, falls back to the current signed-in site.
func (c *Client) ListSites() ([]Site, error) {
	if err := c.EnsureSignedIn(); err != nil {
		return nil, err
	}
	sites, err := getAllPages(func(page int) ([]Site, int, error) {
		u := withPageQuery(c.apiURL("sites"), page)
		status, data, err := c.doAuth(http.MethodGet, u, nil, "")
		if err != nil {
			return nil, 0, fmt.Errorf("list sites: %w", err)
		}
		// 403 is common for non-server-admins — caller handles via empty + fallback.
		if status == http.StatusForbidden || status == http.StatusUnauthorized {
			return nil, 0, errForbidden
		}
		if status != http.StatusOK {
			if apiErr := ParseErrorResponse(bytes.NewReader(data)); apiErr != nil {
				return nil, 0, fmt.Errorf("list sites (HTTP %d): %w", status, apiErr)
			}
			return nil, 0, fmt.Errorf("list sites failed (HTTP %d): %s", status, truncate(string(data), 300))
		}
		list, pag, err := ParseSitesResponse(bytes.NewReader(data))
		if err != nil {
			return nil, 0, err
		}
		return list, paginationTotal(pag), nil
	})
	if err == errForbidden {
		// Fall back to current session site.
		return []Site{{
			ID:         c.cred.SiteID,
			ContentURL: c.cred.ContentURL,
			Name:       c.cred.ContentURL,
		}}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(sites) == 0 && c.cred != nil {
		return []Site{{
			ID:         c.cred.SiteID,
			ContentURL: c.cred.ContentURL,
			Name:       c.cred.ContentURL,
		}}, nil
	}
	return sites, nil
}

// errForbidden is a sentinel used internally when Query Sites is not permitted.
var errForbidden = fmt.Errorf("forbidden")
