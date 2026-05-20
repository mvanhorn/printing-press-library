// Copyright 2026 giuliano-giacaglia. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newDevModeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dev-mode",
		Short: "Dev Mode bundles for one node.",
	}
	cmd.AddCommand(newDevModeDumpCmd(flags))
	return cmd
}

func newDevModeDumpCmd(flags *rootFlags) *cobra.Command {
	var node string
	var format string

	cmd := &cobra.Command{
		Use:   "dump <key>",
		Short: "Emit a portable Markdown bundle for one node.",
		Long: `Fuse dev-resource links, variables in scope, render permalink, and Code Connect
mappings for a single node into a Markdown (or JSON) bundle that is paste-ready
into a Dev Mode hand-off doc.`,
		Example: strings.Trim(`
  # Markdown bundle for one node
  figma-pp-cli dev-mode dump abc123XyZ --node 1234:5678

  # JSON form (same data, structured)
  figma-pp-cli dev-mode dump abc123XyZ --node 1234-5678 --format json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run": true, "command": "dev-mode dump"}`)
				return nil
			}
			if strings.TrimSpace(node) == "" {
				return usageErr(fmt.Errorf("--node is required"))
			}
			key := args[0]
			normID := normalizeNodeID(node)

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// 1. Node tree at depth 2.
			nodesRaw, err := c.Get("/v1/files/"+key+"/nodes", map[string]string{"ids": normID, "depth": "2"})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var nodesEnv struct {
				Nodes map[string]json.RawMessage `json:"nodes"`
			}
			_ = json.Unmarshal(nodesRaw, &nodesEnv)
			var doc map[string]any
			for _, raw := range nodesEnv.Nodes {
				var entry map[string]any
				_ = json.Unmarshal(raw, &entry)
				if d, ok := entry["document"].(map[string]any); ok {
					doc = d
					break
				}
			}

			// 2. Dev resources, server-side filter on node_ids.
			devResRaw, _ := c.Get("/v1/files/"+key+"/dev_resources", map[string]string{"node_ids": normID})
			var devEnv struct {
				DevResources []map[string]any `json:"dev_resources"`
			}
			_ = json.Unmarshal(devResRaw, &devEnv)

			// 3. Published variables.
			varsRaw, _ := c.Get("/v1/files/"+key+"/variables/published", nil)
			var varsList []map[string]any
			if len(varsRaw) > 0 {
				var env struct {
					Meta struct {
						Variables map[string]map[string]any `json:"variables"`
					} `json:"meta"`
				}
				if json.Unmarshal(varsRaw, &env) == nil {
					for _, v := range env.Meta.Variables {
						varsList = append(varsList, v)
					}
				}
			}

			fetchedAt := time.Now().UTC().Format(time.RFC3339)
			renderExpiresAt := time.Now().UTC().Add(30 * 24 * time.Hour).Format(time.RFC3339)

			payload := map[string]any{
				"fileKey":         key,
				"nodeId":          normID,
				"node":            doc,
				"devResources":    devEnv.DevResources,
				"variables":       varsList,
				"codeConnect":     []any{},
				"fetchedAt":       fetchedAt,
				"renderExpiresAt": renderExpiresAt,
			}

			switch strings.ToLower(format) {
			case "json":
				return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
			case "md", "markdown", "":
				w := cmd.OutOrStdout()
				name, _ := doc["name"].(string)
				typ, _ := doc["type"].(string)
				fmt.Fprintf(w, "# Dev Mode dump: %s\n\n", name)
				fmt.Fprintf(w, "**File:** %s · **Node:** %s · **Type:** %s\n", key, normID, typ)
				if abb, ok := doc["absoluteBoundingBox"].(map[string]any); ok {
					x, _ := abb["x"].(float64)
					y, _ := abb["y"].(float64)
					ww, _ := abb["width"].(float64)
					hh, _ := abb["height"].(float64)
					layout, _ := doc["layoutMode"].(string)
					if layout == "" {
						layout = "NONE"
					}
					fmt.Fprintf(w, "**Bounding box:** %g,%g,%g,%g · **Layout:** %s\n\n", x, y, ww, hh, layout)
				} else {
					fmt.Fprintln(w)
				}

				fmt.Fprintf(w, "## Variables in scope (%d)\n", len(varsList))
				for _, v := range varsList {
					vname, _ := v["name"].(string)
					if vname == "" {
						continue
					}
					if rv, ok := v["resolvedValue"]; ok {
						fmt.Fprintf(w, "- %s: %v\n", vname, rv)
					} else {
						fmt.Fprintf(w, "- %s\n", vname)
					}
				}
				fmt.Fprintln(w)

				fmt.Fprintf(w, "## Dev resources (%d)\n", len(devEnv.DevResources))
				for _, r := range devEnv.DevResources {
					rname, _ := r["name"].(string)
					rurl, _ := r["url"].(string)
					fmt.Fprintf(w, "- %s: %s\n", rname, rurl)
				}
				fmt.Fprintln(w)

				fmt.Fprintln(w, "## Code Connect mappings")
				fmt.Fprintln(w, "_(none returned by REST API)_")
				fmt.Fprintln(w)

				fmt.Fprintf(w, "## Render\n_fetched: %s · expires: %s_\n", fetchedAt, renderExpiresAt)
				return nil
			default:
				return usageErr(fmt.Errorf("unknown --format %q (want md or json)", format))
			}
		},
	}

	cmd.Flags().StringVar(&node, "node", "", "Single node id (1234:5678 or 1234-5678 form)")
	cmd.Flags().StringVar(&format, "format", "md", "Output format: md or json")

	return cmd
}
