// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/airport-limousine/internal/limousine"
	"github.com/spf13/cobra"
	"strings"
)

func newNovelConditionsCmd(flags *rootFlags) *cobra.Command {
	var topic string
	var limit int
	cmd := &cobra.Command{Use: "conditions", Short: "Read concise baggage, child-fare and boarding conditions with source freshness", Example: "  airport-limousine-pp-cli conditions --topic baggage --agent", Annotations: limousineAnnotations("--topic=baggage"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "conditions")
		}
		if e := limousineLive(flags); e != nil {
			return e
		}
		if len(args) != 0 {
			return usageErr(fmt.Errorf("conditions takes --topic, not positional arguments"))
		}
		if topic != "all" && topic != "baggage" && topic != "boarding" {
			return usageErr(fmt.Errorf("--topic must be baggage, boarding or all"))
		}
		if e := limousineLimit(limit); e != nil {
			return e
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		p := limousine.New(flags.timeout, flags.rateLimit)
		rows := []limousine.Condition{}
		sources := []string{}
		notices := []map[string]string{}
		for _, item := range []struct{ topic, path string }{{"baggage", "/en/guide/terms/baggage"}, {"boarding", "/en/guide/terms/caution"}} {
			if topic != "all" && topic != item.topic {
				continue
			}
			data, e := p.Data(ctx, limousine.DataPath(item.path, nil))
			if e != nil {
				return limousineError(e)
			}
			facts, e := limousine.Conditions(data, item.topic)
			if e != nil {
				return limousineError(e)
			}
			rows = append(rows, facts...)
			sources = append(sources, limousine.PageURL(item.path, nil))
			content := limousine.M(data["content"])
			links, e := limousine.SourceLinks(limousine.S(content["content"]))
			if e != nil {
				return apiErr(e)
			}
			for _, l := range links {
				if strings.Contains(l["url"], ".pdf") {
					notices = append(notices, l)
				}
			}
		}
		if topic == "baggage" || topic == "all" {
			jpData, e := p.Data(ctx, "/ja/guide/terms/baggage/__data.json")
			if e != nil {
				return limousineError(e)
			}
			jpContent := limousine.M(jpData["content"])
			jpFacts := limousine.JapaneseBaggage(limousine.S(jpContent["content"]))
			for i := range jpFacts {
				jpFacts[i].SourceUpdatedAt = limousine.S(jpContent["updatedAt"])
			}
			rows = append(jpFacts, rows...)
			sources = append(sources, limousine.Origin+"/ja/guide/terms/baggage/")
			body, e := p.Fetch(ctx, "GET", "/en/", "")
			if e != nil {
				return limousineError(e)
			}
			sources = append(sources, limousine.Origin+"/en/")
			links, e := limousine.SourceLinks(string(body))
			if e != nil {
				return apiErr(e)
			}
			for _, l := range links {
				if strings.Contains(strings.ToLower(l["title"]), "baggage") && len(notices) < 6 {
					notices = append(notices, l)
				}
			}
		}
		total := len(rows)
		if total > limit {
			rows = rows[:limit]
		}
		return flags.printJSON(cmd, limousine.Envelope{Meta: p.Meta(sources, total, total, len(rows), "Facts are extracted from the current provider guide; its content update date can predate this observation. Linked baggage notices may contain further restrictions, including special items. Partner operators can differ. Read canonical conditions before travel."), Results: rows, Details: map[string]any{"linked_notices_and_guides": notices}})
	}}
	cmd.Flags().StringVar(&topic, "topic", "all", "Guide topic: baggage, boarding or all")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum recognized facts returned, from 1 to 200")
	return cmd
}
