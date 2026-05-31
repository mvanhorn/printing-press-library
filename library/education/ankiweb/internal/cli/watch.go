// Copyright 2026 paul-bockewitz. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/education/ankiweb/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/education/ankiweb/internal/store"
	"github.com/mvanhorn/printing-press-library/library/education/ankiweb/internal/svc"
	"github.com/spf13/cobra"
)

// watchChange describes one deck that is new or changed since the last
// snapshot of a search term.
type watchChange struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Status    string   `json:"status"` // "new", "new (no prior snapshot)", "changed", or "current"
	Changes   []string `json:"changes,omitempty"`
	Upvotes   int      `json:"upvotes"`
	Downvotes int      `json:"downvotes"`
	Notes     int      `json:"notes"`
	Modified  int      `json:"modified"`
}

// watchResourceType namespaces a term's snapshot in the generic resources
// table so multiple watched terms don't collide.
func watchResourceType(term string) string { return "watch:" + term }

func newWatchCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var sinceLastSync bool

	cmd := &cobra.Command{
		Use:         "watch <term>",
		Short:       "Show shared decks that are new or changed for a term since your last watch snapshot",
		Long:        "Compares the current shared-deck search results for a term against the snapshot stored by the previous 'watch' run, then updates the snapshot.",
		Example:     "  ankiweb-pp-cli watch spanish --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			term := args[0]
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), []watchChange{}, flags)
			}

			c, _, err := flags.newSvcClient()
			if err != nil {
				return err
			}
			decks, err := listDecks(cmd.Context(), c, term)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			if dbPath == "" {
				dbPath = defaultDBPath("ankiweb-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			rt := watchResourceType(term)
			prior := map[string]svc.SharedDeck{}
			rows, err := db.List(rt, 100000)
			if err != nil {
				return fmt.Errorf("reading watch snapshot for %q: %w", term, err)
			}
			for _, raw := range rows {
				var d svc.SharedDeck
				if json.Unmarshal(raw, &d) == nil {
					prior[d.ID] = d
				}
			}
			hadSnapshot := len(prior) > 0

			changes := make([]watchChange, 0)
			for _, d := range decks {
				if !sinceLastSync {
					// --since-last-sync=false: dump the full current catalog
					// instead of only decks new or changed since the snapshot.
					changes = append(changes, mkChange(d, "current", nil))
					continue
				}
				old, seen := prior[d.ID]
				switch {
				case !seen && !hadSnapshot:
					changes = append(changes, mkChange(d, "new (no prior snapshot)", nil))
				case !seen:
					changes = append(changes, mkChange(d, "new", nil))
				default:
					if diffs := deckDiffs(old, d); len(diffs) > 0 {
						changes = append(changes, mkChange(d, "changed", diffs))
					}
				}
			}

			// Update the snapshot to the current set — but only in change-tracking
			// mode. A --since-last-sync=false full-catalog dump is a read-only
			// export; advancing the baseline here would silently reset the user's
			// change-tracking history so the next default run misses prior "new" decks.
			if sinceLastSync {
				for _, d := range decks {
					raw, _ := json.Marshal(d)
					if err := db.Upsert(rt, d.ID, raw); err != nil {
						return fmt.Errorf("updating watch snapshot for %q: %w", term, err)
					}
				}
			}

			return printJSONFiltered(cmd.OutOrStdout(), changes, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/ankiweb-pp-cli/data.db)")
	cmd.Flags().BoolVar(&sinceLastSync, "since-last-sync", true, "Report only decks new or changed since the previous watch snapshot; --since-last-sync=false dumps the full current catalog")
	return cmd
}

func mkChange(d svc.SharedDeck, status string, diffs []string) watchChange {
	return watchChange{
		ID: d.ID, Title: d.Title, Status: status, Changes: diffs,
		Upvotes: d.Upvotes, Downvotes: d.Downvotes, Notes: d.Notes, Modified: d.Modified,
	}
}

// deckDiffs lists the fields that changed between two snapshots of the same deck.
func deckDiffs(a, b svc.SharedDeck) []string {
	var d []string
	if a.Upvotes != b.Upvotes {
		d = append(d, fmt.Sprintf("upvotes %d->%d", a.Upvotes, b.Upvotes))
	}
	if a.Downvotes != b.Downvotes {
		d = append(d, fmt.Sprintf("downvotes %d->%d", a.Downvotes, b.Downvotes))
	}
	if a.Notes != b.Notes {
		d = append(d, fmt.Sprintf("notes %d->%d", a.Notes, b.Notes))
	}
	if a.Modified != b.Modified {
		d = append(d, fmt.Sprintf("modified %d->%d", a.Modified, b.Modified))
	}
	if a.Title != b.Title {
		d = append(d, "title changed")
	}
	return d
}
