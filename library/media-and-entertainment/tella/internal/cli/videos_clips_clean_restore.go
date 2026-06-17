// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tella/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tella/internal/config"
	"github.com/spf13/cobra"
)

func newVideosClipsCleanCmd(flags *rootFlags) *cobra.Command {
	var removeFillers bool
	var removeBuffers bool
	var trimEdges bool
	var findMistakes bool
	var unofficial bool
	var bufferMinMs int
	var apply bool

	cmd := &cobra.Command{
		Use:   "clean <id> <clipId>",
		Short: "Run a safe standard cleanup pass: fillers, silence buffers, edge trims, and optional AI mistakes",
		Long: `clean composes the common Tella Cut-panel cleanup actions into one reviewed workflow.

By default it prints a plan and does not mutate the clip. Pass --apply to run the
planned public-API cuts. Before applying, the command snapshots the clip's
current cuts so undo-last-cuts can restore them.

--find-mistakes uses Tella's undocumented AI service and therefore requires
--unofficial plus TELLA_SESSION_COOKIE. The detected cuts are still applied via
the public /v1/videos/{id}/clips/{clipId}/cut endpoint.`,
		Example: `  tella-pp-cli videos clips clean vid_abc cl_xyz --json
  tella-pp-cli videos clips clean vid_abc cl_xyz --remove-fillers --remove-buffers --trim-edges --apply
  TELLA_SESSION_COOKIE='__Secure-Tella.session=...' tella-pp-cli videos clips clean vid_abc cl_xyz --find-mistakes --unofficial --apply`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			videoID, clipID := args[0], args[1]
			if bufferMinMs < 0 {
				return usageErr(fmt.Errorf("--buffer-min-ms must be >= 0, got %d", bufferMinMs))
			}
			if !removeFillers && !removeBuffers && !trimEdges && !findMistakes {
				removeFillers, removeBuffers, trimEdges = true, true, true
			}
			if findMistakes && !unofficial {
				return usageErr(fmt.Errorf("--find-mistakes calls Tella's unofficial AI service; pass --unofficial to opt in"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// Planning needs reads even when global --dry-run is set.
			c.DryRun = false

			plan, err := planCleanClip(c, videoID, clipID, cleanOptions{RemoveFillers: removeFillers, RemoveBuffers: removeBuffers, TrimEdges: trimEdges, BufferMinMs: bufferMinMs})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var uc *unofficialClient
			if findMistakes {
				cfg, cerr := config.Load(flags.configPath)
				if cerr != nil {
					return configErr(cerr)
				}
				uc, err = newUnofficialClient(cfg.SessionCookie, flags.timeout)
				if err != nil {
					return configErr(err)
				}
				mistakes, unknownEvents, analyzeStatus, err := analyzeMistakes(uc, videoID, clipID)
				if err != nil {
					return apiErr(err)
				}
				plan.UnknownMistakeEvents = unknownEvents
				plan.AnalyzeStatus = analyzeStatus
				for _, m := range mistakes {
					if m.Trim.Duration <= 0 {
						continue
					}
					plan.Cuts = append(plan.Cuts, plannedCleanCut{Op: "find-mistakes", FromMs: int(m.Trim.StartTime + 0.5), ToMs: int(m.Trim.StartTime + m.Trim.Duration + 0.5), Reason: m.Reasoning})
				}
			}

			result := map[string]any{
				"video_id": videoID,
				"clip_id":  clipID,
				"planned":  plan.Cuts,
				"options": map[string]any{
					"remove_fillers": removeFillers,
					"remove_buffers": removeBuffers,
					"trim_edges":     trimEdges,
					"find_mistakes":  findMistakes,
					"buffer_min_ms":  bufferMinMs,
				},
			}
			if findMistakes {
				result["analyze_status"] = plan.AnalyzeStatus
				result["unknown_mistake_events"] = plan.UnknownMistakeEvents
			}

			if flags.dryRun || !apply {
				result["dry_run"] = true
				result["applied"] = false
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			snapshotPath, err := saveCutSnapshot(c, videoID, clipID)
			if err != nil {
				return err
			}
			applied, succeeded, failed := applyCleanCuts(c, videoID, clipID, plan.Cuts)
			result["dry_run"] = false
			result["applied"] = true
			result["snapshot"] = snapshotPath
			result["applied_ops"] = succeeded
			result["failed_ops"] = failed
			result["cuts"] = applied
			if failed > 0 {
				if err := printJSONFiltered(cmd.OutOrStdout(), result, flags); err != nil {
					return err
				}
				return fmt.Errorf("clean failed %d operation(s)", failed)
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().BoolVar(&removeFillers, "remove-fillers", false, "Plan/apply filler-word removal")
	cmd.Flags().BoolVar(&removeBuffers, "remove-buffers", false, "Plan/apply silence-buffer cuts")
	cmd.Flags().BoolVar(&trimEdges, "trim-edges", false, "Plan/apply leading/trailing silence cuts")
	cmd.Flags().BoolVar(&findMistakes, "find-mistakes", false, "Plan/apply Tella AI mistake cuts (requires --unofficial + TELLA_SESSION_COOKIE)")
	cmd.Flags().BoolVar(&unofficial, "unofficial", false, "Required by --find-mistakes: opt in to Tella's undocumented AI service")
	cmd.Flags().IntVar(&bufferMinMs, "buffer-min-ms", defaultBufferMinMs, "Minimum silence duration in milliseconds for --remove-buffers")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually apply planned cleanup; default prints a plan")
	return cmd
}

func newVideosClipsUndoLastCutsCmd(flags *rootFlags) *cobra.Command {
	var snapshotPath string
	var apply bool
	cmd := &cobra.Command{
		Use:     "undo-last-cuts <id> <clipId>",
		Short:   "Restore the most recent cuts snapshot saved before a clean/apply workflow",
		Example: "  tella-pp-cli videos clips undo-last-cuts vid_abc cl_xyz --apply",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			videoID, clipID := args[0], args[1]
			path := snapshotPath
			if path == "" {
				var err error
				path, err = latestCutSnapshotPath(videoID, clipID)
				if err != nil {
					return err
				}
			}
			snap, err := readCutSnapshot(path)
			if err != nil {
				return err
			}
			body := map[string]any{"cuts": snap.Cuts}
			result := map[string]any{"video_id": videoID, "clip_id": clipID, "snapshot": path, "body": body}
			if flags.dryRun || !apply {
				result["dry_run"] = true
				result["applied"] = false
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, status, err := c.Patch(fmt.Sprintf("/v1/videos/%s/clips/%s", videoID, clipID), body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			result["dry_run"] = false
			result["applied"] = true
			result["status"] = status
			result["data"] = jsonRawToAny(data)
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&snapshotPath, "snapshot", "", "Snapshot JSON path; defaults to latest snapshot for the clip")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually restore cuts; default prints request body")
	return cmd
}

func newVideosClipsRestoreCutsCmd(flags *rootFlags) *cobra.Command {
	var cutsJSON string
	var snapshotPath string
	var apply bool
	cmd := &cobra.Command{
		Use:     "restore-cuts <id> <clipId>",
		Short:   "Restore clip cuts from inline JSON or a snapshot file",
		Example: "  tella-pp-cli videos clips restore-cuts vid_abc cl_xyz --cuts '[{\"fromMs\":100,\"toMs\":250}]' --apply",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			videoID, clipID := args[0], args[1]
			var cuts any
			if snapshotPath != "" {
				snap, err := readCutSnapshot(snapshotPath)
				if err != nil {
					return err
				}
				cuts = snap.Cuts
			} else {
				if cutsJSON == "" {
					return usageErr(fmt.Errorf("pass --cuts JSON or --snapshot <path>"))
				}
				if err := json.Unmarshal([]byte(cutsJSON), &cuts); err != nil {
					return fmt.Errorf("parsing --cuts JSON: %w", err)
				}
			}
			body := map[string]any{"cuts": cuts}
			result := map[string]any{"video_id": videoID, "clip_id": clipID, "body": body}
			if snapshotPath != "" {
				result["snapshot"] = snapshotPath
			}
			if flags.dryRun || !apply {
				result["dry_run"] = true
				result["applied"] = false
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, status, err := c.Patch(fmt.Sprintf("/v1/videos/%s/clips/%s", videoID, clipID), body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			result["dry_run"] = false
			result["applied"] = true
			result["status"] = status
			result["data"] = jsonRawToAny(data)
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&cutsJSON, "cuts", "", "Cuts JSON array to restore")
	cmd.Flags().StringVar(&snapshotPath, "snapshot", "", "Snapshot JSON path to restore")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually restore cuts; default prints request body")
	return cmd
}

type cleanOptions struct {
	RemoveFillers bool
	RemoveBuffers bool
	TrimEdges     bool
	BufferMinMs   int
}

type plannedCleanCut struct {
	Op     string `json:"op"`
	FromMs int    `json:"fromMs,omitempty"`
	ToMs   int    `json:"toMs,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type cleanPlan struct {
	Cuts                 []plannedCleanCut `json:"cuts"`
	UnknownMistakeEvents int               `json:"unknown_mistake_events,omitempty"`
	AnalyzeStatus        int               `json:"analyze_status,omitempty"`
}

func planCleanClip(c *client.Client, videoID, clipID string, opts cleanOptions) (cleanPlan, error) {
	plan := cleanPlan{Cuts: []plannedCleanCut{}}
	if opts.RemoveFillers {
		plan.Cuts = append(plan.Cuts, plannedCleanCut{Op: "remove-fillers"})
	}
	if opts.RemoveBuffers || opts.TrimEdges {
		silData, err := c.Get(fmt.Sprintf("/v1/videos/%s/clips/%s/silences", videoID, clipID), nil)
		if err != nil {
			return plan, err
		}
		silences := extractSilenceRanges(silData)
		if opts.RemoveBuffers {
			for _, sil := range silences {
				if sil.End-sil.Start >= opts.BufferMinMs {
					plan.Cuts = append(plan.Cuts, plannedCleanCut{Op: "remove-buffers", FromMs: sil.Start, ToMs: sil.End})
				}
			}
		}
		if opts.TrimEdges && len(silences) > 0 {
			duration, err := fetchClipDurationMs(c, videoID, clipID)
			if err != nil {
				return plan, err
			}
			head, tail := pickBufferRanges(silences, duration)
			if head != nil {
				plan.Cuts = append(plan.Cuts, plannedCleanCut{Op: "trim-edges-head", FromMs: head.Start, ToMs: head.End})
			}
			if tail != nil {
				plan.Cuts = append(plan.Cuts, plannedCleanCut{Op: "trim-edges-tail", FromMs: tail.Start, ToMs: tail.End})
			}
		}
	}
	return plan, nil
}

type appliedCleanCut struct {
	Op     string `json:"op"`
	FromMs int    `json:"fromMs,omitempty"`
	ToMs   int    `json:"toMs,omitempty"`
	Status int    `json:"status,omitempty"`
	Error  string `json:"error,omitempty"`
}

func applyCleanCuts(c *client.Client, videoID, clipID string, cuts []plannedCleanCut) ([]appliedCleanCut, int, int) {
	applied := make([]appliedCleanCut, 0, len(cuts))
	succeeded, failed := 0, 0
	for _, p := range cuts {
		ac := appliedCleanCut{Op: p.Op, FromMs: p.FromMs, ToMs: p.ToMs}
		var status int
		var err error
		if p.Op == "remove-fillers" {
			_, status, err = c.Post(fmt.Sprintf("/v1/videos/%s/clips/%s/remove-fillers", videoID, clipID), map[string]any{})
		} else {
			_, status, err = c.Post(fmt.Sprintf("/v1/videos/%s/clips/%s/cut", videoID, clipID), map[string]any{"fromMs": p.FromMs, "toMs": p.ToMs})
		}
		if err != nil {
			failed++
			ac.Error = err.Error()
		} else {
			succeeded++
			ac.Status = status
		}
		applied = append(applied, ac)
	}
	return applied, succeeded, failed
}

type cutSnapshot struct {
	VideoID   string    `json:"video_id"`
	ClipID    string    `json:"clip_id"`
	CreatedAt time.Time `json:"created_at"`
	Cuts      any       `json:"cuts"`
}

func saveCutSnapshot(c *client.Client, videoID, clipID string) (string, error) {
	clipData, err := c.Get(fmt.Sprintf("/v1/videos/%s/clips/%s", videoID, clipID), nil)
	if err != nil {
		return "", err
	}
	var clip map[string]any
	if err := json.Unmarshal(clipData, &clip); err != nil {
		return "", fmt.Errorf("parsing clip response for cuts snapshot: %w", err)
	}
	cuts := clip["cuts"]
	if cuts == nil {
		cuts = []any{}
	}
	snap := cutSnapshot{VideoID: videoID, ClipID: clipID, CreatedAt: time.Now().UTC(), Cuts: cuts}
	dir, err := cutSnapshotDir(videoID, clipID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, snap.CreatedAt.Format("20060102T150405.000000000Z")+".json")
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func readCutSnapshot(path string) (cutSnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return cutSnapshot{}, err
	}
	var snap cutSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return cutSnapshot{}, fmt.Errorf("parsing cuts snapshot %s: %w", path, err)
	}
	return snap, nil
}

func latestCutSnapshotPath(videoID, clipID string) (string, error) {
	dir, err := cutSnapshotDir(videoID, clipID)
	if err != nil {
		return "", err
	}
	matches, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no cut snapshots found for video %s clip %s; pass --snapshot or use restore-cuts --cuts", videoID, clipID)
	}
	sort.Strings(matches)
	return matches[len(matches)-1], nil
}

func cutSnapshotDir(videoID, clipID string) (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "tella-pp-cli", "cut-snapshots", safePathSegment(videoID), safePathSegment(clipID)), nil
}

func safePathSegment(s string) string {
	out := []rune(s)
	for i, r := range out {
		if !(r == '-' || r == '_' || r == '.' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z') {
			out[i] = '_'
		}
	}
	return string(out)
}
