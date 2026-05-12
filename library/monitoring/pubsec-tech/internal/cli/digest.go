// Digest command — transcendence feature 5.
//
// Composes recompete + news-correlation + opportunity-deadline output into a
// single agent-readable bundle, scoped to a saved NAICS profile. Persona:
// Priya's Monday-morning BD ritual that currently takes 2.5 hours.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/monitoring/pubsec-tech/internal/store"
)

func newDigestCmd(flags *rootFlags) *cobra.Command {
	var since string
	var naicsProfile string
	var naicsCSV string
	var limit int
	cmd := &cobra.Command{
		Use:   "digest",
		Short: "Daily/weekly BD digest: new awards, new opportunities, top news, deadlines closing",
		Long: "Single agent-readable bundle composed from the local store: " +
			"(a) recent USAspending awards inside --since, (b) open SAM opportunities " +
			"with response deadlines within the next 14 days, (c) news articles with linked " +
			"contracts/opps, (d) recompete awards ending in the next 18 months. Scoped to a " +
			"NAICS profile saved via `--save-naics` or passed inline via `--naics`.",
		Example:     "  pubsec-tech-pp-cli digest --since 24h --json\n  pubsec-tech-pp-cli digest --since 7d --naics-profile mine --agent\n  pubsec-tech-pp-cli digest --since 24h --naics 541512,541511,541519 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			s, err := openExtrasStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			sinceT, err := parseSince(since)
			if err != nil {
				return usageErr(err)
			}
			naicsCodes := splitAndTrim(naicsCSV)
			if naicsProfile != "" && len(naicsCodes) == 0 {
				if codes, err := loadNaicsProfile(naicsProfile); err == nil {
					naicsCodes = codes
				}
			}
			d, err := buildDigest(ctx, s, sinceT, naicsCodes, limit)
			if err != nil {
				return err
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), d, flags)
			}
			return renderDigest(cmd, d)
		},
	}
	cmd.Flags().StringVar(&since, "since", "24h", "Time window for new awards and news (24h, 7d, etc.)")
	cmd.Flags().StringVar(&naicsProfile, "naics-profile", "", "Named NAICS profile to use (load from config)")
	cmd.Flags().StringVar(&naicsCSV, "naics", "", "Comma-separated NAICS codes (overrides --naics-profile)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max items per section")
	return cmd
}

// Digest is the structured digest response.
type Digest struct {
	GeneratedAt    time.Time             `json:"generated_at"`
	WindowStart    time.Time             `json:"window_start"`
	NAICSCodes     []string              `json:"naics_codes,omitempty"`
	NewAwardCount  int                   `json:"new_award_count"`
	NewArticles    []VendorRollupArticle `json:"new_articles,omitempty"`
	ArticleCount   int                   `json:"article_count"`
	UpcomingExpiry []recompeteRow        `json:"upcoming_recompetes,omitempty"`
	RecompeteCount int                   `json:"recompete_count"`
	OpenOppHits    []VendorRollupOpp     `json:"open_opportunity_hits,omitempty"`
	OppHitCount    int                   `json:"opp_hit_count"`
	Notes          []string              `json:"notes,omitempty"`
}

type recompeteRow struct {
	AwardID       string `json:"award_id"`
	EndDate       string `json:"end_date"`
	RecipientName string `json:"recipient_name,omitempty"`
	NAICS         string `json:"naics,omitempty"`
}

