package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/diggparse"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/diggstore"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/store"

	"github.com/spf13/cobra"
)

// registerDiggCommands wires all the Digg-specific novel commands onto
// the root. Called from root.go after the generated commands have been
// registered. Each command shows up as a top-level subcommand.
func registerDiggCommands(root *cobra.Command, flags *rootFlags) {
	root.AddCommand(newTopCmd(flags))
	root.AddCommand(newRisingCmd(flags))
	root.AddCommand(newStoryCmd(flags))
	root.AddCommand(newSearchCmd(flags))
	root.AddCommand(newEventsCmd(flags))
	root.AddCommand(newEvidenceCmd(flags))
	root.AddCommand(newSentimentCmd(flags))
	root.AddCommand(newCrossrefCmd(flags))
	root.AddCommand(newReplacedCmd(flags))
	root.AddCommand(newHistoryCmd(flags))
	root.AddCommand(newAuthorsCmd(flags))
	root.AddCommand(newAuthorCmd(flags))
	root.AddCommand(newWatchCmd(flags))
	root.AddCommand(newPipelineCmd(flags))
	root.AddCommand(newOpenCmd(flags))
	root.AddCommand(newStatsCmd(flags))
}

// readOnlyAnnotations declares the MCP-readonly annotation. Used on
// every digg novel command that does not mutate external state — they
// are all read-only against Digg by design.
func readOnlyAnnotations() map[string]string {
	return map[string]string{"mcp:read-only": "true"}
}

// openStore opens the local SQLite store and ensures the digg schema is
// in place. Returns a store wrapper, the *sql.DB, and a close function.
// Callers MUST call close on success and on error.
func openStore(ctx context.Context) (*store.Store, *sql.DB, func() error, error) {
	dbPath := defaultDBPath("digg-pp-cli")
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, nil, func() error { return nil }, fmt.Errorf("opening local database: %w", err)
	}
	db := s.DB()
	if err := diggstore.EnsureSchema(db); err != nil {
		s.Close()
		return nil, nil, func() error { return nil }, err
	}
	return s, db, s.Close, nil
}

// ============== top ==============

type clusterRow struct {
	ClusterID            string  `json:"clusterId"`
	ClusterURLID         string  `json:"clusterUrlId"`
	Label                string  `json:"label,omitempty"`
	Title                string  `json:"title,omitempty"`
	TLDR                 string  `json:"tldr,omitempty"`
	URL                  string  `json:"url,omitempty"`
	Permalink            string  `json:"permalink,omitempty"`
	CurrentRank          int     `json:"currentRank"`
	PeakRank             int     `json:"peakRank,omitempty"`
	PreviousRank         int     `json:"previousRank,omitempty"`
	Delta                int     `json:"delta"`
	GravityScore         float64 `json:"gravityScore,omitempty"`
	NumeratorCount       int     `json:"numeratorCount,omitempty"`
	NumeratorLabel       string  `json:"numeratorLabel,omitempty"`
	Pos6h                float64 `json:"pos6h,omitempty"`
	Pos12h               float64 `json:"pos12h,omitempty"`
	Pos24h               float64 `json:"pos24h,omitempty"`
	Likes                int     `json:"likes,omitempty"`
	Views                int     `json:"views,omitempty"`
	SourceTitle          string  `json:"sourceTitle,omitempty"`
	ReplacementRationale string  `json:"replacementRationale,omitempty"`
	ActivityAt           string  `json:"activityAt,omitempty"`
	LastSeenAt           string  `json:"lastSeenAt,omitempty"`
}

func scanClusters(rows *sql.Rows) ([]clusterRow, error) {
	defer rows.Close()
	var out []clusterRow
	for rows.Next() {
		var c clusterRow
		var peakRank sql.NullInt64
		if err := rows.Scan(&c.ClusterID, &c.ClusterURLID, &c.Label, &c.Title, &c.TLDR,
			&c.URL, &c.Permalink, &c.CurrentRank, &peakRank, &c.PreviousRank, &c.Delta,
			&c.GravityScore, &c.NumeratorCount, &c.NumeratorLabel, &c.Pos6h, &c.Pos12h, &c.Pos24h,
			&c.Likes, &c.Views, &c.SourceTitle, &c.ReplacementRationale, &c.ActivityAt, &c.LastSeenAt); err != nil {
			return nil, err
		}
		c.PeakRank = int(peakRank.Int64)
		out = append(out, c)
	}
	return out, rows.Err()
}

const clusterSelectCols = `cluster_id, cluster_url_id, COALESCE(label,''), COALESCE(title,''), COALESCE(tldr,''),
COALESCE(url,''), COALESCE(permalink,''), COALESCE(current_rank,0), peak_rank, COALESCE(previous_rank,0), COALESCE(delta,0),
COALESCE(gravity_score,0), COALESCE(numerator_count,0), COALESCE(numerator_label,''), COALESCE(pos6h,0), COALESCE(pos12h,0), COALESCE(pos24h,0),
COALESCE(likes,0), COALESCE(views,0), COALESCE(source_title,''), COALESCE(replacement_rationale,''), COALESCE(activity_at,''), COALESCE(last_seen_at,'')`

func newTopCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "top",
		Short:       "List top clusters from the local store, sorted by current rank",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  # Top 20 clusters
  digg-pp-cli top --limit 20

  # JSON for agents, narrowed to a few fields
  digg-pp-cli top --limit 10 --json --select clusterUrlId,label,currentRank,delta,tldr`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			_, db, closeFn, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer closeFn()
			rows, err := db.QueryContext(cmd.Context(),
				`SELECT `+clusterSelectCols+` FROM digg_clusters
				 WHERE current_rank > 0 AND last_seen_at = (SELECT MAX(last_seen_at) FROM digg_clusters)
				 ORDER BY current_rank ASC LIMIT ?`, limit)
			if err != nil {
				return err
			}
			clusters, err := scanClusters(rows)
			if err != nil {
				return err
			}
			if len(clusters) == 0 {
				return emptyHint(cmd, "no clusters in the local store. Run `digg-pp-cli sync` first.")
			}
			return printClusterOutput(cmd, flags, clusters, renderClusterTable)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Max number of clusters to return")
	return cmd
}

func newRisingCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var minDelta int
	cmd := &cobra.Command{
		Use:         "rising",
		Short:       "List clusters with the largest positive rank delta since their last snapshot",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  digg-pp-cli rising --limit 10
  digg-pp-cli rising --min-delta 5 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			_, db, closeFn, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer closeFn()
			rows, err := db.QueryContext(cmd.Context(),
				`SELECT `+clusterSelectCols+` FROM digg_clusters
				 WHERE delta >= ? AND last_seen_at = (SELECT MAX(last_seen_at) FROM digg_clusters)
				 ORDER BY delta DESC, current_rank ASC LIMIT ?`, minDelta, limit)
			if err != nil {
				return err
			}
			clusters, err := scanClusters(rows)
			if err != nil {
				return err
			}
			if len(clusters) == 0 {
				return emptyHint(cmd, "no rising clusters since last sync. Run `digg-pp-cli sync` again later, or lower --min-delta.")
			}
			return printClusterOutput(cmd, flags, clusters, renderClusterTable)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Max number of clusters to return")
	cmd.Flags().IntVar(&minDelta, "min-delta", 1, "Minimum positive rank delta")
	return cmd
}

// ============== story ==============

func newStoryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "story [clusterUrlId]",
		Short:       "Show full detail for one cluster from the local store",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  digg-pp-cli story iq7usf9e
  digg-pp-cli story iq7usf9e --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id := args[0]
			_, db, closeFn, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer closeFn()
			row := db.QueryRowContext(cmd.Context(),
				`SELECT `+clusterSelectCols+`, COALESCE(score_components_json,''), COALESCE(authors_json,'[]'), COALESCE(hacker_news_json,''), COALESCE(techmeme_json,''), COALESCE(raw_json,'')
				 FROM digg_clusters WHERE cluster_url_id = ? OR cluster_id = ?`, id, id)
			var c clusterRow
			var peakRank sql.NullInt64
			var scoreJSON, authorsJSON, hnJSON, tmJSON, rawJSON string
			if err := row.Scan(&c.ClusterID, &c.ClusterURLID, &c.Label, &c.Title, &c.TLDR,
				&c.URL, &c.Permalink, &c.CurrentRank, &peakRank, &c.PreviousRank, &c.Delta,
				&c.GravityScore, &c.NumeratorCount, &c.NumeratorLabel, &c.Pos6h, &c.Pos12h, &c.Pos24h,
				&c.Likes, &c.Views, &c.SourceTitle, &c.ReplacementRationale, &c.ActivityAt, &c.LastSeenAt,
				&scoreJSON, &authorsJSON, &hnJSON, &tmJSON, &rawJSON); err != nil {
				if err == sql.ErrNoRows {
					return fmt.Errorf("cluster not found: %s (run `sync` or pass a clusterUrlId from `top --json --select clusterUrlId`)", id)
				}
				return err
			}
			c.PeakRank = int(peakRank.Int64)
			full := map[string]any{
				"cluster":         c,
				"scoreComponents": asJSONString(scoreJSON),
				"authors":         asJSONString(authorsJSON),
				"hackerNews":      asJSONString(hnJSON),
				"techmeme":        asJSONString(tmJSON),
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), full, flags)
			}
			return renderStoryText(cmd.OutOrStdout(), c, scoreJSON, authorsJSON, hnJSON, tmJSON)
		},
	}
	return cmd
}

func renderStoryText(w io.Writer, c clusterRow, scoreJSON, authorsJSON, hnJSON, tmJSON string) error {
	fmt.Fprintf(w, "%s\n", c.Label)
	if c.Title != "" && c.Title != c.Label {
		fmt.Fprintf(w, "  %s\n", c.Title)
	}
	fmt.Fprintf(w, "  rank %d (peak %d, prev %d, delta %+d)  gravity %.2f\n",
		c.CurrentRank, c.PeakRank, c.PreviousRank, c.Delta, c.GravityScore)
	if c.NumeratorLabel != "" {
		fmt.Fprintf(w, "  %s: %d\n", c.NumeratorLabel, c.NumeratorCount)
	}
	if c.Pos24h > 0 {
		fmt.Fprintf(w, "  positivity 6h=%.2f 12h=%.2f 24h=%.2f\n", c.Pos6h, c.Pos12h, c.Pos24h)
	}
	if c.SourceTitle != "" {
		fmt.Fprintf(w, "  source: %s\n", c.SourceTitle)
	}
	if c.URL != "" {
		fmt.Fprintf(w, "  link:   %s\n", c.URL)
	}
	if c.Permalink != "" {
		fmt.Fprintf(w, "  digg:   %s\n", c.Permalink)
	}
	if c.TLDR != "" {
		fmt.Fprintf(w, "\n%s\n", c.TLDR)
	}
	if c.ReplacementRationale != "" {
		fmt.Fprintf(w, "\nreplacement rationale: %s\n", c.ReplacementRationale)
	}

	// Authors
	if authorsJSON != "" && authorsJSON != "[]" && authorsJSON != "null" {
		var authors []diggparse.ClusterAuthor
		if err := json.Unmarshal([]byte(authorsJSON), &authors); err == nil && len(authors) > 0 {
			fmt.Fprintf(w, "\ncontributors (%d):\n", len(authors))
			for i, a := range authors {
				if i >= 10 {
					fmt.Fprintf(w, "  ... and %d more\n", len(authors)-i)
					break
				}
				name := firstNonEmpty(a.DisplayName, a.Username)
				fmt.Fprintf(w, "  - @%s (%s) [%s]\n", a.Username, name, a.PostType)
			}
		}
	}
	return nil
}

// ============== search ==============

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "search [query]",
		Short:       "Full-text search over cluster titles, labels, and TLDRs",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  digg-pp-cli search "openai gpt-5"
  digg-pp-cli search "robotics" --json --select clusterUrlId,label,tldr`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.Join(args, " ")
			_, db, closeFn, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer closeFn()
			rows, err := db.QueryContext(cmd.Context(),
				`SELECT c.cluster_id, c.cluster_url_id, COALESCE(c.label,''), COALESCE(c.tldr,''), COALESCE(c.current_rank,0)
				 FROM digg_clusters_fts f JOIN digg_clusters c ON c.cluster_id = f.cluster_id
				 WHERE digg_clusters_fts MATCH ?
				 ORDER BY c.current_rank ASC LIMIT ?`, query, limit)
			if err != nil {
				return fmt.Errorf("FTS query: %w", err)
			}
			defer rows.Close()
			type result struct {
				ClusterID    string `json:"clusterId"`
				ClusterURLID string `json:"clusterUrlId"`
				Label        string `json:"label"`
				TLDR         string `json:"tldr,omitempty"`
				CurrentRank  int    `json:"currentRank,omitempty"`
			}
			var results []result
			for rows.Next() {
				var r result
				if err := rows.Scan(&r.ClusterID, &r.ClusterURLID, &r.Label, &r.TLDR, &r.CurrentRank); err != nil {
					return err
				}
				results = append(results, r)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			if len(results) == 0 {
				return emptyHint(cmd, fmt.Sprintf("no matches for %q. Try a different query, or `digg-pp-cli sync` if the local store is empty.", query))
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "#%d  %s  [%s]\n", r.CurrentRank, r.Label, r.ClusterURLID)
				if r.TLDR != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "    %s\n", diggTruncate(r.TLDR, 200))
				}
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Max number of results")
	return cmd
}

// ============== events ==============

func newEventsCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var sinceStr string
	var typeFilter string
	cmd := &cobra.Command{
		Use:         "events",
		Short:       "Tail Digg's ingestion-pipeline event stream from the local store",
		Long:        `Read events that were captured during sync from /api/trending/status: cluster_detected, fast_climb (with delta + previousRank → currentRank), post_understanding (X posts being processed), batch_started/batch_breakdown/posts_stored, and embedding_progress.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  digg-pp-cli events --since 1h --type fast_climb
  digg-pp-cli events --type cluster_detected --limit 10 --json
  digg-pp-cli events --json --select clusterId,label,delta,currentRank,previousRank`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			since := parseSinceWithFallback(sinceStr, 24*time.Hour)
			_, db, closeFn, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer closeFn()

			q := `SELECT id, type, COALESCE(cluster_id,''), COALESCE(label,''), COALESCE(username,''), COALESCE(post_type,''), COALESCE(permalink,''),
				COALESCE(delta,0), COALESCE(current_rank,0), COALESCE(previous_rank,0), COALESCE(count,0), COALESCE(total,0),
				COALESCE(at,''), COALESCE(created_at,''), COALESCE(raw_json,'')
				FROM digg_events WHERE 1=1`
			var argsSQL []any
			if !since.IsZero() {
				q += ` AND at >= ?`
				argsSQL = append(argsSQL, since.UTC().Format(time.RFC3339))
			}
			if typeFilter != "" {
				q += ` AND type = ?`
				argsSQL = append(argsSQL, typeFilter)
			}
			q += ` ORDER BY at DESC LIMIT ?`
			argsSQL = append(argsSQL, limit)

			rows, err := db.QueryContext(cmd.Context(), q, argsSQL...)
			if err != nil {
				return err
			}
			defer rows.Close()
			type evRow struct {
				ID           string `json:"id"`
				Type         string `json:"type"`
				ClusterID    string `json:"clusterId,omitempty"`
				Label        string `json:"label,omitempty"`
				Username     string `json:"username,omitempty"`
				PostType     string `json:"postType,omitempty"`
				Permalink    string `json:"permalink,omitempty"`
				Delta        int    `json:"delta,omitempty"`
				CurrentRank  int    `json:"currentRank,omitempty"`
				PreviousRank int    `json:"previousRank,omitempty"`
				Count        int    `json:"count,omitempty"`
				Total        int    `json:"total,omitempty"`
				At           string `json:"at"`
				CreatedAt    string `json:"createdAt,omitempty"`
			}
			var out []evRow
			for rows.Next() {
				var e evRow
				var rawJSON string
				if err := rows.Scan(&e.ID, &e.Type, &e.ClusterID, &e.Label, &e.Username, &e.PostType, &e.Permalink,
					&e.Delta, &e.CurrentRank, &e.PreviousRank, &e.Count, &e.Total,
					&e.At, &e.CreatedAt, &rawJSON); err != nil {
					return err
				}
				out = append(out, e)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			if len(out) == 0 {
				return emptyHint(cmd, "no events in window. Run `digg-pp-cli sync` first, or widen --since.")
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			for _, e := range out {
				switch e.Type {
				case "fast_climb":
					fmt.Fprintf(cmd.OutOrStdout(), "[%s] fast_climb %+d  %d→%d  %s\n", e.At, e.Delta, e.PreviousRank, e.CurrentRank, e.Label)
				case "cluster_detected":
					fmt.Fprintf(cmd.OutOrStdout(), "[%s] cluster_detected  %s\n", e.At, e.Label)
				case "post_understanding":
					fmt.Fprintf(cmd.OutOrStdout(), "[%s] post @%s [%s]  %s\n", e.At, e.Username, e.PostType, e.Permalink)
				default:
					fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s  count=%d\n", e.At, e.Type, e.Count)
				}
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Max number of events to return")
	cmd.Flags().StringVar(&sinceStr, "since", "24h", "Only events at-or-after this duration ago (e.g. 30m, 6h, 2d) or RFC3339")
	cmd.Flags().StringVar(&typeFilter, "type", "", "Filter to event type (cluster_detected, fast_climb, post_understanding, batch_started, batch_breakdown, posts_stored, embedding_progress)")
	return cmd
}

// ============== evidence ==============

func newEvidenceCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "evidence [clusterUrlId]",
		Short:       "Print Digg's published score components and evidence array for one cluster",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  digg-pp-cli evidence iq7usf9e
  digg-pp-cli evidence iq7usf9e --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id := args[0]
			_, db, closeFn, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer closeFn()
			row := db.QueryRowContext(cmd.Context(),
				`SELECT cluster_id, COALESCE(cluster_url_id,''), COALESCE(label,''),
				 COALESCE(gravity_score,0), COALESCE(numerator_count,0), COALESCE(numerator_label,''),
				 COALESCE(percent_above_average,0), COALESCE(score_components_json,''), COALESCE(evidence_json,'')
				 FROM digg_clusters WHERE cluster_url_id = ? OR cluster_id = ?`, id, id)
			var clusterID, urlID, label string
			var gravity, pct float64
			var numeratorCount int
			var numeratorLabel, scoreJSON, evJSON string
			if err := row.Scan(&clusterID, &urlID, &label, &gravity, &numeratorCount, &numeratorLabel, &pct, &scoreJSON, &evJSON); err != nil {
				if err == sql.ErrNoRows {
					return fmt.Errorf("cluster not found: %s", id)
				}
				return err
			}
			result := map[string]any{
				"clusterId":           clusterID,
				"clusterUrlId":        urlID,
				"label":               label,
				"gravityScore":        gravity,
				"numeratorCount":      numeratorCount,
				"numeratorLabel":      numeratorLabel,
				"percentAboveAverage": pct,
				"scoreComponents":     asJSONString(scoreJSON),
				"evidence":            asJSONString(evJSON),
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s [%s]\n", label, urlID)
			fmt.Fprintf(cmd.OutOrStdout(), "  gravityScore: %.4f\n", gravity)
			if numeratorLabel != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s: %d\n", numeratorLabel, numeratorCount)
			}
			if pct > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  percentAboveAverage: %.2f\n", pct)
			}
			if scoreJSON != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  scoreComponents:\n    %s\n", indentJSON(scoreJSON, 4))
			}
			if evJSON != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  evidence:\n    %s\n", indentJSON(evJSON, 4))
			}
			return nil
		},
	}
	return cmd
}

// ============== sentiment ==============

