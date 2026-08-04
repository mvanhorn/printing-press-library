package client

import (
	"bytes"
	"fmt"
	"net/http"
)

// ListProjects returns all projects on the signed-in site.
func (c *Client) ListProjects() ([]Project, error) {
	if err := c.EnsureSignedIn(); err != nil {
		return nil, err
	}
	return getAllPages(func(page int) ([]Project, int, error) {
		u := withPageQuery(c.siteURL("projects"), page)
		status, data, err := c.doAuth(http.MethodGet, u, nil, "")
		if err != nil {
			return nil, 0, fmt.Errorf("list projects: %w", err)
		}
		if status != http.StatusOK {
			if apiErr := ParseErrorResponse(bytes.NewReader(data)); apiErr != nil {
				return nil, 0, fmt.Errorf("list projects (HTTP %d): %w", status, apiErr)
			}
			return nil, 0, fmt.Errorf("list projects failed (HTTP %d): %s", status, truncate(string(data), 300))
		}
		projects, pag, err := ParseProjectsResponse(bytes.NewReader(data))
		if err != nil {
			return nil, 0, err
		}
		return projects, paginationTotal(pag), nil
	})
}
