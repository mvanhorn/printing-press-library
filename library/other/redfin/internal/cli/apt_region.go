// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/redfin/internal/redfin"

	"github.com/spf13/cobra"
)

func newRegionCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "region",
		Short:       "Resolve and look up Redfin region IDs.",
		Long:        `Subcommands for translating region queries to (region_id, region_type) pairs and back.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newRegionResolveCmd(flags))
	cmd.AddCommand(newRegionLookupCmd(flags))
	return cmd
}

func newRegionResolveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolve <query>",
		Short: "Resolve a region name to its (region_id, region_type) pair.",
		Long: `Calls Stingray's autocomplete to translate a free-text region name to
its canonical Redfin region_id + region_type. When autocomplete is
unavailable (403) the command suggests pasting the region URL slug directly.`,
		Example:     `  redfin-pp-cli region resolve "Austin, TX"`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.Join(args, " ")
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, gerr := c.Get("/stingray/do/location-autocomplete", map[string]string{
				"location": query, "v": "2",
			})
			if gerr != nil {
				return apiErr(fmt.Errorf("%w\nhint: autocomplete blocked. Paste the region URL slug instead, e.g. 'city/30772/TX/Austin'", gerr))
			}
			data = redfin.StripStingrayPrefix(data)
			var env struct {
				Payload struct {
					Sections []struct {
						Rows []struct {
							ID       json.RawMessage `json:"id"`
							Name     string          `json:"name"`
							Type     int             `json:"type"`
							URL      string          `json:"url"`
							Subtitle string          `json:"subtitle"`
						} `json:"rows"`
					} `json:"sections"`
					Exact json.RawMessage `json:"exactMatch"`
				} `json:"payload"`
			}
			if err := json.Unmarshal(data, &env); err != nil {
				return apiErr(fmt.Errorf("decoding autocomplete: %w", err))
			}
			type hit struct {
				Name       string `json:"name"`
				Subtitle   string `json:"subtitle,omitempty"`
				URL        string `json:"url,omitempty"`
				RegionID   string `json:"region_id"`
				RegionType int    `json:"region_type"`
			}
			var hits []hit
			for _, sec := range env.Payload.Sections {
				for _, r := range sec.Rows {
					hits = append(hits, hit{
						Name:       r.Name,
						Subtitle:   r.Subtitle,
						URL:        r.URL,
						RegionID:   string(r.ID),
						RegionType: r.Type,
					})
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), hits, flags)
		},
	}
	return cmd
}

func newRegionLookupCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lookup <region-id-or-slug>",
		Short: "Look up a region's name from local cache (or fall back to live).",
		Long: `Reads the regions cache populated by past sync-search calls. When the
region isn't cached, makes a single homes search to populate basic metadata
and returns whatever the cache now holds.`,
		Example:     `  redfin-pp-cli region lookup 30772:6`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id, typ, err := parseRegionSlug(args[0])
			if err != nil {
				return usageErr(err)
			}
			s, err := openRedfinStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			name, state, ok, err := redfin.LookupRegion(s.DB(), id, typ)
			if err != nil {
				return err
			}
			out := map[string]any{
				"region_id":   id,
				"region_type": typ,
				"name":        name,
				"state":       state,
				"cached":      ok,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}
