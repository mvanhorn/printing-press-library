// News command — sync, list, link articles to federal-IT contracts.
//
// Transcendence feature: news-to-contract correlation. Mention extraction is
// deterministic word-match against the local recipients + agencies tables;
// no LLM/NLP. The result is a tags row per (article, entity) pair, queryable
// via SQL or via `explain` / `vendor` rollup.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/monitoring/pubsec-tech/internal/news"
	"github.com/mvanhorn/printing-press-library/library/monitoring/pubsec-tech/internal/refdata"
	"github.com/mvanhorn/printing-press-library/library/monitoring/pubsec-tech/internal/store"
)

func newNewsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "news",
		Short: "Federal-tech news: sync RSS feeds, list articles, link to contracts",
		Long: "News commands operate against the local SQLite store. Run `news sync` to pull " +
			"new articles from enabled sources, then `news list` to browse them or " +
			"`news link` to correlate articles with the underlying federal contracts and opportunities.",
		Example: "  pubsec-tech-pp-cli news sync\n  pubsec-tech-pp-cli news list --since 7d --json\n  pubsec-tech-pp-cli news link --since 7d --agent",
	}
	cmd.AddCommand(newNewsSyncCmd(flags))
	cmd.AddCommand(newNewsListCmd(flags))
	cmd.AddCommand(newNewsLinkCmd(flags))
	return cmd
}

func newNewsSyncCmd(flags *rootFlags) *cobra.Command {
	var rebuildTags bool
	cmd := &cobra.Command{
		Use:     "sync",
		Short:   "Fetch every enabled RSS source and upsert articles into the local store",
		Example: "  pubsec-tech-pp-cli news sync\n  pubsec-tech-pp-cli news sync --rebuild-tags",
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
			if _, err := ensureSourcesSeeded(ctx, s); err != nil {
				return err
			}
			if err := ensureCodesSeeded(ctx, s); err != nil {
				return err
			}
			sources, err := s.ListSources(ctx, true)
			if err != nil {
				return err
			}
			if len(sources) == 0 {
				return fmt.Errorf("no enabled sources; run `sources list` and `sources enable <id>` to enable some")
			}
			f := news.NewFetcher()
			results := f.FetchAll(ctx, sources)
			vendorNames, _ := s.ListRecipientNames(ctx)
			agencyNames, _ := s.ListAgencyNames(ctx)
			report := newsSyncReport{Started: time.Now().UTC()}
			for _, r := range results {
				row := newsSyncSourceResult{SourceID: r.SourceID, Status: r.Status, NotModified: r.NotModified, TookMillis: r.Took.Milliseconds()}
				if r.Err != nil {
					row.Error = r.Err.Error()
					report.Failures++
				} else if r.NotModified {
					row.NotModified = true
					report.NotModified++
				} else {
					inserted, updated := 0, 0
					for _, it := range r.Items {
						id := news.ItemID(r.SourceID, it.GUID)
						art := store.Article{
							ID:          id,
							SourceID:    r.SourceID,
							GUID:        it.GUID,
							Title:       it.Title,
							Link:        it.Link,
							Summary:     it.Summary,
							Content:     it.Content,
							Author:      it.Author,
							Categories:  strings.Join(it.Categories, ", "),
							PublishedAt: it.PublishedAt,
						}
						newRow, err := s.UpsertArticle(ctx, art)
						if err != nil {
							row.Error = err.Error()
							break
						}
						if newRow {
							inserted++
						} else {
							updated++
						}
						// Extract mentions for new or rebuilt articles
						if newRow || rebuildTags {
							text := art.Title + "\n" + art.Summary + "\n" + art.Content
							tags := news.ExtractMentions(text, vendorNames, agencyNames)
							if err := s.UpsertTagsForArticle(ctx, art.ID, tags); err != nil {
								row.Error = err.Error()
								break
							}
						}
					}
					row.Inserted = inserted
					row.Updated = updated
					report.Inserted += inserted
					report.Updated += updated
					if err := s.SaveSourceFetchMetadata(ctx, r.SourceID, r.ETag, r.LastModified); err != nil {
						row.Error = err.Error()
					}
				}
				report.Sources = append(report.Sources, row)
			}
			report.Finished = time.Now().UTC()
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(report)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Synced %d sources: %d inserted, %d updated, %d not-modified, %d failed.\n",
				len(report.Sources), report.Inserted, report.Updated, report.NotModified, report.Failures)
			for _, r := range report.Sources {
				status := fmt.Sprintf("%d", r.Status)
				if r.Status == 0 {
					status = "err"
				}
				if r.Error != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "  ✗ %s [%s] %s\n", r.SourceID, status, r.Error)
				} else if r.NotModified {
					fmt.Fprintf(cmd.OutOrStdout(), "  · %s [304] not modified\n", r.SourceID)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s [%s] +%d new, ±%d updated (%dms)\n",
						r.SourceID, status, r.Inserted, r.Updated, r.TookMillis)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&rebuildTags, "rebuild-tags", false, "Re-extract entity mentions for all articles (not just new ones)")
	return cmd
}

