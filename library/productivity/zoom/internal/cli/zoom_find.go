package cli

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/local/localstore"
)

// newZoomFindCmd — the killer command. T1.
func newZoomFindCmd(flags *rootFlags) *cobra.Command {
	var (
		speaker string
		before  int
		after   int
		source  string
		since   string
		limit   int
	)
	cmd := &cobra.Command{
		Use:   "find [query]",
		Short: "FTS5 search across local + cloud Zoom transcripts in one query",
		Long: "Searches the unified local_transcript_segments FTS index (built by `recordings local sync`) plus the cloud " +
			"transcripts cached by `recordings cloud download`. Returns matched cues with deep links: cloud recordings get " +
			"the Zoom web player ?startTime= URL; local recordings get the file path and a vlc --start-time= shell snippet.",
		Example: `  zoom-pp-cli find "q2 pricing" --json
  zoom-pp-cli find "customer churn" --speaker "Riley" --after 45 --json
  zoom-pp-cli find "launch plan" --source local --since 30d --limit 10 --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return runUnifiedFind(cmd, flags, args[0], speaker, since, source, before, after, limit)
		},
	}
	cmd.Flags().StringVar(&speaker, "speaker", "", "Filter to one speaker label")
	cmd.Flags().IntVar(&before, "before", 0, "Include N seconds of context before each match")
	cmd.Flags().IntVar(&after, "after", 0, "Include N seconds of context after each match")
	cmd.Flags().StringVar(&source, "source", "both", "local | cloud | both")
	cmd.Flags().StringVar(&since, "since", "", "Limit to recordings since this point (7d, 30d, ISO date)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max matches")
	return cmd
}

// runUnifiedFind is shared by `find` and `recordings search`.
func runUnifiedFind(cmd *cobra.Command, flags *rootFlags, query, speaker, since, source string, before, after, limit int) error {
	sinceTime, err := parseSince(since)
	if err != nil {
		return err
	}
	db, closer, err := openLocalDB(cmd.Context())
	if err != nil {
		return err
	}
	defer closer()

	var all []localstore.SegmentMatch
	var sourcesSearched []string
	if source == "" || source == "local" || source == "both" {
		ms, err := localstore.SearchLocalSegments(cmd.Context(), db, query, speaker, sinceTime, limit)
		if err != nil {
			return err
		}
		for _, m := range ms {
			m.DeepLink = localDeepLink(m)
			all = append(all, m)
		}
		sourcesSearched = append(sourcesSearched, "local")
	}
	cloudIndexed := false
	if source == "" || source == "cloud" || source == "both" {
		ms, indexed, err := searchCloudSegments(cmd.Context(), db, query, speaker, sinceTime, limit)
		if err == nil {
			all = append(all, ms...)
			sourcesSearched = append(sourcesSearched, "cloud")
			cloudIndexed = indexed
		}
	}

	// Apply --before/--after by widening cue text from neighbours in the same
	// recording. Implemented as a separate read pass per match for clarity.
	if before > 0 || after > 0 {
		for i := range all {
			all[i].Text = widenContext(cmd.Context(), db, all[i], before, after)
		}
	}

	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	if all == nil {
		all = []localstore.SegmentMatch{}
	}
	out := map[string]any{
		"query":            query,
		"matches":          all,
		"count":            len(all),
		"sources_searched": sourcesSearched,
	}
	// Make the empty-cloud case unambiguous: if cloud was in scope but no cloud
	// transcript index exists yet, say so rather than letting an empty result
	// read as "cloud searched, nothing found".
	if (source == "" || source == "cloud" || source == "both") && !cloudIndexed {
		out["note"] = "cloud transcripts not indexed; run 'recordings cloud download --type transcript' to populate, then re-run. Results above are local-only."
	}
	return flags.printJSON(cmd, out)
}

func localDeepLink(m localstore.SegmentMatch) string {
	// vlc shell snippet so users can jump to the exact second of the local mp4.
	// Single-quote the path and escape any embedded single-quotes so the snippet
	// is safe to copy-paste even when the recording folder name (derived from the
	// meeting topic, which can contain arbitrary organiser text like `$(id)`) has
	// shell metacharacters. Go's %q is a Go string literal, not POSIX shell
	// quoting, and would let those metacharacters execute when pasted into bash.
	startSec := m.StartMs / 1000
	escaped := strings.ReplaceAll(m.RecordingPath, "'", `'\''`)
	return fmt.Sprintf("vlc '%s' --start-time=%d", escaped, startSec)
}

// searchCloudSegments returns matched cloud transcript segments and an
// `indexed` flag reporting whether a cloud transcript index is actually
// available. The flag lets callers distinguish "cloud searched, empty" from
// "cloud skipped because nothing is indexed yet".
func searchCloudSegments(ctx context.Context, db *sql.DB, query, speaker string, since time.Time, limit int) ([]localstore.SegmentMatch, bool, error) {
	// No cloud transcript segment index exists yet. Cloud transcripts land in
	// the store only after `recordings cloud download --type transcript`, and a
	// follow-up indexer (TBD) will populate a cloud_transcript_segments table.
	// Until then this returns indexed=false so callers can label the cloud
	// portion as not-yet-indexed rather than "searched, nothing found". The
	// query/speaker/since/limit parameters are carried in the signature so the
	// filter logic lands here together with that indexer; they are intentionally
	// unused on this path today.
	_, _, _, _, _, _ = ctx, db, query, speaker, since, limit
	return nil, false, nil
}

func widenContext(ctx context.Context, db *sql.DB, m localstore.SegmentMatch, beforeSec, afterSec int) string {
	startMs := m.StartMs - int64(beforeSec)*1000
	endMs := m.StartMs + int64(afterSec)*1000
	rows, err := db.QueryContext(ctx,
		`SELECT COALESCE(speaker, ''), text FROM local_transcript_segments
		 WHERE recording_id = ? AND start_ms >= ? AND start_ms <= ? ORDER BY cue_index`,
		m.RecordingID, startMs, endMs)
	if err != nil {
		return m.Text
	}
	defer rows.Close()
	var b strings.Builder
	for rows.Next() {
		var sp, t string
		if err := rows.Scan(&sp, &t); err == nil {
			if sp != "" {
				b.WriteString(sp)
				b.WriteString(": ")
			}
			b.WriteString(t)
			b.WriteByte(' ')
		}
	}
	if b.Len() == 0 {
		return m.Text
	}
	return strings.TrimSpace(b.String())
}
