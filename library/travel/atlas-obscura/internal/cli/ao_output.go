// Copyright 2026 David Bryson and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Shared agent-native output helpers for the hand-authored Atlas Obscura commands.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// aoEmit renders any envelope via the standard flag-aware JSON pipeline
// (honors --json, --select, --compact, --csv, --quiet) or a compact human
// table of places when writing to a terminal without --json.
func aoEmitPlaces(cmd *cobra.Command, flags *rootFlags, meta map[string]any, places []AOPlace) error {
	env := map[string]any{
		"source":  aoSourceNote,
		"count":   len(places),
		"results": places,
	}
	for k, v := range meta {
		env[k] = v
	}
	if !flags.asJSON && wantsHumanTable(cmd.OutOrStdout(), flags) {
		printPlacesTable(cmd, places)
		return nil
	}
	return printJSONFiltered(cmd.OutOrStdout(), env, flags)
}

func aoEmit(cmd *cobra.Command, flags *rootFlags, v any) error {
	return printJSONFiltered(cmd.OutOrStdout(), v, flags)
}

func printPlacesTable(cmd *cobra.Command, places []AOPlace) {
	w := cmd.OutOrStdout()
	if len(places) == 0 {
		fmt.Fprintln(w, "No places found.")
		return
	}
	rows := make([]map[string]any, 0, len(places))
	for _, p := range places {
		row := map[string]any{
			"title":    p.Title,
			"location": p.Location,
			"url":      p.URL,
		}
		if p.DistanceFromQuery != "" {
			row["mi"] = p.DistanceFromQuery
		}
		if p.Score > 0 {
			row["score"] = p.Score
		}
		rows = append(rows, row)
	}
	_ = printAutoTable(w, rows)
	fmt.Fprintf(cmd.ErrOrStderr(), "\n%s\n", aoSourceNote)
}