type newsSyncReport struct {
	Started     time.Time              `json:"started_at"`
	Finished    time.Time              `json:"finished_at"`
	Inserted    int                    `json:"inserted"`
	Updated     int                    `json:"updated"`
	NotModified int                    `json:"not_modified"`
	Failures    int                    `json:"failures"`
	Sources     []newsSyncSourceResult `json:"sources"`
}

type newsSyncSourceResult struct {
	SourceID    string `json:"source_id"`
	Status      int    `json:"http_status"`
	NotModified bool   `json:"not_modified,omitempty"`
	Inserted    int    `json:"inserted"`
	Updated     int    `json:"updated"`
	TookMillis  int64  `json:"took_millis"`
	Error       string `json:"error,omitempty"`
}

func newNewsListCmd(flags *rootFlags) *cobra.Command {
	var since string
	var limit int
	var sourceID string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List news articles from the local store",
		Example:     "  pubsec-tech-pp-cli news list --since 24h\n  pubsec-tech-pp-cli news list --since 7d --source fedscoop --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
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
			articles, err := s.ArticlesSince(ctx, sinceT, limit)
			if err != nil {
				return err
			}
			if sourceID != "" {
				filtered := make([]store.Article, 0, len(articles))
				for _, a := range articles {
					if a.SourceID == sourceID {
						filtered = append(filtered, a)
					}
				}
				articles = filtered
			}
			return renderArticles(cmd, flags, articles)
		},
	}
	cmd.Flags().StringVar(&since, "since", "24h", "Time window (e.g. 24h, 7d, 2026-05-01)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum articles to return (0 = unlimited)")
	cmd.Flags().StringVar(&sourceID, "source", "", "Filter to one source ID (e.g. fedscoop)")
	return cmd
}

