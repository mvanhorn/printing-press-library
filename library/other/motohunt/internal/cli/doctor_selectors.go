// Copyright 2026 richardadonnell. Licensed under Apache-2.0. See LICENSE.
// Hand-written: doctor selector-health probe for the scrape commands.

package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/mvanhorn/printing-press-library/library/other/motohunt/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/motohunt/internal/motohunt"
)

// collectSelectorReport fetches a known search on each site, parses the cards,
// and asserts >=1 card with a non-empty title+id. PASS when the selectors still
// match live HTML; WARN when they drift (scrape commands would return empty).
// Short-circuits to "skipped" under the verify env so a mock run stays offline.
func collectSelectorReport(ctx context.Context, flags *rootFlags) map[string]any {
	report := map[string]any{}
	if cliutil.IsVerifyEnv() {
		report["status"] = "skipped"
		report["reason"] = "verify env"
		return report
	}
	client := scrapeClient(flags)

	overall := "pass"
	probes := make([]map[string]any, 0, 2)
	for _, site := range []string{"moto", "atv"} {
		cfg, err := motohunt.ResolveSite(site)
		if err != nil {
			continue
		}
		// Known query: Harley-Davidson near 33705 on moto; bare location on atv.
		params := motohunt.SearchParams{Location: "33705"}
		if site == "moto" {
			params.Make = "Harley-Davidson"
		}
		url, _, _ := cfg.BuildSearchURL(params)
		p := map[string]any{"site": site, "url": url}
		doc, ferr := client.Fetch(ctx, url)
		if ferr != nil {
			p["status"] = "warn"
			p["error"] = ferr.Error()
			overall = "warn"
			probes = append(probes, p)
			continue
		}
		cards := motohunt.ParseCards(doc, cfg)
		good := 0
		for _, c := range cards {
			if c.Title != "" && c.ID != "" {
				good++
			}
		}
		p["cards"] = len(cards)
		p["cards_with_id_and_title"] = good
		if good >= 1 {
			p["status"] = "pass"
		} else {
			p["status"] = "warn"
			p["hint"] = "0 cards parsed with id+title — selectors may have drifted"
			overall = "warn"
		}
		probes = append(probes, p)
	}
	report["status"] = overall
	report["probes"] = probes
	return report
}

func renderSelectorReport(w io.Writer, rep map[string]any) {
	status, _ := rep["status"].(string)
	indicator := green("PASS")
	switch status {
	case "warn":
		indicator = yellow("WARN")
	case "skipped":
		indicator = yellow("INFO")
	}
	fmt.Fprintf(w, "  %s Selectors: %s\n", indicator, status)
	if probesAny, ok := rep["probes"]; ok {
		if probes, ok := probesAny.([]map[string]any); ok {
			for _, p := range probes {
				site, _ := p["site"].(string)
				st, _ := p["status"].(string)
				if cards, ok := p["cards"]; ok {
					good := p["cards_with_id_and_title"]
					fmt.Fprintf(w, "      - %s: %s (%v cards, %v with id+title)\n", site, st, cards, good)
				} else {
					fmt.Fprintf(w, "      - %s: %s\n", site, st)
				}
			}
		}
	}
}
