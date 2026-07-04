// Copyright 2026 richardadonnell and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature (NOT generated).
// pp:data-source live

package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

type icsResult struct {
	File          string `json:"file,omitempty"`
	EventsWritten int    `json:"events_written"`
	Filter        string `json:"filter"`
}

func newNovelIcsCmd(flags *rootFlags) *cobra.Command {
	var flagCity string
	var flagPlaceID string
	var flagCategory string
	var flagWindow string
	var flagOut string
	var flagLimit int
	var flagMaxScanPages int

	cmd := &cobra.Command{
		Use:   "ics",
		Short: "Export a filtered set of events to a .ics calendar file your calendar app can import.",
		Long: "Fetch events for a city, place, or category and serialize them to an RFC 5545 calendar.\n" +
			"Writes to --out when given, otherwise prints the .ics to stdout. The public API returns\n" +
			"JSON only, so this is the calendar export it does not offer.",
		Example:     "  luma-pp-cli ics --city sf --window 30d --out sf.ics",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch events and write an .ics calendar")
				return nil
			}
			if flagCity == "" && flagPlaceID == "" && flagCategory == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("ics needs one of --city, --place-id, or --category"))
			}
			window, err := parseWindow(flagWindow)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --window %q: %w", flagWindow, err))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			params := map[string]string{}
			filter := ""
			switch {
			case flagCity != "":
				params["slug"] = flagCity
				filter = "city:" + flagCity
			case flagPlaceID != "":
				params["discover_place_api_id"] = flagPlaceID
				filter = "place:" + flagPlaceID
			case flagCategory != "":
				params["category_api_id"] = flagCategory
				filter = "category:" + flagCategory
			}

			// Page size is the upstream fetch batch; it is intentionally decoupled
			// from --limit (the output cap applied after window filtering below).
			const icsPageSize = 50
			entries, ferr := fetchEventEntries(ctx, c, params, icsPageSize, scanPagesForEnv(flagMaxScanPages))
			if ferr != nil && len(entries) == 0 {
				return classifyAPIError(ferr, flags)
			}
			entries = dedupeByID(entries)
			now := time.Now()
			kept := make([]lumaEntry, 0, len(entries))
			for _, e := range entries {
				// An event with no parseable start_at cannot become a VEVENT
				// (DTSTART is required by RFC 5545), so it is excluded by design.
				t, ok := e.startTime()
				if !ok {
					continue
				}
				if !withinWindow(t, now, window) {
					continue
				}
				kept = append(kept, e)
			}
			if flagLimit > 0 && len(kept) > flagLimit {
				kept = kept[:flagLimit]
			}

			ics := buildICS(kept, now)
			if flagOut == "" {
				_, err := cmd.OutOrStdout().Write([]byte(ics))
				return err
			}
			if err := os.WriteFile(flagOut, []byte(ics), 0o600); err != nil {
				return fmt.Errorf("writing %s: %w", flagOut, err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), icsResult{File: flagOut, EventsWritten: len(kept), Filter: filter}, flags)
		},
	}
	cmd.Flags().StringVar(&flagCity, "city", "", "City slug to export, e.g. sf")
	cmd.Flags().StringVar(&flagPlaceID, "place-id", "", "Place api_id to export (alternative to --city)")
	cmd.Flags().StringVar(&flagCategory, "category", "", "Category api_id to export, e.g. cat-ai")
	cmd.Flags().StringVar(&flagWindow, "window", "", "Only events within this window from now (e.g. 30d); empty = all upcoming")
	cmd.Flags().StringVar(&flagOut, "out", "", "Output .ics file path; empty writes to stdout")
	cmd.Flags().IntVar(&flagLimit, "limit", 100, "Max events to include")
	cmd.Flags().IntVar(&flagMaxScanPages, "max-scan-pages", 5, "Max pages to fetch before stopping")
	return cmd
}
