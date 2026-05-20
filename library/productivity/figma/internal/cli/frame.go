// Copyright 2026 giuliano-giacaglia. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// normalizeNodeID converts URL-style hyphenated node ids ("1234-5678") into
// the API form ("1234:5678") while preserving chain separators (";") and the
// instance prefix ("I"). Chains like "I5666:180910;1:10515" are
// preserved end-to-end. Multiple chained ids supplied as a single
// hyphenated form ("I5666-180910;1-10515") are normalized member-by-member.
func normalizeNodeID(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	// Split on chain separator first; normalize each segment independently.
	parts := strings.Split(s, ";")
	for i, p := range parts {
		// In each segment, the first hyphen separating the integer-pair becomes
		// the canonical colon. Any subsequent hyphens (rare but possible in
		// some component instance ids) are also converted, since Figma's
		// canonical form uses ":" everywhere inside a single id.
		parts[i] = strings.ReplaceAll(p, "-", ":")
	}
	return strings.Join(parts, ";")
}

// normalizeNodeIDList accepts a comma-separated or repeated list, normalizes
// each id, and returns the comma-joined canonical form.
func normalizeNodeIDList(ids []string) string {
	var out []string
	for _, raw := range ids {
		for _, piece := range strings.Split(raw, ",") {
			piece = strings.TrimSpace(piece)
			if piece == "" {
				continue
			}
			out = append(out, normalizeNodeID(piece))
		}
	}
	return strings.Join(out, ",")
}

func newFrameCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "frame",
		Short: "Codegen-friendly frame extraction.",
	}
	cmd.AddCommand(newFrameExtractCmd(flags))
	return cmd
}

func newFrameExtractCmd(flags *rootFlags) *cobra.Command {
	var ids []string
	var depth int
	var include string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "extract <key>",
		Short: "Extract a single frame as a compact codegen-ready payload.",
		Long: `Fuse a simplified node tree, in-scope variables, dev resources, and Code Connect
mappings for one or more nodes in a file. Heavy fields (geometry, effects, fills)
are deduped into a global styleRegistry; SVG-like one-vector containers are collapsed.`,
		Example: strings.Trim(`
  # Extract a single frame
  figma-pp-cli frame extract abc123XyZ --ids 1234:5678

  # URL form (hyphens) is accepted and normalized
  figma-pp-cli frame extract abc123XyZ --ids 1234-5678 --depth 6

  # Multiple ids, with variables and dev-resources fused
  figma-pp-cli frame extract abc123XyZ --ids 1234:5678,I5666:180910 --include variables,dev-resources
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run": true, "command": "frame extract"}`)
				return nil
			}
			key := args[0]
			normIDs := normalizeNodeIDList(ids)
			if normIDs == "" {
				return usageErr(fmt.Errorf("--ids is required (one or more node ids, comma-separated or repeated)"))
			}
			includeSet := map[string]bool{}
			for _, p := range strings.Split(include, ",") {
				p = strings.TrimSpace(strings.ToLower(p))
				if p != "" {
					includeSet[p] = true
				}
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// 1. Node tree
			nodesPath := "/v1/files/" + key + "/nodes"
			nodesParams := map[string]string{"ids": normIDs, "depth": fmt.Sprintf("%d", depth)}
			nodesRaw, err := c.Get(nodesPath, nodesParams)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var nodesEnv struct {
				Name  string                     `json:"name"`
				Nodes map[string]json.RawMessage `json:"nodes"`
			}
			if err := json.Unmarshal(nodesRaw, &nodesEnv); err != nil {
				return fmt.Errorf("decoding nodes response: %w", err)
			}

			reg := newStyleRegistry()
			simplified := make([]map[string]any, 0, len(nodesEnv.Nodes))
			count := 0
			for _, raw := range nodesEnv.Nodes {
				var entry map[string]any
				if err := json.Unmarshal(raw, &entry); err != nil {
					continue
				}
				docRaw, _ := entry["document"].(map[string]any)
				if docRaw == nil {
					continue
				}
				s := simplifyNode(docRaw, reg, &count)
				if s != nil {
					simplified = append(simplified, s)
				}
			}

			out := map[string]any{
				"fileKey":             key,
				"ids":                 splitCSV(normIDs),
				"depth":               depth,
				"simplifiedNodeCount": count,
				"nodes":               simplified,
				"styleRegistry":       reg.entries,
				"fetchedAt":           time.Now().UTC().Format(time.RFC3339),
			}

			// 2. Variables
			if includeSet["variables"] {
				varsRaw, verr := c.Get("/v1/files/"+key+"/variables/local", nil)
				if verr == nil {
					var dec any
					if json.Unmarshal(varsRaw, &dec) == nil {
						out["variables"] = dec
					}
				} else {
					out["variablesError"] = verr.Error()
				}
			}

			// 3. Dev resources (server-side filter on node_ids when supplied).
			if includeSet["dev-resources"] {
				drParams := map[string]string{}
				if normIDs != "" {
					drParams["node_ids"] = normIDs
				}
				drRaw, derr := c.Get("/v1/files/"+key+"/dev_resources", drParams)
				if derr == nil {
					var dec any
					if json.Unmarshal(drRaw, &dec) == nil {
						out["devResources"] = dec
					}
				} else {
					out["devResourcesError"] = derr.Error()
				}
			}

			// 4. Code Connect — placeholder; the public REST endpoint is
			// limited. Surface a stub so consumers know the channel was
			// requested but no mapping was returned.
			if includeSet["code-connect"] {
				out["codeConnect"] = []any{}
			}

			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}

	cmd.Flags().StringSliceVar(&ids, "ids", nil, "Node ids (comma-separated or repeated). Accepts 1234:5678 or 1234-5678 form.")
	cmd.Flags().IntVar(&depth, "depth", 4, "Max tree depth to include")
	cmd.Flags().StringVar(&include, "include", "variables,dev-resources", "Side-channel data to fuse: variables,dev-resources,code-connect")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/figma-pp-cli/data.db)")

	return cmd
}

