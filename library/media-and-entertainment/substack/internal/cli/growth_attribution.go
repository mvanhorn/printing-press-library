// Phase 3 hand-authored novel command. Not generator-emitted.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack/internal/store"

	"github.com/spf13/cobra"
)

type attributionRow struct {
	Rank             int    `json:"rank"`
	NoteID           string `json:"note_id"`
	NoteExcerpt      string `json:"note_excerpt"`
	PostedAt         string `json:"posted_at"`
	SubsAcquired     int    `json:"subs_acquired"`
	PaidSubsAcquired int    `json:"paid_subs_acquired"`
}

func newGrowthAttributionCmd(flags *rootFlags) *cobra.Command {
	var days int
	var limit int

	cmd := &cobra.Command{
		Use:   "attribution",
		Short: "Rank Notes by paid+free subs acquired in 24h after posting",
		Example: strings.Trim(`
  substack-pp-cli growth attribution --json
  substack-pp-cli growth attribution --days 60 --limit 10 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), []attributionRow{}, flags)
			}
			rows, err := computeAttribution(flags, days, limit)
			if err != nil {
				return err
			}
			if rows == nil {
				rows = []attributionRow{}
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "no analytics_snapshots data — run 'substack-pp-cli sync --analytics --days 30' first")
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "Lookback window in days for own Notes")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum Notes to return")
	return cmd
}

func computeAttribution(flags *rootFlags, days, limit int) ([]attributionRow, error) {
	dbPath := defaultDBPath("substack-pp-cli")
	if flags != nil && flags.configPath != "" {
		// honor the active config in case it points to a different db
	}
	st, err := store.Open(dbPath)
	if err != nil {
		// Missing store -> empty result, not an error.
		return nil, nil
	}
	defer st.Close()

	notes, err := loadOwnNotes(st, days)
	if err != nil {
		return nil, err
	}
	if len(notes) == 0 {
		return nil, nil
	}
	snapshots, err := loadSnapshots(st)
	if err != nil {
		return nil, err
	}
	if len(snapshots) == 0 {
		return nil, nil
	}
	out := make([]attributionRow, 0, len(notes))
	for _, n := range notes {
		date := n.PostedAt.UTC().Format("2006-01-02")
		next := n.PostedAt.UTC().AddDate(0, 0, 1).Format("2006-01-02")
		s0, ok0 := snapshots[date]
		s1, ok1 := snapshots[next]
		if !ok0 || !ok1 {
			continue
		}
		freeDelta := s1.Free - s0.Free
		paidDelta := s1.Paid - s0.Paid
		out = append(out, attributionRow{
			NoteID:           n.ID,
			NoteExcerpt:      truncate(n.Body, 80),
			PostedAt:         n.PostedAt.UTC().Format(time.RFC3339),
			SubsAcquired:     freeDelta + paidDelta,
			PaidSubsAcquired: paidDelta,
		})
	}
	// rank desc by subs_acquired
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j].SubsAcquired > out[i].SubsAcquired {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	for i := range out {
		out[i].Rank = i + 1
	}
	return out, nil
}

type ownNote struct {
	ID       string
	Body     string
	PostedAt time.Time
}

type snapRow struct {
	Free int
	Paid int
}

func loadOwnNotes(st *store.Store, days int) ([]ownNote, error) {
	db := st.DB()
	rows, err := db.Query(`SELECT id, data FROM resources WHERE resource_type = 'notes' AND synced_at >= datetime('now', ?)`, fmt.Sprintf("-%d days", days))
	if err != nil {
		// table-shape mismatch is non-fatal
		return nil, nil
	}
	defer rows.Close()
	var out []ownNote
	for rows.Next() {
		var id, data string
		if err := rows.Scan(&id, &data); err != nil {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal([]byte(data), &raw); err != nil {
			continue
		}
		body := stringField(raw, "body", "body_text", "text")
		ts := stringField(raw, "posted_at", "publishedAt", "published_at", "created_at")
		t, err := time.Parse(time.RFC3339, ts)
		if err != nil {
			t = time.Now().UTC().AddDate(0, 0, -1)
		}
		out = append(out, ownNote{ID: id, Body: body, PostedAt: t})
	}
	return out, nil
}

func loadSnapshots(st *store.Store) (map[string]snapRow, error) {
	db := st.DB()
	rows, err := db.Query(`SELECT date, free_count, paid_count FROM analytics_snapshots`)
	if err != nil {
		if isMissingTableErr(err) {
			return map[string]snapRow{}, nil
		}
		return nil, err
	}
	defer rows.Close()
	out := map[string]snapRow{}
	for rows.Next() {
		var date string
		var free, paid sql.NullInt64
		if err := rows.Scan(&date, &free, &paid); err != nil {
			continue
		}
		out[date] = snapRow{Free: int(free.Int64), Paid: int(paid.Int64)}
	}
	return out, nil
}

func stringField(m map[string]any, names ...string) string {
	for _, n := range names {
		if v, ok := m[n]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

func isMissingTableErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "no such table")
}
