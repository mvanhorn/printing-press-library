// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: see .printing-press-patches/ for context. Hand-authored, not
// generator output — regen-merge preserves this file.

// pp:data-source computed

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/cliutil"
	"github.com/spf13/cobra"
)

type firstSeenRecord struct {
	FirstSeenAt time.Time `json:"first_seen_at"`
	Name        string    `json:"name,omitempty"`
	MAC         string    `json:"mac,omitempty"`
}

type firstSeenFile struct {
	// Resource -> entity id -> record.
	Resources map[string]map[string]firstSeenRecord `json:"resources"`
}

func firstSeenPath(dir, siteID string) string {
	return filepath.Join(dir, fmt.Sprintf("newcomer-first-seen-%s.json", siteID))
}

func loadFirstSeen(path string) (*firstSeenFile, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path built from internal allowlist + synced site id
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var f firstSeenFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &f, nil
}

func saveFirstSeen(path string, f firstSeenFile) error {
	data, err := json.Marshal(f)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

type newcomerEntry struct {
	Resource    string    `json:"resource"`
	ID          string    `json:"id"`
	Name        string    `json:"name,omitempty"`
	MAC         string    `json:"mac,omitempty"`
	FirstSeenAt time.Time `json:"first_seen_at"`
}

func newNovelNewcomerCmd(flags *rootFlags) *cobra.Command {
	var flagSite string
	var flagSince string

	cmd := &cobra.Command{
		Use:   "newcomer",
		Short: "List devices and clients first seen since a given sync, for spotting new hardware joining the network.",
		Long: "Maintains a local 'first seen' record for every device and " +
			"client, updated each time this command runs against the synced " +
			"local mirror. On the very first run for a site, every currently " +
			"synced device/client becomes the baseline (not reported as new) so " +
			"the whole network isn't dumped as 'newcomers' on day one. --since " +
			"filters to entities first recorded within that window (default 24h). " +
			"Run 'unifi-pp-cli sync' before each 'newcomer' to pick up devices/" +
			"clients that joined since the last check.",
		Example:     "  unifi-pp-cli newcomer --since 7d --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "newcomer")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			since := 24 * time.Hour
			if flagSince != "" {
				d, err := cliutil.ParseDurationLoose(flagSince)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("invalid --since %q: %w", flagSince, err))
				}
				since = d
			}

			dbPath := defaultDBPath("unifi-pp-cli")
			db, err := openNovelStore(ctx, dbPath)
			if err != nil {
				return err
			}
			if db == nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: unifi-pp-cli sync\n", dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), []newcomerEntry{}, flags)
				}
				return nil
			}
			defer db.Close()

			siteID, _, err := resolveSiteIDLocal(ctx, db.DB(), flagSite)
			if err != nil {
				if isNoLocalDataYet(err) {
					fmt.Fprintf(cmd.ErrOrStderr(), "%s\nrun: unifi-pp-cli sync\n", err)
					if !wantsHumanTable(cmd.OutOrStdout(), flags) {
						return printJSONFiltered(cmd.OutOrStdout(), []newcomerEntry{}, flags)
					}
					return nil
				}
				return err
			}

			snapDir, err := novelSnapshotDir()
			if err != nil {
				return err
			}
			fsPath := firstSeenPath(snapDir, siteID)
			fs, err := loadFirstSeen(fsPath)
			if err != nil {
				return err
			}
			isBootstrap := fs == nil
			if fs == nil {
				fs = &firstSeenFile{Resources: map[string]map[string]firstSeenRecord{}}
			}
			if fs.Resources == nil {
				fs.Resources = map[string]map[string]firstSeenRecord{}
			}

			now := time.Now().UTC()
			cutoff := now.Add(-since)
			newcomers := make([]newcomerEntry, 0)

			for _, res := range []string{"devices", "clients"} {
				rows, err := resourceRows(ctx, db.DB(), res, siteID)
				if err != nil {
					return err
				}
				if fs.Resources[res] == nil {
					fs.Resources[res] = map[string]firstSeenRecord{}
				}
				for _, id := range sortedKeys(rows) {
					var meta struct {
						Name string `json:"name"`
						MAC  string `json:"macAddress"`
					}
					_ = json.Unmarshal(rows[id], &meta)

					rec, known := fs.Resources[res][id]
					if !known {
						rec = firstSeenRecord{FirstSeenAt: now, Name: meta.Name, MAC: meta.MAC}
						fs.Resources[res][id] = rec
						if !isBootstrap && rec.FirstSeenAt.After(cutoff) {
							newcomers = append(newcomers, newcomerEntry{
								Resource: res, ID: id, Name: meta.Name, MAC: meta.MAC, FirstSeenAt: rec.FirstSeenAt,
							})
						}
					} else if rec.FirstSeenAt.After(cutoff) {
						newcomers = append(newcomers, newcomerEntry{
							Resource: res, ID: id, Name: rec.Name, MAC: rec.MAC, FirstSeenAt: rec.FirstSeenAt,
						})
					}
				}
			}

			if err := saveFirstSeen(fsPath, *fs); err != nil {
				return fmt.Errorf("saving first-seen record: %w", err)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), newcomers, flags)
			}
			w := cmd.OutOrStdout()
			if len(newcomers) == 0 {
				if isBootstrap {
					fmt.Fprintln(w, "First run: baseline captured for every currently synced device/client. Re-run newcomer later to see new joins.")
				} else {
					fmt.Fprintf(w, "No devices or clients first seen within %s.\n", since)
				}
				return nil
			}
			for _, n := range newcomers {
				fmt.Fprintf(w, "%-8s %-24s %-18s %s\n", n.Resource, n.Name, n.MAC, n.FirstSeenAt.Format(time.RFC3339))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSite, "site", "", "Site id, internalReference, or name (default: the only synced site)")
	cmd.Flags().StringVar(&flagSince, "since", "24h", "Show entities first seen within this window (e.g. 24h, 7d)")
	return cmd
}
