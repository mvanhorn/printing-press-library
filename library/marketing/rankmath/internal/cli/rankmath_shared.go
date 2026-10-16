// Copyright 2026 ElizabethcStone and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/rankmath/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/rankmath/internal/store"
	"github.com/spf13/cobra"
)

type publishedInventoryItem struct {
	URL    string
	PostID int
}

func siteListFromFlagOrEnv(flagVal string) []string {
	value := flagVal
	if strings.TrimSpace(value) == "" {
		value = os.Getenv("RANKMATH_SITES")
	}
	if strings.TrimSpace(value) == "" {
		value = os.Getenv("RANKMATH_SITE")
	}
	sites := cliutil.SplitCSV(value)
	return sites
}

func normalizeSiteURL(site string) (*url.URL, error) {
	site = strings.TrimSpace(site)
	if site == "" {
		return nil, fmt.Errorf("invalid site host")
	}
	if !strings.Contains(site, "://") {
		site = "https://" + site
	}
	u, err := url.Parse(site)
	if err != nil || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(site, "#") || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("invalid site URL")
	}
	u.Host = strings.ToLower(u.Host)
	return u, nil
}

func normalizePagePath(raw string) (string, string, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", "", false
	}
	path := strings.TrimRight(u.Path, "/")
	if u.Host == "" {
		return "", path, false
	}
	host := strings.ToLower(u.Host)
	host = strings.TrimPrefix(host, "www.")
	return host, path, true
}

func isNoDataResponse(raw json.RawMessage) bool {
	if len(raw) == 0 || string(raw) == "null" {
		return true
	}
	var response struct {
		Response string `json:"response"`
	}
	return json.Unmarshal(raw, &response) == nil && response.Response == "No Data"
}

func samePagePath(a, b string) bool {
	ha, pa, absA := normalizePagePath(a)
	hb, pb, absB := normalizePagePath(b)
	if absA && absB && ha != hb {
		return false
	}
	return pa == pb
}

func openMirrorOrHint(cmd *cobra.Command, flags *rootFlags, dbPath string) (*store.Store, bool, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("rankmath-pp-cli")
	}
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: rankmath-pp-cli sync --resources posts,pages\n", dbPath)
		return nil, false, nil
	} else if err != nil {
		return nil, false, err
	}
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, false, err
	}
	if !hintIfUnsynced(cmd, db, "posts") {
		hintIfStale(cmd, db, "posts", flags.maxAge)
	}
	return db, true, nil
}

func loadPublishedInventory(db *store.Store) ([]publishedInventoryItem, error) {
	items := make([]publishedInventoryItem, 0)
	siteHost := normalizeSiteHost(os.Getenv("RANKMATH_SITE"))
	for _, resource := range []string{"posts", "pages"} {
		var decodeErr error
		if err := db.ListScan(resource, func(id string, raw json.RawMessage) bool {
			var row struct {
				Link   string `json:"link"`
				Status string `json:"status"`
			}
			if decodeErr = json.Unmarshal(raw, &row); decodeErr != nil {
				return false
			}
			if row.Link != "" && (row.Status == "" || row.Status == "publish") {
				if siteHost != "" {
					if stripWWW(normalizeSiteHost(row.Link)) != stripWWW(siteHost) {
						return true
					}
				}
				postID, _ := strconv.Atoi(id)
				items = append(items, publishedInventoryItem{URL: row.Link, PostID: postID})
			}
			return true
		}); err != nil {
			return nil, err
		}
		if decodeErr != nil {
			return nil, decodeErr
		}
	}
	return items, nil
}

func scanCap(n, dogfoodMax int) int {
	if cliutil.IsDogfoodEnv() && n > dogfoodMax {
		return dogfoodMax
	}
	return n
}

// stripWWW treats www.example.com and example.com as the same site, matching
// WordPress sites whose canonical links differ from RANKMATH_SITE only by www.
func stripWWW(host string) string {
	return strings.TrimPrefix(host, "www.")
}
