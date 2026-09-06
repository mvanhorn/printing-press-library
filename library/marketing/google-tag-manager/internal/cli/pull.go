// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// pull: mirror an entire GTM container into the local SQLite store. The
// foundation every other read-only command reads from.

package cli

import (
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

type pullResult struct {
	SnapshotID int64          `json:"snapshotId"`
	Account    string         `json:"account"`
	Container  string         `json:"container"`
	PublicID   string         `json:"publicId,omitempty"`
	Source     string         `json:"source"`
	VersionID  string         `json:"versionId,omitempty"`
	PulledAt   string         `json:"pulledAt"`
	Entities   int            `json:"entities"`
	Breakdown  map[string]int `json:"breakdown"`
}

func newNovelPullCmd(flags *rootFlags) *cobra.Command {
	var account, container, workspace, dbPath string
	var live bool

	cmd := &cobra.Command{
		Use:         "pull",
		Short:       "Mirror an entire GTM container into the local SQLite store",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Walk a GTM container and store every tag, trigger, variable, built-in
variable, folder, template, client, zone, and gtag config in a local SQLite
mirror. Pull the published live version (--live, default) in a single call, or
a specific workspace (--workspace <id>). Re-pulls become time-stamped snapshots
that audit, diff, fleet, and history read.`,
		Example: `  # Mirror the published live container
  google-tag-manager-pp-cli pull --account 6012345 --container 9876543 --live

  # Mirror a working workspace
  google-tag-manager-pp-cli pull --account 6012345 --container 9876543 --workspace 7`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would pull container config into the local mirror")
				return nil
			}
			if account == "" || container == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--account and --container are required"))
			}
			// GTM account/container/workspace ids are always numeric. Enforce
			// it so user input cannot inject path segments into the request URL.
			for _, p := range []struct{ name, val string }{
				{"--account", account}, {"--container", container}, {"--workspace", workspace},
			} {
				if p.val != "" && !isAllDigits(p.val) {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("%s must be a numeric GTM id, got %q", p.name, p.val))
				}
			}
			source := "live"
			if workspace != "" {
				source = "workspace:" + workspace
			} else if !live {
				// Default to live when neither flag is given.
				source = "live"
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			s, err := openGTMStore(ctx, gtmDBPath(dbPath))
			if err != nil {
				return err
			}
			defer s.Close()

			snap, err := pullContainer(ctx, c, s.DB(), account, container, source, time.Now)
			if err != nil {
				return err
			}

			breakdown := map[string]int{}
			rows, err := s.DB().QueryContext(ctx, `SELECT kind, COUNT(*) FROM gtm_entity WHERE snapshot_id = ? GROUP BY kind`, snap.ID)
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var k string
					var n int
					if rows.Scan(&k, &n) == nil {
						breakdown[k] = n
					}
				}
				if rerr := rows.Err(); rerr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: entity breakdown may be incomplete: %v\n", rerr)
				}
			}

			res := pullResult{
				SnapshotID: snap.ID,
				Account:    firstNonEmpty(snap.AccountName, snap.AccountID),
				Container:  firstNonEmpty(snap.ContainerName, snap.ContainerID),
				PublicID:   snap.PublicID,
				Source:     snap.Source,
				VersionID:  snap.VersionID,
				PulledAt:   snap.PulledAt,
				Entities:   snap.EntityCount,
				Breakdown:  breakdown,
			}
			return gtmEmit(cmd, flags, res, func(w io.Writer) {
				fmt.Fprintf(w, "Pulled %s (%s) source=%s — snapshot #%d, %d entities\n",
					res.Container, firstNonEmpty(res.PublicID, "-"), res.Source, res.SnapshotID, res.Entities)
				kinds := make([]string, 0, len(breakdown))
				for k := range breakdown {
					kinds = append(kinds, k)
				}
				sort.Strings(kinds)
				tw := newTabWriter(w)
				for _, k := range kinds {
					fmt.Fprintf(tw, "  %s\t%d\n", k, breakdown[k])
				}
				tw.Flush()
			})
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "GTM account id")
	cmd.Flags().StringVar(&container, "container", "", "GTM container id")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace id to pull (default: published live version)")
	cmd.Flags().BoolVar(&live, "live", false, "Pull the published live version (default when --workspace is absent)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