func newSentimentCmd(flags *rootFlags) *cobra.Command {
	var window string
	cmd := &cobra.Command{
		Use:         "sentiment [clusterUrlId]",
		Short:       "Print per-time-window positivity ratios for one cluster (pos6h/pos12h/pos24h)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  digg-pp-cli sentiment iq7usf9e
  digg-pp-cli sentiment iq7usf9e --window 6h
  digg-pp-cli sentiment iq7usf9e --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id := args[0]
			_, db, closeFn, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer closeFn()
			row := db.QueryRowContext(cmd.Context(),
				`SELECT cluster_id, COALESCE(cluster_url_id,''), COALESCE(label,''),
				 COALESCE(pos6h,0), COALESCE(pos12h,0), COALESCE(pos24h,0), COALESCE(pos_last,0)
				 FROM digg_clusters WHERE cluster_url_id = ? OR cluster_id = ?`, id, id)
			var clusterID, urlID, label string
			var p6, p12, p24, pLast float64
			if err := row.Scan(&clusterID, &urlID, &label, &p6, &p12, &p24, &pLast); err != nil {
				if err == sql.ErrNoRows {
					return fmt.Errorf("cluster not found: %s", id)
				}
				return err
			}
			result := map[string]any{
				"clusterId":    clusterID,
				"clusterUrlId": urlID,
				"label":        label,
				"pos6h":        p6, "pos12h": p12, "pos24h": p24, "posLast": pLast,
			}
			if window != "" {
				switch window {
				case "6h":
					result["window"] = window
					result["positivity"] = p6
				case "12h":
					result["window"] = window
					result["positivity"] = p12
				case "24h":
					result["window"] = window
					result["positivity"] = p24
				default:
					return fmt.Errorf("--window must be 6h, 12h, or 24h")
				}
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s [%s]\n  pos6h=%.2f  pos12h=%.2f  pos24h=%.2f  posLast=%.2f\n",
				label, urlID, p6, p12, p24, pLast)
			return nil
		},
	}
	cmd.Flags().StringVar(&window, "window", "", "Restrict output to one window: 6h, 12h, or 24h")
	return cmd
}

// ============== crossref ==============

func newCrossrefCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "crossref [clusterUrlId]",
		Short:       "Show this cluster's Hacker News and Techmeme cross-references",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  digg-pp-cli crossref iq7usf9e
  digg-pp-cli crossref iq7usf9e --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id := args[0]
			_, db, closeFn, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer closeFn()
			row := db.QueryRowContext(cmd.Context(),
				`SELECT cluster_id, COALESCE(cluster_url_id,''), COALESCE(label,''),
				 COALESCE(url,''), COALESCE(permalink,''),
				 COALESCE(hacker_news_json,''), COALESCE(techmeme_json,''), COALESCE(external_feeds_json,'')
				 FROM digg_clusters WHERE cluster_url_id = ? OR cluster_id = ?`, id, id)
			var clusterID, urlID, label, sourceURL, perm, hnJSON, tmJSON, extJSON string
			if err := row.Scan(&clusterID, &urlID, &label, &sourceURL, &perm, &hnJSON, &tmJSON, &extJSON); err != nil {
				if err == sql.ErrNoRows {
					return fmt.Errorf("cluster not found: %s", id)
				}
				return err
			}
			result := map[string]any{
				"clusterId":    clusterID,
				"clusterUrlId": urlID,
				"label":        label,
				"source":       sourceURL,
				"diggURL":      perm,
				"hackerNews":   asJSONString(hnJSON),
				"techmeme":     asJSONString(tmJSON),
				"external":     asJSONString(extJSON),
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s [%s]\n", label, urlID)
			fmt.Fprintf(cmd.OutOrStdout(), "  digg:       %s\n", perm)
			if sourceURL != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  source:     %s\n", sourceURL)
			}
			if hnJSON != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  hackerNews: %s\n", indentJSON(hnJSON, 14))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  hackerNews: (not detected by Digg)\n")
			}
			if tmJSON != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  techmeme:   %s\n", indentJSON(tmJSON, 14))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  techmeme:   (not detected by Digg)\n")
			}
			return nil
		},
	}
	return cmd
}

// ============== replaced ==============

