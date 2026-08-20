package cli

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/local/localstore"
)

// newZoomStorageCmd: T2 storage audit.
func newZoomStorageCmd(flags *rootFlags) *cobra.Command {
	var (
		by          string
		alsoInCloud bool
		partialOnly bool
	)
	cmd := &cobra.Command{
		Use:   "storage",
		Short: "Audit local recording storage, group by month/topic/partial, flag duplicates safe to delete",
		Long: "Walks the synced local_recordings table, groups rows by the selected dimension, and computes " +
			"total bytes + safe-to-delete bytes (rows whose cloud_recordings counterpart exists) per group.",
		Example: `  zoom-pp-cli storage --by month --json
  zoom-pp-cli storage --by topic --also-in-cloud --json
  zoom-pp-cli storage --partial-only --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if by == "" {
				by = "month"
			}
			db, closer, err := openLocalDB(cmd.Context())
			if err != nil {
				return err
			}
			defer closer()

			rows, err := localstore.ListLocalRecordings(cmd.Context(), db, localstore.ListLocalOpts{PartialOnly: partialOnly})
			if err != nil {
				return err
			}

			cloudByID := map[string]bool{}
			if alsoInCloud {
				crows, err := db.QueryContext(cmd.Context(), `SELECT meeting_id FROM cloud_recordings`)
				if err == nil {
					defer crows.Close()
					for crows.Next() {
						var mid string
						if err := crows.Scan(&mid); err == nil && mid != "" {
							cloudByID[mid] = true
						}
					}
				}
			}

			type group struct {
				Key               string  `json:"key"`
				Count             int     `json:"count"`
				TotalBytes        int64   `json:"total_bytes"`
				TotalGB           float64 `json:"total_gb"`
				PartialCount      int     `json:"partial_count"`
				SafeToDeleteCount int     `json:"safe_to_delete_count"`
				SafeToDeleteBytes int64   `json:"safe_to_delete_bytes"`
				SafeToDeleteGB    float64 `json:"safe_to_delete_gb"`
			}

			groups := map[string]*group{}
			for _, r := range rows {
				key := groupKey(by, r)
				g := groups[key]
				if g == nil {
					g = &group{Key: key}
					groups[key] = g
				}
				g.Count++
				g.TotalBytes += r.TotalBytes
				if r.HasPartial {
					g.PartialCount++
				}
				if alsoInCloud && r.MeetingID != "" && cloudByID[r.MeetingID] {
					g.SafeToDeleteCount++
					g.SafeToDeleteBytes += r.TotalBytes
				}
			}

			out := make([]group, 0, len(groups))
			for _, g := range groups {
				g.TotalGB = bytesToGB(g.TotalBytes)
				g.SafeToDeleteGB = bytesToGB(g.SafeToDeleteBytes)
				out = append(out, *g)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].TotalBytes > out[j].TotalBytes })

			var totalBytes, safeBytes int64
			var partial int
			for _, g := range out {
				totalBytes += g.TotalBytes
				safeBytes += g.SafeToDeleteBytes
				partial += g.PartialCount
			}
			return flags.printJSON(cmd, map[string]any{
				"by":                by,
				"recording_count":   len(rows),
				"total_bytes":       totalBytes,
				"total_gb":          bytesToGB(totalBytes),
				"partial_count":     partial,
				"safe_to_delete_gb": bytesToGB(safeBytes),
				"groups":            out,
				"note":              noteForStorage(alsoInCloud),
			})
		},
	}
	cmd.Flags().StringVar(&by, "by", "month", "Grouping dimension: month | topic | partial")
	cmd.Flags().BoolVar(&alsoInCloud, "also-in-cloud", false, "Flag rows whose meeting also exists in cloud_recordings (safe-to-delete)")
	cmd.Flags().BoolVar(&partialOnly, "partial-only", false, "Limit to recordings with double_click_to_convert partials")
	return cmd
}

func groupKey(by string, r localstore.LocalRecordingRow) string {
	switch by {
	case "topic":
		if r.Topic == "" {
			return "(unknown)"
		}
		return r.Topic
	case "partial":
		if r.HasPartial {
			return "partial"
		}
		return "complete"
	default:
		if r.Start.IsZero() {
			return "(unknown)"
		}
		return r.Start.Format("2006-01")
	}
}

func bytesToGB(b int64) float64 {
	return float64(b) / (1024 * 1024 * 1024)
}

func noteForStorage(alsoInCloud bool) string {
	if alsoInCloud {
		return "safe_to_delete = local recordings whose meeting_id matches a cloud_recordings row"
	}
	return "rerun with --also-in-cloud after `recordings cloud list` populates the cache to see safe-to-delete bytes"
}

// pluralise is a small formatter used by other display helpers.
func pluralise(n int, singular string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %ss", n, singular)
}

// shortPath truncates a long path to its tail two components, for tabular output.
func shortPath(p string) string {
	if p == "" {
		return ""
	}
	parts := strings.Split(filepath.ToSlash(p), "/")
	if len(parts) <= 2 {
		return p
	}
	return ".../" + strings.Join(parts[len(parts)-2:], "/")
}

// onDateOnly returns just the calendar day component of a time, in local TZ.
func onDateOnly(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}
