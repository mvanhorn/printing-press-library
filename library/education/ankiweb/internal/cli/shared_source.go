// Copyright 2026 paul-bockewitz. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"

	"github.com/mvanhorn/printing-press-library/library/education/ankiweb/internal/store"
	"github.com/mvanhorn/printing-press-library/library/education/ankiweb/internal/svc"
	"github.com/spf13/cobra"
)

// loadSharedDecks fetches the shared-deck catalog for term according to the
// --data-source policy:
//
//   - local: read only the synced `shared` table (offline).
//   - live:  fetch from the API only.
//   - auto:  fetch from the API; on a transport failure (the host is
//     unreachable) fall back to the synced `shared` table when it has rows.
//
// It returns unfiltered decks; callers apply their own filters and sort. API
// errors (HTTP 4xx/5xx, rate limits) are not transport failures and never
// trigger fallback — they surface via classifyAPIError so the user sees the
// real problem instead of silently-stale data.
func (f *rootFlags) loadSharedDecks(cmd *cobra.Command, term string) ([]svc.SharedDeck, error) {
	if f.dataSource == "local" {
		return loadSharedLocal(cmd, term)
	}

	c, _, err := f.newSvcClient()
	if err != nil {
		return nil, err
	}
	decks, err := listDecks(cmd.Context(), c, term)
	if err == nil {
		return decks, nil
	}

	if f.dataSource == "auto" && isOfflineErr(err) {
		if local, lerr := loadSharedLocal(cmd, term); lerr == nil && len(local) > 0 {
			fmt.Fprintf(cmd.ErrOrStderr(),
				"warning: live fetch failed (%v); using locally synced data (--data-source live to disable fallback)\n", err)
			return local, nil
		}
	}
	return nil, classifyAPIError(err, f)
}

// loadSharedLocal returns every synced `shared` row matching term, unfiltered.
func loadSharedLocal(cmd *cobra.Command, term string) ([]svc.SharedDeck, error) {
	dbPath := defaultDBPath("ankiweb-pp-cli")
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local store: %w", err)
	}
	defer db.Close()

	hintIfUnsynced(cmd, db, "shared")
	rows, err := db.SearchShared(term, 1000)
	if err != nil {
		return nil, fmt.Errorf("searching local store: %w", err)
	}
	out := make([]svc.SharedDeck, 0, len(rows))
	for _, raw := range rows {
		var d svc.SharedDeck
		if json.Unmarshal(raw, &d) == nil {
			out = append(out, d)
		}
	}
	return out, nil
}

// isOfflineErr reports whether err is a transport-level failure (no connection,
// DNS failure, timeout) — i.e. the API host is unreachable. The svc client
// surfaces such failures as *url.Error (which implements net.Error), while
// API-level responses (HTTP status errors, *cliutil.RateLimitError) do not — so
// only genuine offline conditions trigger local fallback.
func isOfflineErr(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr)
}
