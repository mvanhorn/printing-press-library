// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored `normalize recommend` command: profiles the local store and
// emits a starter normalization config (recommended entities + shapes, empty
// rules) for the operator/agent to fill in.
// This file is NOT generated and survives `generate --force`.
package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/normalizecfg"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/store"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// profileStats captures the shape characteristics of one entity field.
type profileStats struct {
	// Distinct is the count of distinct raw values observed.
	Distinct int
	// AliasCollisions is the count of distinct raw values that collapse to
	// the same lower(strings.TrimSpace(...)) key as another raw value.
	AliasCollisions int
	// StructuredFrac is the fraction of distinct values that contain a digit
	// or a sub-part separator (" - ", "/", ":"), signalling structured labels.
	StructuredFrac float64
}

// structuredSignalRE matches values that carry a digit or a common sub-part
// separator. Domain-neutral: matches structural patterns only, not keywords.
var structuredSignalRE = regexp.MustCompile(`[0-9]| - |/|:`)

// safeSourcePathRE allows only ASCII letters, digits, underscores, and dots in
// each path component. candidateSources is the only caller today; this guard
// exists so a future user-supplied source cannot inject SQL via the interpolated
// path components in distinctSourceValues.
var safeSourcePathRE = regexp.MustCompile(`^[A-Za-z0-9_.]+$`)

// validateSourcePath rejects source strings whose path components contain
// characters outside the allowlist (ASCII letters, digits, '_', '.').
// Must be called before any SQL interpolation of the parsed components.
func validateSourcePath(source string) error {
	// Strip the [*] array marker before validating — it is syntactic sugar, not
	// a path component, and is removed by parseSourcePath before interpolation.
	stripped := strings.ReplaceAll(source, "[*]", "")
	if !safeSourcePathRE.MatchString(stripped) {
		return fmt.Errorf("unsafe source path %q: path components must contain only ASCII letters, digits, '_', and '.'", source)
	}
	return nil
}

// recommendShape classifies a field's shape from its profile stats.
//   - StructuredFrac >= 0.4  → attributes (structured labels with parseable sub-parts)
//   - Distinct <= 40         → vocab      (low-cardinality controlled set)
//   - otherwise              → alias      (high-cardinality free-text names)
func recommendShape(p profileStats) string {
	if p.StructuredFrac >= 0.4 {
		return normalizecfg.ShapeAttributes
	}
	if p.Distinct <= 40 {
		return normalizecfg.ShapeVocab
	}
	return normalizecfg.ShapeAlias
}

// profileField enumerates distinct source values for the given dotted source
// path over the resources table and returns profiling statistics.
//
// source has the form "<resource_type>.<dotted.path>" for scalar fields
// (e.g. "tickets.ticketType.name") or
// "<resource_type>.<array_field>[*].<leaf>" for array fields
// (e.g. "events.venues[*].name").
//
// Array expansion is done with SQLite's json_each; scalar paths use json_extract.
func profileField(ctx context.Context, db *sql.DB, source string) (profileStats, error) {
	if err := validateSourcePath(source); err != nil {
		return profileStats{}, err
	}
	vals, err := distinctSourceValues(ctx, db, source)
	if err != nil {
		return profileStats{}, err
	}
	if len(vals) == 0 {
		return profileStats{}, nil
	}

	// AliasCollisions: distinct values that share a lowertrim key with another.
	normalised := make(map[string]int, len(vals))
	for _, v := range vals {
		key := strings.ToLower(strings.TrimSpace(v))
		normalised[key]++
	}
	collisions := 0
	for _, count := range normalised {
		if count > 1 {
			collisions += count
		}
	}

	// StructuredFrac: fraction of distinct values matching the structure signal.
	structured := 0
	for _, v := range vals {
		if structuredSignalRE.MatchString(v) {
			structured++
		}
	}

	return profileStats{
		Distinct:        len(vals),
		AliasCollisions: collisions,
		StructuredFrac:  float64(structured) / float64(len(vals)),
	}, nil
}

