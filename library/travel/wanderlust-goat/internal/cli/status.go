package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/closedsignal"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/dispatch"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/googleplaces"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/regions"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/sourcetypes"
)

// newStatusCmd is the new v2 command: explicit operational/closed lookup
// across every source for one named place. Surfaces conflicting signals
// rather than averaging them.
func newStatusCmd(flags *rootFlags) *cobra.Command {
	var (
		anchorFlag string
		country    string
	)
	cmd := &cobra.Command{
		Use:   "status [place-name]",
		Short: "Explicit operational/closed lookup across every Stage-2 source for one place",
		Long: `status fans out by name to every Stage-2 source for the resolved country (or
the user-supplied --country) and every relevant detector in internal/closedsignal/.
Returns a per-source verdict so conflicting signals are explicit:
Google says OPERATIONAL but Tabelog has 閉店 in the page body, etc.

Useful before recommending a place — restaurant data is famously stale.`,
		Example: strings.Trim(`
  wanderlust-goat-pp-cli status "Sushi Saito" --json
  wanderlust-goat-pp-cli status "Le Coucou" --country FR --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(strings.Join(args, " "))
			if name == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			cc := strings.ToUpper(strings.TrimSpace(country))
			if cc == "" && anchorFlag != "" {
				if r, err := dispatch.ResolveAnchor(ctx, anchorFlag); err == nil {
					cc = r.Country
				}
			}
			if cc == "" {
				cc = "*"
			}
			region := regions.Lookup(cc)

			result := StatusResult{Name: name, Country: cc, Region: region}

			// Google Places business_status path (only when an API key is
			// available). Never fail status when the seed isn't reachable —
			// other sources still have signal.
			if g, gerr := googleplaces.NewClient(); gerr == nil {
				if places, perr := g.SearchText(ctx, name, 0, 0, 0, 5, region.PrimaryLanguage); perr == nil && len(places) > 0 {
					top := places[0]
					result.Google = &GoogleSnapshot{
						Name:           top.DisplayName,
						BusinessStatus: top.BusinessStatus,
						Address:        top.Address,
						Lat:            top.Lat,
						Lng:            top.Lng,
					}
					result.Verdicts = append(result.Verdicts, closedsignal.CheckGoogleBusinessStatus(top.BusinessStatus))
				} else if perr != nil {
					result.Notes = append(result.Notes, "google.places: "+perr.Error())
				}
			} else if errors.Is(gerr, googleplaces.ErrMissingAPIKey) {
				result.Notes = append(result.Notes, "google.places: skipped (GOOGLE_PLACES_API_KEY not set)")
			}

			// Stage-2 sources for the country: parallel lookup + closed check.
			reg := dispatch.DefaultRegistry()
			var wg sync.WaitGroup
			var mu sync.Mutex
			for _, slug := range region.LocalReviewSites {
				cli := reg.Get(slug)
				if cli == nil {
					mu.Lock()
					result.Notes = append(result.Notes, slug+": not registered")
					mu.Unlock()
					continue
				}
				wg.Add(1)
				go func(slug string, cli sourcetypes.Client) {
					defer wg.Done()
					hits, err := cli.LookupByName(ctx, name, "", 1)
					mu.Lock()
					defer mu.Unlock()
					if err != nil {
						if errors.Is(err, sourcetypes.ErrNotImplemented) {
							result.StubsSkipped = append(result.StubsSkipped, SourceStub{Slug: slug, Reason: sourcetypes.StubReason(cli)})
						} else {
							result.Notes = append(result.Notes, slug+": "+err.Error())
						}
						return
					}
					if len(hits) == 0 {
						result.NoHits = append(result.NoHits, slug)
						return
					}
					result.Hits = append(result.Hits, SourceHit{Slug: slug, Hit: hits[0]})
					if checker, ok := cli.(interface {
						CheckClosed(ctx context.Context, h sourcetypes.Hit) closedsignal.Verdict
					}); ok {
						v := checker.CheckClosed(ctx, hits[0])
						if v.Closed || v.Temporary {
							result.Verdicts = append(result.Verdicts, v)
						}
					}
				}(slug, cli)
			}
			wg.Wait()

			result.Combined = closedsignal.Combine(result.Verdicts)
			result.Conflict = hasConflict(result.Verdicts)

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			renderStatus(cmd, result)
			return nil
		},
	}
	cmd.Flags().StringVar(&anchorFlag, "anchor", "", "anchor (used to derive country if --country not set)")
	cmd.Flags().StringVar(&country, "country", "", "ISO 3166-1 alpha-2 country code")
	return cmd
}

// StatusResult captures the per-source closed-signal verdict for one
// place. Exposed via JSON.
type StatusResult struct {
	Name         string                 `json:"name"`
	Country      string                 `json:"country"`
	Region       regions.Region         `json:"region"`
	Google       *GoogleSnapshot        `json:"google,omitempty"`
	Hits         []SourceHit            `json:"hits"`
	NoHits       []string               `json:"no_hits,omitempty"`
	StubsSkipped []SourceStub           `json:"stubs_skipped,omitempty"`
	Verdicts     []closedsignal.Verdict `json:"verdicts"`
	Combined     closedsignal.Verdict   `json:"combined"`
	Conflict     bool                   `json:"conflict"`
	Notes        []string               `json:"notes,omitempty"`
}

type GoogleSnapshot struct {
	Name           string  `json:"name"`
	BusinessStatus string  `json:"business_status"`
	Address        string  `json:"address"`
	Lat            float64 `json:"lat"`
	Lng            float64 `json:"lng"`
}

type SourceHit struct {
	Slug string          `json:"slug"`
	Hit  sourcetypes.Hit `json:"hit"`
}

type SourceStub struct {
	Slug   string `json:"slug"`
	Reason string `json:"reason"`
}

func hasConflict(verdicts []closedsignal.Verdict) bool {
	hasOpen := false
	hasClosed := false
	for _, v := range verdicts {
		if v.Closed {
			hasClosed = true
		} else if !v.Temporary {
			hasOpen = true
		}
	}
	return hasOpen && hasClosed
}

func renderStatus(cmd *cobra.Command, r StatusResult) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s\nstatus for %q (country %s):\n", bold("wanderlust-goat"), r.Name, r.Country)
	switch {
	case r.Combined.Closed:
		fmt.Fprintln(w, red("VERDICT: CLOSED"), "—", r.Combined.Source, "evidence:", r.Combined.Evidence)
	case r.Combined.Temporary:
		fmt.Fprintln(w, yellow("VERDICT: TEMPORARILY CLOSED"), "—", r.Combined.Source, "evidence:", r.Combined.Evidence)
	default:
		fmt.Fprintln(w, green("VERDICT: OPERATIONAL (no closed signals)"))
	}
	if r.Conflict {
		fmt.Fprintln(w, yellow("⚠ conflict: at least one source said open and one said closed; review per-source verdicts"))
	}
	if r.Google != nil {
		fmt.Fprintf(w, "  google.places: %s (status=%s)\n", r.Google.Name, r.Google.BusinessStatus)
	}
	for _, h := range r.Hits {
		fmt.Fprintf(w, "  %s: %s\n", h.Slug, h.Hit.URL)
	}
	for _, s := range r.StubsSkipped {
		fmt.Fprintf(w, "  %s: stubbed (%s)\n", s.Slug, s.Reason)
	}
	for _, slug := range r.NoHits {
		fmt.Fprintf(w, "  %s: no hits\n", slug)
	}
	for _, n := range r.Notes {
		fmt.Fprintf(w, "  note: %s\n", n)
	}
}