// styleRegistry collects unique style specs (fills/effects/typography) into
// short ids ("s1", "s2") so the simplified tree can carry references rather
// than inline duplicates.
type styleRegistry struct {
	entries map[string]any
	keys    map[string]string
	next    int
}

func newStyleRegistry() *styleRegistry {
	return &styleRegistry{entries: map[string]any{}, keys: map[string]string{}}
}

// add returns a stable short id for the given spec, deduping by canonical key.
func (r *styleRegistry) add(spec any) string {
	b, err := json.Marshal(spec)
	if err != nil {
		return ""
	}
	k := string(b)
	if id, ok := r.keys[k]; ok {
		return id
	}
	r.next++
	id := fmt.Sprintf("s%d", r.next)
	r.keys[k] = id
	r.entries[id] = spec
	return id
}

// simplifyNode walks the tree pre-order. It keeps lightweight fields, replaces
// fills/effects/style with style-registry refs, and collapses single-vector
// FRAME/GROUP containers into the child while preserving the parent transform.
func simplifyNode(n map[string]any, reg *styleRegistry, count *int) map[string]any {
	if n == nil {
		return nil
	}
	*count++
	out := map[string]any{}
	for _, k := range []string{"id", "type", "name", "componentId", "absoluteBoundingBox", "layoutMode"} {
		if v, ok := n[k]; ok {
			out[k] = v
		}
	}
	if v, ok := n["boundVariables"]; ok {
		out["boundVariables"] = v
	}
	// Compact fills → hex+opacity refs in registry.
	if fills, ok := n["fills"].([]any); ok && len(fills) > 0 {
		var refs []map[string]string
		for _, f := range fills {
			fm, ok := f.(map[string]any)
			if !ok {
				continue
			}
			compact := compactFill(fm)
			id := reg.add(compact)
			if id != "" {
				refs = append(refs, map[string]string{"$ref": id})
			}
		}
		if len(refs) > 0 {
			out["fills"] = refs
		}
	}
	// effects → registry refs.
	if effects, ok := n["effects"].([]any); ok && len(effects) > 0 {
		var refs []map[string]string
		for _, e := range effects {
			id := reg.add(e)
			if id != "" {
				refs = append(refs, map[string]string{"$ref": id})
			}
		}
		if len(refs) > 0 {
			out["effects"] = refs
		}
	}
	// style → ref (typography/etc.)
	if style, ok := n["style"]; ok {
		id := reg.add(style)
		if id != "" {
			out["style"] = map[string]string{"$ref": id}
		}
	}

	// children
	if rawChildren, ok := n["children"].([]any); ok && len(rawChildren) > 0 {
		simplified := make([]map[string]any, 0, len(rawChildren))
		for _, c := range rawChildren {
			cm, ok := c.(map[string]any)
			if !ok {
				continue
			}
			s := simplifyNode(cm, reg, count)
			if s != nil {
				simplified = append(simplified, s)
			}
		}

		// Collapse single-vector containers: FRAME/GROUP with exactly one
		// VECTOR/BOOLEAN_OPERATION child → keep the child but preserve the
		// parent's absoluteBoundingBox so callers still know its on-screen
		// position.
		nodeType, _ := out["type"].(string)
		if (nodeType == "FRAME" || nodeType == "GROUP") && len(simplified) == 1 {
			childType, _ := simplified[0]["type"].(string)
			if childType == "VECTOR" || childType == "BOOLEAN_OPERATION" {
				if abb, ok := out["absoluteBoundingBox"]; ok {
					if _, has := simplified[0]["absoluteBoundingBox"]; !has {
						simplified[0]["absoluteBoundingBox"] = abb
					}
				}
				return simplified[0]
			}
		}

		out["children"] = simplified
	}
	return out
}

// compactFill reduces a Figma fill to a small hex+opacity description.
func compactFill(f map[string]any) map[string]any {
	out := map[string]any{}
	if t, ok := f["type"]; ok {
		out["type"] = t
	}
	if op, ok := f["opacity"]; ok {
		out["opacity"] = op
	}
	if c, ok := f["color"].(map[string]any); ok {
		r, _ := c["r"].(float64)
		g, _ := c["g"].(float64)
		b, _ := c["b"].(float64)
		a, _ := c["a"].(float64)
		out["hex"] = fmt.Sprintf("#%02X%02X%02X", clamp255(r), clamp255(g), clamp255(b))
		if a > 0 && a < 1 {
			out["alpha"] = a
		}
	}
	// Preserve gradient stops as-is (compact already).
	if stops, ok := f["gradientStops"]; ok {
		out["gradientStops"] = stops
	}
	// stable ordering by serializing keys
	return sortedMap(out)
}

func clamp255(v float64) int {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	return int(v*255 + 0.5)
}

// sortedMap returns a copy with deterministic key ordering when serialized.
func sortedMap(m map[string]any) map[string]any {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make(map[string]any, len(m))
	for _, k := range keys {
		out[k] = m[k]
	}
	return out
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
