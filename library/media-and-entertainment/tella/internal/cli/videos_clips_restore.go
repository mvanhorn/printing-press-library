// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/spf13/cobra"
)

type snapshotRestoreCheck struct {
	CurrentCuts               any    `json:"current_cuts,omitempty"`
	ExpectedCuts              any    `json:"expected_cuts,omitempty"`
	CurrentMatchesExpected    bool   `json:"current_matches_expected"`
	ConditionalWriteSupported bool   `json:"conditional_write_supported"`
	RequiresExclusiveEditing  bool   `json:"requires_exclusive_editing"`
	Reason                    string `json:"reason,omitempty"`
}

func inspectSnapshotRestore(api cleanAPI, snapshot cutSnapshot) (snapshotRestoreCheck, error) {
	check := snapshotRestoreCheck{
		ExpectedCuts: snapshot.ExpectedCuts, RequiresExclusiveEditing: true,
		ConditionalWriteSupported: false,
	}
	if snapshot.ExpectedCuts == nil {
		check.Reason = "snapshot has no expected post-clean cuts; snapshot restore is unavailable"
		return check, nil
	}
	current, err := captureCutSnapshot(api, snapshot.VideoID, snapshot.ClipID)
	if err != nil {
		return check, err
	}
	check.CurrentCuts = current.Cuts
	check.CurrentMatchesExpected = reflect.DeepEqual(current.Cuts, snapshot.ExpectedCuts)
	if !check.CurrentMatchesExpected {
		check.Reason = "current cuts differ from the snapshot's expected post-clean state"
	} else {
		check.Reason = "current cuts match, but Tella has no conditional write; apply requires exclusive editing access"
	}
	return check, nil
}

