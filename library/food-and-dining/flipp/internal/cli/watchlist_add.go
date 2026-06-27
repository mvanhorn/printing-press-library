// Copyright 2026 mlabrenz and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/flipp/internal/cliutil"
	"github.com/spf13/cobra"
)

type watchEntry struct {
	Query       string    `json:"query"`
	TargetPrice float64   `json:"target_price"`
	Zip         string    `json:"zip"`
	Locale      string    `json:"locale"`
	AddedAt     time.Time `json:"added_at"`
}

func upsertWatchEntry(entries []watchEntry, entry watchEntry) ([]watchEntry, bool) {
	for i, existing := range entries {
		if strings.EqualFold(existing.Query, entry.Query) &&
			strings.EqualFold(existing.Zip, entry.Zip) &&
			strings.EqualFold(existing.Locale, entry.Locale) {
			entries[i] = entry
			return entries, true
		}
	}
	return append(entries, entry), false
}

func loadWatchEntries(path string) ([]watchEntry, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path is constrained to cliutil.DataDir() by caller.
	if errors.Is(err, os.ErrNotExist) {
		return []watchEntry{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return []watchEntry{}, nil
	}
	entries := []watchEntry{}
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("invalid watchlist JSON at %s: %w", path, err)
	}
	return entries, nil
}

// pp:data-source computed
func newNovelWatchlistAddCmd(flags *rootFlags) *cobra.Command {
	var flagTargetPrice string
	var flagZip string
	var flagLocale string

	cmd := &cobra.Command{
		Use:     "add <query>",
		Short:   "Persist target prices for recurring staples and compare them against future synced snapshots.",
		Example: "  flipp-pp-cli watchlist add milk --target-price 3.50 --zip 85001 --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("query is required"))
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would add %q to the Flipp watchlist for %s\n", args[0], flagZip)
				return nil
			}
			target, err := strconv.ParseFloat(flagTargetPrice, 64)
			if err != nil || target <= 0 {
				return usageErr(fmt.Errorf("--target-price must be a positive number"))
			}
			dir, err := cliutil.DataDir()
			if err != nil {
				return err
			}
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return err
			}
			path := filepath.Join(dir, "watchlist.json")
			entries, err := loadWatchEntries(path)
			if err != nil {
				return err
			}
			entry := watchEntry{Query: args[0], TargetPrice: target, Zip: flagZip, Locale: flagLocale, AddedAt: time.Now().UTC()}
			entries, _ = upsertWatchEntry(entries, entry)
			data, err := json.MarshalIndent(entries, "", "  ")
			if err != nil {
				return err
			}
			if err := cliutil.AtomicWritePrivateFile(path, data, 0o600, 0o750); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			currentBest := ""
			c, clientErr := flags.newClient()
			if clientErr == nil {
				if res, err := fetchFlippSearch(ctx, c, args[0], flagZip, flagLocale, "price_low_to_high"); err == nil {
					matches := append([]flippItem{}, res.Items...)
					matches = append(matches, res.EcomItems...)
					sortByPrice(matches)
					for _, item := range matches {
						if item.CurrentPrice != nil && matchesSearchIntent(item, args[0]) {
							currentBest = fmt.Sprintf("%s at %s for %.2f", item.Name, merchantName(item), *item.CurrentPrice)
							break
						}
					}
				}
			}
			view := struct {
				Added       watchEntry `json:"added"`
				Path        string     `json:"path"`
				Count       int        `json:"count"`
				CurrentBest string     `json:"current_best,omitempty"`
			}{Added: entry, Path: path, Count: len(entries), CurrentBest: currentBest}
			return printRowsOrJSON(cmd, flags, view, []string{"Query", "Target", "ZIP", "Path"}, [][]string{{entry.Query, fmt.Sprintf("%.2f", entry.TargetPrice), entry.Zip, path}})
		},
	}
	cmd.Flags().StringVar(&flagTargetPrice, "target-price", "", "Target price that should trigger a future match")
	cmd.Flags().StringVar(&flagZip, "zip", "85001", "ZIP or postal code for this watchlist entry")
	cmd.Flags().StringVar(&flagLocale, "locale", defaultFlippLocale, "Flipp locale, such as en-us or en-ca")
	return cmd
}
