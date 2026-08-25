// Copyright 2026 Charles Denzel Segovia and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: side-by-side comparison of two deploys.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/cloud/netlify/internal/store"

	"github.com/spf13/cobra"
)

type deployFieldDiff struct {
	Field string `json:"field"`
	A     string `json:"a"`
	B     string `json:"b"`
}

type deployDiffView struct {
	A     string            `json:"a"`
	B     string            `json:"b"`
	Diff  []deployFieldDiff `json:"diff"`
	Equal bool              `json:"equal"`
}

// diffFields are the deploy attributes compared side by side.
var diffFields = []string{
	"state", "context", "branch", "commit_ref", "deploy_url",
	"error_message", "created_at", "published_at", "framework", "title",
}

// pp:data-source auto
func newNovelDeployDiffCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "deploy-diff <deploy-a> <deploy-b>",
		Short: "Compare two deploys side by side (state, commit, context, error).",
		Long: `Fetch two deploys by id — from the local mirror when present, otherwise live —
and report the fields that differ. Useful for pinpointing what changed between a
working deploy and a broken one.`,
		Example:     "  netlify-pp-cli deploy-diff 5a1b... 5c2d... --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("two deploy ids are required: deploy-diff <deploy-a> <deploy-b>"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			var st *store.Store
			if dbPath == "" {
				dbPath = novelDBPath()
			}
			if !mirrorMissing(dbPath) {
				if opened, err := openMirror(ctx, dbPath); err == nil {
					defer opened.Close()
					st = opened
				}
			}

			a, err := fetchDeploy(ctx, flags, st, args[0])
			if err != nil {
				return err
			}
			b, err := fetchDeploy(ctx, flags, st, args[1])
			if err != nil {
				return err
			}

			diff := make([]deployFieldDiff, 0)
			for _, f := range diffFields {
				av, bv := firstStr(a, f), firstStr(b, f)
				if av != bv {
					diff = append(diff, deployFieldDiff{Field: f, A: av, B: bv})
				}
			}
			sort.Slice(diff, func(i, j int) bool { return diff[i].Field < diff[j].Field })

			view := deployDiffView{A: args[0], B: args[1], Diff: diff, Equal: len(diff) == 0}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "local mirror path (default: standard data dir)")
	return cmd
}

// fetchDeploy resolves a deploy by id from the local mirror first, then live.
func fetchDeploy(ctx context.Context, flags *rootFlags, st *store.Store, id string) (map[string]any, error) {
	if st != nil {
		for _, rt := range []string{"deploys", "sites-deploys"} {
			if raw, err := st.Get(rt, id); err == nil && len(raw) > 0 {
				var m map[string]any
				if json.Unmarshal(raw, &m) == nil {
					return m, nil
				}
			}
		}
	}
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	data, err := c.Get(ctx, "/deploys/"+id, nil)
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing deploy %s: %w", id, err)
	}
	return m, nil
}
