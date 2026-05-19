// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.
// Hand-written by the Printing Press operator on top of generated scaffolding.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newCitationCmd(flags *rootFlags) *cobra.Command {
	var (
		style, dbPath string
	)
	cmd := &cobra.Command{
		Use:   "citation [nasa_id]",
		Short: "Generate a ready-to-paste citation string from cached or fetched metadata",
		Long: `Format an attribution-clean citation string in APA, MLA, or Chicago style.
Uses the local mirror first; on miss, fetches /asset/{nasa_id} and its
metadata sidecar so the command works without a prior 'mirror search'.`,
		Example:     "  nasa-images-pp-cli citation PIA24439 --style apa",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			nasaID := args[0]
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would generate %s citation for %q\n", style, nasaID)
				return nil
			}
			styleLower := strings.ToLower(strings.TrimSpace(style))
			switch styleLower {
			case "apa", "mla", "chicago":
			default:
				return fmt.Errorf("invalid --style %q: must be apa, mla, or chicago", style)
			}

			ctx := cmd.Context()
			s, err := openNasaStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer s.Close()

			asset, source, err := loadOrFetchAsset(ctx, s.DB(), flags, nasaID)
			if err != nil {
				return err
			}

			cit := formatCitation(styleLower, nasaID, asset)
			result := map[string]any{
				"nasa_id":  nasaID,
				"style":    styleLower,
				"citation": cit,
				"source":   source,
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.quiet && !flags.plain) {
				return flags.printJSON(cmd, result)
			}
			fmt.Fprintln(cmd.OutOrStdout(), cit)
			return nil
		},
	}
	cmd.Flags().StringVar(&style, "style", "apa", "Citation style: apa, mla, or chicago")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/nasa-images-pp-cli/data.db)")
	return cmd
}

// loadOrFetchAsset returns the asset metadata for nasa_id from the local mirror
// if present, else fetches it via /search?nasa_id=<id> and returns the first item.
// source is "local" or "live".
func loadOrFetchAsset(ctx context.Context, db *sql.DB, flags *rootFlags, nasaID string) (nasaAssetData, string, error) {
	var data []byte
	err := db.QueryRowContext(ctx,
		`SELECT data FROM resources WHERE resource_type='asset' AND id = ?`, nasaID).Scan(&data)
	if err == nil && len(data) > 0 {
		var a nasaAssetData
		if jerr := json.Unmarshal(data, &a); jerr == nil && a.NasaID != "" {
			return a, "local", nil
		}
	}
	// Fall back to live API.
	c, cerr := flags.newClient()
	if cerr != nil {
		return nasaAssetData{}, "", cerr
	}
	raw, gerr := c.Get("/search", map[string]string{"nasa_id": nasaID})
	if gerr != nil {
		return nasaAssetData{}, "", fmt.Errorf("fetching asset %q: %w", nasaID, gerr)
	}
	coll, perr := parseNasaCollection(raw)
	if perr != nil {
		return nasaAssetData{}, "", perr
	}
	// NASA's /search treats nasa_id as a filter, not an exact-match
	// lookup — verify the returned item's NasaID matches what we asked
	// for, in case the upstream ever does fuzzy matching.
	for _, item := range coll.Collection.Items {
		if len(item.Data) == 0 {
			continue
		}
		if item.Data[0].NasaID == nasaID {
			return item.Data[0], "live", nil
		}
	}
	return nasaAssetData{}, "", fmt.Errorf("asset %q not found", nasaID)
}

func formatCitation(style, nasaID string, a nasaAssetData) string {
	author := strings.TrimSpace(a.Photographer)
	if author == "" {
		author = strings.TrimSpace(a.SecondaryCreator)
	}
	if author == "" {
		author = "NASA"
		if a.Center != "" {
			if name, ok := nasaCenters[a.Center]; ok {
				author = "NASA " + name
			}
		}
	}
	year := citationYear(a.DateCreated)
	title := strings.TrimSpace(a.Title)
	if title == "" {
		title = nasaID
	}
	url := fmt.Sprintf("https://images.nasa.gov/details/%s", nasaID)
	switch style {
	case "mla":
		// Author. "Title." NASA Image and Video Library, Year, URL.
		return fmt.Sprintf("%s. \"%s.\" NASA Image and Video Library, %s, %s.",
			author, title, year, url)
	case "chicago":
		// Author. Year. "Title." NASA Image and Video Library. URL.
		return fmt.Sprintf("%s. %s. \"%s.\" NASA Image and Video Library. %s.",
			author, year, title, url)
	default: // apa
		// Author. (Year). Title [NASA ID]. NASA Image and Video Library. URL
		return fmt.Sprintf("%s. (%s). %s [%s]. NASA Image and Video Library. %s",
			author, year, title, nasaID, url)
	}
}

func citationYear(dateCreated string) string {
	if dateCreated == "" {
		return "n.d."
	}
	if t, err := time.Parse(time.RFC3339, dateCreated); err == nil {
		return fmt.Sprintf("%d", t.Year())
	}
	if t, err := time.Parse("2006-01-02", dateCreated); err == nil {
		return fmt.Sprintf("%d", t.Year())
	}
	if len(dateCreated) >= 4 {
		return dateCreated[:4]
	}
	return "n.d."
}
