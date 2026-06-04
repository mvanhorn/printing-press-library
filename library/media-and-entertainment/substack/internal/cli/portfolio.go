// Copyright 2026 Chirantan Rajhans and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH: transcendence-commands — entry point for eight cross-publication
// commands (portfolio / posts twin / posts best / posts pair[s] /
// subs churn / subs cross-sell / grep / schedule board)
// that operate on the local SQLite store. Substack's web UI is
// single-publication, so multi-publication workflows have no Substack-
// side equivalent. Recorded in .printing-press-patches.json.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack/internal/store"

	"github.com/spf13/cobra"
)

type portfolioRow struct {
	Publication      string `json:"publication"`
	Subdomain        string `json:"subdomain"`
	SubscribersTotal int    `json:"subscribers_total"`
	SubscribersPaid  int    `json:"subscribers_paid"`
	PostsPublished   int    `json:"posts_published"`
	DraftsPending    int    `json:"drafts_pending"`
	ScheduledNext    string `json:"scheduled_next,omitempty"`
	LastPublishedAt  string `json:"last_published_at,omitempty"`
}

func newPortfolioCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "portfolio",
		Short: "One-screen status of every publication you own.",
		Long: `Aggregate snapshot of every publication you own from the local SQLite store:
subscriber count, paid count, posts published, drafts pending, scheduled next.

Run 'substack-pp-cli sync --full' first to refresh the local cache.`,
		Example:     "  substack-pp-cli portfolio --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:novel": "portfolio"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("substack-pp-cli")
			}
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer s.Close()
			db := s.DB()

			rows, err := db.QueryContext(cmd.Context(), `
				SELECT id, COALESCE(name, ''), COALESCE(subdomain, ''),
				       COALESCE(subscriber_count, 0), COALESCE(paid_subscriber_count, 0)
				FROM publications
				ORDER BY name
			`)
			if err != nil {
				return fmt.Errorf("listing publications: %w", err)
			}
			defer rows.Close()

			var out []portfolioRow
			for rows.Next() {
				var id, name, subdomain string
				var subs, paid int
				if err := rows.Scan(&id, &name, &subdomain, &subs, &paid); err != nil {
					return err
				}
				r := portfolioRow{
					Publication:      name,
					Subdomain:        subdomain,
					SubscribersTotal: subs,
					SubscribersPaid:  paid,
				}
				// counts from posts table
				_ = db.QueryRowContext(cmd.Context(),
					`SELECT COUNT(*) FROM posts WHERE publication_id = ? AND (scheduled_at IS NULL OR scheduled_at = '')`,
					id).Scan(&r.PostsPublished)
				_ = db.QueryRowContext(cmd.Context(),
					`SELECT COUNT(*) FROM drafts WHERE publication_id = ?`,
					id).Scan(&r.DraftsPending)
				_ = db.QueryRowContext(cmd.Context(),
					`SELECT COALESCE(MIN(scheduled_at), '') FROM posts WHERE publication_id = ? AND scheduled_at IS NOT NULL AND scheduled_at != ''`,
					id).Scan(&r.ScheduledNext)
				_ = db.QueryRowContext(cmd.Context(),
					`SELECT COALESCE(MAX(publish_date), '') FROM posts WHERE publication_id = ?`,
					id).Scan(&r.LastPublishedAt)
				out = append(out, r)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				raw, _ := json.Marshal(out)
				return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
			}
			// Human ASCII table
			w := cmd.OutOrStdout()
			if len(out) == 0 {
				fmt.Fprintln(w, "No publications synced yet. Run 'substack-pp-cli sync --full' after 'substack-pp-cli auth login --chrome'.")
				return nil
			}
			fmt.Fprintln(w, bold("Publication Portfolio"))
			fmt.Fprintln(w, strings.Repeat("─", 78))
			fmt.Fprintf(w, "%-22s %-18s %8s %8s %8s %8s %s\n", "Name", "Subdomain", "Subs", "Paid", "Posts", "Drafts", "Next Scheduled")
			fmt.Fprintln(w, strings.Repeat("─", 78))
			for _, r := range out {
				fmt.Fprintf(w, "%-22s %-18s %8d %8d %8d %8d %s\n",
					truncate(r.Publication, 22), truncate(r.Subdomain, 18),
					r.SubscribersTotal, r.SubscribersPaid, r.PostsPublished, r.DraftsPending,
					truncate(r.ScheduledNext, 20))
			}
			fmt.Fprintln(w, strings.Repeat("─", 78))
			fmt.Fprintf(w, "%d publication(s).\n", len(out))
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	// Multi-publication columnar sync lives under portfolio so the discovery
	// surface ("which publications do I own, and pull their data") sits next to
	// the portfolio readout it feeds. `portfolio` with no subcommand still
	// renders the table; `portfolio sync` populates the columnar tables.
	cmd.AddCommand(newPortfolioSyncCmd(flags))
	return cmd
}
