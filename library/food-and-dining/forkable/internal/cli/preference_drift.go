// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
//
// preference-drift: flag served meals that conflict with the user's stated
// dietary dislikes/restrictions (or miss likes). Live GraphQL fetch of `me`
// (preferences) + `myDeliveries` (served pieces), mechanical set-membership
// match. Hand-authored; preserved across generate --force.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

const mePreferencesQuery = `query { me { id fullName likes dislikes restrictions } }`

type mePreferences struct {
	ID           int64    `json:"id"`
	FullName     string   `json:"fullName"`
	Likes        []string `json:"likes"`
	Dislikes     []string `json:"dislikes"`
	Restrictions []string `json:"restrictions"`
}

type driftHit struct {
	Date        string   `json:"date"`
	Venue       string   `json:"venue"`
	Meal        string   `json:"meal"`
	Diner       string   `json:"diner"`
	Conflicts   []string `json:"conflicts"`
	MatchedType string   `json:"matched_type"` // "dislike" | "restriction"
}

type driftView struct {
	Hits         []driftHit `json:"hits"`
	Count        int        `json:"count"`
	Since        string     `json:"since,omitempty"`
	Dislikes     []string   `json:"dislikes"`
	Restrictions []string   `json:"restrictions"`
	MealsChecked int        `json:"meals_checked"`
}

func fetchMePreferences(cmd *cobra.Command, flags *rootFlags) (mePreferences, error) {
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	data, err := fetchGraphQL(ctx, flags, mePreferencesQuery)
	if err != nil {
		return mePreferences{}, err
	}
	var wrap struct {
		Me mePreferences `json:"me"`
	}
	if err := json.Unmarshal(data, &wrap); err != nil {
		return mePreferences{}, fmt.Errorf("parsing me preferences: %w", err)
	}
	return wrap.Me, nil
}

// conflictTerms returns the lowercased preference terms that appear as
// substrings in the (lowercased) meal name.
func conflictTerms(mealName string, terms []string) []string {
	lc := strings.ToLower(mealName)
	var hits []string
	for _, t := range terms {
		tl := strings.ToLower(strings.TrimSpace(t))
		if tl == "" {
			continue
		}
		if strings.Contains(lc, tl) {
			hits = append(hits, t)
		}
	}
	return hits
}

func newNovelPreferenceDriftCmd(flags *rootFlags) *cobra.Command {
	var flagSince string

	cmd := &cobra.Command{
		Use:         "preference-drift",
		Short:       "Flag served meals that violate your stated dislikes or dietary restrictions, or miss your likes.",
		Long:        "Cross-checks the meals actually served to you against the dietary dislikes and restrictions on your Forkable profile. Both sides are fetched live; the match is a mechanical name-substring check, so review flagged rows before acting on them.",
		Example:     "  forkable-pp-cli preference-drift --since 60d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				emitDryRunShortCircuit(cmd, flags, "compare served meals against stated preferences")
				return nil
			}
			prefs, err := fetchMePreferences(cmd, flags)
			if err != nil {
				return err
			}
			deliveries, err := fetchDeliveries(cmd, flags, servedHistoryQuery)
			if err != nil {
				return err
			}
			cutoff := sinceCutoffISO(flagSince)
			hits := make([]driftHit, 0)
			checked := 0
			for _, d := range deliveries {
				for _, o := range d.Orders {
					when := o.ForDeliveryAt
					if when == "" {
						when = d.ForDeliveryAt
					}
					if !dateOnOrAfter(when, cutoff) {
						continue
					}
					for _, p := range o.Pieces {
						checked++
						if rc := conflictTerms(p.Name, prefs.Restrictions); len(rc) > 0 {
							hits = append(hits, driftHit{Date: when, Venue: o.Venue.label(), Meal: p.Name, Diner: p.UserName, Conflicts: rc, MatchedType: "restriction"})
							continue
						}
						if dc := conflictTerms(p.Name, prefs.Dislikes); len(dc) > 0 {
							hits = append(hits, driftHit{Date: when, Venue: o.Venue.label(), Meal: p.Name, Diner: p.UserName, Conflicts: dc, MatchedType: "dislike"})
						}
					}
				}
			}
			view := driftView{
				Hits:         hits,
				Count:        len(hits),
				Since:        cutoff,
				Dislikes:     prefs.Dislikes,
				Restrictions: prefs.Restrictions,
				MealsChecked: checked,
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(hits) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No preference conflicts in %d served meals.\n", checked)
				return nil
			}
			for _, h := range hits {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%-24s\t%-28s\t[%s: %s]\n", h.Date[:min(10, len(h.Date))], h.Venue, h.Meal, h.MatchedType, strings.Join(h.Conflicts, ", "))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "Only check meals on or after this window (e.g. 60d, 8w)")
	return cmd
}
