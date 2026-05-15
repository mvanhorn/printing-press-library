package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/uspto-tsdr/internal/store"
)

// tmWatchEntry is a single mark's status in the watch report.
type tmWatchEntry struct {
	SerialNumber   string `json:"serialNumber"`
	MarkText       string `json:"markText,omitempty"`
	CurrentStatus  string `json:"currentStatus"`
	PreviousStatus string `json:"previousStatus,omitempty"`
	Changed        bool   `json:"changed"`
	LastChecked    string `json:"lastChecked,omitempty"`
}

func newTrademarkWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch <serial1> [serial2] ...",
		Short: "Monitor multiple trademarks for status changes",
		Long: `Checks the current status of one or more trademarks and compares
against the last cached status. Flags any marks whose status has
changed since the last check.

Statuses are cached locally in SQLite so that subsequent runs can
detect changes. First run always reports all marks as new.`,
		Example: strings.Trim(`
  uspto-tsdr-pp-cli trademark watch 97123456 97654321
  uspto-tsdr-pp-cli trademark watch 97123456 97654321 --json
  uspto-tsdr-pp-cli trademark watch 97123456 --json --select serialNumber,currentStatus,changed`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Open store for reading previous statuses
			dbPath := defaultDBPath("uspto-tsdr-pp-cli")
			db, err := store.OpenWithContext(context.Background(), dbPath)
			if err != nil {
				// Continue without cache — all entries will show as new
				db = nil
			}
			if db != nil {
				defer db.Close()
			}

			var entries []tmWatchEntry
			now := time.Now().UTC().Format(time.RFC3339)

			for _, serial := range args {
				entry := tmWatchEntry{
					SerialNumber: serial,
					LastChecked:  now,
				}

				caseID := normalizeCaseID(serial)
				// PATCH: use GetJSON (plain HTTP) — surf overrides Accept header.
				path := replacePathParam("/casestatus/{caseid}/info", "caseid", caseID)
				data, fetchErr := c.GetJSON(path, nil)
				if fetchErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not fetch %s: %v\n", serial, fetchErr)
					entry.CurrentStatus = "error"
					entries = append(entries, entry)
					continue
				}

				snap := parseTrademarkStatus(data, serial)
				entry.CurrentStatus = snap.CurrentStatus()
				entry.MarkText = snap.MarkText

				// Check previous status from local cache
				if db != nil {
					prevData, _ := db.Get("watch", serial)
					if prevData != nil {
						var prev tmWatchEntry
						if json.Unmarshal(prevData, &prev) == nil {
							entry.PreviousStatus = prev.CurrentStatus
							entry.Changed = prev.CurrentStatus != entry.CurrentStatus
						}
					}
				}

				// Save current status to cache
				if db != nil {
					entryJSON, _ := json.Marshal(entry)
					if _, _, cacheErr := db.UpsertBatch("watch", []json.RawMessage{entryJSON}); cacheErr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to cache status for %s: %v\n", serial, cacheErr)
					}
				}

				entries = append(entries, entry)
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), entries, flags)
			}

			// Human-readable output
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Trademark Watch Report (%s)\n\n", now[:10])

			changed := 0
			for _, e := range entries {
				indicator := green("  ")
				if e.Changed {
					indicator = red("▶ ")
					changed++
				}
				label := e.SerialNumber
				if e.MarkText != "" {
					label += " (" + truncate(e.MarkText, 30) + ")"
				}
				fmt.Fprintf(w, "%s%-45s  %s", indicator, label, e.CurrentStatus)
				if e.Changed && e.PreviousStatus != "" {
					fmt.Fprintf(w, "  (was: %s)", e.PreviousStatus)
				}
				fmt.Fprintln(w)
			}
			fmt.Fprintf(w, "\n%d/%d marks checked, %d changed\n", len(entries), len(args), changed)
			return nil
		},
	}
	return cmd
}

// CurrentStatus returns a display-friendly status string from the snapshot.
func (s *trademarkSnapshot) CurrentStatus() string {
	if s.Status != "" {
		return s.Status
	}
	return "unknown"
}
