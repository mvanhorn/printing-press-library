// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tella/internal/client"

	"github.com/spf13/cobra"
)

// newClipsCmd is the parent for clips-bulk operations that don't fit naturally
// under `videos clips` (which is keyed on a single video).
func newClipsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clips",
		Short: "Bulk and cross-video clip operations",
		RunE:  rejectUnknownSubcommand,
	}
	cmd.AddCommand(newClipsEditPassCmd(flags))
	cmd.AddCommand(newClipsTranscriptDiffCmd(flags))
	cmd.AddCommand(newClipsCaptionsCmd(flags))
	cmd.AddCommand(newNovelClipsFindRetakesCmd(flags))
	return cmd
}

// newClipsEditPassCmd applies a chained set of standard edits across every
// clip in a playlist. Default mode is dry-run: it prints the planned set of
// operations as structured JSON. `--apply` flips it to fire the mutations.
func newClipsEditPassCmd(flags *rootFlags) *cobra.Command {
	var playlistID string
	var removeFillers bool
	// PATCH(library): --remove-buffers mirrors the Tella web UI's "Remove
	// buffers" Cut-panel button. Composes GET /silences + POST /cut per
	// range filtered by --buffer-min-ms. --trim-edges adds the narrower
	// head/tail-only primitive. Mistake detection is retired; compatibility flags refuse.
	var removeBuffers bool
	var bufferMinMs int
	var trimEdges bool
	var findMistakes bool
	var unofficial bool
	var trimSilencesGT string
	var apply bool
	cmd := &cobra.Command{
		Use:     "edit-pass",
		Short:   "Apply remove-fillers, remove-buffers, trim-edges, and trim-silences across every clip in a playlist",
		Example: "  tella-pp-cli clips edit-pass --playlist plst_42 --remove-fillers --remove-buffers --trim-edges --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if removeBuffers && bufferMinMs < 0 {
				return usageErr(fmt.Errorf("--buffer-min-ms must be >= 0, got %d", bufferMinMs))
			}
			if findMistakes {
				return usageErr(fmt.Errorf("%s", mistakesRetired))
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run":     true,
					"playlist_id": playlistID,
					"total_clips": 0,
					"planned":     []any{},
					"applied":     false,
				}, flags)
			}
			if playlistID == "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "hint: pass --playlist <id> to plan edits across that playlist's clips")
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"error":       "missing-playlist",
					"hint":        "pass --playlist <id>",
					"playlist_id": "",
					"total_clips": 0,
					"planned":     []any{},
					"applied":     false,
				}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Reject invalid --trim-silences-gt values loudly rather than
			// silently skipping the silence-trim step (which is what the
			// _ discard used to do for anything that wasn't a Go duration).
			var minSilenceMS int
			if trimSilencesGT != "" {
				minSilence, parseErr := time.ParseDuration(trimSilencesGT)
				if parseErr != nil {
					return usageErr(fmt.Errorf("invalid --trim-silences-gt value %q: must be a Go duration (e.g. 1s, 500ms): %w", trimSilencesGT, parseErr))
				}
				minSilenceMS = int(minSilence / time.Millisecond)
			}

			videoIDs, err := listPlaylistVideoIDs(cmd.Context(), c, playlistID)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}

			type op struct {
				Op   string         `json:"op"`
				Args map[string]any `json:"args,omitempty"`
			}
			type clipPlan struct {
				VideoID string `json:"video_id"`
				ClipID  string `json:"clip_id"`
				Ops     []op   `json:"ops"`
			}
			// enumerationFailure records a per-video planning-stage error so
			// the result envelope can surface partial-plan situations. Before
			// this struct landed, listClipIDs and the silences fetch each
			// silently swallowed errors via bare continue / `if err == nil`,
			// producing a plan that looked complete and an --apply that
			// reported applied=true / failed_ops=0 while entire videos were
			// never touched.
			type enumerationFailure struct {
				VideoID string `json:"video_id"`
				ClipID  string `json:"clip_id,omitempty"`
				Stage   string `json:"stage"`
				Error   string `json:"error"`
			}
			plans := []clipPlan{}
			enumFailures := []enumerationFailure{}
			totalClips := 0
			unknownMistakeEvents := 0

			for _, vid := range videoIDs {
				clipIDs, err := listClipIDs(cmd.Context(), c, vid)
				if err != nil {
					enumFailures = append(enumFailures, enumerationFailure{
						VideoID: vid,
						Stage:   "list_clips",
						Error:   truncate(err.Error(), 200),
					})
					continue
				}
				for _, cid := range clipIDs {
					totalClips++
					p := clipPlan{VideoID: vid, ClipID: cid}
					plannedCuts := map[string]int{}
					appendCut := func(name string, from, to int) {
						key := fmt.Sprintf("%d:%d", from, to)
						if idx, exists := plannedCuts[key]; exists {
							args := p.Ops[idx].Args
							sources, _ := args["sources"].([]string)
							if len(sources) == 0 {
								sources = []string{p.Ops[idx].Op}
							}
							for _, source := range sources {
								if source == name {
									return
								}
							}
							args["sources"] = append(sources, name)
							return
						}
						plannedCuts[key] = len(p.Ops)
						p.Ops = append(p.Ops, op{
							Op:   name,
							Args: map[string]any{"fromMs": from, "toMs": to},
						})
					}
					if removeFillers {
						p.Ops = append(p.Ops, op{Op: "remove-fillers"})
					}
					// Silences fetch is shared across --remove-buffers,
					// --trim-edges, and --trim-silences-gt. Fetch once if
					// any flag asks for it.
					var silRanges []silenceRange
					silencesNeeded := removeBuffers || trimEdges || minSilenceMS > 0
					if silencesNeeded {
						silData, silErr := c.Get(cmd.Context(), fmt.Sprintf("/v1/videos/%s/clips/%s/silences", vid, cid), nil)
						if silErr != nil {
							enumFailures = append(enumFailures, enumerationFailure{
								VideoID: vid,
								ClipID:  cid,
								Stage:   "fetch_silences",
								Error:   truncate(silErr.Error(), 200),
							})
						} else {
							silRanges = extractSilenceRanges(silData)
						}
					}
					// PATCH(library): --remove-buffers plans a cut for every
					// silence ≥ bufferMinMs, matching the Tella web UI's
					// "Remove buffers" button. The /cut endpoint merges
					// overlapping/adjacent cuts server-side, so any overlap
					// with --trim-silences-gt is a no-op rather than a bug.
					if removeBuffers && silRanges != nil {
						for _, sil := range silRanges {
							if sil.End-sil.Start >= bufferMinMs {
								appendCut("remove-buffers", sil.Start, sil.End)
							}
						}
					}
					// PATCH(library): --trim-edges is the narrow primitive
					// — head + tail only. Used when callers want to strip
					// leading/trailing dead-air without touching mid-clip
					// silences (which --remove-buffers would also cut).
					// Needs clip duration for tail tolerance.
					if trimEdges && len(silRanges) > 0 {
						clipDurationMs, durErr := fetchClipDurationMs(cmd.Context(), c, vid, cid)
						if durErr != nil {
							enumFailures = append(enumFailures, enumerationFailure{
								VideoID: vid,
								ClipID:  cid,
								Stage:   "fetch_clip_duration",
								Error:   truncate(durErr.Error(), 200),
							})
						} else {
							head, tail := pickBufferRanges(silRanges, clipDurationMs)
							if head != nil {
								appendCut("trim-edges-head", head.Start, head.End)
							}
							if tail != nil {
								appendCut("trim-edges-tail", tail.Start, tail.End)
							}
						}
					}
					if minSilenceMS > 0 && silRanges != nil {
						for _, sil := range silRanges {
							if sil.End-sil.Start >= minSilenceMS {
								// PATCH(library): args shape switched from
								// {start, end} to {fromMs, toMs} to match
								// the public spec's CutClipRequest. Apply
								// switch below reads the same field names.
								appendCut("cut", sil.Start, sil.End)
							}
						}
					}
					if len(p.Ops) > 0 {
						plans = append(plans, p)
					}
				}
			}

			result := map[string]any{
				"playlist_id": playlistID,
				"total_clips": totalClips,
				"planned":     plans,
				"applied":     false,
			}
			// Only attach enumeration_failures when non-empty so the
			// happy-path envelope shape stays clean.
			if len(enumFailures) > 0 {
				result["enumeration_failures"] = enumFailures
			}
			if unknownMistakeEvents > 0 {
				result["unknown_mistake_events"] = unknownMistakeEvents
			}
			if apply {
				type failure struct {
					VideoID string `json:"video_id"`
					ClipID  string `json:"clip_id"`
					Op      string `json:"op"`
					Error   string `json:"error"`
				}
				succeeded := 0
				failed := 0
				failures := []failure{}
				for _, p := range plans {
					for _, o := range p.Ops {
						var postErr error
						switch o.Op {
						case "remove-fillers":
							_, _, postErr = c.Post(cmd.Context(), fmt.Sprintf("/v1/videos/%s/clips/%s/remove-fillers", p.VideoID, p.ClipID), map[string]any{})
						case "cut", "remove-buffers", "trim-edges-head", "trim-edges-tail":
							// PATCH(library): every cut-producing op
							// applies via the same public /cut endpoint.
							// Distinct op names keep the dry-run plan
							// envelope auditable; server-side /cut merges
							// overlapping cuts so any cross-flag overlap
							// is idempotent.
							cutBody := map[string]any{"fromMs": o.Args["fromMs"], "toMs": o.Args["toMs"]}
							_, _, postErr = c.Post(cmd.Context(), fmt.Sprintf("/v1/videos/%s/clips/%s/cut", p.VideoID, p.ClipID), cutBody)
						default:
							postErr = fmt.Errorf("unrecognized op %q: not applied", o.Op)
						}
						if postErr != nil {
							failed++
							failures = append(failures, failure{
								VideoID: p.VideoID,
								ClipID:  p.ClipID,
								Op:      o.Op,
								Error:   postErr.Error(),
							})
							continue
						}
						succeeded++
					}
				}
				result["applied"] = true
				result["applied_ops"] = succeeded
				result["failed_ops"] = failed
				if len(failures) > 0 {
					result["failures"] = failures
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&playlistID, "playlist", "", "Playlist ID to iterate")
	cmd.Flags().BoolVar(&removeFillers, "remove-fillers", false, "Plan a remove-fillers pass for every clip")
	// PATCH(library): UI-button-matching buffer trim + narrow head/tail
	// primitives; retired mistake flags refuse before HTTP. Cataloged in
	// .printing-press-patches.json#add-cut-panel-parity.
	cmd.Flags().BoolVar(&removeBuffers, "remove-buffers", false, "Plan a remove-buffers pass for every clip (matches the Tella web UI's Cut-panel button; tune with --buffer-min-ms)")
	cmd.Flags().IntVar(&bufferMinMs, "buffer-min-ms", defaultBufferMinMs, "Minimum silence duration in milliseconds for --remove-buffers (0 = cut every silence)")
	cmd.Flags().BoolVar(&trimEdges, "trim-edges", false, "Plan head+tail silence cuts for every clip (narrow primitive; distinct from --remove-buffers which targets every silence)")
	cmd.Flags().BoolVar(&findMistakes, "find-mistakes", false, "Retired; use clips find-retakes or agent-chats start")
	cmd.Flags().BoolVar(&unofficial, "unofficial", false, "Retired compatibility flag")
	cmd.Flags().StringVar(&trimSilencesGT, "trim-silences-gt", "", "Plan cuts for silences longer than this duration (e.g. 1s)")
	_ = cmd.Flags().MarkDeprecated("find-mistakes", mistakesRetired)
	_ = cmd.Flags().MarkDeprecated("unofficial", mistakesRetired)
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually fire the planned mutations (default off — print plan only)")
	return cmd
}

