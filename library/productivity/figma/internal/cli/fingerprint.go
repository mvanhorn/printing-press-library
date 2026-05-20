// Copyright 2026 giuliano-giacaglia. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/figma/internal/store"
	"github.com/spf13/cobra"
)

// canonical entry for fingerprinting — keeps the surface tiny so a stable
// sha256 round-trips across syncs that may reorder responses.
type canonicalEntry struct {
	ID    string `json:"id"`
	Name  string `json:"name,omitempty"`
	Value any    `json:"value,omitempty"`
}

// fingerprintManifest is the canonicalized payload that feeds the hash.
type fingerprintManifest struct {
	Variables  []canonicalEntry `json:"variables"`
	Components []canonicalEntry `json:"components"`
	Styles     []canonicalEntry `json:"styles"`
}

// canonicalize returns deterministic JSON bytes for the manifest. Entries
// within each slice are sorted by id so input ordering does not perturb the
// hash. Map values inside `Value` are normalized via canonicalKeys so map
// iteration order does not bleed through either.
func canonicalize(m fingerprintManifest) []byte {
	sortEntries := func(s []canonicalEntry) []canonicalEntry {
		out := make([]canonicalEntry, len(s))
		copy(out, s)
		for i := range out {
			if mp, ok := out[i].Value.(map[string]interface{}); ok {
				out[i].Value = canonicalKeys(mp)
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
		return out
	}
	canonical := fingerprintManifest{
		Variables:  sortEntries(m.Variables),
		Components: sortEntries(m.Components),
		Styles:     sortEntries(m.Styles),
	}
	b, _ := json.Marshal(canonical)
	return b
}

func newFingerprintCmd(flags *rootFlags) *cobra.Command {
	var expect, format, dbPath string

	cmd := &cobra.Command{
		Use:   "fingerprint <key>",
		Short: "Stable hash of a file's tokens + components + styles surface.",
		Long: `Read locally synced variables, components, and styles for a file; canonicalize and
sha256-hash them. Pass --expect <hash> to verify and exit non-zero on mismatch (exit 2).`,
		Example: strings.Trim(`
  # Print the fingerprint
  figma-pp-cli fingerprint abc123XyZ

  # Verify against an expected value
  figma-pp-cli fingerprint abc123XyZ --expect 9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08

  # Include the manifest
  figma-pp-cli fingerprint abc123XyZ --format json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run": true, "command": "fingerprint"}`)
				return nil
			}
			key := args[0]

			if dbPath == "" {
				dbPath = defaultDBPath("figma-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nlocal cache empty — run 'figma-pp-cli sync' first", err)
			}
			defer db.Close()

			vars, vErr := readEntries(db, "SELECT id, data FROM variables WHERE files_id = ?", key, "valuesByMode")
			if vErr != nil {
				return vErr
			}
			comps, cErr := readEntries(db, "SELECT id, data FROM files_components WHERE files_id = ?", key, "")
			if cErr != nil {
				return cErr
			}
			styles, sErr := readEntries(db, "SELECT id, data FROM files_styles WHERE files_id = ?", key, "")
			if sErr != nil {
				return sErr
			}

			if len(vars)+len(comps)+len(styles) == 0 {
				return fmt.Errorf("local cache empty for file %s — run 'figma-pp-cli sync' first", key)
			}

			manifest := fingerprintManifest{Variables: vars, Components: comps, Styles: styles}
			canonBytes := canonicalize(manifest)
			sum := sha256.Sum256(canonBytes)
			hash := hex.EncodeToString(sum[:])

			if expect != "" && !strings.EqualFold(strings.TrimSpace(expect), hash) {
				w := cmd.OutOrStdout()
				fmt.Fprintf(w, "fingerprint mismatch\nexpected: %s\nactual:   %s\nvariables=%d components=%d styles=%d\n",
					expect, hash, len(vars), len(comps), len(styles))
				return &cliError{code: 2, err: fmt.Errorf("fingerprint mismatch")}
			}

			switch strings.ToLower(format) {
			case "json":
				out := map[string]any{
					"file_key": key,
					"hash":     hash,
					"manifest": json.RawMessage(canonBytes),
					"counts": map[string]int{
						"variables":  len(vars),
						"components": len(comps),
						"styles":     len(styles),
					},
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			case "hash", "":
				fmt.Fprintln(cmd.OutOrStdout(), hash)
				return nil
			default:
				return usageErr(fmt.Errorf("unknown --format %q (want hash or json)", format))
			}
		},
	}

	cmd.Flags().StringVar(&expect, "expect", "", "Expected sha256 hex; mismatch exits 2")
	cmd.Flags().StringVar(&format, "format", "hash", "Output format: hash or json")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/figma-pp-cli/data.db)")

	return cmd
}

// readEntries pulls (id, data) rows and projects them to canonicalEntry.
// valueKey, when non-empty, is the JSON field on `data` to capture as Value.
func readEntries(db *store.Store, sqlStr, key, valueKey string) ([]canonicalEntry, error) {
	rows, err := db.DB().Query(sqlStr, key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []canonicalEntry
	for rows.Next() {
		var id string
		var data []byte
		if err := rows.Scan(&id, &data); err != nil {
			continue
		}
		var obj map[string]any
		_ = json.Unmarshal(data, &obj)
		entry := canonicalEntry{ID: id}
		if name, ok := obj["name"].(string); ok {
			entry.Name = name
		}
		if valueKey != "" {
			if v, ok := obj[valueKey]; ok {
				entry.Value = v
			}
		}
		out = append(out, entry)
	}
	return out, nil
}