func newReplacedCmd(flags *rootFlags) *cobra.Command {
	var sinceStr string
	var limit int
	cmd := &cobra.Command{
		Use:         "replaced",
		Short:       "Stories that were knocked out of the rankings, with Digg's published rationale",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  digg-pp-cli replaced --since 24h
  digg-pp-cli replaced --json --select clusterUrlId,label,rationale,previousRank`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			since := parseSinceWithFallback(sinceStr, 24*time.Hour)
			_, db, closeFn, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer closeFn()
			rows, err := db.QueryContext(cmd.Context(),
				`SELECT cluster_id, COALESCE(cluster_url_id,''), COALESCE(label,''),
				 observed_at, COALESCE(rationale,''), COALESCE(previous_rank,0)
				 FROM digg_replacements WHERE observed_at >= ?
				 ORDER BY observed_at DESC LIMIT ?`,
				since.UTC().Format(time.RFC3339Nano), limit)
			if err != nil {
				return err
			}
			defer rows.Close()
			type repRow struct {
				ClusterID    string `json:"clusterId"`
				ClusterURLID string `json:"clusterUrlId"`
				Label        string `json:"label"`
				ObservedAt   string `json:"observedAt"`
				Rationale    string `json:"rationale"`
				PreviousRank int    `json:"previousRank,omitempty"`
			}
			var out []repRow
			for rows.Next() {
				var r repRow
				if err := rows.Scan(&r.ClusterID, &r.ClusterURLID, &r.Label, &r.ObservedAt, &r.Rationale, &r.PreviousRank); err != nil {
					return err
				}
				out = append(out, r)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			if len(out) == 0 {
				return emptyHint(cmd, "no replacements recorded in window. Replacement archaeology needs at least 2 syncs over time.")
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			for _, r := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s [%s]  was rank #%d\n  rationale: %s\n",
					r.ObservedAt, r.Label, r.ClusterURLID, r.PreviousRank, r.Rationale)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&sinceStr, "since", "24h", "Lookback window (e.g. 6h, 24h, 7d)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max number of replacements to return")
	return cmd
}

// ============== history ==============

func newHistoryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "history [clusterUrlId]",
		Short:       "Show the rank trajectory of one cluster from local snapshot history",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  digg-pp-cli history iq7usf9e
  digg-pp-cli history iq7usf9e --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id := args[0]
			_, db, closeFn, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer closeFn()
			// Resolve clusterUrlId → clusterId
			var clusterID, label, urlID string
			err = db.QueryRowContext(cmd.Context(),
				`SELECT cluster_id, COALESCE(label,''), COALESCE(cluster_url_id,'')
				 FROM digg_clusters WHERE cluster_url_id = ? OR cluster_id = ?`, id, id).Scan(&clusterID, &label, &urlID)
			if err != nil {
				if err == sql.ErrNoRows {
					return fmt.Errorf("cluster not found: %s", id)
				}
				return err
			}
			rows, err := db.QueryContext(cmd.Context(),
				`SELECT fetched_at, COALESCE(current_rank,0), COALESCE(peak_rank,0), COALESCE(delta,0), COALESCE(gravity_score,0)
				 FROM digg_snapshots WHERE cluster_id = ? ORDER BY fetched_at ASC`, clusterID)
			if err != nil {
				return err
			}
			defer rows.Close()
			type snap struct {
				At           string  `json:"at"`
				Rank         int     `json:"rank"`
				PeakRank     int     `json:"peakRank,omitempty"`
				Delta        int     `json:"delta"`
				GravityScore float64 `json:"gravityScore,omitempty"`
			}
			var snaps []snap
			for rows.Next() {
				var s snap
				if err := rows.Scan(&s.At, &s.Rank, &s.PeakRank, &s.Delta, &s.GravityScore); err != nil {
					return err
				}
				snaps = append(snaps, s)
			}
			result := map[string]any{
				"clusterId":    clusterID,
				"clusterUrlId": urlID,
				"label":        label,
				"snapshots":    snaps,
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			if len(snaps) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "%s [%s]\n  no snapshots yet — run sync over time to build history.\n", label, urlID)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s [%s]\n  %d snapshots\n", label, urlID, len(snaps))
			for _, s := range snaps {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  rank=%d  peak=%d  delta=%+d  gravity=%.2f\n",
					s.At, s.Rank, s.PeakRank, s.Delta, s.GravityScore)
			}
			return nil
		},
	}
	return cmd
}

// ============== authors ==============

func newAuthorsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "authors",
		Short:       "Inspect the Digg AI 1000 — the curated leaderboard of AI accounts on X",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newAuthorsTopCmd(flags))
	cmd.AddCommand(newAuthorsListCmd(flags))
	return cmd
}

func newAuthorsTopCmd(flags *rootFlags) *cobra.Command {
	var by string
	var limit int
	cmd := &cobra.Command{
		Use:         "top",
		Short:       "Top contributors across the Digg AI 1000, ranked by influence, post count, or reach",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  digg-pp-cli authors top --by influence --limit 25
  digg-pp-cli authors top --by posts --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			_, db, closeFn, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer closeFn()
			var orderBy string
			switch by {
			case "influence":
				orderBy = "influence DESC"
			case "posts", "count":
				orderBy = "contributed_count DESC"
			case "reach":
				orderBy = "podist DESC"
			default:
				return fmt.Errorf("--by must be one of: influence, posts, reach")
			}
			rows, err := db.QueryContext(cmd.Context(),
				`SELECT username, COALESCE(display_name,''), COALESCE(x_id,''),
				 COALESCE(influence,0), COALESCE(podist,0), COALESCE(contributed_count,0), COALESCE(last_seen_at,'')
				 FROM digg_authors WHERE username != '' ORDER BY `+orderBy+` LIMIT ?`, limit)
			if err != nil {
				return err
			}
			defer rows.Close()
			type authorRow struct {
				Username         string  `json:"username"`
				DisplayName      string  `json:"displayName,omitempty"`
				XID              string  `json:"xId,omitempty"`
				Influence        float64 `json:"influence,omitempty"`
				Podist           float64 `json:"podist,omitempty"`
				ContributedCount int     `json:"contributedCount"`
				LastSeenAt       string  `json:"lastSeenAt,omitempty"`
			}
			var out []authorRow
			for rows.Next() {
				var a authorRow
				if err := rows.Scan(&a.Username, &a.DisplayName, &a.XID, &a.Influence, &a.Podist, &a.ContributedCount, &a.LastSeenAt); err != nil {
					return err
				}
				out = append(out, a)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			if len(out) == 0 {
				return emptyHint(cmd, "no authors known yet. Run `digg-pp-cli sync` to populate.")
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			for i, a := range out {
				name := firstNonEmpty(a.DisplayName, a.Username)
				fmt.Fprintf(cmd.OutOrStdout(), "%2d. @%-25s %s  influence=%.2f  posts=%d\n",
					i+1, a.Username, diggTruncate(name, 30), a.Influence, a.ContributedCount)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&by, "by", "influence", "Sort by: influence | posts | reach")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max number of authors")
	return cmd
}

// ============== authors list ==============
//
// `authors list` exposes the full ranked AI 1000 with rich per-author
// fields (rank, previousRank, rankChange, score, category, categoryRank,
// followers, bio, githubUrl, vibeDistribution). Live by default: hits
// /ai/1000, parses the embedded RSC stream, upserts into digg_authors,
// reads back from the local store. `--data-source local` skips the
// fetch and reads only the cached roster.
//
// This is the load-bearing surface for "biggest movers", "newly listed",
// and category-rank browsing — the upstream page renders all 1,000
// accounts in one shot but has no JSON endpoint, so the RSC parser plus
// local cache is the only way to make these views agent-friendly.

type rosterAuthorRow struct {
	Username           string             `json:"username"`
	DisplayName        string             `json:"displayName,omitempty"`
	XID                string             `json:"xId,omitempty"`
	Rank               int                `json:"rank"`
	PreviousRank       *int               `json:"previousRank"`
	RankChange         *int               `json:"rankChange"`
	Score              float64            `json:"score,omitempty"`
	Category           string             `json:"category,omitempty"`
	CategoryRank       int                `json:"categoryRank,omitempty"`
	CategoryConfidence float64            `json:"categoryConfidence,omitempty"`
	FollowersCount     int                `json:"followersCount,omitempty"`
	FollowedByCount    int                `json:"followedByCount,omitempty"`
	Bio                string             `json:"bio,omitempty"`
	GithubURL          string             `json:"githubUrl,omitempty"`
	ProfileImageURL    string             `json:"profileImageUrl,omitempty"`
	VibeDistribution   map[string]float64 `json:"vibeDistribution,omitempty"`
	VibeTweetCount     int                `json:"vibeTweetCount,omitempty"`
	XURL               string             `json:"xUrl,omitempty"`
	LastSeenAt         string             `json:"lastSeenAt,omitempty"`
}

type rosterEnvelope struct {
	Meta    map[string]any    `json:"meta"`
	Results []rosterAuthorRow `json:"results"`
}

func newAuthorsListCmd(flags *rootFlags) *cobra.Command {
	var by string
	var limit int
	var category string
	var onlyNew bool
	var onlyFallers bool
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "Full ranked AI 1000 with rich fields (rank, category, bio, vibeDistribution); fetches /ai/1000 by default",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `List the full Digg AI 1000 with per-author rank, category, bio, GitHub URL, and vibe distribution.

Live by default: fetches /ai/1000, parses the embedded RSC payload, upserts
the roster into the local store, then reads back. Pass --data-source local
to skip the network fetch and use only what's already cached.

Sortable by:
  rank          ascending (default)
  rankChange    biggest movers first (positive=climbed, negative=fell)
  category      grouped by category, then categoryRank ascending
  followers     followers_count descending`,
		Example: `  digg-pp-cli authors list --limit 20
  digg-pp-cli authors list --by rankChange --limit 10 --json
  digg-pp-cli authors list --only-new --json
  digg-pp-cli authors list --category "AI Safety" --json
  digg-pp-cli authors list --data-source local --limit 1000 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			_, db, closeFn, err := openStore(ctx)
			if err != nil {
				return err
			}
			defer closeFn()

			source := "live"
			ds := flags.dataSource
			if ds == "local" {
				source = "local"
			} else {
				// Live fetch + upsert.
				html, ferr := fetchURL(ctx, "https://di.gg/ai/1000")
				if ferr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: live /ai/1000 fetch failed (%v); falling back to local cache\n", ferr)
					source = "local"
				} else {
					authors, perr := diggparse.ParseRoster1000(html)
					if perr != nil && len(authors) == 0 {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: /ai/1000 parse failed (%v); falling back to local cache\n", perr)
						source = "local"
					} else {
						if perr != nil {
							// Partial parse — keep going.
							fmt.Fprintf(cmd.ErrOrStderr(), "warning: /ai/1000 partial parse: %v\n", perr)
						}
						if _, err := diggstore.UpsertRoster1000(db, authors, time.Now().UTC()); err != nil {
							return fmt.Errorf("persisting roster: %w", err)
						}
					}
				}
			}

			// When --category is set and the user didn't override --by,
			// sort by category_rank within the filter — that's what the
			// upstream "category leaderboard" view shows on di.gg/ai/1000.
			effectiveBy := by
			if category != "" && (by == "" || by == "rank") {
				effectiveBy = "category"
			}
			orderBy, err := rosterOrderBy(effectiveBy)
			if err != nil {
				return err
			}

			var (
				where  []string
				params []any
			)
			where = append(where, "rank > 0")
			if onlyNew {
				where = append(where, "previous_rank IS NULL")
			}
			if onlyFallers {
				where = append(where, "rank_change IS NOT NULL AND rank_change < 0")
			}
			if category != "" {
				where = append(where, "category = ?")
				params = append(params, category)
			}

			query := `SELECT
				username, COALESCE(display_name,''), COALESCE(x_id,''),
				COALESCE(rank,0), previous_rank, rank_change, COALESCE(score,0),
				COALESCE(category,''), COALESCE(category_rank,0), COALESCE(category_confidence,0),
				COALESCE(followers_count,0), COALESCE(followed_by_count,0),
				COALESCE(bio,''), COALESCE(github_url,''), COALESCE(profile_image_url,''),
				COALESCE(vibe_distribution_json,''), COALESCE(vibe_tweet_count,0),
				COALESCE(last_seen_at,'')
				FROM digg_authors WHERE ` + strings.Join(where, " AND ") + ` ORDER BY ` + orderBy + ` LIMIT ?`
			params = append(params, limit)

			rows, err := db.QueryContext(ctx, query, params...)
			if err != nil {
				return fmt.Errorf("querying digg_authors: %w", err)
			}
			defer rows.Close()
			var out []rosterAuthorRow
			for rows.Next() {
				var r rosterAuthorRow
				var prevRank, rankChange sql.NullInt64
				var vibeJSON string
				if err := rows.Scan(
					&r.Username, &r.DisplayName, &r.XID,
					&r.Rank, &prevRank, &rankChange, &r.Score,
					&r.Category, &r.CategoryRank, &r.CategoryConfidence,
					&r.FollowersCount, &r.FollowedByCount,
					&r.Bio, &r.GithubURL, &r.ProfileImageURL,
					&vibeJSON, &r.VibeTweetCount,
					&r.LastSeenAt,
				); err != nil {
					return err
				}
				if prevRank.Valid {
					v := int(prevRank.Int64)
					r.PreviousRank = &v
				}
				if rankChange.Valid {
					v := int(rankChange.Int64)
					r.RankChange = &v
				}
				if vibeJSON != "" {
					var m map[string]float64
					if err := json.Unmarshal([]byte(vibeJSON), &m); err == nil {
						r.VibeDistribution = m
					}
				}
				if r.Username != "" {
					r.XURL = "https://x.com/" + r.Username
				}
				out = append(out, r)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			if len(out) == 0 {
				if source == "local" {
					return emptyHint(cmd, "no authors in local store. Run `digg-pp-cli authors list` (without --data-source local) to ingest /ai/1000 first.")
				}
				return emptyHint(cmd, "no authors after applying filters. Try widening --category or dropping --only-new/--only-fallers.")
			}

			env := rosterEnvelope{
				Meta: map[string]any{
					"source": source,
					"by":     effectiveBy,
					"count":  len(out),
				},
				Results: out,
			}
			if onlyNew {
				env.Meta["onlyNew"] = true
			}
			if onlyFallers {
				env.Meta["onlyFallers"] = true
			}
			if category != "" {
				env.Meta["category"] = category
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), env, flags)
			}
			// Human-readable table.
			for i, a := range out {
				dn := firstNonEmpty(a.DisplayName, a.Username)
				prev := "-"
				if a.PreviousRank != nil {
					prev = fmt.Sprintf("%d", *a.PreviousRank)
				}
				change := ""
				if a.RankChange != nil {
					change = fmt.Sprintf(" (%+d)", *a.RankChange)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%4d. @%-22s %s  prev=%s%s  cat=%s\n",
					i+1, a.Username, diggTruncate(dn, 28), prev, change, a.Category)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&by, "by", "rank", "Sort by: rank | rankChange | category | followers")
	cmd.Flags().IntVar(&limit, "limit", 1000, "Max number of authors to return")
	cmd.Flags().StringVar(&category, "category", "", "Filter to authors with this category (e.g. \"AI Safety\")")
	cmd.Flags().BoolVar(&onlyNew, "only-new", false, "Only authors with previous_rank IS NULL (newly-listed)")
	cmd.Flags().BoolVar(&onlyFallers, "only-fallers", false, "Only authors with rank_change < 0")
	return cmd
}