func newNewsLinkCmd(flags *rootFlags) *cobra.Command {
	var since string
	var limit int
	var entityKind string
	var entityValue string
	cmd := &cobra.Command{
		Use:         "link",
		Short:       "List news articles linked to the underlying federal contracts and opportunities",
		Example:     "  pubsec-tech-pp-cli news link --since 7d --agent\n  pubsec-tech-pp-cli news link --vendor \"Leidos\" --since 30d",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
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
			var articles []store.Article
			if entityValue != "" {
				kind := "recipient"
				if entityKind == "agency" {
					kind = "agency"
				}
				articles, err = s.ArticlesForEntity(ctx, kind, entityValue, sinceT, limit)
			} else {
				articles, err = s.ArticlesSince(ctx, sinceT, limit)
			}
			if err != nil {
				return err
			}
			type linkedArticle struct {
				ID          string      `json:"id"`
				SourceID    string      `json:"source_id"`
				Title       string      `json:"title"`
				Link        string      `json:"link"`
				PublishedAt time.Time   `json:"published_at"`
				Tags        []taggedHit `json:"linked_entities"`
				Awards      []entityHit `json:"awards"`
				Opps        []entityHit `json:"opportunities"`
			}
			out := make([]linkedArticle, 0, len(articles))
			// Detect whether the local entity tables are populated so we can
			// surface a clear prerequisite note when the mention extractor
			// has nothing to match against.
			vendorCount := 0
			agencyCount := 0
			if names, err := s.ListRecipientNames(ctx); err == nil {
				vendorCount = len(names)
			}
			if names, err := s.ListAgencyNames(ctx); err == nil {
				agencyCount = len(names)
			}
			for _, a := range articles {
				tags, _ := s.TagsForArticle(ctx, a.ID)
				row := linkedArticle{
					ID: a.ID, SourceID: a.SourceID, Title: a.Title, Link: a.Link, PublishedAt: a.PublishedAt,
					Tags: make([]taggedHit, 0, len(tags)),
				}
				for _, t := range tags {
					row.Tags = append(row.Tags, taggedHit{Kind: t.Kind, Value: t.Value, MatchSpan: t.MatchSpan})
					// Cross-reference: for each recipient tag, find awards in the store
					if t.Kind == "recipient" {
						if rd, _ := s.RecipientByName(ctx, t.Value); rd != nil {
							var m map[string]any
							if json.Unmarshal(rd, &m) == nil {
								if id, _ := m["recipient_id"].(string); id != "" {
									row.Awards = append(row.Awards, entityHit{ID: id, Name: t.Value, Kind: "recipient_profile"})
								}
							}
						}
						// Best-effort opportunity lookup by vendor in title
						opps, _ := s.OpportunitiesByTitle(ctx, t.Value, 3)
						for _, o := range opps {
							row.Opps = append(row.Opps, entityHit{ID: o.ID, Name: t.Value, Kind: "opportunity"})
						}
					}
				}
				out = append(out, row)
			}
			// Wrap in an envelope so we can attach prerequisite notes when the
			// link mechanism has nothing to match against - per Phase 4.85
			// agentic output review, empty linked_entities arrays without
			// explanation read as "feature broken" to users.
			type envelope struct {
				Articles []linkedArticle `json:"articles"`
				Notes    []string        `json:"notes,omitempty"`
			}
			env := envelope{Articles: out}
			if vendorCount == 0 && agencyCount == 0 {
				env.Notes = append(env.Notes, "no synced vendors or agencies in the local store - mention extractor has nothing to match against; run `sync --resources recipients,agencies` to populate")
			} else if vendorCount == 0 {
				env.Notes = append(env.Notes, "no synced vendors; only agency mentions will be detected. Run `sync --resources recipients` to enable vendor mention detection")
			} else if agencyCount == 0 {
				env.Notes = append(env.Notes, "no synced agencies; only vendor mentions will be detected. Run `sync --resources agencies` to enable agency mention detection")
			}
			// Per Phase 4.85 agentic output review: surface enabled sources
			// that contributed zero articles in the window so a reader can
			// distinguish "no recent articles" from "never synced" or "sync
			// failed". Suppressed when the caller filtered by --vendor (the
			// per-source distribution is meaningless under entity filtering).
			if entityValue == "" {
				contributing := make(map[string]struct{}, len(out))
				for _, a := range out {
					if a.SourceID != "" {
						contributing[a.SourceID] = struct{}{}
					}
				}
				env.Notes = append(env.Notes, silentZeroSourceNotes(ctx, s, contributing, sinceT)...)
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), env, flags)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "DATE\tSOURCE\tLINKED\tTITLE")
			for _, a := range out {
				date := ""
				if !a.PublishedAt.IsZero() {
					date = a.PublishedAt.Format("2006-01-02")
				}
				linkedSummary := fmt.Sprintf("%d entities, %d opps", len(a.Tags), len(a.Opps))
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", date, a.SourceID, linkedSummary, truncate(a.Title, 90))
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			for _, n := range env.Notes {
				fmt.Fprintf(cmd.OutOrStdout(), "\n• %s\n", n)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "Time window")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum articles (0 = unlimited)")
	cmd.Flags().StringVar(&entityValue, "vendor", "", "Filter to articles linked to one vendor name")
	cmd.Flags().StringVar(&entityKind, "kind", "recipient", "Entity kind for --vendor: recipient or agency")
	return cmd
}

type taggedHit struct {
	Kind      string `json:"kind"`
	Value     string `json:"value"`
	MatchSpan string `json:"match_span,omitempty"`
}

