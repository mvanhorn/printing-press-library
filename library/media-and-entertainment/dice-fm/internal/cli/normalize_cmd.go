// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
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
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/normalizecfg"
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
	// All classifies every entity declared in the loaded normalize config.
	All bool
	// Entities, when non-empty, limits classification to these declared entity
	// names (validated against the loaded config).
	Entities []string
	// Fuzzy enables a second-pass Jaro-Winkler clustering of near-duplicate
	// canonical names.
	Fuzzy bool
	// FuzzyThreshold is the Jaro-Winkler similarity bar for the fuzzy pass.
	// Zero (unset) resolves to the default (0.92) in classifyOpts.fuzzyThreshold.
	FuzzyThreshold float64
	// ClassifierVersion is stamped on every written row.
	ClassifierVersion int
	// ExportUnmatched, when non-empty, is the file path to write unmatched
	// source values for external classification.
	ExportUnmatched string
	// ExportFormat controls the shape of the --export-unmatched file.
	// "prompt" (default) writes a self-describing classification request with
	// the tier-axis rubric and import schema. "names" writes only the CSV of
	// source_value names for programmatic use.
	ExportFormat string
	// ImportData, when non-nil, is pre-loaded import bytes to feed to
	// importMapping before the classify pipeline runs.
	ImportData []byte
	// ImportFormat is "csv" or "json" for the ImportData bytes.
	ImportFormat string
	// ImportEntity is the entity type the ImportData file is for. The default
	// "ticket_type" parses the fixed tier-axis schema (back-compat); any other
	// value parses the generic source_value + attribute-columns schema and writes
	// to entity_attributes.
	ImportEntity string
}

// classifyResultSummary is the JSON-serializable summary for one classify axis.
type classifyResultSummary struct {
	CanonicalCount int `json:"canonical_count"`
	Matched        int `json:"matched"`
	Unmatched      int `json:"unmatched"`
}

// normalizeSummary maps a friendly result key to its classify summary. Keys are
// "tiers"/"venues" for the two original entities (back-compat) and the entity
// name otherwise. Marshals to a flat JSON object.
type normalizeSummary map[string]classifyResultSummary

// summaryKeyFor maps an entity type to its friendly summary key. The two
// original spines keep their historical keys ("tiers"/"venues") so the JSON
// wire format is preserved; any other entity is keyed by its entity type.
func summaryKeyFor(entityType string) string {
	switch entityType {
	case "ticket_type":
		return "tiers"
	case "venue":
		return "venues"
	default:
		return entityType
	}
}

// runNormalize executes the normalization pipeline over s, writing a JSON
// summary to w. It is separated from the cobra plumbing so tests can call it
// directly with a seeded store.
func runNormalize(ctx context.Context, s *store.Store, opts normalizeOpts, w io.Writer) error {
	// Pre-import step: feed caller-supplied mappings before classification so
	// the imported manual rows are in place before the pipeline skips them.
	if len(opts.ImportData) > 0 {
		importEntity := opts.ImportEntity
		if importEntity == "" {
			importEntity = "ticket_type"
		}
		n, err := importMapping(s, "dice", importEntity, opts.ImportData, opts.ImportFormat)
		if err != nil {
			return fmt.Errorf("import: %w", err)
		}
		_ = n
	}

	classOpts := classifyOpts{
		ClassifierVersion: opts.ClassifierVersion,
		Fuzzy:             opts.Fuzzy,
		FuzzyThreshold:    opts.FuzzyThreshold,
	}

	// Load the active normalize config once so both target resolution and the
	// per-entity declaration lookup share a single view.
	cfg, err := loadNormalizeConfig()
	if err != nil {
		return fmt.Errorf("loading normalize config: %w", err)
	}

	targets, err := resolveNormalizeTargets(opts, cfg)
	if err != nil {
		return err
	}

	summary := normalizeSummary{}
	// Export to a per-entity path only when more than one entity is classified,
	// so a single-target run keeps the original filename and multi-target runs
	// don't clobber a shared --export-unmatched path.
	multiTarget := len(targets) > 1

	for _, entityType := range targets {
		res, err := classifyTarget(ctx, s, entityType, cfg, classOpts)
		if err != nil {
			return err
		}
		if res == nil {
			// An --all/--entity target absent from the config: skip with a warning.
			continue
		}
		summary[summaryKeyFor(entityType)] = classifyResultSummary{
			CanonicalCount: res.CanonicalCount,
			Matched:        res.Matched,
			Unmatched:      res.Unmatched,
		}
		if opts.ExportUnmatched != "" && res.Unmatched > 0 {
			exportPath := opts.ExportUnmatched
			if multiTarget {
				exportPath = perEntityExportPath(opts.ExportUnmatched, entityType)
			}
			if err := exportUnmatchedWithFormat(ctx, s, entityType, exportPath, opts.ExportFormat); err != nil {
				fmt.Fprintf(os.Stderr, "warning: export-unmatched %s: %v\n", entityType, err)
			}
		}
	}

	return json.NewEncoder(w).Encode(summary)
}