// rosterOrderBy maps the --by flag to a SQL ORDER BY clause.
// Trailing tiebreaker on `rank ASC` keeps output stable.
func rosterOrderBy(by string) (string, error) {
	switch by {
	case "", "rank":
		return "rank ASC", nil
	case "rankChange", "rank_change":
		// Largest absolute movers first (positive climbs OR negative falls).
		// Nulls sort last so newly-listed accounts don't dominate when the
		// caller asked for movement.
		return "CASE WHEN rank_change IS NULL THEN 1 ELSE 0 END ASC, ABS(rank_change) DESC, rank ASC", nil
	case "category":
		return "category ASC, category_rank ASC, rank ASC", nil
	case "followers":
		return "followers_count DESC, rank ASC", nil
	default:
		return "", fmt.Errorf("--by must be one of: rank, rankChange, category, followers")
	}
}

// ============== author ==============

func newAuthorCmd(flags *rootFlags) *cobra.Command {
	var sinceStr string
	cmd := &cobra.Command{
		Use:         "author [username]",
		Short:       "Show every cluster a given X account contributed to",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  digg-pp-cli author Scobleizer
  digg-pp-cli author GaryMarcus --since 7d --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			username := strings.TrimPrefix(args[0], "@")
			since := parseSinceWithFallback(sinceStr, 30*24*time.Hour)
			_, db, closeFn, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer closeFn()
			rows, err := db.QueryContext(cmd.Context(),
				`SELECT c.cluster_id, c.cluster_url_id, COALESCE(c.label,''), COALESCE(c.current_rank,0),
				 COALESCE(c.activity_at,''), COALESCE(ca.post_type,''), COALESCE(ca.post_permalink,'')
				 FROM digg_cluster_authors ca
				 JOIN digg_clusters c ON c.cluster_id = ca.cluster_id
				 WHERE ca.username = ? AND COALESCE(c.activity_at, c.last_seen_at) >= ?
				 ORDER BY COALESCE(c.activity_at, c.last_seen_at) DESC`,
				username, since.UTC().Format(time.RFC3339Nano))
			if err != nil {
				return err
			}
			defer rows.Close()
			type contribRow struct {
				ClusterID     string `json:"clusterId"`
				ClusterURLID  string `json:"clusterUrlId"`
				Label         string `json:"label"`
				CurrentRank   int    `json:"currentRank"`
				ActivityAt    string `json:"activityAt,omitempty"`
				PostType      string `json:"postType,omitempty"`
				PostPermalink string `json:"postPermalink,omitempty"`
			}
			var out []contribRow
			for rows.Next() {
				var r contribRow
				if err := rows.Scan(&r.ClusterID, &r.ClusterURLID, &r.Label, &r.CurrentRank, &r.ActivityAt, &r.PostType, &r.PostPermalink); err != nil {
					return err
				}
				out = append(out, r)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			if len(out) == 0 {
				return emptyHint(cmd, fmt.Sprintf("no contributions seen for @%s in the window. Try `--since 30d` or run `digg-pp-cli sync` first.", username))
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "@%s contributed to %d clusters\n", username, len(out))
			for _, r := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "  #%-3d %s [%s] (%s)\n", r.CurrentRank, diggTruncate(r.Label, 80), r.ClusterURLID, r.PostType)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&sinceStr, "since", "30d", "Lookback window")
	return cmd
}

