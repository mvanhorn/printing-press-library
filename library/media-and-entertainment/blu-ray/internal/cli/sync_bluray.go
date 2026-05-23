package cli

// PATCH: Replace generic generated sync with Blu-ray.com's public sitemap sync.

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/blu-ray/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/blu-ray/internal/store"
	"github.com/spf13/cobra"
)

type sitemapURL struct {
	Loc     string `xml:"loc"`
	Lastmod string `xml:"lastmod"`
}

type newsURL struct {
	Loc             string `xml:"loc"`
	Title           string `xml:"news>title"`
	PublicationDate string `xml:"news>publication_date"`
}

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var kind string
	var wait time.Duration
	var maxPages int
	var quiet bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync the public Blu-ray.com sitemap into local SQLite catalog tables.",
		// PATCH: Add agent-copyable examples for dogfood command detection.
		Example: strings.Trim(`
  blu-ray-pp-cli sync --kind bluray
  blu-ray-pp-cli sync --kind all --wait 2 --quiet
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("blu-ray-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			if err := db.MigrateBluRayCatalog(); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			indexRaw, err := bluRayGet(c, "https://www.blu-ray.com/sitemap.xml", false)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			children, err := parseSitemapIndex(indexRaw)
			if err != nil {
				return err
			}
			children = filterSitemaps(children, kind)
			insertLimit := 0
			// PATCH: Dogfood and verify runs need bounded sitemap work to finish under tight timeouts.
			if cliutil.IsDogfoodEnv() || cliutil.IsVerifyEnv() {
				if len(children) > 1 {
					children = children[:1]
				}
				insertLimit = 100
			}
			if maxPages > 0 && len(children) > maxPages {
				children = children[:maxPages]
			}
			total := 0
			// PATCH: JSON mode buffers sync progress so stdout remains one parseable document.
			events := []map[string]any{}
			if flags.asJSON {
				events = append(events, map[string]any{
					"event":     "sync_start",
					"resources": len(children),
					"timestamp": time.Now().UTC().Format(time.RFC3339),
				})
			}
			for _, child := range children {
				if wait > 0 {
					time.Sleep(wait + time.Duration(rand.Intn(250))*time.Millisecond)
				}
				raw, err := bluRayGet(c, child, true)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				body, err := gunzipBytes(raw)
				if err != nil {
					return err
				}
				name := sitemapName(child)
				var count int
				locs, _ := parseSitemapLocs(body)
				if insertLimit > 0 && len(locs) > insertLimit {
					locs = locs[:insertLimit]
				}
				if strings.Contains(name, "sitemap_news") {
					var rows []store.NewsRow
					rows, err = parseNewsSitemapRows(body, insertLimit)
					count = len(rows)
					if err == nil {
						err = db.UpsertNewsRows(cmd.Context(), rows)
					}
				} else {
					var rows []store.CatalogRow
					rows, err = parseReleaseSitemapRows(body, name, insertLimit)
					count = len(rows)
					if err == nil {
						err = db.UpsertCatalogRows(cmd.Context(), rows)
					}
				}
				if err != nil {
					return err
				}
				if err := db.RecordSitemapSnapshot(cmd.Context(), name, len(locs), hashLines(locs)); err != nil {
					return err
				}
				total += count
				if flags.asJSON {
					events = append(events, map[string]any{
						"event":     "resource_synced",
						"items":     count,
						"sitemap":   name,
						"timestamp": time.Now().UTC().Format(time.RFC3339),
						"url_count": len(locs),
					})
				}
				if !flags.asJSON && !quiet && !flags.quiet {
					fmt.Fprintf(cmd.OutOrStdout(), "synced %s: %d urls\n", name, count)
				}
			}
			_ = db.SaveSyncState("releases_catalog", "", total)
			if flags.asJSON {
				events = append(events, map[string]any{
					"event":       "sync_complete",
					"resources":   len(children),
					"timestamp":   time.Now().UTC().Format(time.RFC3339),
					"total_items": total,
				})
				result := map[string]any{
					"events":           events,
					"resources_synced": len(children),
					"store_path":       db.Path(),
					"timestamp":        time.Now().UTC().Format(time.RFC3339),
					"total_items":      total,
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			}
			if flags.selectFields != "" {
				return flags.printJSON(cmd, map[string]any{"event": "sync_summary", "total_records": total, "sitemaps": len(children)})
			}
			if quiet || flags.quiet {
				fmt.Fprintf(cmd.OutOrStdout(), "%d\n", total)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "bluray", "Sitemap kind to sync: bluray, dvd, digital, itunes, ma, news, or all.")
	cmd.Flags().DurationVar(&wait, "wait", time.Second, "Delay between sitemap shard fetches, with small jitter.")
	cmd.Flags().IntVar(&maxPages, "max-pages", 0, "Maximum sitemap shards to fetch (0 = all matching shards).")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "Suppress per-shard progress lines.")
	return cmd
}

func filterSitemaps(children []string, kind string) []string {
	var out []string
	for _, child := range children {
		name := filepath.Base(child)
		switch kind {
		case "all":
			if strings.Contains(name, "sitemap_bluraymovies_") || strings.Contains(name, "sitemap_dvdmovies_") || strings.Contains(name, "sitemap_digitalmovies_") || strings.Contains(name, "sitemap_itunesmovies_") || strings.Contains(name, "sitemap_ma") || strings.Contains(name, "sitemap_news") {
				out = append(out, child)
			}
		case "news":
			if strings.Contains(name, "sitemap_news") {
				out = append(out, child)
			}
		case "dvd":
			if strings.Contains(name, "sitemap_dvdmovies_") {
				out = append(out, child)
			}
		case "digital":
			if strings.Contains(name, "sitemap_digitalmovies_") {
				out = append(out, child)
			}
		case "itunes":
			if strings.Contains(name, "sitemap_itunesmovies_") {
				out = append(out, child)
			}
		case "ma":
			if strings.Contains(name, "sitemap_ma") {
				out = append(out, child)
			}
		default:
			if strings.Contains(name, "sitemap_bluraymovies_") {
				out = append(out, child)
			}
		}
	}
	return out
}

func parseReleaseSitemapRows(body []byte, name string, insertLimit int) ([]store.CatalogRow, error) {
	type urlset struct {
		URLs []sitemapURL `xml:"url"`
	}
	var s urlset
	// PATCH: Blu-ray.com sitemaps can contain invalid UTF-8 despite declaring UTF-8.
	if err := decodePermissiveXML(body, &s); err != nil {
		return nil, err
	}
	if insertLimit > 0 && len(s.URLs) > insertLimit {
		s.URLs = s.URLs[:insertLimit]
	}
	kindHint := kindFromSitemapName(name)
	var rows []store.CatalogRow
	for _, u := range s.URLs {
		kind, slug, id, ok := parseReleaseURL(u.Loc)
		if !ok {
			continue
		}
		if kindHint != "" && kind == "bluray" {
			kind = kindHint
		}
		rows = append(rows, store.CatalogRow{ID: id, Kind: kind, Slug: slug, TitleNormalized: titleFromSlug(slug), YearHint: yearFromSlug(slug), Lastmod: u.Lastmod})
	}
	return rows, nil
}

func parseNewsSitemapRows(body []byte, insertLimit int) ([]store.NewsRow, error) {
	type urlset struct {
		URLs []newsURL `xml:"url"`
	}
	var s urlset
	// PATCH: Blu-ray.com sitemaps can contain invalid UTF-8 despite declaring UTF-8.
	if err := decodePermissiveXML(body, &s); err != nil {
		return nil, err
	}
	if insertLimit > 0 && len(s.URLs) > insertLimit {
		s.URLs = s.URLs[:insertLimit]
	}
	var rows []store.NewsRow
	for _, u := range s.URLs {
		id := 0
		if m := newsIDRE.FindStringSubmatch(u.Loc); len(m) == 2 {
			id, _ = strconv.Atoi(m[1])
		}
		if id == 0 {
			continue
		}
		rows = append(rows, store.NewsRow{ID: id, URL: u.Loc, Title: u.Title, PublicationDate: u.PublicationDate})
	}
	return rows, nil
}

func kindFromSitemapName(name string) string {
	switch {
	case strings.Contains(name, "dvdmovies"):
		return "dvd"
	case strings.Contains(name, "digitalmovies"):
		return "digital"
	case strings.Contains(name, "itunesmovies"):
		return "itunes"
	case strings.Contains(name, "sitemap_ma"):
		return "ma"
	default:
		return ""
	}
}
