// Copyright 2026 martin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/canvas/internal/store"
	"github.com/spf13/cobra"
)

type driftRow struct {
	CourseID      string  `json:"course_id"`
	PrevScore     float64 `json:"prev_score"`
	CurrentScore  float64 `json:"current_score"`
	Delta         float64 `json:"delta"`
	SnapshottedAt string  `json:"snapshotted_at"`
}

type driftSnapshot struct {
	Score float64 `json:"score"`
	At    string  `json:"at"`
}

func driftSnapshotPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home dir: %w", err)
	}
	return filepath.Join(home, ".canvas-lms-pp-cli", "drift-snapshot.json"), nil
}

func newDriftCmd(flags *rootFlags) *cobra.Command {
	var reset bool
	var threshold float64

	cmd := &cobra.Command{
		Use:         "drift",
		Short:       "Grade Drift Tracker — compare current grades to a saved snapshot",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Reads current enrollment grades and compares them to a local snapshot.
On first run (or with --reset), creates a fresh snapshot and exits.
On subsequent runs, reports courses where your score changed by at least --threshold points.`,
		Example: strings.Trim(`
  canvas-lms-pp-cli drift
  canvas-lms-pp-cli drift --reset
  canvas-lms-pp-cli drift --threshold 1.0 --json
  canvas-lms-pp-cli drift --agent --select course_id,delta`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			snapPath, err := driftSnapshotPath()
			if err != nil {
				return err
			}

			db, err := store.OpenWithContext(cmd.Context(), flags.defaultDBPath("canvas-lms-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			// Gather current scores
			current := map[string]float64{}
			rows, err := db.DB().QueryContext(cmd.Context(), `SELECT courses_id, data FROM courses_enrollments`)
			if err != nil {
				return fmt.Errorf("querying enrollments: %w", err)
			}
			defer rows.Close()
			for rows.Next() {
				var cid string
				var raw []byte
				if err := rows.Scan(&cid, &raw); err != nil {
					continue
				}
				var e struct {
					Type   string `json:"type"`
					Grades struct {
						CurrentScore *float64 `json:"current_score"`
					} `json:"grades"`
					CourseID string `json:"course_id"`
				}
				if err := json.Unmarshal(raw, &e); err != nil {
					continue
				}
				if e.Type != "StudentEnrollment" {
					continue
				}
				ecid := e.CourseID
				if ecid == "" {
					ecid = cid
				}
				if e.Grades.CurrentScore != nil {
					current[ecid] = *e.Grades.CurrentScore
				}
			}

			now := time.Now().UTC().Format(time.RFC3339)

			// --reset: write snapshot and exit
			if reset {
				snap := map[string]driftSnapshot{}
				for cid, score := range current {
					snap[cid] = driftSnapshot{Score: score, At: now}
				}
				if err := writeDriftSnapshot(snapPath, snap); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Snapshot saved for %d courses.\n", len(snap))
				return nil
			}

			// Load existing snapshot
			existing, err := loadDriftSnapshot(snapPath)
			if err != nil || len(existing) == 0 {
				// First run: create snapshot
				snap := map[string]driftSnapshot{}
				for cid, score := range current {
					snap[cid] = driftSnapshot{Score: score, At: now}
				}
				if werr := writeDriftSnapshot(snapPath, snap); werr != nil {
					return werr
				}
				fmt.Fprintf(cmd.OutOrStdout(), "First run: snapshot created for %d courses. Run again to see drift.\n", len(snap))
				return nil
			}

			var results []driftRow
			for cid, curScore := range current {
				prev, ok := existing[cid]
				if !ok {
					prev = driftSnapshot{Score: curScore, At: now}
				}
				delta := curScore - prev.Score
				if delta < 0 {
					delta = -delta
				}
				if delta < threshold {
					continue
				}
				results = append(results, driftRow{
					CourseID:      cid,
					PrevScore:     prev.Score,
					CurrentScore:  curScore,
					Delta:         roundTo2(curScore - prev.Score),
					SnapshottedAt: prev.At,
				})
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}

			if len(results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No grade drift above %.1f points detected.\n", threshold)
				return nil
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "COURSE\tPREV\tCURRENT\tDELTA\tSNAPSHOTTED")
			for _, r := range results {
				delta := fmt.Sprintf("%+.2f", r.Delta)
				fmt.Fprintf(tw, "%s\t%.1f\t%.1f\t%s\t%s\n",
					r.CourseID, r.PrevScore, r.CurrentScore, delta, r.SnapshottedAt[:10])
			}
			return tw.Flush()
		},
	}

	cmd.Flags().BoolVar(&reset, "reset", false, "Take a fresh snapshot and exit")
	cmd.Flags().Float64Var(&threshold, "threshold", 2.0, "Minimum point delta to report")
	return cmd
}

func loadDriftSnapshot(path string) (map[string]driftSnapshot, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var snap map[string]driftSnapshot
	if err := json.NewDecoder(f).Decode(&snap); err != nil {
		return nil, err
	}
	return snap, nil
}

func writeDriftSnapshot(path string, snap map[string]driftSnapshot) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating snapshot dir: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating snapshot file: %w", err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(snap)
}
