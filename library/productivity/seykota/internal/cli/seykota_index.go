// Copyright 2026 kjuju600. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/seykota/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/seykota/internal/crawl"
	"github.com/mvanhorn/printing-press-library/library/productivity/seykota/internal/store"
)

func newIndexCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "index",
		Short: "Manage the local seykota.com archive (the SQLite + FTS index that powers search/faq/tsp/risk)",
	}
	cmd.AddCommand(newIndexBuildCmd(flags))
	cmd.AddCommand(newIndexStatusCmd(flags))
	return cmd
}

func newIndexBuildCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var rate float64
	var fullArchive bool
	var force bool
	var maxFAQ int
	var staleAfterDays int
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Refresh the local seykota.com archive (re-crawls only when stale or --force)",
		Long: `Re-crawl seykota.com and rebuild the local SQLite + FTS index that 'search',
'faq', 'tsp', and 'risk' read from.

This is idempotent: if the local archive already exists and is fresher than
--stale-after-days (default 30), it skips the crawl and reports it's current.
Pass --force to re-crawl regardless.

A full crawl fetches the ~266 monthly FAQ pages indexed at /tt/FAQ_Index/
(2010-2023), the ~10 Trading System Project section pages, and the risk
essay — roughly 280 small pages at ~1.5 requests/second, so it takes a few
minutes. Pass --full-archive to also crawl the pre-2010 /tribe/FAQ/
day-pages. The CLI ships with a bundled snapshot, so you only need this to
refresh after the site changes.`,
		Example: strings.Trim(`
  seykota-pp-cli index build
  seykota-pp-cli index build --force --full-archive
  seykota-pp-cli index build --force --rate 2 --max-faq 20    # quick partial re-crawl
`, "\n"),
		Annotations: map[string]string{"pp:typed-exit-codes": "0,7"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would crawl seykota.com and rebuild the local archive at", corpusDBPath(dbPath))
				return nil
			}
			path := corpusDBPath(dbPath)
			s, err := store.OpenWithContext(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("opening %s: %w", path, err)
			}
			defer s.Close()
			if err := s.EnsureCorpus(cmd.Context()); err != nil {
				return err
			}
			// seed an empty archive from the bundled snapshot before deciding
			// whether a re-crawl is needed (the bundle counts as "fresh").
			if s.CorpusEmpty(cmd.Context()) && len(snapshotGz) >= restoreMinBytes {
				s.Close()
				if rerr := restoreFromSnapshot(path); rerr != nil {
					return fmt.Errorf("restoring bundled archive: %w", rerr)
				}
				s, err = store.OpenWithContext(cmd.Context(), path)
				if err != nil {
					return err
				}
				if err := s.EnsureCorpus(cmd.Context()); err != nil {
					return err
				}
			}
			// idempotent: if the archive exists and is fresh, skip the crawl
			if !force {
				if total, _ := s.CorpusCount(""); total > 0 {
					fetched := s.CorpusFetchedAt()
					fresh := false
					if t, perr := time.Parse(time.RFC3339, fetched); perr == nil {
						if staleAfterDays <= 0 {
							staleAfterDays = 30
						}
						fresh = time.Since(t) < time.Duration(staleAfterDays)*24*time.Hour
					}
					if fresh {
						if wantsJSON(cmd, flags) {
							return emitJSON(cmd, flags, map[string]any{"db": path, "documents": total, "last_fetched": fetched, "crawled": false, "reason": "archive is current; pass --force to re-crawl"})
						}
						fmt.Fprintf(cmd.OutOrStdout(), "Local archive at %s is current (%d documents, last fetched %s).\nPass --force to re-crawl seykota.com anyway.\n", path, total, fetched)
						return nil
					}
				}
			}
			progress := func(msg string) {
				if !flags.quiet {
					fmt.Fprintln(os.Stderr, msg)
				}
			}
			start := time.Now()
			c := crawl.New(crawl.Options{FullArchive: fullArchive, RatePerSec: rate, MaxFAQ: maxFAQ}, progress)
			docs, err := c.Crawl(cmd.Context(), crawl.Options{FullArchive: fullArchive, MaxFAQ: maxFAQ})
			if err != nil {
				var rle *cliutil.RateLimitError
				if errors.As(err, &rle) {
					return rateLimitErr(fmt.Errorf("seykota.com rate-limited the crawl: %w", err))
				}
				return err
			}
			n, err := s.ReplaceCorpus(cmd.Context(), docs)
			if err != nil {
				return err
			}
			// fold the WAL back into the main file so the .db is self-contained
			_, _ = s.DB().ExecContext(cmd.Context(), `PRAGMA wal_checkpoint(TRUNCATE)`)
			elapsed := time.Since(start).Round(time.Second)

			faq, _ := s.CorpusCount("faq")
			tsp, _ := s.CorpusCount("tsp")
			risk, _ := s.CorpusCount("risk")
			if wantsJSON(cmd, flags) {
				return emitJSON(cmd, flags, map[string]any{
					"db": path, "documents": n, "faq_pages": faq, "tsp_sections": tsp,
					"risk_pages": risk, "full_archive": fullArchive, "crawled": true, "elapsed_seconds": int(elapsed.Seconds()),
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Rebuilt local archive at %s\n  %d documents — %d FAQ months, %d TSP sections, %d risk page(s)  (%s)\n", path, n, faq, tsp, risk, elapsed)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Archive DB path (default: the standard data dir)")
	cmd.Flags().Float64Var(&rate, "rate", 1.5, "Polite request ceiling, requests/second")
	cmd.Flags().BoolVar(&fullArchive, "full-archive", false, "Also crawl the pre-2010 /tribe/FAQ/ day-pages")
	cmd.Flags().BoolVar(&force, "force", false, "Re-crawl even if the local archive is still fresh")
	cmd.Flags().IntVar(&maxFAQ, "max-faq", 0, "Cap the number of FAQ month-pages fetched (0 = all; for quick partial refreshes)")
	cmd.Flags().IntVar(&staleAfterDays, "stale-after-days", 30, "Without --force, only re-crawl if the archive is older than this many days")
	return cmd
}

func newIndexStatusCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "status",
		Short:       "Show what's in the local archive (document counts, FAQ year span, DB path)",
		Example:     "  seykota-pp-cli index status",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := corpusDBPath(dbPath)
			s, err := store.OpenWithContext(cmd.Context(), path)
			if err != nil {
				return err
			}
			defer s.Close()
			if err := s.EnsureCorpus(cmd.Context()); err != nil {
				return err
			}
			restored := false
			if s.CorpusEmpty(cmd.Context()) && len(snapshotGz) >= restoreMinBytes {
				s.Close()
				if err := restoreFromSnapshot(path); err == nil {
					if s2, err2 := store.OpenWithContext(cmd.Context(), path); err2 == nil {
						s = s2
						_ = s.EnsureCorpus(cmd.Context())
						restored = true
					}
				}
				if !restored {
					s, _ = store.OpenWithContext(cmd.Context(), path)
					_ = s.EnsureCorpus(cmd.Context())
				}
			}
			total, _ := s.CorpusCount("")
			faq, _ := s.CorpusCount("faq")
			tsp, _ := s.CorpusCount("tsp")
			risk, _ := s.CorpusCount("risk")
			years, _ := s.FAQYears()
			yearSpan := ""
			if len(years) > 0 {
				yearSpan = years[0] + "–" + years[len(years)-1]
			}
			if wantsJSON(cmd, flags) {
				return emitJSON(cmd, flags, map[string]any{
					"db": path, "documents": total, "faq_pages": faq, "faq_year_span": yearSpan,
					"tsp_sections": tsp, "risk_pages": risk, "bundled_snapshot": len(snapshotGz) >= restoreMinBytes,
					"restored_from_bundle": restored,
				})
			}
			if total == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Local archive at %s is empty.\nRun 'seykota-pp-cli index build' to fetch it from seykota.com.\n", path)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Local archive: %s\n  %d documents — %d FAQ months (%s), %d TSP sections, %d risk page(s)\n  bundled snapshot: %v\n",
				path, total, faq, yearSpan, tsp, risk, len(snapshotGz) >= restoreMinBytes)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Archive DB path (default: the standard data dir)")
	return cmd
}
