// Copyright 2026 daniel-larson. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"flowiseai-pp-cli/internal/store"

	"github.com/spf13/cobra"
)

func newChatflowDepsCmd(flags *rootFlags) *cobra.Command {
	var showOverrides bool

	cmd := &cobra.Command{
		Use:   "deps [chatflowId]",
		Short: "Show every tool, assistant, variable, and document store a chatflow references",
		Long: `Parse the cached flowData JSON of a chatflow and emit the tools, assistants,
variables, and document stores it references. Cross-check each reference
against the local cache and flag any that no longer exist on the server (run
` + "`sync`" + ` first to refresh the cache).

Pass --show-overrides to also list the overrideConfig keys the flow exposes,
useful when constructing a ` + "`prediction --override-config '{...}'`" + ` invocation.`,
		Example: "  flowiseai-pp-cli chatflow deps abc-123 --json --show-overrides",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			chatflowID := args[0]

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("flowiseai-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			var flowDataRaw, name string
			err = db.DB().QueryRowContext(cmd.Context(),
				`SELECT COALESCE(flow_data, ''), COALESCE(name, '') FROM chatflows WHERE id = ?`,
				chatflowID).Scan(&flowDataRaw, &name)
			if err != nil {
				return notFoundErr(fmt.Errorf("chatflow %s not found in local cache; run `sync` first", chatflowID))
			}
			if flowDataRaw == "" {
				return apiErr(fmt.Errorf("chatflow %s has no flow_data in the local cache", chatflowID))
			}

			// flow_data is typically a JSON string {nodes: [...], edges: [...]} but
			// Flowise sometimes stores it as a JSON-encoded string-of-a-string. Try
			// both shapes.
			var flow struct {
				Nodes []struct {
					ID   string         `json:"id"`
					Type string         `json:"type"`
					Data map[string]any `json:"data"`
				} `json:"nodes"`
				Edges []map[string]any `json:"edges"`
			}
			if err := json.Unmarshal([]byte(flowDataRaw), &flow); err != nil {
				// Try double-decode (Flowise often stores stringified JSON)
				var inner string
				if err2 := json.Unmarshal([]byte(flowDataRaw), &inner); err2 == nil {
					if err3 := json.Unmarshal([]byte(inner), &flow); err3 != nil {
						return apiErr(fmt.Errorf("parsing flow_data JSON: %w", err3))
					}
				} else {
					return apiErr(fmt.Errorf("parsing flow_data JSON: %w", err))
				}
			}

			type ref struct {
				NodeID   string `json:"nodeId"`
				NodeType string `json:"nodeType"`
				Label    string `json:"label"`
				Category string `json:"category"`
			}
			result := struct {
				ChatflowID    string   `json:"chatflowId"`
				ChatflowName  string   `json:"chatflowName"`
				NodeCount     int      `json:"nodeCount"`
				Tools         []ref    `json:"tools"`
				Assistants    []ref    `json:"assistants"`
				Variables     []ref    `json:"variables"`
				DocumentStore []ref    `json:"documentStore"`
				LLM           []ref    `json:"llm"`
				Memory        []ref    `json:"memory"`
				Overrides     []string `json:"overrideConfigKeys,omitempty"`
				MissingRefs   []string `json:"missingReferences,omitempty"`
			}{
				ChatflowID:   chatflowID,
				ChatflowName: name,
				NodeCount:    len(flow.Nodes),
			}

			// Categorize nodes by category in their data block (Flowise convention).
			for _, n := range flow.Nodes {
				cat, _ := n.Data["category"].(string)
				label, _ := n.Data["label"].(string)
				nm, _ := n.Data["name"].(string)
				display := label
				if display == "" {
					display = nm
				}
				r := ref{NodeID: n.ID, NodeType: n.Type, Label: display, Category: cat}
				lc := strings.ToLower(cat)
				switch {
				case strings.Contains(lc, "tool"):
					result.Tools = append(result.Tools, r)
				case strings.Contains(lc, "assistant"):
					result.Assistants = append(result.Assistants, r)
				case strings.Contains(lc, "document"):
					result.DocumentStore = append(result.DocumentStore, r)
				case strings.Contains(lc, "memory"):
					result.Memory = append(result.Memory, r)
				case strings.Contains(lc, "llm") || strings.Contains(lc, "chat models"):
					result.LLM = append(result.LLM, r)
				}

				// Variable references appear as nodes of type "Variable" or in inputs.
				if strings.EqualFold(nm, "setVariable") || strings.EqualFold(nm, "getVariable") || strings.EqualFold(cat, "variables") {
					result.Variables = append(result.Variables, r)
				}
			}

			// overrideConfig keys: scan each node's inputs for top-level fields the
			// flow accepts as overrides. Flowise exposes these via the `inputs`
			// map inside each node's data block.
			if showOverrides {
				keys := map[string]bool{}
				for _, n := range flow.Nodes {
					if inputs, ok := n.Data["inputs"].(map[string]any); ok {
						for k := range inputs {
							keys[k] = true
						}
					}
				}
				for k := range keys {
					result.Overrides = append(result.Overrides, k)
				}
				sort.Strings(result.Overrides)
			}

			// Cross-check tool/docstore names against local cache.
			if len(result.Tools) > 0 {
				toolRows, qerr := db.DB().QueryContext(cmd.Context(), `SELECT COALESCE(name,'') FROM tools`)
				if qerr == nil {
					known := map[string]bool{}
					for toolRows.Next() {
						var n string
						_ = toolRows.Scan(&n)
						known[strings.ToLower(n)] = true
					}
					toolRows.Close()
					for _, t := range result.Tools {
						if t.Label != "" && !known[strings.ToLower(t.Label)] {
							result.MissingRefs = append(result.MissingRefs, "tool:"+t.Label)
						}
					}
				}
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return flags.printJSON(cmd, result)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Chatflow: %s (%s)\n", result.ChatflowName, result.ChatflowID)
			fmt.Fprintf(cmd.OutOrStdout(), "Nodes: %d\n\n", result.NodeCount)
			printRefSection := func(title string, rs []ref) {
				if len(rs) == 0 {
					return
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s (%d):\n", title, len(rs))
				for _, r := range rs {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s [%s]\n", r.Label, r.NodeType)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			printRefSection("Tools", result.Tools)
			printRefSection("Assistants", result.Assistants)
			printRefSection("LLM nodes", result.LLM)
			printRefSection("Memory", result.Memory)
			printRefSection("Document stores", result.DocumentStore)
			printRefSection("Variables", result.Variables)
			if showOverrides && len(result.Overrides) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "overrideConfig keys (%d):\n", len(result.Overrides))
				for _, k := range result.Overrides {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", k)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			if len(result.MissingRefs) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", yellow(fmt.Sprintf("Missing references (%d):", len(result.MissingRefs))))
				for _, m := range result.MissingRefs {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", m)
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&showOverrides, "show-overrides", false, "List overrideConfig keys exposed by this flow's input nodes")
	return cmd
}
