// Copyright 2026 giuliano-giacaglia. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/figma/internal/store"
	"github.com/spf13/cobra"
)

// variable is the minimal token surface we diff between two file_versions.
type variable struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Type         string                 `json:"type,omitempty"`
	ValuesByMode map[string]interface{} `json:"values_by_mode,omitempty"`
}

// diffResult lists the four buckets a token diff produces.
type diffResult struct {
	Added        []variable   `json:"added"`
	Removed      []variable   `json:"removed"`
	Renamed      []renamePair `json:"renamed"`
	ValueChanged []valueDiff  `json:"value_changed"`
}

type renamePair struct {
	ID      string `json:"id"`
	OldName string `json:"old_name"`
	NewName string `json:"new_name"`
}

type valueDiff struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	OldValuesMode map[string]interface{} `json:"old_values_by_mode"`
	NewValuesMode map[string]interface{} `json:"new_values_by_mode"`
}

// diffVariables returns the 4-way diff between two variable slices,
// matching by id. Mode-aware: the value_changed bucket fires when any mode's
// value differs.
func diffVariables(a, b []variable) diffResult {
	aByID := map[string]variable{}
	for _, v := range a {
		aByID[v.ID] = v
	}
	bByID := map[string]variable{}
	for _, v := range b {
		bByID[v.ID] = v
	}
	var res diffResult
	for id, av := range aByID {
		bv, ok := bByID[id]
		if !ok {
			res.Removed = append(res.Removed, av)
			continue
		}
		if av.Name != bv.Name {
			res.Renamed = append(res.Renamed, renamePair{ID: id, OldName: av.Name, NewName: bv.Name})
		}
		if !valuesEqual(av.ValuesByMode, bv.ValuesByMode) {
			res.ValueChanged = append(res.ValueChanged, valueDiff{
				ID:            id,
				Name:          bv.Name,
				OldValuesMode: av.ValuesByMode,
				NewValuesMode: bv.ValuesByMode,
			})
		}
	}
	for id, bv := range bByID {
		if _, ok := aByID[id]; !ok {
			res.Added = append(res.Added, bv)
		}
	}
	// Stable order for tests/output.
	sort.Slice(res.Added, func(i, j int) bool { return res.Added[i].ID < res.Added[j].ID })
	sort.Slice(res.Removed, func(i, j int) bool { return res.Removed[i].ID < res.Removed[j].ID })
	sort.Slice(res.Renamed, func(i, j int) bool { return res.Renamed[i].ID < res.Renamed[j].ID })
	sort.Slice(res.ValueChanged, func(i, j int) bool { return res.ValueChanged[i].ID < res.ValueChanged[j].ID })
	return res
}

// valuesEqual compares two valuesByMode maps via canonical JSON encoding.
func valuesEqual(a, b map[string]interface{}) bool {
	ab, _ := json.Marshal(canonicalKeys(a))
	bb, _ := json.Marshal(canonicalKeys(b))
	return string(ab) == string(bb)
}

func canonicalKeys(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make(map[string]interface{}, len(m))
	for _, k := range keys {
		out[k] = m[k]
	}
	return out
}

func newTokensCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tokens",
		Short: "Design-token operations on Figma variables.",
	}
	cmd.AddCommand(newTokensDiffCmd(flags))
	return cmd
}

