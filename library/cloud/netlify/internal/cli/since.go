// Copyright 2026 Charles Denzel Segovia and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: deploy/build timeline across all sites within a time window.

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/cloud/netlify/internal/cliutil"

	"github.com/spf13/cobra"
)

type deployEvent struct {
	SiteID    string `json:"site_id,omitempty"`
	SiteName  string `json:"site_name,omitempty"`
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	State     string `json:"state,omitempty"`
	Context   string `json:"context,omitempty"`
	Branch    string `json:"branch,omitempty"`
	CommitRef string `json:"commit_ref,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

type sinceView struct {
	Window  string        `json:"window"`
	Since   string        `json:"since"`
	Count   int           `json:"count"`
	Deploys []deployEvent `json:"deploys"`
}

// pp:data-source local
func newNovelSinceCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "since <window>",
		Short: "List deploys and builds across all synced sites within a time window (e.g. 2h, 7d).",
		Long: `Aggregate deploys and builds from the local mirror created within the given
window and list them newest-first. The window accepts Go durations plus d/w
shorthand (e.g. 90m, 2h, 7d, 1w). Run 'sync' first to populate deploys locally.`,
		Example:     "  netlify-pp-cli since 2h --json --select site_name,state,commit_ref",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a time window is required, e.g. 'since 2h'"))
			}
			dur, err := cliutil.ParseDurationLoose(args[0])
			if err != nil {
				return usageErr(fmt.Errorf("invalid window %q: %w", args[0], err))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = novelDBPath()
			}
			if mirrorMissing(dbPath) {
				return noMirror(cmd, flags, dbPath, "sites,deploys", sinceView{
					Window:  args[0],
					Deploys: []deployEvent{},
				})
			}
			st, err := openMirror(ctx, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			cutoff := time.Now().Add(-dur)

			// Resolve site id -> name for labeling.
			siteName := map[string]string{}
			for _, s := range loadTyped(st, "sites") {
				siteName[firstStr(s, "id", "site_id")] = firstStr(s, "name", "custom_domain", "url")
			}

			events := make([]deployEvent, 0)
			collect := func(kind string, rows []map[string]any) {
				for _, d := range rows {
					created := firstStr(d, "created_at", "created")
					t, ok := parseTimeLoose(created)
					if !ok || t.Before(cutoff) {
						continue
					}
					sid := firstStr(d, "site_id", "siteId")
					events = append(events, deployEvent{
						SiteID:    sid,
						SiteName:  siteName[sid],
						ID:        firstStr(d, "id"),
						Kind:      kind,
						State:     firstStr(d, "state"),
						Context:   firstStr(d, "context"),
						Branch:    firstStr(d, "branch"),
						CommitRef: firstStr(d, "commit_ref", "commit_url"),
						CreatedAt: created,
					})
				}
			}
			collect("deploy", loadTyped(st, "deploys", "sites-deploys"))
			collect("build", loadTyped(st, "builds", "sites-builds"))

			sort.Slice(events, func(i, j int) bool { return timestampAfter(events[i].CreatedAt, events[j].CreatedAt) })

			view := sinceView{
				Window:  args[0],
				Since:   cutoff.UTC().Format(time.RFC3339),
				Count:   len(events),
				Deploys: events,
			}
			if len(events) == 0 {
				cmd.PrintErrln("no deploys/builds in window; run: netlify-pp-cli sync --resources deploys first, or widen the window")
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "local mirror path (default: standard data dir)")
	return cmd
}