// resolveNormalizeTargets computes the deduplicated, stably-ordered list of
// entity types to classify from the opts flags and the loaded config:
//   - --tiers adds "ticket_type"; --venues adds "venue".
//   - --all adds every entity declared in cfg.
//   - --entity X,Y adds those names; each must be declared in cfg or the call
//     errors clearly.
//   - when none of the above is set, the default is ["ticket_type"] (today's
//     default behavior).
func resolveNormalizeTargets(opts normalizeOpts, cfg *normalizecfg.Config) ([]string, error) {
	set := map[string]bool{}
	if opts.Tiers {
		set["ticket_type"] = true
	}
	if opts.Venues {
		set["venue"] = true
	}
	if opts.All {
		for name := range cfg.Entities {
			set[name] = true
		}
	}
	for _, name := range opts.Entities {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := cfg.Entities[name]; !ok {
			return nil, fmt.Errorf("unknown --entity %q: not declared in the normalize config", name)
		}
		set[name] = true
	}

	if len(set) == 0 {
		// Default: classify the ticket_type spine (keyed "tiers" in the summary).
		return []string{"ticket_type"}, nil
	}

	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}

// classifyTarget classifies one entity type and returns its result. The two
// original spines route through classifyTiers/classifyVenues so their
// hardcoded-fallback path is preserved; any other entity is resolved directly
// from cfg and classified via the generic classifyEntity. A target absent from
// cfg returns (nil, nil) after a warning so an --all/--entity run skips it
// rather than failing the whole pass.
func classifyTarget(ctx context.Context, s *store.Store, entityType string, cfg *normalizecfg.Config, classOpts classifyOpts) (*classifyResult, error) {
	switch entityType {
	case "ticket_type":
		res, err := classifyTiers(ctx, s, classOpts)
		if err != nil {
			return nil, fmt.Errorf("classify tiers: %w", err)
		}
		return &res, nil
	case "venue":
		res, err := classifyVenues(ctx, s, classOpts)
		if err != nil {
			return nil, fmt.Errorf("classify venues: %w", err)
		}
		return &res, nil
	default:
		ent, ok := cfg.Entities[entityType]
		if !ok {
			fmt.Fprintf(os.Stderr, "warning: entity %q not declared in normalize config; skipping\n", entityType)
			return nil, nil
		}
		res, err := classifyEntity(ctx, s, entityType, ent, classOpts)
		if err != nil {
			return nil, fmt.Errorf("classify %s: %w", entityType, err)
		}
		return &res, nil
	}
}

// perEntityExportPath inserts the entity key before the file extension so a
// multi-entity --export-unmatched run writes one file per entity instead of
// clobbering a shared path (e.g. "unmatched.csv" → "unmatched.ticket_type.csv").
func perEntityExportPath(path, entityType string) string {
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	return base + "." + entityType + ext
}

// exportUnmatchedWithFormat writes unmatched source values for the given entity
// type to path in one of two formats controlled by format:
//   - "prompt" or "" (default): a self-describing classification request
//     containing the tier-axis rubric, import schema, and the unmatched names.
//   - "names": a minimal CSV of just the source_value names, for programmatic
//     use by callers that do not need the rubric preamble.
//
// Unknown format values return an error immediately without touching the store.
func exportUnmatchedWithFormat(ctx context.Context, s *store.Store, entityType, path, format string) error {
	// Validate format before touching the store; nil store is valid only for
	// pre-flight format validation (used by tests that pass nil).
	switch format {
	case "prompt", "":
		// default — OK
	case "names":
		// explicit names — OK
	default:
		return fmt.Errorf("unknown --export-format %q: must be prompt or names", format)
	}

	if s == nil {
		return nil
	}

	dbRows, err := s.DB().QueryContext(ctx,
		`SELECT source_value FROM entity_crosswalk
		 WHERE entity_type = ? AND method = 'unmatched'
		 ORDER BY source_value`, entityType)
	if err != nil {
		return err
	}
	defer dbRows.Close()

	var names []string
	for dbRows.Next() {
		var sv string
		if err := dbRows.Scan(&sv); err != nil {
			return err
		}
		names = append(names, sv)
	}
	if err := dbRows.Err(); err != nil {
		return err
	}

	f, err := os.Create(filepath.Clean(path))
	if err != nil {
		return err
	}
	defer f.Close()

	if format == "names" {
		return writeNamesCSV(f, names)
	}
	// Default: prompt format.
	return writePromptExport(f, names)
}

