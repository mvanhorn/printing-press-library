// Copyright 2026 Charles Denzel Segovia and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: offline full-text search across all form submissions.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type submissionHit struct {
	ID        string `json:"id,omitempty"`
	SiteName  string `json:"site_name,omitempty"`
	FormName  string `json:"form_name,omitempty"`
	Name      string `json:"name,omitempty"`
	Email     string `json:"email,omitempty"`
	Summary   string `json:"summary,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

type submissionSearchView struct {
	Query       string          `json:"query"`
	Scanned     int             `json:"scanned_submissions"`
	Count       int             `json:"count"`
	Submissions []submissionHit `json:"submissions"`
}

// pp:data-source local
func newNovelSubmissionsSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var dbPath string
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Offline full-text search across every form submission from every site.",
		Long: `Case-insensitive search over all synced form submissions (name, email, summary,
and every submitted field). Run 'netlify-pp-cli sync' first to populate
submissions locally.`,
		Example:     "  netlify-pp-cli submissions search \"refund\" --json --limit 20",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search query is required, e.g. submissions search \"acme\""))
			}
			query := strings.TrimSpace(args[0])
			needle := strings.ToLower(query)

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = novelDBPath()
			}
			if mirrorMissing(dbPath) {
				return noMirror(cmd, flags, dbPath, "sites,submissions", submissionSearchView{
					Query:       query,
					Submissions: []submissionHit{},
				})
			}
			st, err := openMirror(ctx, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			rows := loadTyped(st, "submissions", "forms-submissions", "sites-submissions")

			hits := make([]submissionHit, 0)
			scanned := 0
			for _, s := range rows {
				scanned++
				raw, _ := json.Marshal(s)
				if !strings.Contains(strings.ToLower(string(raw)), needle) {
					continue
				}
				name := firstStr(s, "name")
				if name == "" {
					name = strings.TrimSpace(firstStr(s, "first_name") + " " + firstStr(s, "last_name"))
				}
				hits = append(hits, submissionHit{
					ID:        firstStr(s, "id"),
					SiteName:  firstStr(s, "site_name", "site_url"),
					FormName:  firstStr(s, "form_name"),
					Name:      name,
					Email:     firstStr(s, "email"),
					Summary:   firstStr(s, "summary"),
					CreatedAt: firstStr(s, "created_at"),
				})
			}
			// Sort all matches newest-first, then cap: applying the limit before
			// the sort would drop the newest matches and keep arbitrary storage-order ones.
			sort.Slice(hits, func(i, j int) bool { return timestampAfter(hits[i].CreatedAt, hits[j].CreatedAt) })
			if limit > 0 && len(hits) > limit {
				hits = hits[:limit]
			}

			view := submissionSearchView{
				Query:       query,
				Scanned:     scanned,
				Count:       len(hits),
				Submissions: hits,
			}
			if scanned == 0 {
				cmd.PrintErrln("no submissions in local mirror; run: netlify-pp-cli sync --resources submissions")
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "maximum matching submissions to return (0 = no cap)")
	cmd.Flags().StringVar(&dbPath, "db", "", "local mirror path (default: standard data dir)")
	return cmd
}
