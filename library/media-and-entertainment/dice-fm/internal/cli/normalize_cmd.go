// Copyright 2026 vinny-pasceri. Licensed under Apache-2.0. See LICENSE.
// Hand-authored `normalize` command: runs the entity normalization pipeline over
// the local store, writing canonical entity, crosswalk, and attribute rows.
// This file is NOT generated and survives `generate --force`.
package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/store"
	"github.com/spf13/cobra"
)

// normalizeOpts drives a runNormalize call. The cobra command translates flag
// values into this struct so the inner function is independently testable.
type normalizeOpts struct {
	// Tiers runs the tier-axis classification pipeline.
	Tiers bool
	// Venues runs the venue complex/room classification pipeline.
	Venues bool
	// Fuzzy enables a second-pass Jaro-Winkler clustering of near-duplicate
	// canonical names.
	Fuzzy bool
	// ClassifierVersion is stamped on every written row.
	ClassifierVersion int
	// ExportUnmatched, when non-empty, is the file path to write unmatched
	// source values for external classification.
	ExportUnmatched string
	// ImportData, when non-nil, is pre-loaded import bytes to feed to
	// importMapping before the classify pipeline runs.
	ImportData []byte
	// ImportFormat is "csv" or "json" for the ImportData bytes.
	ImportFormat string
}

// classifyResultSummary is the JSON-serializable summary for one classify axis.
type classifyResultSummary struct {
	CanonicalCount int `json:"canonical_count"`
	Matched        int `json:"matched"`
	Unmatched      int `json:"unmatched"`
}

// normalizeSummary is the top-level JSON output of a normalize run.
type normalizeSummary struct {
	Tiers  *classifyResultSummary `json:"tiers,omitempty"`
	Venues *classifyResultSummary `json:"venues,omitempty"`
}

// runNormalize executes the normalization pipeline over s, writing a JSON
// summary to w. It is separated from the cobra plumbing so tests can call it
// directly with a seeded store.
func runNormalize(ctx context.Context, s *store.Store, opts normalizeOpts, w io.Writer) error {
	// Pre-import step: feed caller-supplied mappings before classification so
	// the imported manual rows are in place before the pipeline skips them.
	if len(opts.ImportData) > 0 {
		n, err := importMapping(s, "dice", opts.ImportData, opts.ImportFormat)
		if err != nil {
			return fmt.Errorf("import: %w", err)
		}
		_ = n
	}

	var summary normalizeSummary
	classOpts := classifyOpts{
		ClassifierVersion: opts.ClassifierVersion,
		Fuzzy:             opts.Fuzzy,
	}

	if opts.Tiers {
		res, err := classifyTiers(ctx, s, classOpts)
		if err != nil {
			return fmt.Errorf("classify tiers: %w", err)
		}
		summary.Tiers = &classifyResultSummary{
			CanonicalCount: res.CanonicalCount,
			Matched:        res.Matched,
			Unmatched:      res.Unmatched,
		}
		// Export unmatched names when requested.
		if opts.ExportUnmatched != "" && res.Unmatched > 0 {
			if err := exportUnmatched(ctx, s, "ticket_type", opts.ExportUnmatched); err != nil {
				fmt.Fprintf(os.Stderr, "warning: export-unmatched: %v\n", err)
			}
		}
	}

	if opts.Venues {
		res, err := classifyVenues(ctx, s, classOpts)
		if err != nil {
			return fmt.Errorf("classify venues: %w", err)
		}
		summary.Venues = &classifyResultSummary{
			CanonicalCount: res.CanonicalCount,
			Matched:        res.Matched,
			Unmatched:      res.Unmatched,
		}
		if opts.ExportUnmatched != "" && res.Unmatched > 0 {
			if err := exportUnmatched(ctx, s, "venue", opts.ExportUnmatched); err != nil {
				fmt.Fprintf(os.Stderr, "warning: export-unmatched venues: %v\n", err)
			}
		}
	}

	return json.NewEncoder(w).Encode(summary)
}

