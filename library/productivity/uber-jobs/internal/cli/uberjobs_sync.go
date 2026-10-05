// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

// sync is hand-written (the generator emits no framework sync for this CLI).
// It is registered through the preserved hook in uberjobs_hooks.go and is
// deliberately not listed in research.json novel_features.

package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/uberjobs"
)

type syncResult struct {
	uberjobs.SyncRun
	Inserted      int    `json:"inserted"`
	Updated       int    `json:"updated"`
	FacetsUpdated bool   `json:"facets_updated"`
	FacetsNote    string `json:"facets_note,omitempty"`
}

func newUberSyncCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var maxPages int
	var withFacets bool
	cmd := &cobra.Command{
		Use:   "sync [query]",
		Short: "Mirror every open Uber posting into the local store so history, new-since, and stats work offline",
		Long: strings.Trim(`
Read the whole careers corpus (a size probe plus one full page) and upsert it
into the local store with first_seen and last_seen per posting. A complete,
uncapped, unfiltered read also marks postings that disappeared as closed; a
keyword-scoped or partial read never closes anything. The facet lists are
refreshed too (one more request) unless --facets=false.

An optional [query] scopes the read to a keyword; an empty query ('') means
the whole corpus. With no arguments and no flags, sync prints this help; any
flag (for example --json) runs a whole-corpus sync. --max-pages is accepted for compatibility: the corpus is
read in one page, so it only caps a scan when the site ever exceeds that.

If the careers site refuses, the read falls back to Uber's Oracle
candidate-experience API; existing rows keep their descriptions and no
closures are marked from a fallback read.`, "\n"),
		Example: strings.Trim(`
  uber-jobs-pp-cli sync --json
  uber-jobs-pp-cli sync '' --max-pages 20 --json
  uber-jobs-pp-cli sync strategy --facets=false`, "\n"),
		Annotations: map[string]string{
			"mcp:local-write":    "true",
			"pp:data-source":     "live",
			"pp:live-happy-path": "true",
			"pp:happy-args":      "query=strategy;--facets=false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			query := ""
			if len(args) > 0 {
				query = strings.TrimSpace(args[0])
			}
			if len(args) > 1 {
				return usageErr(fmt.Errorf("sync takes at most one [query] argument, got %d", len(args)))
			}
			if maxPages < 0 {
				return usageErr(fmt.Errorf("--max-pages must be zero or positive"))
			}
			if _, err := resolveDataSource(cmd, flags, "auto", "live"); err != nil {
				return err
			}
			f := uberjobs.Filters{Query: query}
			if dryRunOK(flags) {
				action := describeSource(flags, "live", dbPath, f) + " and upsert into " + uberDBPath(dbPath)
				if withFacets && query == "" {
					action += "; then GET " + baseURLFor(flags) + "/en/jobs/ for facets"
				}
				return writeDryRun(cmd.OutOrStdout(), flags, action)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := newUberClient(flags)
			if err != nil {
				return err
			}
			started := nowUTC()
			read, err := readLive(ctx, c, f)
			if err != nil {
				return uberErr(err)
			}
			s, db, err := openUberStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			scope := "all"
			if query != "" {
				scope = "query:" + query
			}
			if read.Fallback {
				scope = "all-oracle"
				if query != "" {
					scope = "query-oracle:" + query
				}
			}
			run := uberjobs.SyncRun{
				StartedAt:   started.Format("2006-01-02T15:04:05Z"),
				Source:      read.Source,
				Scope:       scope,
				Total:       read.Total,
				UniqueCount: len(read.Postings),
				Complete:    read.Complete,
				ScanCapHit:  read.ScanCapHit,
				Note:        read.Note,
			}
			run, stats, err := uberjobs.ApplyRead(ctx, db, read.Postings, run, nowUTC(), uberjobs.ApplyOptions{PreserveExisting: read.Fallback})
			if err != nil {
				return fmt.Errorf("writing the local store: %w", err)
			}
			res := syncResult{SyncRun: run, Inserted: stats.Inserted, Updated: stats.Updated}
			if withFacets && query == "" && !read.Fallback {
				if facets, ferr := c.FetchFacets(ctx); ferr != nil {
					res.FacetsNote = "facets not refreshed: " + ferr.Error()
				} else if serr := uberjobs.SaveFacets(ctx, db, facets, nowUTC()); serr != nil {
					res.FacetsNote = "facets not saved: " + serr.Error()
				} else {
					res.FacetsUpdated = true
				}
			}
			env := uberjobs.NewEnvelope("sync", read.Source, []syncResult{res}, 1)
			env.Meta.Fallback = read.Fallback
			env.Meta.FallbackFrom = read.FallbackFrom
			env.Meta.Complete = read.Complete
			env.Meta.Requests = c.Requests()
			env.Meta.Note = read.Note
			env.Meta.Cache = read.Cache
			env.Hits = len(read.Postings)
			env.Scanned = len(read.Postings)
			env.ScanCapHit = read.ScanCapHit
			return printEnvelope(cmd, flags, env, func(w io.Writer) error {
				fmt.Fprintf(w, "synced %d postings from %s (%d new, %d updated, %d marked closed)\n", run.UniqueCount, read.Source, stats.Inserted, stats.Updated, run.ClosedMarked)
				if !read.Complete {
					fmt.Fprintln(w, "note: the read was incomplete, so no closures were marked")
				}
				if res.FacetsNote != "" {
					fmt.Fprintln(w, "note:", res.FacetsNote)
				}
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI data dir)")
	cmd.Flags().IntVar(&maxPages, "max-pages", 0, "Accepted for compatibility; the corpus is read in one page")
	cmd.Flags().BoolVar(&withFacets, "facets", true, "Also refresh the facet lists (one more request)")
	addDataSourceFlag(cmd, "live", "sync always reads live; auto is accepted as live")
	return cmd
}