// ============== watch ==============

func newWatchCmd(flags *rootFlags) *cobra.Command {
	var interval time.Duration
	var minDelta int
	var iterations int
	var alertExpr string
	cmd := &cobra.Command{
		Use:         "watch",
		Short:       "Poll /ai on an interval and alert when any cluster moves N+ ranks",
		Long:        "Polls Digg, parses the feed, diffs against the previous local snapshot, and prints any cluster whose absolute rank delta is at-or-above --min-delta (or matches --alert). READ-ONLY: never writes anything to Digg.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  digg-pp-cli watch --interval 60s --min-delta 5
  digg-pp-cli watch --alert 'rank.delta>=10'
  digg-pp-cli watch --interval 30s --iterations 3   # for verify`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			_, db, closeFn, err := openStore(ctx)
			if err != nil {
				return err
			}
			defer closeFn()
			it := 0
			for {
				it++
				html, err := fetchURL(ctx, "https://di.gg/ai")
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "watch: fetch error: %v\n", err)
				} else {
					clusters, _, _, err := diggparse.ParseHomeFeed(html)
					if err == nil {
						alerts := computeWatchAlerts(db, clusters, minDelta)
						now := time.Now().UTC().Format(time.RFC3339)
						if len(alerts) == 0 {
							fmt.Fprintf(cmd.OutOrStdout(), "[%s] watch: %d clusters, no movers >= %d\n", now, len(clusters), minDelta)
						} else {
							fmt.Fprintf(cmd.OutOrStdout(), "[%s] watch: %d alerts\n", now, len(alerts))
							for _, a := range alerts {
								fmt.Fprintf(cmd.OutOrStdout(), "  %+d  %s [%s]\n", a.Delta, a.Label, a.ClusterURLID)
							}
						}
						// Persist snapshots so future polls have history
						for _, c := range clusters {
							_ = diggstore.UpsertCluster(db, c, time.Now())
						}
					}
				}
				if iterations > 0 && it >= iterations {
					return nil
				}
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(interval):
				}
			}
		},
	}
	cmd.Flags().DurationVar(&interval, "interval", 60*time.Second, "Poll interval (e.g. 30s, 2m)")
	cmd.Flags().IntVar(&minDelta, "min-delta", 5, "Minimum |rank delta| to alert on")
	cmd.Flags().IntVar(&iterations, "iterations", 0, "Stop after N iterations (0 = run until interrupted)")
	cmd.Flags().StringVar(&alertExpr, "alert", "", "Shorthand for --min-delta: 'rank.delta>=N' sets --min-delta to N")
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if alertExpr != "" {
			// Accept the shorthand form: "rank.delta>=N" or "rank.delta>N"
			parts := strings.Split(alertExpr, ">=")
			if len(parts) != 2 {
				parts = strings.Split(alertExpr, ">")
			}
			if len(parts) == 2 {
				if n, err := fmt.Sscanf(parts[1], "%d", &minDelta); err == nil && n == 1 {
					return nil
				}
			}
			return fmt.Errorf("--alert format must be rank.delta>=N (got %q)", alertExpr)
		}
		return nil
	}
	return cmd
}

type watchAlert struct {
	ClusterID    string
	ClusterURLID string
	Label        string
	Delta        int
}

func computeWatchAlerts(db *sql.DB, current []diggparse.Cluster, minDelta int) []watchAlert {
	prev := make(map[string]int)
	rows, err := db.Query(`SELECT cluster_id, COALESCE(current_rank,0) FROM digg_clusters
		WHERE last_seen_at = (SELECT MAX(last_seen_at) FROM digg_clusters)`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var rank int
		if err := rows.Scan(&id, &rank); err == nil {
			prev[id] = rank
		}
	}
	var out []watchAlert
	for _, c := range current {
		oldRank, hadPrev := prev[c.ClusterID]
		if !hadPrev || c.CurrentRank == 0 {
			continue
		}
		delta := oldRank - c.CurrentRank // climbing → positive
		if abs(delta) < minDelta {
			continue
		}
		out = append(out, watchAlert{
			ClusterID:    c.ClusterID,
			ClusterURLID: c.ClusterURLID,
			Label:        c.Label,
			Delta:        delta,
		})
	}
	sort.Slice(out, func(i, j int) bool { return abs(out[i].Delta) > abs(out[j].Delta) })
	return out
}

// ============== pipeline ==============

func newPipelineCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "pipeline",
		Short:       "Inspect Digg's ingestion pipeline (status + events)",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newPipelineStatusCmd(flags))
	return cmd
}

