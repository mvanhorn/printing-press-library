// Copyright 2026 bk20260126-code. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type agentCanvasContext struct {
	ElementCount int            `json:"elementCount"`
	Types        map[string]int `json:"types"`
	BoundingBox  struct {
		Width  float64 `json:"width"`
		Height float64 `json:"height"`
	} `json:"boundingBox"`
	Snapshots []string `json:"snapshots,omitempty"`
	Summary   string   `json:"summary"`
}

func newAgentCanvasContextCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent-canvas-context",
		Short: "Emit a compact canvas summary optimized for agent context windows.",
		Long: `Fetch the current canvas state and emit a compact JSON summary
suitable for passing to an AI agent as context. Includes element count,
type distribution, bounding box, and snapshot names.

Use --select to narrow fields and minimize token usage.`,
		Example: strings.Trim(`
  excalidraw-mcp-pp-cli agent-canvas-context --agent
  excalidraw-mcp-pp-cli agent-canvas-context --agent --compact
  excalidraw-mcp-pp-cli agent-canvas-context --agent --select elementCount,types,summary`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Fetch elements
			elemData, err := c.Get("/api/elements", nil)
			if err != nil {
				return fmt.Errorf("fetching elements: %w", err)
			}

			var elemResp struct {
				Elements []map[string]any `json:"elements"`
			}
			if parseErr := json.Unmarshal(elemData, &elemResp); parseErr != nil {
				var arr []map[string]any
				if arrErr := json.Unmarshal(elemData, &arr); arrErr == nil {
					elemResp.Elements = arr
				}
			}

			stats := buildCanvasStats(elemResp.Elements)
			ctx := agentCanvasContext{
				ElementCount: stats.ElementCount,
				Types:        stats.Types,
				Snapshots:    []string{},
			}
			ctx.BoundingBox.Width = stats.BoundingBox.Width
			ctx.BoundingBox.Height = stats.BoundingBox.Height

			// Fetch snapshot names
			snapData, snapErr := c.Get("/api/snapshots", nil)
			if snapErr == nil {
				var snapResp struct {
					Snapshots []struct {
						Name string `json:"name"`
					} `json:"snapshots"`
				}
				if json.Unmarshal(snapData, &snapResp) == nil {
					for _, s := range snapResp.Snapshots {
						ctx.Snapshots = append(ctx.Snapshots, s.Name)
					}
				}
			}

			// Build summary sentence
			typeList := make([]string, 0, len(ctx.Types))
			for t, n := range ctx.Types {
				typeList = append(typeList, fmt.Sprintf("%d %s", n, t))
			}
			ctx.Summary = fmt.Sprintf("%d elements (%s); canvas %.0fx%.0f; %d snapshot(s)",
				ctx.ElementCount, strings.Join(typeList, ", "),
				ctx.BoundingBox.Width, ctx.BoundingBox.Height, len(ctx.Snapshots))

			raw := json.RawMessage(jsonMarshalAny(ctx))
			if flags.selectFields != "" {
				raw = filterFields(raw, flags.selectFields)
			}
			return printOutput(cmd.OutOrStdout(), raw, true)
		},
	}
	return cmd
}
