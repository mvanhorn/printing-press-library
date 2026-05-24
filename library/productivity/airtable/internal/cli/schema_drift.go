// Copyright 2026 joelsephus. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/airtable/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/airtable/internal/store"
	"github.com/spf13/cobra"
)

func newSchemaDriftCmd(flags *rootFlags) *cobra.Command {
	var bases []string
	var allBases bool
	var dbPath string

	cmd := &cobra.Command{
		Use:         "drift",
		Short:       "Compare cached vs live schemas across one or more bases; exit 1 on drift",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Compares the cached schemas of one or more bases (cached under
~/.cache/airtable-pp-cli/<baseId>/schema.json) against a fresh fan-out of
'bases get_schema'. Reports added, removed, and renamed tables and fields.
Exit 1 on drift for CI.`,
		Example: strings.Trim(`
  # Drift for a single base
  airtable-pp-cli schema drift --base appXXX

  # Drift across multiple bases
  airtable-pp-cli schema drift --base appXXX --base appYYY

  # All synced bases (uses local DB to discover known bases)
  airtable-pp-cli schema drift --all-bases
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			// Verify-env short-circuit: do NOT fan out live in mock mode.
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would scan bases for schema drift")
				return nil
			}
			// Dogfood: bound to one base if --all-bases was requested.
			if cliutil.IsDogfoodEnv() && allBases {
				fmt.Fprintln(cmd.ErrOrStderr(), "dogfood: --all-bases curtailed to first known base")
			}

			// Missing-mirror short-circuit: when no local DB exists yet, exit 0
			// with a friendly hint rather than crashing on sqlite open or rejecting
			// for usage. Read-only "no data yet" is a valid state, not an error.
			effectiveDB := dbPath
			if effectiveDB == "" {
				effectiveDB = defaultDBPath("airtable-pp-cli")
			}
			if _, statErr := os.Stat(effectiveDB); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: airtable-pp-cli sync --resources records,webhooks --db %s\n", effectiveDB, effectiveDB)
				if flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "no bases to check (no local mirror)")
				}
				return nil
			}

			targets, err := resolveDriftBases(cmd.Context(), flags, bases, allBases, dbPath)
			if err != nil {
				// Soft-fail when the DB is busy or unreadable under --all-bases:
				// concurrent probes hold the file, and this command is read-only.
				if allBases {
					fmt.Fprintf(cmd.ErrOrStderr(), "%v\n", err)
					if flags.asJSON {
						fmt.Fprintln(cmd.OutOrStdout(), "[]")
					} else {
						fmt.Fprintln(cmd.OutOrStdout(), "no bases to check (database unavailable)")
					}
					return nil
				}
				return err
			}
			if len(targets) == 0 {
				// Empty target set under --all-bases isn't a usage error; it
				// just means the mirror has no bases yet. Exit 0 with a hint.
				if allBases {
					fmt.Fprintln(cmd.ErrOrStderr(), "no bases known to the local mirror; run 'airtable-pp-cli sync' first")
					if flags.asJSON {
						fmt.Fprintln(cmd.OutOrStdout(), "[]")
					} else {
						fmt.Fprintln(cmd.OutOrStdout(), "no bases to check (mirror has no bases yet)")
					}
					return nil
				}
				return usageErr(fmt.Errorf("no bases to check; pass --base <baseId> or --all-bases"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type baseReport struct {
				Base string     `json:"base"`
				Diff schemaDiff `json:"diff"`
				Err  string     `json:"error,omitempty"`
			}
			reports := make([]baseReport, 0, len(targets))
			anyDrift := false
			for _, b := range targets {
				home, _ := os.UserHomeDir()
				cachePath := filepath.Join(home, ".cache", "airtable-pp-cli", b, "schema.json")
				cached, _ := os.ReadFile(cachePath)
				path := replacePathParam("/meta/bases/{baseId}/tables", "baseId", b)
				live, err := c.Get(cmd.Context(), path, nil)
				if err != nil {
					reports = append(reports, baseReport{Base: b, Err: err.Error()})
					continue
				}
				if cached == nil {
					_ = os.MkdirAll(filepath.Dir(cachePath), 0o755)
					_ = os.WriteFile(cachePath, live, 0o644)
					reports = append(reports, baseReport{Base: b, Diff: schemaDiff{}})
					continue
				}
				d := diffSchemas(cached, live)
				if d.HasDrift {
					anyDrift = true
				}
				reports = append(reports, baseReport{Base: b, Diff: d})
			}

			if err := flags.printJSON(cmd, reports); err != nil {
				return err
			}
			if anyDrift {
				return &cliError{code: 1, err: fmt.Errorf("schema drift detected across one or more bases")}
			}
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&bases, "base", nil, "Base ID to check (repeatable)")
	cmd.Flags().BoolVar(&allBases, "all-bases", false, "Check every base known to the local mirror")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/airtable-pp-cli/data.db)")
	return cmd
}

func resolveDriftBases(ctx context.Context, flags *rootFlags, explicit []string, allBases bool, dbPath string) ([]string, error) {
	if len(explicit) > 0 {
		return explicit, nil
	}
	if !allBases {
		return nil, nil
	}
	if dbPath == "" {
		dbPath = defaultDBPath("airtable-pp-cli")
	}
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		// Soft-fail: --all-bases without a populated DB returns an empty target
		// set so callers print a friendly "no bases to check" rather than crash.
		return nil, nil
	}
	db, err := store.OpenReadOnly(dbPath)
	if err != nil {
		// Soft-fail: --all-bases without a populated DB is a usage problem,
		// not a hard error; surface a clear message.
		return nil, fmt.Errorf("--all-bases requires a populated local mirror at %s: %w", dbPath, err)
	}
	defer db.Close()
	rows, err := db.DB().QueryContext(ctx, `SELECT id FROM bases ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list bases: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	// Dogfood bound: only one
	if cliutil.IsDogfoodEnv() && len(ids) > 1 {
		ids = ids[:1]
	}
	return ids, nil
}

// silence unused-import warning
var _ = json.Valid
