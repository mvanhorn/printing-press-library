package gplay

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Developer returns the apps published by a developer. devID may be a numeric
// id (uses /store/apps/dev) or a display name (uses /store/apps/developer).
func (c *Client) Developer(ctx context.Context, devID string, num int) ([]LiteApp, error) {
	if devID == "" {
		return nil, fmt.Errorf("developer id or name is required")
	}
	if num <= 0 {
		num = 60
	}
	numeric := isNumeric(devID)
	path := "/store/apps/developer"
	if numeric {
		path = "/store/apps/dev"
	}
	q := url.Values{}
	q.Set("id", devID)
	html, err := c.getHTML(ctx, path, q)
	if err != nil {
		return nil, err
	}
	ds, err := extractAFData(html)
	if err != nil {
		return nil, fmt.Errorf("parsing developer page for %s: %w", devID, err)
	}
	raw, ok := ds["ds:3"]
	if !ok {
		return nil, fmt.Errorf("developer block (ds:3) not found for %s", devID)
	}
	root := decode(raw)
	// /dev (numeric) apps live at [0,1,0,21,0]; /developer (name) at [0,1,0,22,0].
	var apps node
	if numeric {
		apps = root.path(0, 1, 0, 21, 0)
		if !apps.isArray() {
			apps = root.path(0, 1, 0, 22, 0)
		}
	} else {
		apps = root.path(0, 1, 0, 22, 0)
		if !apps.isArray() {
			apps = root.path(0, 1, 0, 21, 0)
		}
	}
	var out []LiteApp
	for _, e := range apps.arr() {
		la := parseChartApp(e)
		if la.AppID == "" {
			la = parseDevApp(e)
		}
		if la.AppID != "" {
			out = append(out, la)
		}
		if len(out) >= num {
			break
		}
	}
	return out, nil
}

// parseDevApp tries the unwrapped developer item shape as a fallback.
func parseDevApp(e node) LiteApp {
	la := LiteApp{
		AppID:     e.path(0, 0).str(),
		Title:     e.path(3).cleanStr(),
		Developer: e.path(14).cleanStr(),
		ScoreText: e.path(4, 0).str(),
		Score:     e.path(4, 1).float(),
		Icon:      e.path(1, 3, 2).str(),
		Summary:   e.path(13, 1).cleanStr(),
	}
	if u := e.path(10, 4, 2).str(); u != "" {
		if !strings.HasPrefix(u, "http") {
			u = baseURL + u
		}
		la.URL = u
	}
	la.Free = true
	return la
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	_, err := strconv.ParseInt(s, 10, 64)
	return err == nil
}