// listPlaylistVideoIDs returns every video ID belonging to a playlist by
// querying `GET /v1/videos?playlistId=<id>`. Tella's playlist GET returns
// only a count under `videos`, not an array, so the membership listing has
// to come from the videos endpoint. Pages through the cursor so larger
// workspaces don't silently drop videos past the first page.
func listPlaylistVideoIDs(ctx context.Context, c *client.Client, playlistID string) ([]string, error) {
	return paginatedListIDs(ctx, c, "/v1/videos", map[string]string{"playlistId": playlistID}, "videos")
}

type silenceRange struct {
	Start int
	End   int
}

// extractSilenceRanges parses a /silences response into [{Start, End}] ranges.
// The current public API returns objects shaped `{startTimeMs, durationMs}`
// (verified against api.tella.com on 2026-05-16) — the legacy field-name
// lookups (`start`/`end`, `startMs`/`endMs`) below remained from an earlier
// API shape and missed today's response, so trim-silences-gt silently planned
// zero cuts. The expanded lookup handles both the modern duration-based
// shape and the legacy explicit-end shape without forking parsers.
// PATCH(library): {startTimeMs, durationMs} support; cataloged in
// .printing-press-patches.json#add-cut-panel-parity.
func extractSilenceRanges(data json.RawMessage) []silenceRange {
	var out []silenceRange
	candidates := []json.RawMessage{data}
	var env map[string]json.RawMessage
	if err := json.Unmarshal(data, &env); err == nil {
		for _, k := range []string{"silences", "data", "ranges"} {
			if r, ok := env[k]; ok {
				candidates = append(candidates, r)
			}
		}
	}
	for _, c := range candidates {
		var arr []map[string]any
		if err := json.Unmarshal(c, &arr); err == nil {
			for _, item := range arr {
				start := intField(item, "startTimeMs", "startMs", "start", "from", "begin")
				// Prefer explicit end fields; fall back to start + duration when
				// the response shape only carries a duration (the current API
				// behavior). intField truncates floats to ints via int(x).
				end, hasEnd := intFieldOK(item, "end", "to", "stop", "endMs", "endTimeMs")
				if !hasEnd {
					if dur, hasDuration := intFieldOK(item, "durationMs", "duration"); hasDuration && dur > 0 {
						end = start + dur
					}
				}
				if end > start {
					out = append(out, silenceRange{Start: start, End: end})
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	return out
}

func intField(m map[string]any, keys ...string) int {
	v, _ := intFieldOK(m, keys...)
	return v
}

func intFieldOK(m map[string]any, keys ...string) (int, bool) {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch x := v.(type) {
			case float64:
				return int(x), true
			case int:
				return x, true
			case string:
				// Best-effort parse like "1500ms"
				x = strings.TrimSuffix(x, "ms")
				var n int
				_, err := fmt.Sscanf(x, "%d", &n)
				if err == nil {
					return n, true
				}
			}
		}
	}
	return 0, false
}
