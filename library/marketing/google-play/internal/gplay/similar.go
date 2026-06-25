package gplay

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// clusterHrefRe captures a full similar-apps cluster href including its gsr
// token. Run it on HTML with =/& already unescaped to = and &.
var clusterHrefRe = regexp.MustCompile(`/store/apps/collection/cluster\?[A-Za-z0-9=&_:%.\-]+`)

// Similar returns apps similar to appID. It loads the details page, locates the
// "similar apps" cluster URL (service id ag2B9c), then fetches that cluster.
func (c *Client) Similar(ctx context.Context, appID string, num int) ([]LiteApp, error) {
	if appID == "" {
		return nil, fmt.Errorf("appId is required")
	}
	if num <= 0 {
		num = 30
	}
	q := url.Values{}
	q.Set("id", appID)
	html, err := c.getHTML(ctx, "/store/apps/details", q)
	if err != nil {
		return nil, err
	}
	clusterURL := findSimilarCluster(html)
	if clusterURL == "" {
		return nil, fmt.Errorf("no similar-apps cluster found for %s", appID)
	}
	if strings.HasPrefix(clusterURL, "http") {
		if u, perr := url.Parse(clusterURL); perr == nil {
			clusterURL = u.Path + "?" + u.RawQuery
		}
	}
	// Split path and query for getHTML.
	cpath := clusterURL
	var cq url.Values
	if i := strings.Index(clusterURL, "?"); i >= 0 {
		cpath = clusterURL[:i]
		cq, _ = url.ParseQuery(clusterURL[i+1:])
	}
	chtml, err := c.getHTML(ctx, cpath, cq)
	if err != nil {
		return nil, err
	}
	ds, err := extractAFData(chtml)
	if err != nil {
		return nil, fmt.Errorf("parsing similar cluster for %s: %w", appID, err)
	}
	raw, ok := ds["ds:3"]
	if !ok {
		return nil, fmt.Errorf("similar cluster block (ds:3) not found for %s", appID)
	}
	ds3 := decode(raw)
	// The cluster grid lives at [0][1][0][N][0]; N is 21 or 22 depending on the
	// layout. Grid entries carry appId at [0][0] and title at [3].
	var apps []node
	for _, n := range [][]int{{0, 1, 0, 21, 0}, {0, 1, 0, 22, 0}, {0, 1, 0, 23, 0}} {
		cand := ds3.path(n...)
		if !cand.isArray() {
			continue
		}
		parsed := 0
		for _, e := range cand.arr() {
			if e.path(0, 0).str() != "" && e.path(3).str() != "" {
				parsed++
			}
		}
		if parsed > len(apps) {
			apps = cand.arr()
		}
		if parsed >= 3 {
			break
		}
	}
	var out []LiteApp
	for _, e := range apps {
		la := parseClusterGridApp(e)
		if la.AppID != "" && la.Title != "" && la.AppID != appID {
			out = append(out, la)
		}
		if len(out) >= num {
			break
		}
	}
	return out, nil
}

// parseClusterGridApp maps a similar-cluster grid entry (appId at [0,0],
// title at [3]).
func parseClusterGridApp(e node) LiteApp {
	la := LiteApp{
		AppID:     e.path(0, 0).str(),
		Title:     e.path(3).cleanStr(),
		ScoreText: e.path(4, 0).str(),
	}
	if s := e.path(4, 1).float(); s > 0 && s <= 5 {
		la.Score = s
	}
	if la.AppID != "" {
		la.URL = baseURL + "/store/apps/details?id=" + la.AppID
	}
	la.Free = true
	return la
}

// findSimilarCluster pulls the longest cluster href (the one carrying a full
// gsr token) out of the details HTML, after unescaping = and &.
func findSimilarCluster(html string) string {
	unescaped := strings.NewReplacer("\\u003d", "=", "\\u0026", "&").Replace(html)
	best := ""
	for _, m := range clusterHrefRe.FindAllString(unescaped, -1) {
		// Require a gsr token with real content; the longest match has the
		// complete token.
		if strings.Contains(m, "gsr=") && len(m) > len(best) {
			best = m
		}
	}
	return best
}
