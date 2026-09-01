// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tella/internal/config"
	"github.com/spf13/cobra"
)

type cleanCommandFlags struct {
	removeFillers  bool
	removeBuffers  bool
	removeSilences string
	trimEdges      bool
	findMistakes   bool
	unofficial     bool
	bufferMinMs    int
	timeRanges     []string
	wordRanges     []string
	apply          bool
}

func newVideosClipsCleanCmd(flags *rootFlags) *cobra.Command {
	var cleanFlags cleanCommandFlags
	cmd := &cobra.Command{
		Use:   "clean <id> <clipId>",
		Short: "Preview or apply a recoverable clip cleanup pass",
		Long: `clean composes Tella's public cut, transcript-cut, filler, and silence tools.

The default is a read-only preview. --apply snapshots the exact existing cuts
before mutation. If any step fails, clean reports recovery state and keeps the
snapshot for an explicit undo; it never overwrites cuts automatically.

--find-mistakes remains an explicit unofficial, cookie-authenticated feature;
detected ranges are applied through the official batched cut endpoint.`,
		Example: `  tella-pp-cli videos clips clean vid_abc cl_xyz --remove-fillers --remove-silences natural --dry-run
  tella-pp-cli videos clips clean vid_abc cl_xyz --range 1200:1800 --word-range 42:46 --apply
  TELLA_SESSION_COOKIE='__Secure-Tella.session=...' tella-pp-cli videos clips clean vid_abc cl_xyz --find-mistakes --unofficial --apply`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := cleanFlags.options()
			if err != nil {
				return usageErr(err)
			}
			api, err := flags.newClient()
			if err != nil {
				return err
			}
			api.DryRun = false // planning reads must be real; mutation is gated below
			api.NoCache = true // snapshots must reflect the current server-side cut set

			mistakeMeta, err := cleanFlags.addMistakeRanges(&opts, args[0], args[1], flags)
			if err != nil {
				return err
			}
			plan, snapshot, err := planCleanClip(api, args[0], args[1], opts)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			result := map[string]any{
				"video_id": args[0], "clip_id": args[1], "planned": plan,
				"dry_run": true, "applied": false,
			}
			if mistakeMeta != nil {
				result["find_mistakes"] = mistakeMeta
			}
			if flags.dryRun || !cleanFlags.apply {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			snapshotPath, err := persistCutSnapshot(snapshot)
			if err != nil {
				return fmt.Errorf("saving pre-clean cuts snapshot: %w", err)
			}
			snapshots := []cutSnapshot{snapshot}
			applyResult, applyErr := applyCleanPlans(api, []cleanClipPlan{plan}, snapshots)
			applyErr = errors.Join(applyErr, updateCutSnapshotFiles(map[string]string{snapshot.ClipID: snapshotPath}, snapshots))
			result["dry_run"] = false
			result["applied"] = applyErr == nil
			result["snapshot"] = snapshotPath
			result["result"] = applyResult
			if err := printJSONFiltered(cmd.OutOrStdout(), result, flags); err != nil {
				return err
			}
			return applyErr
		},
	}
	cleanFlags.add(cmd)
	return cmd
}

func (flags *cleanCommandFlags) add(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&flags.removeFillers, "remove-fillers", false, "Remove detected filler words through Tella's public API")
	cmd.Flags().StringVar(&flags.removeSilences, "remove-silences", "", "Official silence-removal mode: natural, fast, or faster")
	cmd.Flags().BoolVar(&flags.removeBuffers, "remove-buffers", false, "Legacy compatibility: cut detected silences at --buffer-min-ms via get-silences + cut")
	cmd.Flags().IntVar(&flags.bufferMinMs, "buffer-min-ms", defaultBufferMinMs, "Minimum silence duration for legacy --remove-buffers (default 200ms; official faster mode uses 300ms)")
	cmd.Flags().BoolVar(&flags.trimEdges, "trim-edges", false, "Cut leading and trailing silence only")
	cmd.Flags().StringArrayVar(&flags.timeRanges, "range", nil, "Playback-time range to cut as fromMs:toMs; repeatable")
	cmd.Flags().StringArrayVar(&flags.wordRanges, "word-range", nil, "Transcript word-index range to cut as from:to; repeatable")
	cmd.Flags().BoolVar(&flags.findMistakes, "find-mistakes", false, "Analyze unofficial Tella AI mistakes and cut their ranges")
	cmd.Flags().BoolVar(&flags.unofficial, "unofficial", false, "Opt in to the undocumented cookie-authenticated Find Mistakes service")
	cmd.Flags().BoolVar(&flags.apply, "apply", false, "Apply the previewed edits; default is read-only")
}

