// Hand-authored novel command: evidence. Builds a cited evidence bundle
// mirroring the agent-context kinds, assembled entirely from the local mirror.
//
// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/techtwitter/internal/store"
)

type ttEvidenceRow struct {
	ID           string         `json:"id"`
	SourceType   string         `json:"sourceType"`
	Title        string         `json:"title,omitempty"`
	Summary      string         `json:"summary,omitempty"`
	CanonicalURL string         `json:"canonicalUrl,omitempty"`
	QualityScore float64        `json:"qualityScore"`
	Metrics      map[string]int `json:"metrics,omitempty"`
	WhyIncluded  string         `json:"whyIncluded,omitempty"`
}

type ttEvidenceBundle struct {
	Kind         string          `json:"kind"`
	Window       string          `json:"window"`
	GeneratedAt  string          `json:"generatedAt"`
	Instructions []string        `json:"instructions"`
	Evidence     []ttEvidenceRow `json:"evidence"`
}

var ttEvidenceKinds = map[string]bool{
	"auto": true, "what-changed": true, "arguments": true,
	"read-list": true, "narrative-alert": true,
}

func newNovelEvidenceCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var window string
	var limit int

	cmd := &cobra.Command{
		Use:   "evidence <kind>",
		Short: "Build an evidence bundle mirroring the agent-context kinds from local SQLite",
		Long: "Assemble a cited evidence bundle from the offline mirror for one kind: " +
			"what-changed, arguments, read-list, or narrative-alert (auto = what-changed). " +
			"Each row carries a canonical citation URL. The `launches` kind is live-only " +
			"(products aren't stored locally); use `agent --kind launches` for that. " +
			"Run `sync` first to populate the mirror.",
		Example:     "  techtwitter-pp-cli evidence read-list --agent --select evidence.title,evidence.canonicalUrl",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compose an evidence bundle from the local mirror")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("kind is required: what-changed, arguments, read-list, or narrative-alert"))
			}
			kind := args[0]
			if !ttEvidenceKinds[kind] {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("unknown kind %q (want: what-changed, arguments, read-list, narrative-alert)", kind))
			}
			if kind == "auto" {
				kind = "what-changed"
			}
			dur, err := ttParseWindow(window)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}

			dbPath = ttResolveDB(dbPath)
			if ttMissingMirror(cmd, flags, dbPath, "{}") {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			rows, err := buildEvidence(db, kind, ttCutoff(dur), limit)
			if err != nil {
				return err
			}
			bundle := ttEvidenceBundle{
				Kind:        kind,
				Window:      dur.String(),
				GeneratedAt: time.Now().UTC().Format(time.RFC3339),
				Instructions: []string{
					"Return evidence, not a generated prose answer.",
					"Use canonicalUrl values as citations in downstream answers.",
					"Treat summaries and metrics as evidence fields; do not infer unsupported facts.",
				},
				Evidence: rows,
			}

			if ttWantsJSON(cmd, flags) {
				return ttEmitJSON(cmd, flags, bundle)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s\n\n", bold(fmt.Sprintf("EVIDENCE — %s (last %s, %d rows)", kind, dur, len(rows))))
			for i, r := range rows {
				fmt.Fprintf(w, "  %2d. [%s] %s\n", i+1, r.SourceType, truncate(r.Title, 100))
				if r.CanonicalURL != "" {
					fmt.Fprintf(w, "      %s\n", r.CanonicalURL)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/techtwitter-pp-cli/data.db)")
	cmd.Flags().StringVar(&window, "window", "24h", "Lookback window (24h, 48h, 7d); ignored for the narrative-alert kind (always the current heatmap)")
	cmd.Flags().IntVar(&limit, "limit", 8, "Maximum evidence rows (max 20)")
	return cmd
}

func tweetEvidence(t ttTweet, why string) ttEvidenceRow {
	title := t.Summary
	if title == "" {
		title = truncate(t.Text, 100)
	}
	return ttEvidenceRow{
		ID:           t.ID,
		SourceType:   "tweet",
		Title:        title,
		Summary:      t.Summary,
		CanonicalURL: t.URL,
		QualityScore: t.Quality,
		Metrics:      map[string]int{"likes": t.Likes, "retweets": t.Retweets, "comments": t.Comments, "bookmarks": t.Bookmarks},
		WhyIncluded:  why,
	}
}

func buildEvidence(db *store.Store, kind, cutoff string, limit int) ([]ttEvidenceRow, error) {
	if limit > 20 {
		limit = 20
	}
	engExpr := "(COALESCE(bookmark_count,0)*4 + COALESCE(comment_count,0)*3 + COALESCE(retweet_count,0)*2 + COALESCE(like_count,0))"
	out := make([]ttEvidenceRow, 0, limit)

	switch kind {
	case "arguments":
		// High-reply, debate-heavy tweets from the stored hot-takes stream.
		raw, err := scanResourceData(db, "command-hot-takes")
		if err != nil {
			return nil, err
		}
		tweets := decodeTweets(raw)
		sort.Slice(tweets, func(i, j int) bool { return tweets[i].Comments > tweets[j].Comments })
		for i, t := range tweets {
			if i >= limit {
				break
			}
			out = append(out, tweetEvidence(t, "High reply volume — debate-heavy thread."))
		}

	case "narrative-alert":
		// The heatmap (`command` table) holds only the latest snapshot of topic
		// momentum, so this kind is time-invariant: `--window`/`cutoff` does not
		// apply here and is intentionally a no-op (documented in the command Long).
		trows, err := db.DB().Query(`SELECT keyword, COALESCE(slug,''), COALESCE(count,0), COALESCE(engagement,0)
			FROM command WHERE keyword IS NOT NULL ORDER BY engagement DESC LIMIT ?`, limit)
		if err != nil {
			return nil, err
		}
		defer trows.Close()
		for trows.Next() {
			var k, slug string
			var count, eng int
			if err := trows.Scan(&k, &slug, &count, &eng); err != nil {
				return nil, err
			}
			out = append(out, ttEvidenceRow{
				ID:           slug,
				SourceType:   "topic",
				Title:        k,
				Metrics:      map[string]int{"count": count, "engagement": eng},
				WhyIncluded:  "Topic with high recent momentum.",
				CanonicalURL: "https://www.techtwitter.com/topics/" + slug,
			})
		}
		if err := trows.Err(); err != nil {
			return nil, fmt.Errorf("iterating topic rows: %w", err)
		}

	case "read-list":
		tweets, err := ttScanTweets(db.DB(),
			`WHERE content_type != 'article' AND timestamp >= ? ORDER BY quality_score DESC, `+engExpr+` DESC LIMIT ?`,
			cutoff, limit)
		if err != nil {
			return nil, err
		}
		for _, t := range tweets {
			out = append(out, tweetEvidence(t, "High-quality curated tweet worth reading."))
		}
		articles, _ := ttScanArticles(db.DB(), `ORDER BY timestamp DESC LIMIT 3`)
		for _, a := range articles {
			out = append(out, ttEvidenceRow{
				ID:           a.ID,
				SourceType:   "article",
				Title:        a.Title,
				Summary:      a.Summary,
				CanonicalURL: a.URL,
				QualityScore: a.Quality,
				WhyIncluded:  "Recent long-form article.",
			})
		}

	default: // what-changed
		tweets, err := ttScanTweets(db.DB(),
			`WHERE content_type != 'article' AND timestamp >= ? ORDER BY timestamp DESC, `+engExpr+` DESC LIMIT ?`,
			cutoff, limit)
		if err != nil {
			return nil, err
		}
		for _, t := range tweets {
			out = append(out, tweetEvidence(t, "Recent curated tweet within the window."))
		}
	}
	return out, nil
}

// scanResourceData pulls the raw JSON payloads for a resource_type from the
// generic resources table.
func scanResourceData(db *store.Store, resourceType string) ([]json.RawMessage, error) {
	rows, err := db.DB().Query(`SELECT data FROM resources WHERE resource_type = ?`, resourceType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]json.RawMessage, 0)
	for rows.Next() {
		var d []byte
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		out = append(out, json.RawMessage(d))
	}
	return out, rows.Err()
}
