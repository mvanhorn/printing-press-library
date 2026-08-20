// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mvanhorn/printing-press-library/library/travel/faa-registry/internal/faaparse"
	"github.com/mvanhorn/printing-press-library/library/travel/faa-registry/internal/registrydb"
	"github.com/spf13/cobra"
	"time"
)

// parseFAAHTMLResponse converts a raw aircraftinquiry HTML page into typed
// JSON: a registration detail object for N-number results, or a
// rows/pagination object for search-result lists. Registry-reported error
// banners become errors.
func parseFAAHTMLResponse(raw json.RawMessage) (json.RawMessage, error) {
	res, err := faaparse.ParseAuto(raw)
	if err != nil {
		return nil, fmt.Errorf("parsing registry page: %w", err)
	}
	if res.Error != "" {
		return nil, fmt.Errorf("registry: %s", res.Error)
	}
	if res.Kind == "detail" {
		return json.Marshal(res.Detail)
	}
	return json.Marshal(res.List)
}

// fetchAllFAAPages merges the remaining pages of a paginated aircraftinquiry
// result into the first page's parsed list. Page fetches go straight through
// the client with the same query params plus Page=N. maxPages caps the fan-out.
func fetchAllFAAPages(ctx context.Context, c faaPageGetter, path string, params map[string]string, firstPage json.RawMessage, maxPages int, progress func(string)) (json.RawMessage, error) {
	var list faaparse.List
	if err := json.Unmarshal(firstPage, &list); err != nil {
		return firstPage, nil // not a list payload; nothing to paginate
	}
	if list.Pages <= 1 {
		return firstPage, nil
	}
	pages := list.Pages
	truncated := false
	if maxPages > 0 && pages > maxPages {
		pages = maxPages
		truncated = true
	}
	for p := 2; p <= pages; p++ {
		pp := map[string]string{}
		for k, v := range params {
			pp[k] = v
		}
		pp["Page"] = fmt.Sprintf("%d", p)
		raw, err := c.Get(ctx, path, pp)
		if err != nil {
			return nil, fmt.Errorf("fetching page %d/%d: %w", p, list.Pages, err)
		}
		pageList, err := faaparse.ParseList(raw)
		if err != nil {
			return nil, fmt.Errorf("parsing page %d/%d: %w", p, list.Pages, err)
		}
		list.Rows = append(list.Rows, pageList.Rows...)
		if progress != nil {
			progress(fmt.Sprintf("fetched page %d/%d (%d rows)", p, list.Pages, len(list.Rows)))
		}
	}
	list.ShowingFrom = 1
	list.ShowingTo = len(list.Rows)
	list.Page = 1
	if truncated && progress != nil {
		progress(fmt.Sprintf("stopped at --max-pages %d of %d total pages; %d rows fetched", maxPages, list.Pages, len(list.Rows)))
	}
	return json.Marshal(&list)
}

// faaPageGetter is the client subset fetchAllFAAPages needs.
type faaPageGetter interface {
	Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error)
}

// registryDBPath is where the offline registry SQLite database lives. It is
// a separate file from the framework store so a large import never bloats
// the command cache database. FAA_REGISTRY_DB overrides the location (custom
// data dirs, CI, sharing one synced copy across sandboxes).
func registryDBPath() string {
	if p := os.Getenv("FAA_REGISTRY_DB"); p != "" {
		return p
	}
	base := defaultDBPath("faa-registry-pp-cli")
	return filepath.Join(filepath.Dir(base), "registry.db")
}

// registryZipPath is the cached copy of the FAA daily download.
func registryZipPath() string {
	base := defaultDBPath("faa-registry-pp-cli")
	return filepath.Join(filepath.Dir(base), "ReleasableAircraft.zip")
}

// openRegistryDB opens the offline registry database, creating the data
// directory when needed.
func openRegistryDB(ctx context.Context) (*registrydb.DB, error) {
	path := registryDBPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return registrydb.Open(ctx, path)
}

// registryStaleAfter is the age past which the local registry is worth
// refreshing. The FAA publishes a new database each federal working day, so a
// copy older than this is likely behind.
const registryStaleAfter = 7 * 24 * time.Hour

// emitRegistryStaleHint prints a one-line stderr hint (suppressed under
// --quiet/--agent) when the local registry hasn't been synced recently. It is
// advisory only and never fails the command.
func emitRegistryStaleHint(cmd *cobra.Command, db *registrydb.DB, flags *rootFlags) {
	if cmd == nil || db == nil || flags == nil || flags.quiet {
		return
	}
	synced, err := db.Meta(cmd.Context(), "synced_at")
	if err != nil || synced == "" {
		return
	}
	t, err := time.Parse(time.RFC3339, synced)
	if err != nil {
		return
	}
	age := time.Since(t)
	if age <= registryStaleAfter {
		return
	}
	days := int(age.Hours() / 24)
	fmt.Fprintf(cmd.ErrOrStderr(), "hint: local registry was last synced %dd ago (%s); the FAA updates daily — run 'faa-registry-pp-cli sync' to refresh.\n", days, t.Format("2006-01-02"))
}