func newTokensDiffCmd(flags *rootFlags) *cobra.Command {
	var fromVer, toVer, format, dbPath string

	cmd := &cobra.Command{
		Use:   "diff <key>",
		Short: "Diff Figma variables between two file versions (mode-aware).",
		Long: `Resolve --from and --to to file_version ids, fetch the variables-local snapshot
for each, and emit added / removed / renamed / value_changed groups. Use HEAD to
mean the current synced snapshot.`,
		Example: strings.Trim(`
  # Diff between two saved versions
  figma-pp-cli tokens diff abc123XyZ --from 4242:design-tokens --to HEAD

  # JSON output
  figma-pp-cli tokens diff abc123XyZ --from 4242:design-tokens --to 4998:rebrand --format json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run": true, "command": "tokens diff"}`)
				return nil
			}
			if fromVer == "" || toVer == "" {
				return usageErr(fmt.Errorf("both --from and --to are required (use HEAD for current snapshot)"))
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

			fromVars, ferr := loadVariablesSnapshot(db, key, fromVer)
			if ferr != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "no variables snapshot for version %s — run sync at that version first or pass HEAD\n", fromVer)
				return nil
			}
			toVars, terr := loadVariablesSnapshot(db, key, toVer)
			if terr != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "no variables snapshot for version %s — run sync at that version first or pass HEAD\n", toVer)
				return nil
			}

			diff := diffVariables(fromVars, toVars)

			switch strings.ToLower(format) {
			case "json":
				out := map[string]any{
					"file_key":      key,
					"from":          fromVer,
					"to":            toVer,
					"added":         diff.Added,
					"removed":       diff.Removed,
					"renamed":       diff.Renamed,
					"value_changed": diff.ValueChanged,
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			case "md", "markdown", "":
				w := cmd.OutOrStdout()
				fmt.Fprintf(w, "# Tokens diff: %s..%s\n\n", fromVer, toVer)
				fmt.Fprintf(w, "**Added:** %d · **Removed:** %d · **Renamed:** %d · **Value changed:** %d\n\n",
					len(diff.Added), len(diff.Removed), len(diff.Renamed), len(diff.ValueChanged))
				if len(diff.Added) > 0 {
					fmt.Fprintln(w, "## Added")
					for _, v := range diff.Added {
						fmt.Fprintf(w, "- `%s` (id: %s)\n", v.Name, v.ID)
					}
					fmt.Fprintln(w)
				}
				if len(diff.Removed) > 0 {
					fmt.Fprintln(w, "## Removed")
					for _, v := range diff.Removed {
						fmt.Fprintf(w, "- `%s` (id: %s)\n", v.Name, v.ID)
					}
					fmt.Fprintln(w)
				}
				if len(diff.Renamed) > 0 {
					fmt.Fprintln(w, "## Renamed")
					for _, r := range diff.Renamed {
						fmt.Fprintf(w, "- `%s` -> `%s` (id: %s)\n", r.OldName, r.NewName, r.ID)
					}
					fmt.Fprintln(w)
				}
				if len(diff.ValueChanged) > 0 {
					fmt.Fprintln(w, "## Value changed")
					for _, v := range diff.ValueChanged {
						oldB, _ := json.Marshal(v.OldValuesMode)
						newB, _ := json.Marshal(v.NewValuesMode)
						fmt.Fprintf(w, "- `%s`: %s -> %s\n", v.Name, string(oldB), string(newB))
					}
					fmt.Fprintln(w)
				}
				return nil
			default:
				return usageErr(fmt.Errorf("unknown --format %q (want md or json)", format))
			}
		},
	}

	cmd.Flags().StringVar(&fromVer, "from", "", "Source version id (or HEAD)")
	cmd.Flags().StringVar(&toVer, "to", "", "Target version id (or HEAD)")
	cmd.Flags().StringVar(&format, "format", "md", "Output format: md or json")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/figma-pp-cli/data.db)")

	return cmd
}

// loadVariablesSnapshot reads variables for a file from the local store. The
// store currently holds only the current (HEAD) snapshot keyed on (id), with
// no version column, so version-tagged lookups are not yet wired. Until they
// are, accept HEAD (or empty) and refuse anything else explicitly rather than
// silently returning the same HEAD snapshot for every version, which would
// make `tokens diff --from <v1> --to <v2>` look successful while always
// comparing identical data.
func loadVariablesSnapshot(db *store.Store, fileKey, version string) ([]variable, error) {
	if version != "" && !strings.EqualFold(version, "HEAD") {
		return nil, fmt.Errorf("version-tagged variable snapshots are not yet stored; only HEAD..HEAD is supported today. Use HEAD for both --from and --to, or pin the file at the version you want and run 'figma-pp-cli sync' first")
	}
	rows, err := db.DB().Query(`SELECT id, data FROM variables WHERE files_id = ?`, fileKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []variable
	hasAny := false
	for rows.Next() {
		hasAny = true
		var id string
		var data []byte
		if err := rows.Scan(&id, &data); err != nil {
			continue
		}
		var obj map[string]any
		_ = json.Unmarshal(data, &obj)
		v := variable{ID: id}
		if name, ok := obj["name"].(string); ok {
			v.Name = name
		}
		if t, ok := obj["resolvedType"].(string); ok {
			v.Type = t
		} else if t, ok := obj["type"].(string); ok {
			v.Type = t
		}
		if vbm, ok := obj["valuesByMode"].(map[string]any); ok {
			v.ValuesByMode = vbm
		}
		out = append(out, v)
	}
	if !hasAny {
		return nil, fmt.Errorf("no variables for file %s", fileKey)
	}
	return out, nil
}