func newVideosClipsUndoLastCutsCmd(flags *rootFlags) *cobra.Command {
	var snapshotPath string
	var apply bool
	var confirmExclusiveEditing bool
	cmd := &cobra.Command{
		Use:   "undo-last-cuts <id> <clipId>",
		Short: "Restore the latest clean snapshot with a divergence check and exclusive-editing confirmation",
		Long: `Restore the latest pre-clean cut snapshot after checking the current cuts.

Tella exposes no revision token or conditional cut update, so the read-time
divergence check is advisory. Applying requires --confirm-exclusive-editing,
which asserts that nobody else can save this clip until the command finishes.`,
		Example: "  tella-pp-cli videos clips undo-last-cuts vid_abc cl_xyz --apply --confirm-exclusive-editing",
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
			snapshot, err := readCutSnapshot(path)
			if err != nil {
				return err
			}
			if snapshot.VideoID != videoID || snapshot.ClipID != clipID {
				return usageErr(fmt.Errorf("snapshot belongs to video %s clip %s, not %s/%s", snapshot.VideoID, snapshot.ClipID, videoID, clipID))
			}
			api, err := flags.newClient()
			if err != nil {
				return err
			}
			api.DryRun = false
			api.NoCache = true
			check, err := inspectSnapshotRestore(api, snapshot)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			result := map[string]any{
				"video_id": videoID, "clip_id": clipID, "snapshot": path,
				"body": map[string]any{"cuts": snapshot.Cuts}, "restore_check": check,
				"exclusive_editing_confirmed": confirmExclusiveEditing,
			}
			if flags.dryRun || !apply {
				result["dry_run"] = true
				result["applied"] = false
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			if !check.CurrentMatchesExpected {
				if err := printJSONFiltered(cmd.OutOrStdout(), result, flags); err != nil {
					return err
				}
				return usageErr(fmt.Errorf("refusing snapshot restore: %s", check.Reason))
			}
			if !confirmExclusiveEditing {
				if err := printJSONFiltered(cmd.OutOrStdout(), result, flags); err != nil {
					return err
				}
				return usageErr(fmt.Errorf("refusing snapshot restore: pass --confirm-exclusive-editing only after ensuring nobody else can save this clip until the command finishes"))
			}
			status, err := restoreCutSnapshot(api, snapshot)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			result["dry_run"] = false
			result["applied"] = true
			result["status"] = status
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&snapshotPath, "snapshot", "", "Snapshot JSON path; defaults to latest snapshot for the clip")
	cmd.Flags().BoolVar(&apply, "apply", false, "Restore after the divergence check and exclusive-editing confirmation")
	cmd.Flags().BoolVar(&confirmExclusiveEditing, "confirm-exclusive-editing", false, "Confirm nobody else can save this clip until restore finishes (required with --apply)")
	return cmd
}

func newVideosClipsRestoreCutsCmd(flags *rootFlags) *cobra.Command {
	var cutsJSON string
	var snapshotPath string
	var apply bool
	var confirmExclusiveEditing bool
	cmd := &cobra.Command{
		Use:     "restore-cuts <id> <clipId>",
		Short:   "Restore clip cuts from inline JSON or a snapshot file",
		Example: "  tella-pp-cli videos clips restore-cuts vid_abc cl_xyz --snapshot cuts.json --apply --confirm-exclusive-editing",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			videoID, clipID := args[0], args[1]
			var cuts any
			var snapshot *cutSnapshot
			var api cleanAPI
			if snapshotPath != "" {
				loaded, err := readCutSnapshot(snapshotPath)
				if err != nil {
					return err
				}
				if loaded.VideoID != videoID || loaded.ClipID != clipID {
					return usageErr(fmt.Errorf("snapshot belongs to video %s clip %s, not %s/%s", loaded.VideoID, loaded.ClipID, videoID, clipID))
				}
				snapshot = &loaded
				cuts = loaded.Cuts
			} else {
				if cutsJSON == "" {
					return usageErr(fmt.Errorf("pass --cuts JSON or --snapshot <path>"))
				}
				if err := json.Unmarshal([]byte(cutsJSON), &cuts); err != nil {
					return fmt.Errorf("parsing --cuts JSON: %w", err)
				}
				if _, ok := cuts.([]any); !ok {
					return usageErr(fmt.Errorf("--cuts must be a JSON array"))
				}
			}
			body := map[string]any{"cuts": cuts}
			result := map[string]any{"video_id": videoID, "clip_id": clipID, "body": body}
			if snapshotPath != "" {
				result["snapshot"] = snapshotPath
				result["exclusive_editing_confirmed"] = confirmExclusiveEditing
				client, err := flags.newClient()
				if err != nil {
					return err
				}
				client.DryRun = false
				client.NoCache = true
				api = client
				check, err := inspectSnapshotRestore(api, *snapshot)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				result["restore_check"] = check
				if apply && !flags.dryRun && !check.CurrentMatchesExpected {
					if err := printJSONFiltered(cmd.OutOrStdout(), result, flags); err != nil {
						return err
					}
					return usageErr(fmt.Errorf("refusing snapshot restore: %s", check.Reason))
				}
				if apply && !flags.dryRun && !confirmExclusiveEditing {
					if err := printJSONFiltered(cmd.OutOrStdout(), result, flags); err != nil {
						return err
					}
					return usageErr(fmt.Errorf("refusing snapshot restore: pass --confirm-exclusive-editing only after ensuring nobody else can save this clip until the command finishes"))
				}
			}
			if flags.dryRun || !apply {
				result["dry_run"] = true
				result["applied"] = false
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			if api == nil {
				client, err := flags.newClient()
				if err != nil {
					return err
				}
				api = client
			}
			_, status, err := api.Patch(fmt.Sprintf("/v1/videos/%s/clips/%s", videoID, clipID), body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			result["dry_run"] = false
			result["applied"] = true
			result["status"] = status
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&cutsJSON, "cuts", "", "Exact stored cuts JSON array to restore")
	cmd.Flags().StringVar(&snapshotPath, "snapshot", "", "Snapshot JSON path to restore")
	cmd.Flags().BoolVar(&apply, "apply", false, "Restore cuts; snapshot restores also require exclusive-editing confirmation")
	cmd.Flags().BoolVar(&confirmExclusiveEditing, "confirm-exclusive-editing", false, "Confirm nobody else can save this clip until snapshot restore finishes")
	return cmd
}