func newPipelineStatusCmd(flags *rootFlags) *cobra.Command {
	var watchMode bool
	var interval time.Duration
	cmd := &cobra.Command{
		Use:         "status",
		Short:       "One-screen view of /api/trending/status: isFetching, nextFetchAt, storiesToday, clustersToday",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  digg-pp-cli pipeline status
  digg-pp-cli pipeline status --watch --interval 60s`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			fetchOnce := func() error {
				body, err := fetchURL(cmd.Context(), "https://di.gg/api/trending/status")
				if err != nil {
					return err
				}
				ts, err := diggparse.ParseTrendingStatus(body)
				if err != nil {
					return err
				}
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), ts, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Pipeline status (computed at %s)\n", ts.ComputedAt)
				fmt.Fprintf(cmd.OutOrStdout(), "  isFetching:           %v\n", ts.IsFetching)
				fmt.Fprintf(cmd.OutOrStdout(), "  storiesToday:         %d\n", ts.StoriesToday)
				fmt.Fprintf(cmd.OutOrStdout(), "  clustersToday:        %d\n", ts.ClustersToday)
				fmt.Fprintf(cmd.OutOrStdout(), "  nextFetchAt:          %s\n", ts.NextFetchAt)
				fmt.Fprintf(cmd.OutOrStdout(), "  lastFetchCompletedAt: %s\n", ts.LastFetchCompletedAt)
				fmt.Fprintf(cmd.OutOrStdout(), "  recent events: %d\n", len(ts.Events))
				for i, e := range ts.Events {
					if i >= 5 {
						break
					}
					fmt.Fprintf(cmd.OutOrStdout(), "    %s  %s\n", e.At, e.Type)
				}
				return nil
			}
			if !watchMode {
				return fetchOnce()
			}
			for {
				if err := fetchOnce(); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "status: %v\n", err)
				}
				select {
				case <-cmd.Context().Done():
					return nil
				case <-time.After(interval):
				}
			}
		},
	}
	cmd.Flags().BoolVar(&watchMode, "watch", false, "Re-poll the endpoint on an interval")
	cmd.Flags().DurationVar(&interval, "interval", 60*time.Second, "Poll interval")
	return cmd
}

// ============== open ==============

func newOpenCmd(flags *rootFlags) *cobra.Command {
	var launch bool
	cmd := &cobra.Command{
		Use:         "open [clusterUrlId]",
		Short:       "Print or open the Digg URL for one cluster",
		Long:        "Prints the digg.com permalink for the given cluster. By default does NOT launch a browser; pass --launch to actually open. Per the printing-press side-effect convention.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  # Print the URL only (default; safe in scripts)
  digg-pp-cli open iq7usf9e

  # Actually launch the browser
  digg-pp-cli open iq7usf9e --launch`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id := args[0]
			_, db, closeFn, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer closeFn()
			var perm, urlID string
			err = db.QueryRowContext(cmd.Context(),
				`SELECT COALESCE(permalink,''), COALESCE(cluster_url_id,'')
				 FROM digg_clusters WHERE cluster_url_id = ? OR cluster_id = ?`, id, id).Scan(&perm, &urlID)
			if err != nil {
				if err == sql.ErrNoRows {
					return fmt.Errorf("cluster not found: %s (run sync first)", id)
				}
				return err
			}
			if perm == "" {
				perm = "https://di.gg/ai/" + urlID
			}
			// Verify-environment short-circuit (printing-press side-effect convention).
			if isVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would launch: %s\n", perm)
				return nil
			}
			if !launch {
				fmt.Fprintf(cmd.OutOrStdout(), "would launch: %s\n", perm)
				fmt.Fprintf(cmd.OutOrStdout(), "(re-run with --launch to actually open in your browser)\n")
				return nil
			}
			return launchBrowser(perm)
		},
	}
	cmd.Flags().BoolVar(&launch, "launch", false, "Actually open the URL in the default browser")
	return cmd
}

// ============== stats ==============

func newStatsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "stats",
		Short:       "Show local store statistics: cluster count, author count, snapshot history depth",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  digg-pp-cli stats
  digg-pp-cli stats --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			_, db, closeFn, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer closeFn()
			var s struct {
				Clusters     int    `json:"clusters"`
				Authors      int    `json:"authors"`
				Snapshots    int    `json:"snapshots"`
				Events       int    `json:"events"`
				Replacements int    `json:"replacements"`
				LastSync     string `json:"lastSync,omitempty"`
			}
			_ = db.QueryRowContext(cmd.Context(), `SELECT COUNT(*) FROM digg_clusters`).Scan(&s.Clusters)
			_ = db.QueryRowContext(cmd.Context(), `SELECT COUNT(*) FROM digg_authors`).Scan(&s.Authors)
			_ = db.QueryRowContext(cmd.Context(), `SELECT COUNT(*) FROM digg_snapshots`).Scan(&s.Snapshots)
			_ = db.QueryRowContext(cmd.Context(), `SELECT COUNT(*) FROM digg_events`).Scan(&s.Events)
			_ = db.QueryRowContext(cmd.Context(), `SELECT COUNT(*) FROM digg_replacements`).Scan(&s.Replacements)
			_ = db.QueryRowContext(cmd.Context(), `SELECT MAX(last_seen_at) FROM digg_clusters`).Scan(&s.LastSync)
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), s, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Local store: %d clusters, %d authors, %d snapshots, %d events, %d replacements\n",
				s.Clusters, s.Authors, s.Snapshots, s.Events, s.Replacements)
			if s.LastSync != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  last sync: %s\n", s.LastSync)
			}
			return nil
		},
	}
	return cmd
}

// ============== shared helpers ==============

func renderClusterTable(w io.Writer, rows []clusterRow) error {
	for _, c := range rows {
		extra := ""
		if c.Delta != 0 {
			extra = fmt.Sprintf("  delta=%+d", c.Delta)
		}
		display := firstNonEmpty(c.Label, c.Title, "(no label)")
		fmt.Fprintf(w, "#%-3d %s [%s]%s\n", c.CurrentRank, diggTruncate(display, 100), c.ClusterURLID, extra)
		if c.TLDR != "" {
			fmt.Fprintf(w, "    %s\n", diggTruncate(c.TLDR, 200))
		}
	}
	return nil
}

func printClusterOutput(cmd *cobra.Command, flags *rootFlags, rows []clusterRow, render func(io.Writer, []clusterRow) error) error {
	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
	}
	return render(cmd.OutOrStdout(), rows)
}

func emptyHint(cmd *cobra.Command, hint string) error {
	fmt.Fprintln(cmd.OutOrStdout(), hint)
	return nil
}

func diggTruncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func firstNonEmpty(xs ...string) string {
	for _, x := range xs {
		if x != "" {
			return x
		}
	}
	return ""
}

func indentJSON(s string, indent int) string {
	if s == "" {
		return ""
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return s
	}
	pad := strings.Repeat(" ", indent)
	pretty, err := json.MarshalIndent(v, pad, "  ")
	if err != nil {
		return s
	}
	return string(pretty)
}

func parseSinceWithFallback(s string, fallback time.Duration) time.Time {
	if s == "" {
		return time.Now().Add(-fallback)
	}
	if d, err := time.ParseDuration(s); err == nil {
		return time.Now().Add(-d)
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	// Custom day suffix
	if strings.HasSuffix(s, "d") {
		var days int
		if _, err := fmt.Sscanf(s, "%dd", &days); err == nil {
			return time.Now().Add(-time.Duration(days) * 24 * time.Hour)
		}
	}
	return time.Now().Add(-fallback)
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func isVerifyEnv() bool {
	return os.Getenv("PRINTING_PRESS_VERIFY") == "1" || os.Getenv("PRINTING_PRESS_VERIFY") == "true"
}

func launchBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	return cmd.Start()
}
