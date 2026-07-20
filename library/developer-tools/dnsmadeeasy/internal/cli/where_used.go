// Copyright 2026 Derick Ng and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: cross-zone value provenance. Reads the local zone-record
// mirror populated by `sync-records`.
// pp:data-source local

package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/dnsmadeeasy/internal/store"

	"github.com/spf13/cobra"
)

type whereUsedMatch struct {
	Zone     string `json:"zone"`
	DomainID string `json:"domain_id"`
	RecordID string `json:"record_id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Value    string `json:"value"`
	TTL      int    `json:"ttl"`
}

type whereUsedView struct {
	Query        string           `json:"query"`
	Exact        bool             `json:"exact"`
	ScannedZones int              `json:"scanned_zones"`
	ScannedRecs  int              `json:"scanned_records"`
	TotalMatches int              `json:"total_matches"`
	Truncated    bool             `json:"truncated"`
	Matches      []whereUsedMatch `json:"matches"`
	Note         string           `json:"note,omitempty"`
}

func newNovelWhereUsedCmd(flags *rootFlags) *cobra.Command {
	var flagType string
	var flagExact bool
	var flagLimit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "where-used <ip-or-value>",
		Short: "Find every zone and record whose value/target matches an IP, hostname, or string",
		Long: strings.Trim(`
Search the local zone-record mirror for every record across ALL zones whose
value or CNAME/MX/ANAME target matches the given IP, hostname, or string.

Use this command to READ where a value appears across zones (audit, migration
planning). Do NOT use it to change records — use 'bulk-apply' instead.

Run 'dnsmadeeasy-pp-cli sync-records' first to populate the mirror.`, "\n"),
		Example:     "  dnsmadeeasy-pp-cli where-used 52.10.4.7 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search the local zone-record mirror")
				return nil
			}
			if len(args) == 0 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a value to search for is required, e.g. an IP or hostname"))
			}
			query := args[0]
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("dnsmadeeasy-pp-cli")
			}
			s, ok, err := openZoneMirror(ctx, cmd, dbPath)
			if err != nil {
				return err
			}
			if !ok {
				if flags.asJSON || flags.agent {
					return printJSONFiltered(cmd.OutOrStdout(), whereUsedView{
						Query: query, Exact: flagExact, Matches: []whereUsedMatch{},
						Note: "no local record mirror yet; run 'dnsmadeeasy-pp-cli sync-records' first",
					}, flags)
				}
				return nil
			}
			defer s.Close()

			recs, err := loadZoneRecords(ctx, s)
			if err != nil {
				return fmt.Errorf("reading zone mirror: %w", err)
			}
			scannedZones := map[string]struct{}{}
			scannedRecords := 0
			q := strings.ToLower(query)
			matches := make([]whereUsedMatch, 0)
			matchZones := map[string]struct{}{}
			totalMatches := 0
			for _, r := range recs {
				if flagType != "" && !strings.EqualFold(r.Type, flagType) {
					continue
				}
				scannedRecords++
				scannedZones[r.DomainID] = struct{}{}
				var hit bool
				if flagExact {
					hit = strings.EqualFold(r.Value, query) || strings.EqualFold(r.Name, query)
				} else {
					hit = strings.Contains(strings.ToLower(r.Value), q) || strings.Contains(strings.ToLower(r.Name), q)
				}
				if !hit {
					continue
				}
				totalMatches++
				matchZones[r.DomainID] = struct{}{}
				if flagLimit <= 0 || len(matches) < flagLimit {
					matches = append(matches, whereUsedMatch{
						Zone: r.DomainName, DomainID: r.DomainID, RecordID: r.ID.String(),
						Name: r.Name, Type: r.Type, Value: r.Value, TTL: r.TTL,
					})
				}
			}
			truncated := totalMatches > len(matches)
			view := whereUsedView{
				Query: query, Exact: flagExact,
				ScannedZones: len(scannedZones), ScannedRecs: scannedRecords,
				TotalMatches: totalMatches, Truncated: truncated, Matches: matches,
			}
			if truncated {
				view.Note = fmt.Sprintf("showing first %d of %d matching records; increase --limit to return more", len(matches), totalMatches)
			} else if len(matches) == 0 {
				view.Note = fmt.Sprintf("no records across %d zones matched %q; re-run 'dnsmadeeasy-pp-cli sync-records' if the mirror is stale", len(scannedZones), query)
			}

			if flags.asJSON || flags.agent || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(matches) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			rows := make([]map[string]any, 0, len(matches))
			for _, m := range matches {
				rows = append(rows, map[string]any{"zone": m.Zone, "name": m.Name, "type": m.Type, "value": m.Value, "ttl": m.TTL, "record_id": m.RecordID})
			}
			if err := printAutoTable(cmd.OutOrStdout(), rows); err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "\n%d record(s) across %d zone(s) use %q.\n", totalMatches, len(matchZones), query)
			if truncated {
				fmt.Fprintln(cmd.ErrOrStderr(), view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagType, "type", "", "Only match records of this type (A, CNAME, MX, TXT, ...)")
	cmd.Flags().BoolVar(&flagExact, "exact", false, "Require an exact value/name match instead of substring")
	cmd.Flags().IntVar(&flagLimit, "limit", 500, "Maximum matches to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local mirror database")
	return cmd
}

// openZoneMirror opens the local store, ensures the extension tables exist, and
// verifies the zone-record mirror is populated. When empty it prints a hint to
// stderr, emits an empty JSON result for machine consumers, and returns ok=false
// so the caller returns cleanly (a missing mirror is empty state, not an error).
func openZoneMirror(ctx context.Context, cmd *cobra.Command, dbPath string) (*store.Store, bool, error) {
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		emitMissingMirror(cmd, dbPath)
		return nil, false, nil
	}
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, false, fmt.Errorf("opening database: %w", err)
	}
	if err := s.EnsureDNSMEExtensions(ctx); err != nil {
		_ = s.Close()
		return nil, false, fmt.Errorf("preparing mirror tables: %w", err)
	}
	n, err := zoneRecordCount(ctx, s)
	if err != nil {
		_ = s.Close()
		return nil, false, fmt.Errorf("checking mirror: %w", err)
	}
	if n == 0 {
		_ = s.Close()
		emitMissingMirror(cmd, dbPath)
		return nil, false, nil
	}
	return s, true, nil
}

func emitMissingMirror(cmd *cobra.Command, dbPath string) {
	fmt.Fprintf(cmd.ErrOrStderr(), "no local record mirror yet\nrun: dnsmadeeasy-pp-cli sync-records --db %s\n", dbPath)
}