// distinctSourceValues returns distinct non-null raw string values for the
// given source path from the resources table.
func distinctSourceValues(ctx context.Context, db *sql.DB, source string) ([]string, error) {
	// Re-validate here so direct callers (not only profileField) cannot inject
	// SQL via the path components interpolated into the queries below.
	if err := validateSourcePath(source); err != nil {
		return nil, err
	}
	resourceType, jsonPath, isArray, err := parseSourcePath(source)
	if err != nil {
		return nil, err
	}

	var rows *sql.Rows
	if isArray {
		// e.g. events.venues[*].name → json_each expand then json_extract leaf
		// jsonPath is the array field (e.g. "venues"), leafPath is the sub-field (e.g. "name")
		arrayField, leafField := splitArrayPath(jsonPath)
		if leafField == "" {
			// Scalar array (e.g. events.genres[*]): each json_each element IS the
			// value, so take v.value directly instead of extracting a leaf field.
			rows, err = db.QueryContext(ctx,
				`SELECT DISTINCT v.value
				 FROM resources r, json_each(json_extract(r.data, '$.`+arrayField+`')) v
				 WHERE r.resource_type = ?
				   AND v.value IS NOT NULL
				 ORDER BY v.value`,
				resourceType,
			)
		} else {
			rows, err = db.QueryContext(ctx,
				`SELECT DISTINCT json_extract(v.value, '$.`+leafField+`')
				 FROM resources r, json_each(json_extract(r.data, '$.`+arrayField+`')) v
				 WHERE r.resource_type = ?
				   AND json_extract(v.value, '$.`+leafField+`') IS NOT NULL`,
				resourceType,
			)
		}
	} else {
		rows, err = db.QueryContext(ctx,
			`SELECT DISTINCT json_extract(data, '$.`+jsonPath+`')
			 FROM resources
			 WHERE resource_type = ?
			   AND json_extract(data, '$.`+jsonPath+`') IS NOT NULL`,
			resourceType,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vals []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		vals = append(vals, v)
	}
	return vals, rows.Err()
}

// parseSourcePath splits a source string into resource_type, JSON path, and
// whether the path targets an array element.
//
// Formats accepted:
//   - "tickets.ticketType.name"   → ("tickets", "ticketType.name", false)
//   - "events.venues[*].name"     → ("events", "venues.name", true)
//   - "events.genres"             → ("events", "genres", false)
func parseSourcePath(source string) (resourceType, jsonPath string, isArray bool, err error) {
	dot := strings.Index(source, ".")
	if dot < 0 {
		return "", "", false, fmt.Errorf("source %q: expected <resource_type>.<path>", source)
	}
	resourceType = source[:dot]
	rest := source[dot+1:]
	if strings.Contains(rest, "[*]") {
		isArray = true
		rest = strings.ReplaceAll(rest, "[*]", "")
	}
	jsonPath = rest
	return resourceType, jsonPath, isArray, nil
}

// splitArrayPath splits "venues.name" into ("venues", "name"). If there is no
// dot (bare array field with no sub-path), returns (path, "").
func splitArrayPath(path string) (arrayField, leafField string) {
	dot := strings.Index(path, ".")
	if dot < 0 {
		return path, ""
	}
	return path[:dot], path[dot+1:]
}

// candidateSources maps entity names to source paths for the dice-fm CLI. The
// set mirrors the entities declared in the embedded starter config
// (normalize_starter.yaml) so `normalize recommend` profiles every entity the
// operator can wire — including price_tier and ticket_pool, which were
// previously absent here and so were invisible to the recommend→classify
// bootstrap.
var candidateSources = map[string]string{
	"ticket_type": "tickets.ticketType.name",
	"price_tier":  "tickets.ticketType.price",
	"venue":       "events.venues[*].name",
	"genre":       "events.genres[*]",
	"artist":      "events.artists[*].name",
	"ticket_pool": "events.ticketPools[*].name",
}

// runRecommend profiles each candidate source against the store and returns a
// starter Config with recommended shapes and empty rules.
func runRecommend(ctx context.Context, s *store.Store, sources map[string]string) (*normalizecfg.Config, error) {
	cfg := &normalizecfg.Config{
		Version:  1,
		Entities: make(map[string]normalizecfg.Entity),
	}

	for name, sourcePath := range sources {
		stats, err := profileField(ctx, s.DB(), sourcePath)
		if err != nil {
			// Skip sources that error (e.g. missing table columns, malformed data).
			fmt.Fprintf(os.Stderr, "warning: profile %q (%s): %v\n", name, sourcePath, err)
			continue
		}
		if stats.Distinct == 0 {
			// No data yet — skip.
			continue
		}

		cfg.Entities[name] = normalizecfg.Entity{
			Source: sourcePath,
			Shape:  recommendShape(stats),
			// Rules intentionally empty — for the operator/agent to fill in.
		}
	}

	return cfg, nil
}

// newNormalizeRecommendCmd returns the `normalize recommend` subcommand. It
// profiles the local store and emits a starter normalization config (recommended
// entities + shapes, empty rules) for the operator/agent to fill in.
// This command is read-only: it only reads from the store and never mutates it.
func newNormalizeRecommendCmd(flags *rootFlags) *cobra.Command {
	var printOnly bool

	cmd := &cobra.Command{
		Use:   "recommend",
		Short: "Profile the local store and emit a starter normalization config",
		Long: "Reads the local SQLite store, profiles each candidate entity field " +
			"(ticket type, venue, genre, artist) for cardinality and structure, " +
			"and writes a starter normalize.yaml with recommended shapes and empty " +
			"rules for the operator or agent to fill in.\n\n" +
			"Shape heuristic:\n" +
			"  structured labels (structured fraction >= 40%) → attributes\n" +
			"  low-cardinality controlled set (≤40 distinct)  → vocab\n" +
			"  high-cardinality free-text names                → alias\n\n" +
			"The emitted config has NO rules — those are for the operator to supply.",
		Example: "  dice-fm-pp-cli normalize recommend\n" +
			"  dice-fm-pp-cli normalize recommend --print",
		// No mcp:read-only: the default path writes a starter config file to
		// disk (only --print is read-only), so the tool must not advertise
		// readOnlyHint and have hosts auto-approve a filesystem write.
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			s, err := openStoreForRead(cmd.Context(), diceCLIName)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			if s == nil {
				// No sync has been run — emit an empty config rather than error.
				cfg := &normalizecfg.Config{Version: 1, Entities: map[string]normalizecfg.Entity{}}
				return writeRecommendConfig(cmd, cfg, printOnly)
			}
			defer s.Close()

			cfg, err := runRecommend(cmd.Context(), s, candidateSources)
			if err != nil {
				return err
			}
			return writeRecommendConfig(cmd, cfg, printOnly)
		},
	}

	cmd.Flags().BoolVar(&printOnly, "print", false, "Print the recommended config to stdout instead of writing it to the default config path")
	return cmd
}

// writeRecommendConfig marshals cfg to YAML and either writes it to the default
// config path or prints it to stdout when printOnly is true.
func writeRecommendConfig(cmd *cobra.Command, cfg *normalizecfg.Config, printOnly bool) error {
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if printOnly {
		_, err = cmd.OutOrStdout().Write(out)
		return err
	}

	// Under verify, skip the disk write and print the YAML to stdout instead.
	// Read-only store profiling above is fine; only the config-file write is gated.
	if cliutil.IsVerifyEnv() {
		_, err = cmd.OutOrStdout().Write(out)
		return err
	}

	cfgPath := defaultConfigPath(diceCLIName)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	if err := os.WriteFile(cfgPath, out, 0o600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "wrote starter config to %s\n", cfgPath)
	return nil
}