func buildDigest(ctx context.Context, s *store.Store, since time.Time, naicsCodes []string, limit int) (*Digest, error) {
	d := &Digest{GeneratedAt: time.Now().UTC(), WindowStart: since, NAICSCodes: naicsCodes}
	// (a) recent news
	articles, err := s.ArticlesSince(ctx, since, limit)
	if err != nil {
		return nil, err
	}
	for _, a := range articles {
		d.NewArticles = append(d.NewArticles, VendorRollupArticle{
			ID: a.ID, SourceID: a.SourceID, Title: a.Title, Link: a.Link, PublishedAt: a.PublishedAt,
		})
	}
	d.ArticleCount = len(d.NewArticles)
	// (b) recompete window
	in18m := time.Now().AddDate(0, 18, 0)
	awards, _ := s.AwardsRecompete(ctx, in18m, naicsCodes, limit)
	for _, a := range awards {
		var m map[string]any
		if json.Unmarshal(a.Data, &m) != nil {
			continue
		}
		row := recompeteRow{AwardID: a.ID}
		if pop, ok := m["period_of_performance"].(map[string]any); ok {
			if end, _ := pop["end_date"].(string); end != "" {
				row.EndDate = end
			}
		}
		if r, ok := m["recipient"].(map[string]any); ok {
			if n, _ := r["recipient_name"].(string); n != "" {
				row.RecipientName = n
			}
		}
		if n, ok := m["naics"].(map[string]any); ok {
			if c, _ := n["code"].(string); c != "" {
				row.NAICS = c
			}
		}
		d.UpcomingExpiry = append(d.UpcomingExpiry, row)
	}
	d.RecompeteCount = len(d.UpcomingExpiry)
	// (c) opportunity title hits matching any provided NAICS keyword (best-effort)
	for _, c := range naicsCodes {
		if hits, _ := s.OpportunitiesByTitle(ctx, c, 5); len(hits) > 0 {
			for _, h := range hits {
				vo := VendorRollupOpp{ID: h.ID}
				var m map[string]any
				if json.Unmarshal(h.Data, &m) == nil {
					if t, _ := m["title"].(string); t != "" {
						vo.Title = t
					}
				}
				d.OpenOppHits = append(d.OpenOppHits, vo)
			}
		}
	}
	d.OppHitCount = len(d.OpenOppHits)
	if d.ArticleCount == 0 && d.RecompeteCount == 0 && d.OppHitCount == 0 {
		d.Notes = append(d.Notes, "no items in window; run `sync` and `news sync` to populate the local store")
	} else {
		// Partial-empty signaling: explain why a section may be empty even when
		// others have data. Surfaces the sync prerequisites users actually hit.
		if d.RecompeteCount == 0 {
			d.Notes = append(d.Notes, "no awards in window; run `sync --resources awards` to populate the recompete section")
		}
		if d.OppHitCount == 0 && len(naicsCodes) > 0 {
			d.Notes = append(d.Notes, "no opportunity hits for the chosen NAICS codes; run `sync --resources opportunities` (needs DATA_GOV_API_KEY) to populate the opps section")
		}
		if d.ArticleCount == 0 {
			d.Notes = append(d.Notes, "no recent articles; run `news sync` to populate the news section")
		}
	}
	// Per Phase 4.85 agentic output review: surface enabled news sources
	// that contributed zero articles in the window so a reader can
	// distinguish "no recent articles" from "never synced" or "sync failed".
	// Suppressed when the news section is wholly empty (a single broader
	// note above already covers that case).
	if d.ArticleCount > 0 {
		contributing := make(map[string]struct{}, len(d.NewArticles))
		for _, a := range d.NewArticles {
			if a.SourceID != "" {
				contributing[a.SourceID] = struct{}{}
			}
		}
		d.Notes = append(d.Notes, silentZeroSourceNotes(ctx, s, contributing, since)...)
	}
	return d, nil
}

func renderDigest(cmd *cobra.Command, d *Digest) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "pubsec-tech digest — window since %s\n", d.WindowStart.Format("2006-01-02 15:04"))
	if len(d.NAICSCodes) > 0 {
		fmt.Fprintf(w, "NAICS profile: %s\n", strings.Join(d.NAICSCodes, ", "))
	}
	fmt.Fprintf(w, "\nNew articles (%d):\n", d.ArticleCount)
	for _, a := range d.NewArticles {
		date := ""
		if !a.PublishedAt.IsZero() {
			date = a.PublishedAt.Format("01-02")
		}
		fmt.Fprintf(w, "  - %s  [%s]  %s\n", date, a.SourceID, truncate(a.Title, 100))
	}
	fmt.Fprintf(w, "\nUpcoming recompetes (%d):\n", d.RecompeteCount)
	for _, r := range d.UpcomingExpiry {
		fmt.Fprintf(w, "  - %s  NAICS %s  %s\n", r.EndDate, r.NAICS, truncate(r.RecipientName, 60))
	}
	fmt.Fprintf(w, "\nOpportunity hits (%d):\n", d.OppHitCount)
	for _, o := range d.OpenOppHits {
		fmt.Fprintf(w, "  - %s  %s\n", o.ID, truncate(o.Title, 100))
	}
	for _, n := range d.Notes {
		fmt.Fprintf(w, "\n• %s\n", n)
	}
	return nil
}

// loadNaicsProfile reads a named NAICS list from ~/.config/pubsec-tech/profiles/<name>.txt
// Each non-empty non-comment line is one NAICS code.
func loadNaicsProfile(name string) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, ".config", "pubsec-tech", "profiles", name+".txt")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out, nil
}
