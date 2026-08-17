// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/parallel/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/parallel/internal/types"
	"github.com/spf13/cobra"
)

func newNovelFindallPromoteCmd(flags *rootFlags) *cobra.Command {
	var flagFindallID string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "promote",
		Short: "Turn FindAll candidates into a Task Group enrichment job.",
		Example: strings.Trim(`
  parallel-pp-cli findall promote --findall-id findall_demo --limit 10 --dry-run --json --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, map[string]any{
					"dry_run":    true,
					"findall_id": strings.TrimSpace(flagFindallID),
					"limit":      flagLimit,
				})
			}
			if err := validateDataSourceStrategy(flags, "auto"); err != nil {
				return err
			}

			findallID := strings.TrimSpace(flagFindallID)
			if findallID == "" && !hasChangedLocalFlags(cmd) && len(args) == 0 {
				return cmd.Help()
			}
			if findallID == "" {
				return usageErr(fmt.Errorf("--findall-id is required"))
			}

			limit := flagLimit
			if limit <= 0 {
				limit = 10
			}
			if cliutil.IsDogfoodEnv() {
				limit = 1
			}

			candidates, err := loadFindallCandidates(cmd, flags, findallID)
			if err != nil {
				return err
			}
			if len(candidates) > limit {
				candidates = candidates[:limit]
			}

			inputs := buildPromoteInputs(candidates)
			body := map[string]any{
				"metadata": map[string]any{
					"source":      "findall_promote",
					"findall_id":  findallID,
					"candidates":  len(candidates),
					"inputs_plan": inputs,
				},
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, statusCode, err := c.PostWithParams(cmd.Context(), "/v1/tasks/groups", nil, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if statusCode < 200 || statusCode >= 300 {
				return fmt.Errorf("task group create failed with status %d: %s", statusCode, string(data))
			}

			writeMutationResponseToStore(cmd.Context(), "tasks", data, "")
			return flags.printJSON(cmd, map[string]any{
				"findall_id": findallID,
				"candidates": len(candidates),
				"response":   jsonRawToAny(data),
			})
		},
	}
	cmd.Flags().StringVar(&flagFindallID, "findall-id", "", "FindAll run id to promote")
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Maximum candidates to include")
	return cmd
}

func loadFindallCandidates(cmd *cobra.Command, flags *rootFlags, findallID string) ([]types.FindAllCandidate, error) {
	if db, err := openStoreForRead(cmd.Context(), "parallel-pp-cli"); err == nil && db != nil {
		defer db.Close()
		if raw, err := db.Get("findall", findallID); err == nil {
			if cands := parseFindallCandidates(raw); len(cands) > 0 {
				return cands, nil
			}
		}
	}

	path := replacePathParam("/v1beta/findall/runs/{findall_id}/result", "findall_id", findallID)
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	data, prov, err := resolveReadWithStrategyAndResponsePath(cmd.Context(), c, flags, "auto", "findall", false, path, nil, nil, "candidates", cmd.ErrOrStderr())
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	_ = prov
	return parseFindallCandidates(data), nil
}

func parseFindallCandidates(data json.RawMessage) []types.FindAllCandidate {
	var direct []types.FindAllCandidate
	if json.Unmarshal(data, &direct) == nil && len(direct) > 0 {
		return direct
	}
	var wrapped struct {
		Candidates []types.FindAllCandidate `json:"candidates"`
	}
	if json.Unmarshal(data, &wrapped) == nil && len(wrapped.Candidates) > 0 {
		return wrapped.Candidates
	}
	return nil
}

func buildPromoteInputs(candidates []types.FindAllCandidate) []map[string]any {
	inputs := make([]map[string]any, 0, len(candidates))
	for _, c := range candidates {
		input := map[string]any{
			"processor": "core",
			"input": map[string]any{
				"task": firstNonEmpty(c.Name, c.Description, c.Url, c.CandidateId),
			},
		}
		if c.Url != "" {
			input["input"].(map[string]any)["url"] = c.Url
		}
		if c.Name != "" {
			input["input"].(map[string]any)["name"] = c.Name
		}
		inputs = append(inputs, input)
	}
	return inputs
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return "enrich findall candidate"
}

func jsonRawToAny(raw json.RawMessage) any {
	var v any
	if json.Unmarshal(raw, &v) == nil {
		return v
	}
	return string(raw)
}