// exportUnmatched writes source values with method="unmatched" for the given
// entity type to a CSV file at path. Each row is: entity_type,source_value.
// Values are written via encoding/csv so source values containing commas,
// quotes, or newlines are correctly escaped and can be round-tripped.
func exportUnmatched(ctx context.Context, s *store.Store, entityType, path string) error {
	rows, err := s.DB().QueryContext(ctx,
		`SELECT source_value FROM entity_crosswalk
		 WHERE entity_type = ? AND method = 'unmatched'
		 ORDER BY source_value`, entityType)
	if err != nil {
		return err
	}
	defer rows.Close()

	f, err := os.Create(filepath.Clean(path))
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write([]string{"entity_type", "source_value"}); err != nil {
		return err
	}
	for rows.Next() {
		var sv string
		if err := rows.Scan(&sv); err != nil {
			return err
		}
		if err := w.Write([]string{entityType, sv}); err != nil {
			return err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}
	return rows.Err()
}

// newNormalizeCmd returns the `normalize` cobra command and its subcommands.
// It writes to the local store (classification is a write path) and therefore
// must NOT be annotated mcp:read-only. The `normalize stats` subcommand is
// read-only and carries the annotation.
func newNormalizeCmd(flags *rootFlags) *cobra.Command {
	var (
		doTiers           bool
		doVenues          bool
		fuzzy             bool
		classifierVersion int
		exportUnmatched   string
		importFile        string
		tiersSet          bool
		venuesSet         bool
	)

	cmd := &cobra.Command{
		Use:   "normalize",
		Short: "Normalize raw ticket-type and venue names into canonical, structured form",
		Long: "Classify raw DICE ticket-type and venue names into canonical entities, " +
			"tier axes (access class, sales stage, entry window, group size, comp flag), " +
			"and venue parts (complex, room). Results are written to the local SQLite store " +
			"and survive re-classification; rows imported with --import are tagged method=manual " +
			"and are never overwritten by subsequent runs.\n\n" +
			"Workflow: sync → normalize [--fuzzy] → normalize --export-unmatched unmatched.csv " +
			"→ classify externally → normalize --import mapped.csv → analytics (future --by-axis).",
		Example: "  dice-fm-pp-cli normalize --tiers --fuzzy\n" +
			"  dice-fm-pp-cli normalize --tiers --export-unmatched unmatched.csv\n" +
			"  dice-fm-pp-cli normalize --import mapped.csv\n" +
			"  dice-fm-pp-cli normalize stats",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			// Default: run tiers when neither axis flag was set.
			if !tiersSet && !venuesSet {
				doTiers = true
			}

			// Load import data from file if --import was provided.
			var importData []byte
			var importFormat string
			if importFile != "" {
				b, err := os.ReadFile(importFile)
				if err != nil {
					return fmt.Errorf("reading import file: %w", err)
				}
				importData = b
				ext := strings.ToLower(filepath.Ext(importFile))
				switch ext {
				case ".csv":
					importFormat = "csv"
				case ".json":
					importFormat = "json"
				default:
					return fmt.Errorf("cannot detect import format from extension %q: rename to .csv or .json", ext)
				}
			}

			dbPath := defaultDBPath(diceCLIName)
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			opts := normalizeOpts{
				Tiers:             doTiers,
				Venues:            doVenues,
				Fuzzy:             fuzzy,
				ClassifierVersion: classifierVersion,
				ExportUnmatched:   exportUnmatched,
				ImportData:        importData,
				ImportFormat:      importFormat,
			}
			return runNormalize(cmd.Context(), s, opts, cmd.OutOrStdout())
		},
	}

	cmd.Flags().BoolVar(&doTiers, "tiers", false, "Classify ticket-type names into tier axes (default when neither --tiers nor --venues given)")
	cmd.Flags().BoolVar(&doVenues, "venues", false, "Classify venue names into complex/room parts")
	cmd.Flags().BoolVar(&fuzzy, "fuzzy", false, "Enable Jaro-Winkler clustering of near-duplicate canonical names (default false; deterministic without it)")
	cmd.Flags().IntVar(&classifierVersion, "classifier-version", 1, "Classifier version stamped on written rows (default 1)")
	cmd.Flags().StringVar(&exportUnmatched, "export-unmatched", "", "Write unmatched source values to this CSV file path for external classification")
	cmd.Flags().StringVar(&importFile, "import", "", "Import a caller-supplied CSV or JSON mapping file (method=manual rows survive re-classification)")

	// Track whether the caller explicitly set --tiers or --venues so the default
	// logic can fire when neither is given.
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		tiersSet = cmd.Flags().Changed("tiers")
		venuesSet = cmd.Flags().Changed("venues")
		return nil
	}

	cmd.AddCommand(newNormalizeStatsCmd(flags))
	return cmd
}

// normalizeStatsOutput is the JSON shape for `normalize stats`.
type normalizeStatsOutput struct {
	TierCanonicals  int            `json:"tier_canonicals"`
	VenueCanonicals int            `json:"venue_canonicals"`
	TierByAxis      map[string]int `json:"tier_by_access_class,omitempty"`
}

// newNormalizeStatsCmd returns the read-only `normalize stats` subcommand that
// prints per-axis counts from the attribute tables.
func newNormalizeStatsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "stats",
		Short:       "Print normalized entity counts per axis from the local attribute tables",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			s, err := openStoreForRead(cmd.Context(), diceCLIName)
			if err != nil {
				return err
			}
			if s == nil {
				return printJSONFiltered(cmd.OutOrStdout(), normalizeStatsOutput{}, flags)
			}
			defer s.Close()

			tierRows, err := s.ListTierAttributes("ticket_type")
			if err != nil {
				return fmt.Errorf("listing tier attributes: %w", err)
			}

			// Count venue canonicals from venue_attributes (matched only) so the
			// metric is symmetric with TierCanonicals which counts tier_attributes.
			venueRows, err := s.ListVenueAttributes("venue")
			if err != nil {
				return fmt.Errorf("listing venue attributes: %w", err)
			}

			axisCount := map[string]int{}
			for _, r := range tierRows {
				if r.AccessClass != "" {
					axisCount[r.AccessClass]++
				}
			}

			out := normalizeStatsOutput{
				TierCanonicals:  len(tierRows),
				VenueCanonicals: len(venueRows),
				TierByAxis:      axisCount,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}
