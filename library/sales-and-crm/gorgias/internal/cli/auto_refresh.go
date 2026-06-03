// Auto-refresh hook: when a read command targets the local SQLite mirror
// and the mirror is stale (older than the configured TTL), trigger a
// background sync for the affected resource before serving the read.
//
// Wiring: root.go's PersistentPreRunE invokes `autoRefreshIfStale` so every
// read command gets the check transparently. Opt-in: set
// `GORGIAS_AUTO_REFRESH_TTL` (e.g., `1h`, `15m`) to enable; default is
// disabled to keep behavior predictable for agents that prefer explicit syncs.

package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gorgias/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gorgias/internal/store"
	"github.com/spf13/cobra"
)

// autoRefreshIfStale checks the local mirror's freshness for the resource
// the current command targets and triggers an opportunistic sync when
// stale. Always returns nil — a refresh failure is a warning, not a
// command-killing error.
//
// Trigger conditions (ALL must be true):
//  1. env GORGIAS_AUTO_REFRESH_TTL is set to a parseable duration
//  2. command has pp:endpoint annotation (read-shaped commands only)
//  3. command's resource has a typed sync handler in the generated CLI
//  4. last sync is older than the TTL
func autoRefreshIfStale(cmd *cobra.Command, flags *rootFlags) {
	ttlSpec := strings.TrimSpace(os.Getenv("GORGIAS_AUTO_REFRESH_TTL"))
	if ttlSpec == "" {
		return
	}
	ttl, err := time.ParseDuration(ttlSpec)
	if err != nil || ttl <= 0 {
		return
	}
	resource := autoRefreshResourceFor(cmd)
	if resource == "" {
		return
	}
	dbPath := defaultDBPath("gorgias-pp-cli")
	s, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return
	}
	defer s.Close()
	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()
	res, err := cliutil.EnsureFresh(ctx, freshnessAdapter{s: s}, resource, ttl)
	if err != nil {
		return
	}
	if !res.Stale {
		return
	}
	c, err := flags.newClient()
	if err != nil {
		return
	}
	c.NoCache = true
	r := syncResource(c, s, resource, "", false, 1, false, nil)
	if r.Err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "auto-refresh: %s sync failed (%v)\n", resource, r.Err)
		return
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "auto-refresh: %s mirror was %s stale; refreshed %d items\n",
		resource, cliutil.HumanAge(res.Age), r.Count)
}

func autoRefreshResourceFor(cmd *cobra.Command) string {
	annot := cmd.Annotations
	if annot == nil {
		return ""
	}
	if annot["mcp:read-only"] != "true" {
		return ""
	}
	endpoint := annot["pp:endpoint"]
	if endpoint == "" {
		return ""
	}
	if idx := strings.Index(endpoint, "."); idx > 0 {
		return endpoint[:idx]
	}
	return ""
}

type freshnessAdapter struct{ s *store.Store }

func (f freshnessAdapter) LastSyncedAt(ctx context.Context, resource string) (time.Time, error) {
	_, ts, _, err := f.s.GetSyncState(resource)
	if err != nil {
		return time.Time{}, err
	}
	if ts.IsZero() {
		return time.Time{}, cliutil.ErrNoSyncHistory
	}
	return ts, nil
}
