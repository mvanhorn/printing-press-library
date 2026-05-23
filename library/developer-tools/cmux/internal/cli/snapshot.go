// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/cmux/internal/cmuxclient"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/cmux/internal/snapshotstore"

	"github.com/spf13/cobra"
)

// newSnapshotCmd captures the current cmux state into the local snapshot
// store. Idempotent: same value as the last observation does NOT create a
// new transition row, just a fresh observation timestamp.
func newSnapshotCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Record current cmux agent state into the local snapshot store",
		Long: `Snapshot writes one row per (workspace, status key) into the local
status_snapshots table. Re-run any time you want a fresh observation of
where every agent stands; status changes and status timeline read from
these rows. Sync invokes this automatically; you rarely need to call
snapshot directly.`,
		Example: "  cmux-pp-cli snapshot --json",
		Annotations: map[string]string{
			"mcp:read-only": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if dryRunOK(flags) {
				return nil
			}
			result, err := takeSnapshot(ctx)
			if err != nil {
				return err
			}
			return renderSnapshotResult(cmd, flags, result)
		},
	}
	return cmd
}

type snapshotResult struct {
	Observations int                            `json:"observations"`
	Transitions  int                            `json:"transitions"`
	Latest       []snapshotstore.StatusSnapshot `json:"latest"`
}

// takeSnapshot reads every workspace's statusEntries and records each.
func takeSnapshot(ctx context.Context) (*snapshotResult, error) {
	wss, err := cmuxclient.ListWorkspaces(ctx)
	if err != nil {
		return nil, err
	}
	entries, err := cmuxclient.ListStatusEntries(ctx, "")
	if err != nil {
		return nil, err
	}
	ss, err := snapshotstore.Open(ctx, "")
	if err != nil {
		return nil, err
	}
	defer ss.Close()

	titles := titleByWorkspace(ctx, wss)
	stranded := strandedCountByWorkspace(ctx, wss)

	res := &snapshotResult{}
	now := snapshotstore.Now()
	for _, se := range entries {
		canonical := string(cmuxclient.CanonicalState(se.Value, titles[se.WorkspaceRef], stranded[se.WorkspaceRef]))
		transitioned, err := ss.RecordObservation(ctx, se.WorkspaceRef, se.Key, se.Value, se.Icon, se.Color, canonical, now)
		if err != nil {
			return nil, err
		}
		res.Observations++
		if transitioned {
			res.Transitions++
		}
	}
	latest, err := ss.LatestPerWorkspaceKey(ctx)
	if err != nil {
		return nil, err
	}
	res.Latest = latest
	if err := evaluateAlerts(ctx, ss); err != nil {
		fmt.Fprintf(os.Stderr, "alert evaluation: %v\n", err)
	}
	return res, nil
}

func renderSnapshotResult(cmd *cobra.Command, flags *rootFlags, r *snapshotResult) error {
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(r)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "observations=%d transitions=%d latest_rows=%d\n", r.Observations, r.Transitions, len(r.Latest))
	return nil
}
