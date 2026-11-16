// Copyright 2026 Josselin and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/autom/internal/serp"
)

type newsWatchView struct {
	Query      string        `json:"query"`
	Country    string        `json:"country,omitempty"`
	Fetched    int           `json:"fetched"`
	New        []serp.Result `json:"new"`
	SeenSkips  int           `json:"already_seen"` // shown by an earlier watch of the same query and edition
	Duplicates int           `json:"duplicates"`   // repeated within this page
	FirstRun   bool          `json:"first_run"`    // nothing recorded yet for this query and edition (peeks and empty runs record nothing)
}

func newNovelNewsWatchCmd(flags *rootFlags) *cobra.Command {
	var country, lang, dbPath string
	var peek bool
	cmd := &cobra.Command{
		Use:   "watch [query]",
		Short: "Get only the news articles about a brand or keyword that you have not seen in earlier watches",
		Long: `Use this command for NEW news articles about a brand or keyword since the last watch.
It runs one Google News search and hides every article already shown by an earlier watch
(same canonical URL, or same outlet and title). Schedule it with cron for monitoring.
Costs 1 credit. --peek shows new articles without marking them as seen.
Do NOT use this command for a cited, deduplicated source list that answers a question; use 'ground' instead.`,
		Example: strings.Trim(`
  autom-pp-cli news watch "Autom" --country fr --agent
  autom-pp-cli news watch "serp api" --peek`, "\n"),
		Annotations: map[string]string{
			"mcp:local-write": "true", // remembers shown articles unless --peek
			"pp:data-source":  "live",
			"pp:happy-args":   "query=Autom;--peek",
			// With --peek nothing is recorded: live dogfood runs it for real.
			"pp:preview-happy-path": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "news watch")
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a brand or keyword is required, for example: news watch \"Autom\""))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			// Open the store before the billed search, so a store problem costs nothing.
			db, err := openSerpStore(ctx, dbPath)
			if err != nil && !localDataDisabled() {
				return err
			}
			if db != nil {
				defer db.Close()
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "note: %s=1, so every article counts as new and none is remembered\n", noLocalDataEnvVar)
			}
			// Each watch (query + edition) keeps its own memory, so an article
			// shown by one watch still appears in another.
			scope := newsScope(query, country, lang)
			view := newsWatchView{Query: query, Country: strings.ToLower(country), New: make([]serp.Result, 0), FirstRun: true}
			if db != nil {
				seenBefore, err := db.NewsScopeSeen(ctx, scope)
				if err != nil {
					return err
				}
				view.FirstRun = !seenBefore
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			articles, err := fetchNews(ctx, c, query, country, lang)
			if err != nil {
				return err
			}
			view.Fetched = len(articles)
			now := time.Now()
			fresh := make([][]string, 0)
			for _, a := range articles {
				keys := scopedKeys(scope, serp.NewsKeys(a))
				seen := false
				if db != nil {
					if seen, err = db.NewsSeen(ctx, keys); err != nil {
						return err
					}
				}
				if seen {
					view.SeenSkips++
					continue
				}
				// Two results of the same page can share a key; keep the first.
				dup := false
				for _, prev := range fresh {
					for _, k := range keys {
						if slices.Contains(prev, k) {
							dup = true
						}
					}
				}
				if dup {
					view.Duplicates++
					continue
				}
				fresh = append(fresh, keys)
				view.New = append(view.New, a)
			}
			if err := writeNewsWatch(cmd.OutOrStdout(), flags, view); err != nil {
				return err
			}
			// Mark articles as seen only once the report went out, so a failed
			// output or delivery leaves them for the next run.
			if peek || db == nil || localDataDisabled() || len(view.New) == 0 {
				return nil
			}
			mark := func(ctx context.Context, st newsMarker) error {
				for i, a := range view.New {
					if err := st.MarkNewsSeen(ctx, scope, a, fresh[i], now); err != nil {
						return err
					}
				}
				return nil
			}
			if flags.deliverBuf == nil {
				return mark(ctx, db)
			}
			// --deliver ships the buffered report after this command returns.
			afterDelivery(func() error {
				st, err := openSerpStore(context.Background(), dbPath)
				if err != nil {
					return err
				}
				defer st.Close()
				return mark(context.Background(), st)
			})
			return nil
		},
	}
	cmd.Flags().StringVar(&country, "country", "", "Two-letter country code of the Google News edition, for example fr")
	cmd.Flags().StringVar(&lang, "lang", "", "Language code, for example fr")
	cmd.Flags().BoolVar(&peek, "peek", false, "Show new articles without marking them as seen")
	addDBFlag(cmd, &dbPath)
	return cmd
}

type newsMarker interface {
	MarkNewsSeen(ctx context.Context, scope string, a serp.Result, keys []string, at time.Time) error
}

func writeNewsWatch(w io.Writer, flags *rootFlags, view newsWatchView) error {
	if !wantsHumanTable(w, flags) {
		return printJSONFiltered(w, view, flags)
	}
	if len(view.New) == 0 {
		_, err := fmt.Fprintf(w, "No new articles for %q (%d already seen).\n", view.Query, view.SeenSkips)
		return err
	}
	fmt.Fprintf(w, "%d new article(s) for %q", len(view.New), view.Query)
	if view.SeenSkips > 0 {
		fmt.Fprintf(w, ", %d already seen", view.SeenSkips)
	}
	fmt.Fprintln(w)
	for _, a := range view.New {
		src := a.Source
		if src == "" {
			src = a.Domain
		}
		line := fmt.Sprintf("- %s (%s", a.Title, src)
		if a.Date != "" {
			line += ", " + a.Date
		}
		if _, err := fmt.Fprintf(w, "%s)\n  %s\n", line, a.URL); err != nil {
			return err
		}
	}
	return nil
}

// newsScope identifies one watch: the query and the news edition.
func newsScope(query, country, lang string) string {
	return strings.ToLower(strings.TrimSpace(query)) + "|" + strings.ToLower(country) + "|" + strings.ToLower(lang)
}

// scopedKeys prefixes article keys with the watch scope.
func scopedKeys(scope string, keys []string) []string {
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, scope+"#"+k)
	}
	return out
}
