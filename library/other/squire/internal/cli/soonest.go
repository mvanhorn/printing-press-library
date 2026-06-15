// Copyright 2026 Dev Basu and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored transcendence command (Phase 3). Safe to edit.
// pp:data-source live

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type soonestEntry struct {
	Shop              string `json:"shop"`
	Barber            string `json:"barber"`
	NextAvailableTime string `json:"next_available_time"`
	NextAvailableText string `json:"next_available_text"`
	DefaultService    string `json:"default_service"`
}

type soonestResult struct {
	ShopsQueried  int            `json:"shops_queried"`
	Soonest       []soonestEntry `json:"soonest"`
	FetchFailures []fetchFailure `json:"fetch_failures"`
}

type fetchFailure struct {
	Shop  string `json:"shop"`
	Error string `json:"error"`
}

func parseShopList(near string, args []string) []string {
	out := make([]string, 0)
	seen := map[string]bool{}
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, a := range args {
		add(a)
	}
	for _, p := range strings.Split(near, ",") {
		add(p)
	}
	return out
}

func newNovelSoonestCmd(flags *rootFlags) *cobra.Command {
	var flagNear string
	var flagService string

	cmd := &cobra.Command{
		Use:   "soonest [shop...]",
		Short: "Find the barber who can cut your hair soonest across several shops, ranked by next open slot.",
		Example: strings.Trim(`
  squire-pp-cli soonest barber-theory-toronto another-shop-route
  squire-pp-cli soonest --near barber-theory-toronto,another-shop --service Haircut --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			shops := parseShopList(flagNear, args)
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would rank next-available barbers across %d shop(s)\n", len(shops))
				return nil
			}
			if len(shops) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("provide at least one shop (positional or --near)"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			entries := make([]soonestEntry, 0)
			failures := make([]fetchFailure, 0)
			term := strings.ToLower(strings.TrimSpace(flagService))
			for _, shop := range shops {
				pros, err := fetchProfessionals(ctx, c, shop)
				if err != nil {
					failures = append(failures, fetchFailure{Shop: shop, Error: err.Error()})
					continue
				}
				for _, p := range pros {
					next := sqString(p, "nextAvailableTime")
					if next == "" {
						continue
					}
					b := sqMap(p, "barber")
					name := strings.TrimSpace(sqString(b, "firstName") + " " + sqString(b, "lastName"))
					def := sqString(p, "defaultServiceName")
					if term != "" && !strings.Contains(strings.ToLower(def), term) {
						continue
					}
					entries = append(entries, soonestEntry{
						Shop:              shop,
						Barber:            name,
						NextAvailableTime: next,
						NextAvailableText: sqString(p, "nextAvailableTimeText"),
						DefaultService:    def,
					})
				}
			}
			sort.SliceStable(entries, func(i, j int) bool {
				ti, ei := time.Parse(time.RFC3339, entries[i].NextAvailableTime)
				tj, ej := time.Parse(time.RFC3339, entries[j].NextAvailableTime)
				if ei != nil || ej != nil {
					return entries[i].NextAvailableTime < entries[j].NextAvailableTime
				}
				return ti.Before(tj)
			})
			if len(entries) == 0 && len(failures) == len(shops) {
				return apiErr(fmt.Errorf("all %d shop fetches failed", len(shops)))
			}
			return printJSONFiltered(cmd.OutOrStdout(), soonestResult{
				ShopsQueried:  len(shops),
				Soonest:       entries,
				FetchFailures: failures,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&flagNear, "near", "", "Comma-separated shop slugs or UUIDs to scan")
	cmd.Flags().StringVar(&flagService, "service", "", "Only barbers whose default service name contains this term")
	return cmd
}
