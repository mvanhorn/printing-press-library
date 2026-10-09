// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source computed

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/internal/parks"
)

type parkCoverage struct {
	LiveWaits      string  `json:"live_waits"`
	Hours          *string `json:"hours"`
	HoursKind      string  `json:"hours_kind,omitempty"`
	HoursUnknown   string  `json:"hours_unknown_reason,omitempty"`
	Tickets        *string `json:"tickets"`
	TicketsUnknown string  `json:"tickets_unknown_reason,omitempty"`
}

type parkView struct {
	Key              string       `json:"key"`
	QueueTimesID     int          `json:"queue_times_id"`
	NameEN           string       `json:"name_en"`
	NameJA           string       `json:"name_ja"`
	Aliases          []string     `json:"aliases"`
	Timezone         string       `json:"timezone"`
	OfficialURL      string       `json:"official_url"`
	CrowdCalendarURL string       `json:"crowd_calendar_url"`
	Coverage         parkCoverage `json:"coverage"`
}

func parkToView(p parks.Park) parkView {
	cov := parkCoverage{LiveWaits: parks.SourceQueueTimes}
	if p.HoursSource != "" {
		s := p.HoursSource
		cov.Hours = &s
		if p.HoursSource == parks.SourceThemeParks {
			cov.HoursKind = "third-party aggregator (not official), about one month ahead"
		} else {
			cov.HoursKind = "official, published sale months only"
		}
	} else {
		cov.HoursUnknown = p.HoursUnknownWhy()
	}
	if p.TicketsSource != "" {
		s := p.TicketsSource
		cov.Tickets = &s
	} else {
		cov.TicketsUnknown = p.TicketsUnknownWhy
	}
	return parkView{
		Key: p.Key, QueueTimesID: p.QueueTimesID, NameEN: p.NameEN, NameJA: p.NameJA, Aliases: p.Aliases,
		Timezone: p.Timezone, OfficialURL: p.OfficialURL, CrowdCalendarURL: p.CrowdCalendarURL, Coverage: cov,
	}
}

func newParksCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "parks [park]",
		Short: "List the Japan parks, their ids and which sources cover waits, hours and tickets",
		Long: strings.Trim(`
List the five Japan parks with Queue-Times ids, Japanese and English names,
aliases accepted by other commands, and which source covers live waits, hours
and dated tickets. Missing coverage is shown with a reason, never guessed.

The crowd_calendar_url field is a link to the Queue-Times crowd calendar. This
CLI does not read or reproduce crowd forecasts.`, "\n"),
		Example: strings.Trim(`
  japan-theme-parks-pp-cli parks
  japan-theme-parks-pp-cli parks tds --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "computed", "pp:happy-args": "park=tds"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "parks")
			}
			list := parks.Registry
			if len(args) > 0 {
				p, err := lookupParkArg(args[0])
				if err != nil {
					return err
				}
				list = []parks.Park{p}
			}
			views := make([]parkView, 0, len(list))
			for _, p := range list {
				views = append(views, parkToView(p))
			}
			out := struct {
				Meta    respMeta   `json:"meta"`
				Results []parkView `json:"results"`
			}{Meta: newMeta(), Results: views}
			out.Meta.Sources = append(out.Meta.Sources, sourceRef{
				Name: "built-in registry", Kind: "computed", URL: "https://queue-times.com/parks.json",
				Note: "Park ids from Queue-Times; coverage reflects this CLI's sources.",
			})
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "KEY\tID\tPARK\t名前\tHOURS\tTICKETS")
			for _, v := range views {
				hours, tickets := "unknown", "unknown"
				if v.Coverage.Hours != nil {
					hours = *v.Coverage.Hours
				}
				if v.Coverage.Tickets != nil {
					tickets = *v.Coverage.Tickets
				}
				fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\t%s\n", v.Key, v.QueueTimesID, v.NameEN, v.NameJA, hours, tickets)
			}
			return tw.Flush()
		},
	}
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newParksCmd(flags))
	})
}
