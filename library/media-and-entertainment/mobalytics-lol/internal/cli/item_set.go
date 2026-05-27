// Copyright 2026 quantumglitch. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	moba "github.com/mvanhorn/printing-press-library/library/media-and-entertainment/mobalytics-lol/internal/mobalytics"
)

// clientItemsetRoot returns the LoL client config root for the current OS,
// or empty string if unknown.
//
// On Windows the install location varies — most users have it under
// C:\Riot Games\, but it can land under any drive letter or under Program
// Files. Respect an explicit override via LEAGUE_OF_LEGENDS_PATH first, then
// LOCALAPPDATA-based fallback, then the default install. On non-default
// layouts the caller still sees a clear "client not installed" message
// rather than silently writing to nowhere.
func clientItemsetRoot() string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Riot Games", "League of Legends", "Config", "Champions")
	case "windows":
		if override := os.Getenv("LEAGUE_OF_LEGENDS_PATH"); override != "" {
			return filepath.Join(override, "Config", "Champions")
		}
		// LOCALAPPDATA is where Riot's installer records the install root by
		// default; check there before falling through to C:\Riot Games\.
		if localApp := os.Getenv("LOCALAPPDATA"); localApp != "" {
			candidate := filepath.Join(localApp, "Riot Games", "League of Legends", "Config", "Champions")
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
		// Fall back to the default install root.
		return `C:\Riot Games\League of Legends\Config\Champions`
	}
	return ""
}

// newItemSetCmd writes Mobalytics ARAM builds to the LoL client as item
// sets so they show up in champion-select. If --to=stdout (default) we
// emit the JSON instead of touching disk.
func newItemSetCmd(flags *rootFlags) *cobra.Command {
	var aram string
	var to string
	cmd := &cobra.Command{
		Use:   "item-set",
		Short: "Write LoL client item-set JSON for N champions in ARAM mode.",
		Long: `For each champion in --aram, fetch the ARAM build, serialize to
the LoL client item-set format, and either print to stdout (--to=stdout)
or write into the live client Config folder (--to=client). On macOS the
target is ~/Library/Application Support/Riot Games/League of Legends/
Config/Champions/<Champion>/Recommended/<file>.json. If that folder
doesn't exist (no client installed), the command emits a skip message
per champion.`,
		Example:     `  mobalytics-lol-pp-cli item-set --aram jinx,kaisa,ezreal --to client`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if aram == "" {
				return fmt.Errorf("--aram requires a CSV of champion slugs (e.g. --aram jinx,kaisa,ezreal)")
			}
			if strings.HasPrefix(aram, "-") {
				return fmt.Errorf("--aram value %q looks like a flag; did you forget the CSV? (e.g. --aram jinx,kaisa,ezreal)", aram)
			}
			if dryRunOK(flags) {
				return nil
			}
			slugs := splitCSV(aram)
			if len(slugs) == 0 {
				return fmt.Errorf("--aram parsed to zero champion slugs from %q", aram)
			}
			client := moba.NewClient(flags.timeout)
			type result struct {
				Slug    string      `json:"slug"`
				Path    string      `json:"path,omitempty"`
				Skipped string      `json:"skipped,omitempty"`
				Itemset interface{} `json:"itemset,omitempty"`
			}
			results := make([]result, 0, len(slugs))
			root := clientItemsetRoot()
			for _, slug := range slugs {
				html, err := client.Fetch(moba.ChampionPath(slug, "aram-builds"))
				if err != nil {
					results = append(results, result{Slug: slug, Skipped: err.Error()})
					continue
				}
				builds := moba.ParseBuilds(html)
				if len(builds) == 0 {
					results = append(results, result{Slug: slug, Skipped: "no builds parsed from page"})
					continue
				}
				b := builds[0]
				// championID is unknown without the Data Dragon join; emit 0
				// and let the client fill it in (the file path itself maps
				// to the champion, which is the authoritative key).
				itemset := moba.BuildToItemset(b, 0, slug, "aram")
				r := result{Slug: slug, Itemset: itemset}
				if strings.EqualFold(to, "client") {
					if root == "" {
						r.Skipped = "no LoL client root known for this OS"
						results = append(results, r)
						continue
					}
					champFolder := strings.ToUpper(slug[:1]) + slug[1:]
					dir := filepath.Join(root, champFolder, "Recommended")
					// Check the Champions/ base dir (created by the installer
					// and always present), not the per-champion subdir
					// (only created after the user has loaded that champion).
					// Probing filepath.Dir(dir) would falsely report "client
					// not installed" for any champion the user hasn't played
					// yet, defeating the pre-load use case.
					if _, err := os.Stat(root); err != nil {
						r.Skipped = fmt.Sprintf("client not installed at %s", root)
						r.Itemset = nil
						results = append(results, r)
						continue
					}
					if err := os.MkdirAll(dir, 0o755); err != nil {
						r.Skipped = err.Error()
						results = append(results, r)
						continue
					}
					target := filepath.Join(dir, fmt.Sprintf("mobalytics-aram-%s.json", slug))
					b, mErr := json.MarshalIndent(itemset, "", "  ")
					if mErr != nil {
						r.Skipped = mErr.Error()
						results = append(results, r)
						continue
					}
					if err := os.WriteFile(target, b, 0o644); err != nil {
						r.Skipped = err.Error()
					} else {
						r.Path = target
						r.Itemset = nil // already on disk
					}
				}
				results = append(results, r)
			}
			return flags.printJSON(cmd, results)
		},
	}
	cmd.Flags().StringVar(&aram, "aram", "", "CSV of champion slugs to serialize from ARAM builds (required).")
	cmd.Flags().StringVar(&to, "to", "stdout", "Output sink: stdout (print JSON) or client (write into LoL config).")
	return cmd
}
