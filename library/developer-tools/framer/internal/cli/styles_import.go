// Copyright 2026 ioncom. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/framer/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/framer/internal/store"
	"github.com/spf13/cobra"
)

// styleToken represents a single color token parsed from an external file.
type styleToken struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Status string `json:"status"` // "new", "exists", "changed"
}

// cssColorRE matches CSS custom properties with hex or rgb/rgba values.
var cssColorRE = regexp.MustCompile(`--([a-zA-Z0-9-]+)\s*:\s*(#[0-9a-fA-F]{3,8}|rgba?\([^)]+\))`)

// tailwindColorRE matches simple color assignments in Tailwind config files.
var tailwindColorRE = regexp.MustCompile(`['"]?([a-zA-Z0-9-]+)['"]?\s*:\s*['"]?(#[0-9a-fA-F]{3,8})['"]?`)

// jsonColorRE matches hex color values in JSON token files.
var jsonColorValueRE = regexp.MustCompile(`^#[0-9a-fA-F]{3,8}$|^rgba?\([^)]+\)$`)

func newStylesImportCmd(flags *rootFlags) *cobra.Command {
	var fromFile string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "styles-import",
		Short: "Import CSS variables or Tailwind config colors as Framer color styles (local preview/dry-run)",
		Long: strings.Trim(`
Import color tokens from CSS custom properties, Tailwind config, or JSON
design tokens and preview them as Framer color styles.

Supported file types:
  .css          Parse CSS custom properties (--variable-name: #hex)
  .js / .ts     Parse Tailwind config colors section
  .json         Parse design tokens JSON (top-level key → color value)

By default this command runs in dry-run mode, showing what would be created.
Live push requires FRAMER_API_KEY to be set.`, "\n"),
		Example: strings.Trim(`
  # Preview CSS variable import
  framer-pp-cli styles-import --from design-tokens.css --dry-run

  # Preview Tailwind config import
  framer-pp-cli styles-import --from tailwind.config.js

  # Import JSON design tokens with JSON output
  framer-pp-cli styles-import --from tokens.json --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if fromFile == "" {
				return usageErr(fmt.Errorf("required flag \"from\" not set"))
			}
			// NOTE: no early dryRunOK() guard — styles-import previews via the
			// table + the `if flags.dryRun` summary below, gating only the live
			// push on the flag. An early return would make --dry-run emit nothing.

			// Read the source file.
			data, err := os.ReadFile(fromFile)
			if err != nil {
				return fmt.Errorf("reading %s: %w", fromFile, err)
			}

			// Parse tokens based on file extension.
			ext := strings.ToLower(filepath.Ext(fromFile))
			var tokens []styleToken
			switch ext {
			case ".css":
				tokens = parseCSSTokens(string(data))
			case ".js", ".ts":
				tokens = parseTailwindTokens(string(data))
			case ".json":
				tokens, err = parseJSONTokens(data)
				if err != nil {
					return fmt.Errorf("parsing JSON tokens: %w", err)
				}
			default:
				return usageErr(fmt.Errorf("unsupported file type %q; supported: .css, .js, .ts, .json", ext))
			}

			if len(tokens) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No color tokens found in", fromFile)
				return nil
			}

			// Compare against existing color styles in SQLite store if available.
			existing := loadExistingColorStyles(cmd, dbPath)
			for i := range tokens {
				if prev, ok := existing[tokens[i].Name]; ok {
					if prev == tokens[i].Value {
						tokens[i].Status = "exists"
					} else {
						tokens[i].Status = "changed"
					}
				} else {
					tokens[i].Status = "new"
				}
			}

			// Output.
			if flags.asJSON {
				return flags.printJSON(cmd, tokens)
			}

			// Table output.
			headers := []string{"TOKEN NAME", "COLOR VALUE", "STATUS"}
			rows := make([][]string, len(tokens))
			for i, t := range tokens {
				rows[i] = []string{t.Name, t.Value, t.Status}
			}
			if err := flags.printTable(cmd, headers, rows); err != nil {
				return err
			}

			if flags.dryRun {
				fmt.Fprintf(cmd.ErrOrStderr(), "\nDry-run: %d tokens parsed. No changes made.\n", len(tokens))
				return nil
			}

			// Live push: create color styles via bridge for new/changed tokens.
			bc, err := client.NewBridgeClient()
			if err != nil {
				return err
			}

			created := 0
			for _, t := range tokens {
				if t.Status != "new" && t.Status != "changed" {
					continue
				}
				payload := map[string]string{
					"name":  t.Name,
					"value": t.Value,
				}
				payloadJSON, err := json.Marshal(payload)
				if err != nil {
					return fmt.Errorf("marshaling token %s: %w", t.Name, err)
				}
				if _, err := bc.Call("styles-colors-create", string(payloadJSON)); err != nil {
					return fmt.Errorf("creating style %q: %w", t.Name, err)
				}
				created++
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "\n%d color style(s) created.\n", created)
			return nil
		},
	}

	cmd.Flags().StringVar(&fromFile, "from", "", "Source file path (.css, .js, .ts, or .json)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/framer-pp-cli/data.db)")

	return cmd
}

// parseCSSTokens extracts CSS custom properties that have color values.
func parseCSSTokens(content string) []styleToken {
	matches := cssColorRE.FindAllStringSubmatch(content, -1)
	var tokens []styleToken
	for _, m := range matches {
		tokens = append(tokens, styleToken{
			Name:  m[1],
			Value: strings.TrimSpace(m[2]),
		})
	}
	return tokens
}

// parseTailwindTokens extracts color names and hex values from a Tailwind config.
func parseTailwindTokens(content string) []styleToken {
	// Find the colors section.
	colorsIdx := strings.Index(content, "colors")
	if colorsIdx < 0 {
		// Fall back to scanning the whole file.
		colorsIdx = 0
	}
	section := content[colorsIdx:]

	matches := tailwindColorRE.FindAllStringSubmatch(section, -1)
	seen := map[string]bool{}
	var tokens []styleToken
	for _, m := range matches {
		name := m[1]
		value := m[2]
		// Skip common Tailwind config keys that aren't color names.
		if name == "colors" || name == "extend" || name == "theme" {
			continue
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		tokens = append(tokens, styleToken{
			Name:  name,
			Value: value,
		})
	}
	return tokens
}

// parseJSONTokens walks top-level keys of a JSON file looking for color values.
func parseJSONTokens(data []byte) ([]styleToken, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}

	var tokens []styleToken
	// Sort keys for deterministic output.
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := obj[k]
		// Try as a simple string value.
		var strVal string
		if json.Unmarshal(v, &strVal) == nil {
			if jsonColorValueRE.MatchString(strings.TrimSpace(strVal)) {
				tokens = append(tokens, styleToken{
					Name:  k,
					Value: strings.TrimSpace(strVal),
				})
			}
			continue
		}
		// Try as an object with a "value" field (design tokens format).
		var nested map[string]json.RawMessage
		if json.Unmarshal(v, &nested) == nil {
			if valRaw, ok := nested["value"]; ok {
				var val string
				if json.Unmarshal(valRaw, &val) == nil && jsonColorValueRE.MatchString(strings.TrimSpace(val)) {
					tokens = append(tokens, styleToken{
						Name:  k,
						Value: strings.TrimSpace(val),
					})
				}
			}
		}
	}
	return tokens, nil
}

// loadExistingColorStyles reads color styles from the local SQLite store.
// Returns a map of name → value for comparison. Returns an empty map on error.
func loadExistingColorStyles(cmd *cobra.Command, dbPath string) map[string]string {
	if dbPath == "" {
		dbPath = defaultDBPath("framer-pp-cli")
	}
	existing := map[string]string{}

	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return existing
	}
	defer db.Close()

	// Existing color styles live in the styles-colors resource. (This previously
	// loaded cms-collections — unrelated CMS data — which never matched a token.)
	rows, err := db.List("styles-colors", 0)
	if err != nil {
		return existing
	}

	for _, raw := range rows {
		var obj map[string]any
		if json.Unmarshal(raw, &obj) != nil {
			continue
		}
		name, _ := obj["name"].(string)
		value, _ := obj["value"].(string)
		if name != "" && value != "" {
			existing[name] = value
		}
	}
	return existing
}
