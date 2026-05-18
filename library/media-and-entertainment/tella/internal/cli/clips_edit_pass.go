// Copyright 2026 gregce. Licensed under Apache-2.0. See LICENSE.

package cli

import (
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
	return cmd
}

// newClipsEditPassCmd applies a chained set of standard edits across every
// clip in a playlist. Default mode is dry-run: it prints the planned set of
// operations as structured JSON. `--apply` flips it to fire the mutations.
func newClipsEditPassCmd(flags *rootFlags) *cobra.Command {
	var playlistID string
	var removeFillers bool
	var trimSilencesGT string
	var apply bool
	cmd := &cobra.Command{
		Use:     "edit-pass",
		Short:   "Apply remove-fillers and trim-silences across every clip in a playlist",
		Example: "  tella-pp-cli clips edit-pass --playlist plst_42 --remove-fillers --trim-silences-gt 1s --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run":     true,
					"playlist_id": playlistID,
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

			videoIDs, err := listPlaylistVideoIDs(c, playlistID)
			if err != nil {
				return classifyAPIError(err, flags)
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

			for _, vid := range videoIDs {
				clipIDs, err := listClipIDs(c, vid)
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
					if removeFillers {
						p.Ops = append(p.Ops, op{Op: "remove-fillers"})
					}
					if minSilenceMS > 0 {
						silData, silErr := c.Get(fmt.Sprintf("/v1/videos/%s/clips/%s/silences", vid, cid), nil)
						if silErr != nil {
							enumFailures = append(enumFailures, enumerationFailure{
								VideoID: vid,
								ClipID:  cid,
								Stage:   "fetch_silences",
								Error:   truncate(silErr.Error(), 200),
							})
						} else {
							for _, sil := range extractSilenceRanges(silData) {
								if sil.End-sil.Start >= minSilenceMS {
									p.Ops = append(p.Ops, op{
										Op:   "cut",
										Args: map[string]any{"start": sil.Start, "end": sil.End},
									})
								}
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
							_, _, postErr = c.Post(fmt.Sprintf("/v1/videos/%s/clips/%s/remove-fillers", p.VideoID, p.ClipID), map[string]any{})
						case "cut":
							_, _, postErr = c.Post(fmt.Sprintf("/v1/videos/%s/clips/%s/cut", p.VideoID, p.ClipID), o.Args)
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
	cmd.Flags().StringVar(&trimSilencesGT, "trim-silences-gt", "", "Plan cuts for silences longer than this duration (e.g. 1s)")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually fire the planned mutations (default off — print plan only)")
	return cmd
}

// listPlaylistVideoIDs returns every video ID belonging to a playlist by
// querying `GET /v1/videos?playlistId=<id>`. Tella's playlist GET returns
// only a count under `videos`, not an array, so the membership listing has
// to come from the videos endpoint. Pages through the cursor so larger
// workspaces don't silently drop videos past the first page.
func listPlaylistVideoIDs(c *client.Client, playlistID string) ([]string, error) {
	return paginatedListIDs(c, "/v1/videos", map[string]string{"playlistId": playlistID}, "videos")
}

type silenceRange struct {
	Start int
	End   int
}

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
				start := intField(item, "start", "from", "begin", "startMs")
				end := intField(item, "end", "to", "stop", "endMs")
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
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch x := v.(type) {
			case float64:
				return int(x)
			case int:
				return x
			case string:
				// Best-effort parse like "1500ms"
				x = strings.TrimSuffix(x, "ms")
				var n int
				_, err := fmt.Sscanf(x, "%d", &n)
				if err == nil {
					return n
				}
			}
		}
	}
	return 0
}
