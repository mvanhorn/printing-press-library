package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/atlasobscura"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/dispatch"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/goatstore"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/overpass"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/reddit"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/regions"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/sourcetypes"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/walking"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/wikipedia"
)

// newSyncCityCmd pre-caches the geo-anchored sources for offline use AND
// wires every implemented Stage-2 source in the country's region — v1 only
// wired 6 sources here; v2's invariant (per the brief) is that every
// non-stub client in dispatch.DefaultRegistry gets touched during sync.
func newSyncCityCmd(flags *rootFlags) *cobra.Command {
	var (
		country  string
		radiusKm float64
	)
	cmd := &cobra.Command{
		Use:   "sync-city [city]",
		Short: "Pre-cache geo-anchored sources + wire every implemented Stage-2 source for the country",
		Long: `sync-city resolves the named city, fans out to every geo-anchored source
(Overpass POIs, Wikipedia geosearch, Atlas Obscura, Reddit threads in the
region's forums), and persists results to the goatstore. It also "prewarms"
every implemented Stage-2 source for the country's region with one anchor
name — proving the wiring is intact. Stub sources are listed in the report
with their deferral reason.`,
		Example: strings.Trim(`
  wanderlust-goat-pp-cli sync-city "Tokyo" --country JP
  wanderlust-goat-pp-cli sync-city "Paris" --country FR --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			city := strings.Join(args, " ")
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			anchor, err := resolveAnchor(ctx, city)
			if err != nil {
				return apiErr(err)
			}
			cc := strings.ToUpper(strings.TrimSpace(country))
			if cc == "" {
				cc = anchor.Country
			}
			region := regions.Lookup(cc)

			store, err := openGoatStore(cmd, flags)
			if err != nil {
				return configErr(err)
			}
			defer store.Close()

			report := SyncCityReport{
				City:      city,
				Country:   cc,
				Region:    region,
				StartedAt: time.Now().UTC().Format(time.RFC3339),
			}

			radiusM := radiusKm * 1000
			if radiusM <= 0 {
				radiusM = 3000 // 3 km default
			}

			// === Geo-anchored sources, sequentially (cheap network) ===
			report.Overpass = syncOverpass(ctx, store, anchor, radiusM)
			report.Wikipedia = syncWikipedia(ctx, store, anchor, region, radiusM)
			report.AtlasObscura = syncAtlasObscura(ctx, store, anchor)
			report.Reddit = syncRedditForums(ctx, store, region, city)

			// === Stage-2 prewarm: confirm wiring of every implemented source ===
			report.Stage2 = prewarmStage2(ctx, region, city)

			report.FinishedAt = time.Now().UTC().Format(time.RFC3339)

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			renderSyncCity(cmd, report)
			return nil
		},
	}
	cmd.Flags().StringVar(&country, "country", "", "ISO 3166-1 alpha-2 country code (default: derived from anchor)")
	cmd.Flags().Float64Var(&radiusKm, "radius-km", 3.0, "geo-search radius for Overpass/Wikipedia in km")
	return cmd
}

// SyncCityReport is the JSON envelope returned to agents.
type SyncCityReport struct {
	City         string         `json:"city"`
	Country      string         `json:"country"`
	Region       regions.Region `json:"region"`
	StartedAt    string         `json:"started_at"`
	FinishedAt   string         `json:"finished_at,omitempty"`
	Overpass     SyncStat       `json:"overpass"`
	Wikipedia    SyncStat       `json:"wikipedia"`
	AtlasObscura SyncStat       `json:"atlas_obscura"`
	Reddit       SyncStat       `json:"reddit"`
	Stage2       []Stage2Stat   `json:"stage2_sources"`
}

type SyncStat struct {
	OK    bool   `json:"ok"`
	Count int    `json:"count"`
	Error string `json:"error,omitempty"`
}

type Stage2Stat struct {
	Slug   string `json:"slug"`
	Locale string `json:"locale"`
	Stub   bool   `json:"stub"`
	Reason string `json:"reason,omitempty"`
	Hits   int    `json:"hits"`
	Error  string `json:"error,omitempty"`
}

func syncOverpass(ctx context.Context, store *goatstore.Store, anchor dispatch.AnchorResolution, radiusM float64) SyncStat {
	cli := overpass.New(nil, userAgent())
	radiusInt := int(radiusM)
	resp, err := cli.NearbyByTags(ctx, anchor.Lat, anchor.Lng, radiusInt, []overpass.TagFilter{
		{Key: "tourism", Value: "viewpoint"},
	})
	if err != nil {
		return SyncStat{OK: false, Error: err.Error()}
	}
	count := 0
	for _, e := range resp.Elements {
		name := e.Tags["name"]
		if name == "" {
			continue
		}
		lat, lng := e.LatLng()
		err := store.UpsertPlace(ctx, goatstore.Place{
			ID:      fmt.Sprintf("osm.%d", e.ID),
			Source:  "overpass",
			Intent:  "viewpoint",
			Name:    name,
			Lat:     lat,
			Lng:     lng,
			Country: anchor.Country,
			Trust:   0.45,
			Updated: time.Now().UTC(),
			Data:    map[string]any{"tags": e.Tags},
		})
		if err == nil {
			count++
		}
	}
	return SyncStat{OK: true, Count: count}
}

func syncWikipedia(ctx context.Context, store *goatstore.Store, anchor dispatch.AnchorResolution, region regions.Region, radiusM float64) SyncStat {
	cli := wikipedia.New(region.PrimaryLanguage, nil, userAgent())
	resp, err := cli.GeoSearch(ctx, anchor.Lat, anchor.Lng, int(radiusM), 25)
	if err != nil {
		// Fall back to English on any failure (Korean wiki sometimes 503s).
		cliEN := wikipedia.New("en", nil, userAgent())
		resp, err = cliEN.GeoSearch(ctx, anchor.Lat, anchor.Lng, int(radiusM), 25)
	}
	if err != nil {
		return SyncStat{OK: false, Error: err.Error()}
	}
	count := 0
	for _, p := range resp.Pages {
		err := store.UpsertPlace(ctx, goatstore.Place{
			ID:      fmt.Sprintf("wiki.%s.%d", region.PrimaryLanguage, p.PageID),
			Source:  "wikipedia." + region.PrimaryLanguage,
			Intent:  "historic",
			Name:    p.Title,
			Lat:     p.Lat,
			Lng:     p.Lon,
			Country: anchor.Country,
			Trust:   0.55,
			Updated: time.Now().UTC(),
		})
		if err == nil {
			count++
		}
	}
	return SyncStat{OK: true, Count: count}
}

func syncAtlasObscura(ctx context.Context, store *goatstore.Store, anchor dispatch.AnchorResolution) SyncStat {
	cli := atlasobscura.New(nil, userAgent())
	citySlug := atlasObscuraCitySlug(anchor.Display)
	if citySlug == "" {
		return SyncStat{OK: true, Count: 0, Error: "could not derive Atlas Obscura city slug"}
	}
	entries, err := cli.City(ctx, citySlug)
	if err != nil {
		// Atlas Obscura returns 404 for many cities; treat as zero-count, not failure.
		return SyncStat{OK: true, Count: 0}
	}
	count := 0
	for _, e := range entries {
		err := store.UpsertPlace(ctx, goatstore.Place{
			ID:      "atlasobscura." + e.URL,
			Source:  "atlasobscura",
			Intent:  "hidden",
			Name:    e.Title,
			Lat:     e.Lat,
			Lng:     e.Lng,
			Country: anchor.Country,
			Trust:   0.50,
			Updated: time.Now().UTC(),
		})
		if err == nil {
			count++
		}
	}
	return SyncStat{OK: true, Count: count}
}

func syncRedditForums(ctx context.Context, store *goatstore.Store, region regions.Region, city string) SyncStat {
	cli := reddit.New(nil, userAgent())
	count := 0
	for _, sub := range region.LocalForums {
		threads, err := cli.Search(ctx, sub, city, reddit.SearchOpts{Limit: 10})
		if err != nil {
			continue
		}
		for _, th := range threads {
			err := store.UpsertRedditThread(ctx, goatstore.RedditThread{
				ID:          th.ID,
				Subreddit:   sub,
				Title:       th.Title,
				URL:         th.URL,
				Permalink:   th.Permalink,
				Score:       th.Score,
				NumComments: th.NumComments,
				Body:        th.Body,
				CitySlug:    strings.ToLower(city),
			})
			if err == nil {
				count++
			}
		}
	}
	return SyncStat{OK: true, Count: count}
}

// prewarmStage2 calls every implemented Stage-2 source for the region with
// the city name as the seed query. Each call's result count is recorded;
// stubs are surfaced with their deferral reason so the user knows why the
// row reads zero. This is the v2 invariant: "wire ALL implemented Stage-2
// sources during sync-city".
func prewarmStage2(ctx context.Context, region regions.Region, city string) []Stage2Stat {
	reg := dispatch.DefaultRegistry()
	out := []Stage2Stat{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, slug := range region.LocalReviewSites {
		cli := reg.Get(slug)
		if cli == nil {
			out = append(out, Stage2Stat{Slug: slug, Error: "not registered"})
			continue
		}
		wg.Add(1)
		go func(slug string, cli sourcetypes.Client) {
			defer wg.Done()
			stat := Stage2Stat{Slug: slug, Locale: cli.Locale(), Stub: cli.IsStub()}
			if cli.IsStub() {
				stat.Reason = sourcetypes.StubReason(cli)
				mu.Lock()
				out = append(out, stat)
				mu.Unlock()
				return
			}
			hits, err := cli.LookupByName(ctx, city, "", 1)
			if err != nil && !errors.Is(err, sourcetypes.ErrNotImplemented) {
				stat.Error = err.Error()
			}
			stat.Hits = len(hits)
			mu.Lock()
			out = append(out, stat)
			mu.Unlock()
		}(slug, cli)
	}
	wg.Wait()
	return out
}

func atlasObscuraCitySlug(display string) string {
	parts := strings.Split(display, ",")
	if len(parts) == 0 {
		return ""
	}
	city := strings.ToLower(strings.TrimSpace(parts[0]))
	city = strings.ReplaceAll(city, " ", "-")
	return city
}

func renderSyncCity(cmd *cobra.Command, r SyncCityReport) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s synced %s (%s)\n", bold("wanderlust-goat"), r.City, r.Country)
	statLine := func(name string, s SyncStat) {
		ok := green("ok")
		if !s.OK {
			ok = red("err")
		}
		fmt.Fprintf(w, "  %-15s %s  count=%d", name, ok, s.Count)
		if s.Error != "" {
			fmt.Fprintf(w, "  err=%s", truncate(s.Error, 60))
		}
		fmt.Fprintln(w)
	}
	statLine("overpass", r.Overpass)
	statLine("wikipedia", r.Wikipedia)
	statLine("atlasobscura", r.AtlasObscura)
	statLine("reddit", r.Reddit)
	fmt.Fprintln(w, "  Stage-2 prewarm (regional sources):")
	for _, s := range r.Stage2 {
		flag := green("real")
		extra := ""
		if s.Stub {
			flag = yellow("stub")
			extra = "  reason=" + truncate(s.Reason, 50)
		}
		if s.Error != "" {
			extra += "  err=" + truncate(s.Error, 40)
		}
		fmt.Fprintf(w, "    %-18s %s  hits=%d%s\n", s.Slug, flag, s.Hits, extra)
	}
}

// Silence unused-variable lints for the radius helper.
var _ = walking.MetersFromMinutes
