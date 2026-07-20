// Copyright 2026 Charles Denzel Segovia and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: cross-site health dashboard.

package cli

import (
	"sort"

	"github.com/spf13/cobra"
)

type siteOverview struct {
	Name       string `json:"name"`
	URL        string `json:"url"`
	State      string `json:"state"`
	Account    string `json:"account,omitempty"`
	LastDeploy string `json:"last_deploy,omitempty"`
	FormCount  int    `json:"form_count"`
	SiteID     string `json:"site_id,omitempty"`
}

// pp:data-source local
func newNovelOverviewCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "overview",
		Short: "Cross-site health: build state, last deploy, and form count for every synced site.",
		Long: `Show one row per site with its state, last published deploy time, and the
number of forms attached, aggregated from the local mirror. Run 'sync' first to
populate sites (and optionally forms) locally.`,
		Example:     "  netlify-pp-cli overview --json --select name,last_deploy,state",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = novelDBPath()
			}
			if mirrorMissing(dbPath) {
				return noMirror(cmd, flags, dbPath, "sites,forms", []siteOverview{})
			}
			st, err := openMirror(ctx, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			sites := loadTyped(st, "sites")
			forms := loadTyped(st, "forms", "sites-forms")

			formCounts := map[string]int{}
			for _, f := range forms {
				if sid := firstStr(f, "site_id", "siteId"); sid != "" {
					formCounts[sid]++
				}
			}

			out := make([]siteOverview, 0, len(sites))
			for _, s := range sites {
				id := firstStr(s, "id", "site_id")
				row := siteOverview{
					Name:       firstStr(s, "name", "custom_domain", "url"),
					URL:        firstStr(s, "ssl_url", "url"),
					State:      firstStr(s, "state"),
					Account:    firstStr(s, "account_slug", "account_name"),
					LastDeploy: nestedStr(s, "published_deploy", "published_at"),
					FormCount:  formCounts[id],
					SiteID:     id,
				}
				if row.LastDeploy == "" {
					row.LastDeploy = firstStr(s, "published_at", "updated_at")
				}
				out = append(out, row)
			}
			sort.Slice(out, func(i, j int) bool { return timestampAfter(out[i].LastDeploy, out[j].LastDeploy) })

			if len(out) == 0 {
				cmd.PrintErrln("no sites in local mirror; run: netlify-pp-cli sync --resources sites")
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "local mirror path (default: standard data dir)")
	return cmd
}