// writeNamesCSV writes a minimal CSV with a single source_value column.
func writeNamesCSV(w io.Writer, names []string) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"source_value"}); err != nil {
		return err
	}
	for _, name := range names {
		if err := cw.Write([]string{name}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// promptHeader is the static preamble for the prompt export format.
// It contains the tier-axis rubric and the import schema that the classifying
// LLM must produce. Only the names section that follows it is tenant data.
const promptHeader = `CLASSIFICATION REQUEST — Tier-Axis Labeling
===========================================

Your task: classify each ticket-type name below against the rubric and return
a CSV (or JSON array) in the exact import schema. Do not add rows that are not
listed. Return only the data — no explanation.

TIER-AXIS RUBRIC
----------------

access_class   : the access level or area of the ticket.
  Allowed values: ga, vip, premium, or empty.

sales_stage    : the pricing/release wave.
  Allowed values: super_early_bird, early_bird, final_release, last_chance, tier_n, or empty.

entry_window_type : the entry timing constraint.
  Allowed values: deadline, anytime, door, or empty.
  When type=deadline, set entry_window_time to HH:MM (24h); otherwise leave empty.

entry_window_time : HH:MM (24h) only when entry_window_type=deadline; otherwise empty.

group_size     : integer party size (e.g. "You+2" means 3); 0 or empty = single ticket.

comp_flag      : true if the ticket is complimentary/free; false otherwise.

IMPORT SCHEMA (CSV columns, in order)
--------------------------------------
source_value,access_class,sales_stage,entry_window_type,entry_window_time,group_size,comp_flag

JSON alternative (array of objects with the same keys) is also accepted.

Rules:
- Every name below must appear as source_value exactly as written.
- Use empty string for any axis you cannot confidently classify.
- Do not invent or rename any column.
- Do not include a header row when returning JSON.

---
source_value
`

// writePromptExport writes the self-describing classification request to w.
// The header section contains the rubric and import schema; the names section
// is a CSV of source_value entries (one per line, properly encoded).
func writePromptExport(w io.Writer, names []string) error {
	if _, err := io.WriteString(w, promptHeader); err != nil {
		return err
	}
	// Write each name as a CSV row so embedded commas/quotes are properly escaped.
	cw := csv.NewWriter(w)
	for _, name := range names {
		if err := cw.Write([]string{name}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// newNormalizeCmd returns the `normalize` cobra command and its subcommands.
// It writes to the local store (classification is a write path) and therefore
// must NOT be annotated mcp:read-only. The `normalize stats` subcommand is
// read-only and carries the annotation.
func newNormalizeCmd(flags *rootFlags) *cobra.Command {
	var (
		doTiers           bool
		doVenues          bool
		doAll             bool
		entities          []string
		fuzzy             bool
		fuzzyThreshold    float64
		classifierVersion int
		exportUnmatched   string
		exportFormat      string
		importFile        string
		importEntity      string
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
			"  dice-fm-pp-cli normalize --all\n" +
			"  dice-fm-pp-cli normalize --entity artist,genre\n" +
			"  dice-fm-pp-cli normalize --tiers --export-unmatched unmatched.csv\n" +
			"  dice-fm-pp-cli normalize --import mapped.csv\n" +
			"  dice-fm-pp-cli normalize stats",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
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
				All:               doAll,
				Entities:          entities,
				Fuzzy:             fuzzy,
				FuzzyThreshold:    fuzzyThreshold,
				ClassifierVersion: classifierVersion,
				ExportUnmatched:   exportUnmatched,
				ExportFormat:      exportFormat,
				ImportData:        importData,
				ImportFormat:      importFormat,
				ImportEntity:      importEntity,
			}
			return runNormalize(cmd.Context(), s, opts, cmd.OutOrStdout())
		},
	}

	cmd.Flags().BoolVar(&doTiers, "tiers", false, "Classify ticket-type names into tier axes (default when none of --tiers/--venues/--all/--entity given)")
	cmd.Flags().BoolVar(&doVenues, "venues", false, "Classify venue names into complex/room parts")
	cmd.Flags().BoolVar(&doAll, "all", false, "Classify every entity declared in the normalize config (ticket_type, venue, artist, genre, ticket_pool, …)")
	cmd.Flags().StringSliceVar(&entities, "entity", nil, "Classify only these declared entities by name (e.g. --entity artist,genre). Repeatable/comma-separated.")
	cmd.Flags().BoolVar(&fuzzy, "fuzzy", false, "Enable Jaro-Winkler clustering of near-duplicate canonical names (default false; deterministic without it)")
	cmd.Flags().Float64Var(&fuzzyThreshold, "fuzzy-threshold", defaultFuzzyThreshold, "Jaro-Winkler similarity bar for --fuzzy clustering, in (0,1]; higher is stricter (default 0.92). Ignored unless --fuzzy is set")
	cmd.Flags().IntVar(&classifierVersion, "classifier-version", 1, "Classifier version stamped on written rows (default 1)")
	cmd.Flags().StringVar(&exportUnmatched, "export-unmatched", "", "Write unmatched source values to this CSV file path for external classification")
	cmd.Flags().StringVar(&exportFormat, "export-format", "prompt", `Format for --export-unmatched: "prompt" (default) writes a self-describing classification request with tier-axis rubric; "names" writes only the source_value CSV`)
	cmd.Flags().StringVar(&importFile, "import", "", "Import a caller-supplied CSV or JSON mapping file (method=manual rows survive re-classification)")
	cmd.Flags().StringVar(&importEntity, "import-entity", "ticket_type", "Which entity the --import file is for (default ticket_type). ticket_type uses the fixed tier-axis schema; any other entity uses the generic source_value + attribute-columns schema written to entity_attributes")

	cmd.AddCommand(newNormalizeStatsCmd(flags))
	cmd.AddCommand(newNormalizeRecommendCmd(flags))
	cmd.AddCommand(newNormalizePromoteRulesCmd(flags))
	return cmd
}

// normalizeStatsOutput is the JSON shape for `normalize stats`.
type normalizeStatsOutput struct {
	TierCanonicals  int            `json:"tier_canonicals"`
	VenueCanonicals int            `json:"venue_canonicals"`
	TierByAxis      map[string]int `json:"tier_by_access_class,omitempty"`
}

// entityAttrStatsOutput is the JSON shape for `normalize stats --entity <name>`
// when the requested entity has no dedicated typed table. It summarizes the
// generic entity_attributes rows: a count of distinct canonical IDs plus, for
// each attribute key, a value→count map.
type entityAttrStatsOutput struct {
	Entity       string                    `json:"entity"`
	AttrRows     int                       `json:"attr_rows"`
	CanonicalIDs int                       `json:"canonical_ids"`
	ByAttrKey    map[string]map[string]int `json:"by_attr_key,omitempty"`
}

// newNormalizeStatsCmd returns the read-only `normalize stats` subcommand that
// prints per-axis counts from the attribute tables.
func newNormalizeStatsCmd(flags *rootFlags) *cobra.Command {
	var statsEntity string
	cmd := &cobra.Command{
		Use:         "stats",
		Short:       "Print normalized entity counts per axis from the local attribute tables",
		Example:     "  dice-fm-pp-cli normalize stats --entity ticket_type --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			s, err := openStoreForRead(cmd.Context(), diceCLIName)
			if err != nil {
				return err
			}

			// --entity for a non-tier/venue entity summarizes the generic
			// entity_attributes store instead of the typed tier/venue tables.
			if statsEntity != "" && statsEntity != "ticket_type" && statsEntity != "venue" {
				if s == nil {
					return printJSONFiltered(cmd.OutOrStdout(), entityAttrStatsOutput{Entity: statsEntity}, flags)
				}
				defer s.Close()
				out, err := entityAttrStats(s, statsEntity)
				if err != nil {
					return err
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
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
	cmd.Flags().StringVar(&statsEntity, "entity", "", "Summarize the generic entity_attributes store for this entity (counts by attr_key/value); for entities without a typed tier/venue table")
	return cmd
}

// entityAttrStats summarizes the generic entity_attributes rows for an entity
// type: a per-attr_key value→count map, the total row count, and the number of
// distinct canonical IDs carrying at least one attribute.
func entityAttrStats(s *store.Store, entity string) (entityAttrStatsOutput, error) {
	rows, err := s.ListEntityAttributes(entity)
	if err != nil {
		return entityAttrStatsOutput{}, fmt.Errorf("listing entity attributes for %q: %w", entity, err)
	}
	byKey := map[string]map[string]int{}
	cids := map[string]struct{}{}
	for _, r := range rows {
		cids[r.CanonicalID] = struct{}{}
		if byKey[r.AttrKey] == nil {
			byKey[r.AttrKey] = map[string]int{}
		}
		byKey[r.AttrKey][r.AttrValue]++
	}
	out := entityAttrStatsOutput{
		Entity:       entity,
		AttrRows:     len(rows),
		CanonicalIDs: len(cids),
		ByAttrKey:    byKey,
	}
	if len(byKey) == 0 {
		out.ByAttrKey = nil
	}
	return out, nil
}