func (flags *cleanCommandFlags) options() (cleanOptions, error) {
	if flags.bufferMinMs < 0 {
		return cleanOptions{}, fmt.Errorf("--buffer-min-ms must be >= 0, got %d", flags.bufferMinMs)
	}
	if flags.removeBuffers && flags.removeSilences != "" {
		return cleanOptions{}, fmt.Errorf("choose --remove-buffers or --remove-silences, not both")
	}
	if flags.findMistakes && !flags.unofficial {
		return cleanOptions{}, fmt.Errorf("--find-mistakes calls Tella's unofficial AI service; pass --unofficial to opt in")
	}
	if flags.removeSilences != "" {
		switch flags.removeSilences {
		case "natural", "fast", "faster":
		default:
			return cleanOptions{}, fmt.Errorf("--remove-silences must be natural, fast, or faster")
		}
	}
	timeRanges, err := parseCleanTimeRanges(flags.timeRanges)
	if err != nil {
		return cleanOptions{}, err
	}
	wordRanges, err := parseCleanWordRanges(flags.wordRanges)
	if err != nil {
		return cleanOptions{}, err
	}
	removeFillers, removeBuffers, trimEdges := flags.removeFillers, flags.removeBuffers, flags.trimEdges
	if !removeFillers && !removeBuffers && flags.removeSilences == "" && !trimEdges && !flags.findMistakes && len(timeRanges) == 0 && len(wordRanges) == 0 {
		removeFillers, removeBuffers, trimEdges = true, true, true
	}
	return cleanOptions{
		RemoveFillers: removeFillers, RemoveBuffers: removeBuffers,
		RemoveSilences: flags.removeSilences, TrimEdges: trimEdges,
		BufferMinMs: flags.bufferMinMs, TimeRanges: timeRanges, WordRanges: wordRanges,
	}, nil
}

func (flags *cleanCommandFlags) addMistakeRanges(opts *cleanOptions, videoID, clipID string, root *rootFlags) (map[string]any, error) {
	if !flags.findMistakes {
		return nil, nil
	}
	cfg, err := config.Load(root.configPath)
	if err != nil {
		return nil, configErr(err)
	}
	api, err := newUnofficialClient(cfg.SessionCookie, root.timeout)
	if err != nil {
		return nil, configErr(err)
	}
	mistakes, unknownEvents, status, err := analyzeMistakes(api, videoID, clipID)
	if err != nil {
		return nil, apiErr(err)
	}
	for _, mistake := range mistakes {
		if mistake.Trim.Duration <= 0 {
			continue
		}
		opts.TimeRanges = append(opts.TimeRanges, cleanRange{
			FromMs: int(mistake.Trim.StartTime + 0.5),
			ToMs:   int(mistake.Trim.StartTime + mistake.Trim.Duration + 0.5),
		})
	}
	return map[string]any{"status": status, "detected": len(mistakes), "unknown_events": unknownEvents, "official": false}, nil
}

func parseCleanTimeRanges(values []string) ([]cleanRange, error) {
	ranges := make([]cleanRange, 0, len(values))
	for _, value := range values {
		from, to, err := parseCleanRange(value, "--range")
		if err != nil {
			return nil, err
		}
		ranges = append(ranges, cleanRange{FromMs: from, ToMs: to})
	}
	return ranges, nil
}

func parseCleanWordRanges(values []string) ([]cleanWordRange, error) {
	ranges := make([]cleanWordRange, 0, len(values))
	for _, value := range values {
		from, to, err := parseCleanRange(value, "--word-range")
		if err != nil {
			return nil, err
		}
		ranges = append(ranges, cleanWordRange{FromWordIndex: from, ToWordIndex: to})
	}
	return ranges, nil
}

func parseCleanRange(value, flag string) (int, int, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid %s %q: expected from:to", flag, value)
	}
	from, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid %s %q start: %w", flag, value, err)
	}
	to, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid %s %q end: %w", flag, value, err)
	}
	if from < 0 || to <= from {
		return 0, 0, fmt.Errorf("invalid %s %q: require 0 <= from < to", flag, value)
	}
	return from, to, nil
}