// silentZeroSourceNotes returns one note per enabled source that contributed
// zero articles in the given window. Lets a reader distinguish "this source
// had no recent articles" from "this source was never synced" or "this source
// failed". The set of contributing source IDs comes from the caller (the
// articles already fetched for the window); we only enumerate enabled sources
// here.
func silentZeroSourceNotes(ctx context.Context, s *store.Store, contributing map[string]struct{}, since time.Time) []string {
	srcs, err := s.ListSources(ctx, true)
	if err != nil || len(srcs) == 0 {
		return nil
	}
	notes := make([]string, 0)
	sinceLabel := since.Format("2006-01-02")
	for _, src := range srcs {
		if _, ok := contributing[src.ID]; ok {
			continue
		}
		notes = append(notes, fmt.Sprintf("no %s articles in window since %s (source is enabled; run `news sync` if you expect items, or `sources disable %s` to mute)", src.ID, sinceLabel, src.ID))
	}
	return notes
}

type entityHit struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

// renderArticles is the shared rendering path for `news list` and similar commands.
func renderArticles(cmd *cobra.Command, flags *rootFlags, articles []store.Article) error {
	if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
		type row struct {
			ID          string    `json:"id"`
			SourceID    string    `json:"source_id"`
			Title       string    `json:"title"`
			Link        string    `json:"link"`
			Summary     string    `json:"summary,omitempty"`
			Author      string    `json:"author,omitempty"`
			Categories  string    `json:"categories,omitempty"`
			PublishedAt time.Time `json:"published_at"`
		}
		rows := make([]row, 0, len(articles))
		for _, a := range articles {
			rows = append(rows, row{
				ID: a.ID, SourceID: a.SourceID, Title: a.Title, Link: a.Link,
				Summary: a.Summary, Author: a.Author, Categories: a.Categories,
				PublishedAt: a.PublishedAt,
			})
		}
		return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
	}
	tw := newTabWriter(cmd.OutOrStdout())
	fmt.Fprintln(tw, "DATE\tSOURCE\tTITLE")
	for _, a := range articles {
		date := ""
		if !a.PublishedAt.IsZero() {
			date = a.PublishedAt.Format("2006-01-02")
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", date, a.SourceID, truncate(a.Title, 120))
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\n%d articles. Use --json for full structure or --since <window> to widen.\n", len(articles))
	return nil
}

// parseSince accepts duration strings ("24h", "7d") or absolute dates ("2026-05-01").
func parseSince(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Now().Add(-24 * time.Hour), nil
	}
	// Treat "Nd" as N*24h
	if strings.HasSuffix(s, "d") {
		s = strings.TrimSuffix(s, "d") + "h"
		s = expandDays(s)
	}
	if d, err := time.ParseDuration(s); err == nil {
		return time.Now().Add(-d), nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("could not parse --since %q (try 24h, 7d, or 2026-05-01)", s)
}

func expandDays(s string) string {
	// "30h" stays "30h"; "7h" was "7d" before trim so multiply by 24
	suffix := "h"
	core := strings.TrimSuffix(s, suffix)
	var n int
	if _, err := fmt.Sscanf(core, "%d", &n); err == nil {
		return fmt.Sprintf("%dh", n*24)
	}
	return s
}

// ensureCodesSeeded inserts the curated NAICS + PSC seed if the tables are empty.
func ensureCodesSeeded(ctx context.Context, s *store.Store) error {
	if n, err := s.CountCodes(ctx, "naics"); err != nil {
		return err
	} else if n == 0 {
		entries := make([]store.CodeEntry, 0, len(refdata.NAICSSeeds))
		for _, c := range refdata.NAICSSeeds {
			entries = append(entries, store.CodeEntry{Code: c.Code, Title: c.Title, Category: c.Category, Parent: c.Parent, Depth: c.Depth})
		}
		if err := s.UpsertCodes(ctx, "naics_codes", entries); err != nil {
			return err
		}
	}
	if n, err := s.CountCodes(ctx, "psc"); err != nil {
		return err
	} else if n == 0 {
		entries := make([]store.CodeEntry, 0, len(refdata.PSCSeeds))
		for _, c := range refdata.PSCSeeds {
			entries = append(entries, store.CodeEntry{Code: c.Code, Title: c.Title, Category: c.Category, Parent: c.Parent, Depth: c.Depth})
		}
		if err := s.UpsertCodes(ctx, "psc_codes", entries); err != nil {
			return err
		}
	}
	return nil
}
