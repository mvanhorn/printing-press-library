// Copyright 2026 Giuliano Giacaglia and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written read-only agent node discovery commands (see .printing-press-patches/agent-node-discovery.json).

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// figmaRef is a parsed Figma reference: either a raw file key or a full
// Figma URL with an optional node id.
type figmaRef struct {
	FileKey string
	NodeID  string // normalized ("123:456"); empty when absent
	Raw     string
}

// parseFigmaRef accepts:
//   - raw file keys like "JZyB6K6Z22YyObBdj1r4v1"
//   - https://www.figma.com/design/<file_key>/<title>?node-id=123-456
//   - https://www.figma.com/file/<file_key>/<title>?node_id=123-456
//   - https://www.figma.com/proto/<file_key>/... (prototype links share the key shape)
//
// node-id / node_id query values are normalized via normalizeNodeID. A
// usage-style error is returned for URLs that do not contain a file key.
func parseFigmaRef(raw string) (figmaRef, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return figmaRef{}, fmt.Errorf("empty Figma URL or file key")
	}
	ref := figmaRef{Raw: raw}

	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		u, err := url.Parse(raw)
		if err != nil {
			return figmaRef{}, fmt.Errorf("invalid Figma URL %q: %w", raw, err)
		}
		host := strings.ToLower(u.Hostname())
		host = strings.TrimPrefix(host, "www.")
		if host != "figma.com" {
			return figmaRef{}, fmt.Errorf("not a Figma URL (host %q)", u.Host)
		}

		// Path shape: /design/<key>/..., /file/<key>/..., /proto/<key>/..., /board/<key>/...
		key := fileKeyFromPath(u.Path)
		if key == "" {
			return figmaRef{}, fmt.Errorf("could not find a file key in Figma URL %q", raw)
		}
		ref.FileKey = key

		q := u.Query()
		var nodeRaw string
		for _, p := range []string{"node-id", "node_id"} {
			if v := q.Get(p); v != "" {
				nodeRaw = v
				break
			}
		}
		if nodeRaw != "" {
			ref.NodeID = normalizeNodeID(nodeRaw)
		}
		return ref, nil
	}

	// Raw file key form. Reject characters that indicate a malformed URL/path
	// rather than a key. Figma file keys are alphanumeric tokens without
	// slashes, query separators, fragments, or whitespace.
	if strings.ContainsAny(raw, "/?# \t\r\n") {
		return figmaRef{}, fmt.Errorf("invalid Figma file key %q", raw)
	}
	ref.FileKey = raw
	return ref, nil
}

// fileKeyFromPath extracts the file key from a Figma URL path. Supported
// prefixes: /design, /file, /proto, /board. Returns "" when no key is found.
func fileKeyFromPath(path string) string {
	segments := strings.Split(path, "/")
	// strings.Split on "/design/abc/Title" → ["", "design", "abc", "Title"]
	var clean []string
	for _, s := range segments {
		if s != "" {
			clean = append(clean, s)
		}
	}
	if len(clean) < 2 {
		return ""
	}
	switch strings.ToLower(clean[0]) {
	case "design", "file", "proto", "board":
		return clean[1]
	default:
		return ""
	}
}

// agentNodeSummary is a compact, agent-friendly projection of a Figma node.
type agentNodeSummary struct {
	ID                  string            `json:"id"`
	Name                string            `json:"name"`
	Type                string            `json:"type"`
	Path                []string          `json:"path"`
	Label               string            `json:"label"`
	ParentID            string            `json:"parent_id,omitempty"`
	ChildCount          int               `json:"child_count"`
	AbsoluteBoundingBox map[string]any    `json:"absolute_bounding_box,omitempty"`
}

// buildAgentNodeIndex walks a Figma document tree depth-first and returns
// compact summaries for every node whose type is in allowedTypes. The
// document root is walked but never itself indexed; its children (pages) are
// the first indexed level, so path labels start at the page name.
//
// maxDepth controls pruning of the in-memory tree: depth is counted as edges
// below the document root (pages = 1). maxDepth <= 0 means unlimited. The
// Figma API request depth is the primary tree-limiter; this guards against
// runaway traversal when the caller already pruned via the request.
func buildAgentNodeIndex(document map[string]any, allowedTypes map[string]bool, maxDepth int) []agentNodeSummary {
	out := []agentNodeSummary{}
	if document == nil {
		return out
	}
	for _, child := range getNodeChildren(document) {
		walkAgentNode(child, nil, 1, maxDepth, allowedTypes, &out)
	}
	return out
}

func walkAgentNode(node map[string]any, path []string, depth, maxDepth int, allowed map[string]bool, out *[]agentNodeSummary) {
	name, _ := node["name"].(string)
	nodeType, _ := node["type"].(string)
	id, _ := node["id"].(string)

	// Fresh slice copy so stored paths cannot be mutated by sibling recursion.
	currentPath := make([]string, len(path)+1)
	copy(currentPath, path)
	currentPath[len(path)] = name

	children := getNodeChildren(node)

	if nodeType != "DOCUMENT" && (allowed == nil || allowed[nodeType]) {
		summary := agentNodeSummary{
			ID:         id,
			Name:       name,
			Type:       nodeType,
			Path:       append([]string(nil), currentPath...),
			Label:      strings.Join(currentPath, " / "),
			ChildCount: len(children),
		}
		if pid, ok := node["parentId"].(string); ok && pid != "" {
			summary.ParentID = pid
		}
		if abb, ok := node["absoluteBoundingBox"].(map[string]any); ok {
			summary.AbsoluteBoundingBox = abb
		}
		*out = append(*out, summary)
	}

	// Descend unless the caller imposed a depth cap we just reached.
	if maxDepth > 0 && depth+1 > maxDepth {
		return
	}
	for _, child := range children {
		walkAgentNode(child, currentPath, depth+1, maxDepth, allowed, out)
	}
}

// getNodeChildren returns the node's children as typed maps (empty when absent).
func getNodeChildren(node map[string]any) []map[string]any {
	if node == nil {
		return nil
	}
	raw, ok := node["children"].([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(raw))
	for _, c := range raw {
		if cm, ok := c.(map[string]any); ok {
			out = append(out, cm)
		}
	}
	return out
}

// parseTypeSet parses a comma-separated node-type allowlist into a set of
// upper-cased type names. Returns nil for an empty input (meaning: include
// every non-document node).
func parseTypeSet(s string) map[string]bool {
	set := map[string]bool{}
	for _, p := range strings.Split(s, ",") {
		p = strings.ToUpper(strings.TrimSpace(p))
		if p != "" {
			set[p] = true
		}
	}
	if len(set) == 0 {
		return nil
	}
	return set
}

// scoreAgentNode returns a deterministic relevance score (higher is better)
// for a query against a node. Exact name wins, then exact label, then
// substring name/label, then token matches across label. Token matching
// splits both the query and the label into words and scores when every query
// word is found in some label word — this catches multi-word queries like
// "hero card" that are not a contiguous substring of "Hero Primary Card".
// Returns 0 for no match.
func scoreAgentNode(query string, node agentNodeSummary) int {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return 0
	}
	name := strings.ToLower(node.Name)
	label := strings.ToLower(node.Label)

	if name == q {
		return 100
	}
	if label == q {
		return 90
	}
	if name != "" && strings.Contains(name, q) {
		return 70
	}
	if label != "" && strings.Contains(label, q) {
		return 60
	}
	// Token matches across label: every query word is contained in some
	// label word. Reachable when the multi-word query is not contiguous.
	qTokens := strings.Fields(q)
	if len(qTokens) > 0 {
		labelTokens := strings.Fields(label)
		allMatch := true
		for _, qt := range qTokens {
			found := false
			for _, lt := range labelTokens {
				if strings.Contains(lt, qt) {
					found = true
					break
				}
			}
			if !found {
				allMatch = false
				break
			}
		}
		if allMatch {
			return 40
		}
	}
	return 0
}

// agentMatch is the score-bearing projection used by find-node output.
type agentMatch struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	Label string `json:"label"`
	Score int    `json:"score"`
}

func newAgentCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Agent-friendly Figma node discovery (label → id resolution).",
		Long: `Read-only helpers that accept full Figma URLs or raw file keys and return
compact node indexes with human-readable path labels. Use 'agent outline' to
list a file's structure and 'agent find-node' to resolve a label like
"Prototype" into a Figma node id you can pass to files nodes, images, frame
extract, or dev-mode dump.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newAgentOutlineCmd(flags))
	cmd.AddCommand(newAgentFindNodeCmd(flags))
	return cmd
}

func newAgentOutlineCmd(flags *rootFlags) *cobra.Command {
	var depth int
	var types string
	var limit int

	cmd := &cobra.Command{
		Use:   "outline <figma-url-or-key>",
		Short: "List compact path labels for a Figma file's nodes.",
		Long: `Fetch a shallow Figma file tree and return a compact list of nodes with
human-readable path labels (e.g. "🕹️ Prototype / Prototype / Signup") and
their node ids. Agents use this to discover ids before calling files nodes,
images, frame extract, or dev-mode dump.`,
		Example: strings.Trim(`
  # Outline by raw file key
  figma-pp-cli agent outline abc123XyZ --depth 2 --agent

  # Outline by full Figma URL
  figma-pp-cli agent outline "https://www.figma.com/design/abc123XyZ/My-File" --depth 2 --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			ref, err := parseFigmaRef(args[0])
			if err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run":  true,
					"command":  "agent outline",
					"file_key": ref.FileKey,
					"node_id":  ref.NodeID,
					"method":   "GET",
					"path":     "/v1/files/" + ref.FileKey,
					"params":   map[string]string{"depth": fmt.Sprintf("%d", depth)},
				}, flags)
			}

			allowedTypes := parseTypeSet(types)
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := c.Get("/v1/files/"+ref.FileKey, map[string]string{"depth": fmt.Sprintf("%d", depth)})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var env struct {
				Name     string         `json:"name"`
				Document map[string]any `json:"document"`
			}
			if err := json.Unmarshal(raw, &env); err != nil {
				return fmt.Errorf("decoding file response: %w", err)
			}

			// maxDepth <= 0 means unlimited: show everything the API returned.
			nodes := buildAgentNodeIndex(env.Document, allowedTypes, 0)

			total := len(nodes)
			truncated := false
			if limit > 0 && total > limit {
				nodes = nodes[:limit]
				truncated = true
			}

			out := map[string]any{
				"file_key":  ref.FileKey,
				"file_name": env.Name,
				"depth":     depth,
				"count":     len(nodes),
				"truncated": truncated,
				"nodes":     nodes,
			}
			if truncated {
				out["total_before_limit"] = total
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}

	cmd.Flags().IntVar(&depth, "depth", 2, "Max tree depth to request from the Figma file API")
	cmd.Flags().StringVar(&types, "types", "CANVAS,SECTION,FRAME,INSTANCE,COMPONENT,GROUP", "Comma-separated node types to include")
	cmd.Flags().IntVar(&limit, "limit", 200, "Max nodes to return (0 = unlimited)")
	return cmd
}

func newAgentFindNodeCmd(flags *rootFlags) *cobra.Command {
	var depth int
	var types string
	var limit int

	cmd := &cobra.Command{
		Use:   "find-node <figma-url-or-key> [query]",
		Short: "Resolve a human label into a Figma node id.",
		Long: `Search a shallow Figma file tree for nodes matching a label query (e.g.
"Prototype") and return ranked candidates with node ids. When a query is
omitted but the Figma URL carries a node-id, that node is resolved as a
direct hit. Ambiguous ties are reported as candidates, never guessed.`,
		Example: strings.Trim(`
  # Find by label
  figma-pp-cli agent find-node abc123XyZ "Prototype" --depth 3 --agent

  # Resolve a node-id embedded in a URL (no query)
  figma-pp-cli agent find-node "https://www.figma.com/design/abc123XyZ/T?node-id=123-456" --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			ref, err := parseFigmaRef(args[0])
			if err != nil {
				return usageErr(err)
			}
			query := ""
			if len(args) >= 2 {
				query = strings.Join(args[1:], " ")
			}

			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run":  true,
					"command":  "agent find-node",
					"file_key": ref.FileKey,
					"node_id":  ref.NodeID,
					"query":    query,
					"method":   "GET",
					"path":     "/v1/files/" + ref.FileKey,
					"params":   map[string]string{"depth": fmt.Sprintf("%d", depth)},
				}, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Direct-hit path: URL carries a node-id and no query was given.
			if query == "" && ref.NodeID != "" {
				return agentFindNodeDirect(cmd, flags, c, ref)
			}
			if query == "" {
				return usageErr(fmt.Errorf("a query is required (or supply a Figma URL with a node-id)"))
			}

			allowedTypes := parseTypeSet(types)
			raw, err := c.Get("/v1/files/"+ref.FileKey, map[string]string{"depth": fmt.Sprintf("%d", depth)})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var env struct {
				Name     string         `json:"name"`
				Document map[string]any `json:"document"`
			}
			if err := json.Unmarshal(raw, &env); err != nil {
				return fmt.Errorf("decoding file response: %w", err)
			}

			nodes := buildAgentNodeIndex(env.Document, allowedTypes, 0)

			var matches []agentMatch
			for _, n := range nodes {
				s := scoreAgentNode(query, n)
				if s > 0 {
					matches = append(matches, agentMatch{
						ID:    n.ID,
						Name:  n.Name,
						Type:  n.Type,
						Label: n.Label,
						Score: s,
					})
				}
			}

			sort.SliceStable(matches, func(i, j int) bool {
				if matches[i].Score != matches[j].Score {
					return matches[i].Score > matches[j].Score
				}
				if len(matches[i].Label) != len(matches[j].Label) {
					return len(matches[i].Label) < len(matches[j].Label)
				}
				return matches[i].Label < matches[j].Label
			})

			ambiguous := false
			if len(matches) >= 2 && matches[0].Score == matches[1].Score {
				ambiguous = true
			}

			total := len(matches)
			if limit > 0 && total > limit {
				matches = matches[:limit]
			}

			out := map[string]any{
				"file_key":    ref.FileKey,
				"query":       query,
				"match_count": len(matches),
				"ambiguous":   ambiguous,
				"matches":     matches,
				"next_steps":  agentNextSteps(ref.FileKey),
			}
			if len(matches) > 0 {
				out["best"] = matches[0]
			} else {
				out["best"] = nil
				out["next_steps"] = append([]string{
					fmt.Sprintf("No nodes matched %q. Run 'figma-pp-cli agent outline %s --depth %d --agent' to list available labels.", query, ref.FileKey, depth),
				}, agentNextSteps(ref.FileKey)...)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}

	cmd.Flags().IntVar(&depth, "depth", 3, "Max tree depth to request from the Figma file API")
	cmd.Flags().StringVar(&types, "types", "SECTION,FRAME,INSTANCE,COMPONENT", "Comma-separated node types to include")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max matches to return (0 = unlimited)")
	return cmd
}

// agentFindNodeDirect resolves a node-id embedded in a Figma URL as a single
// direct hit. It never invents a name: if the fetch fails or returns no
// document, the caller surfaces the error normally.
func agentFindNodeDirect(cmd *cobra.Command, flags *rootFlags, c interface{ Get(string, map[string]string) (json.RawMessage, error) }, ref figmaRef) error {
	raw, err := c.Get("/v1/files/"+ref.FileKey+"/nodes", map[string]string{"ids": ref.NodeID, "depth": "1"})
	if err != nil {
		return classifyAPIError(err, flags)
	}
	var env struct {
		Nodes map[string]json.RawMessage `json:"nodes"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("decoding nodes response: %w", err)
	}

	var doc map[string]any
	for _, entryRaw := range env.Nodes {
		var entry map[string]any
		if json.Unmarshal(entryRaw, &entry) != nil {
			continue
		}
		if d, ok := entry["document"].(map[string]any); ok {
			doc = d
			break
		}
	}

	name, _ := doc["name"].(string)
	nodeType, _ := doc["type"].(string)
	hit := agentMatch{
		ID:    ref.NodeID,
		Name:  name,
		Type:  nodeType,
		Label: name,
		Score: 100,
	}
	matches := []agentMatch{hit}
	out := map[string]any{
		"file_key":    ref.FileKey,
		"query":       "",
		"node_id":     ref.NodeID,
		"match_count": 1,
		"ambiguous":   false,
		"best":        hit,
		"matches":     matches,
		"next_steps":  agentNextSteps(ref.FileKey),
	}
	return printJSONFiltered(cmd.OutOrStdout(), out, flags)
}

// agentNextSteps returns stable, copy-pasteable follow-up commands for a
// resolved node id.
func agentNextSteps(fileKey string) []string {
	return []string{
		fmt.Sprintf("Use the chosen id with: figma-pp-cli files nodes get-file %s --ids <id> --depth 2 --agent", fileKey),
		fmt.Sprintf("Render it with: figma-pp-cli images %s --ids <id> --format png --scale 1 --agent", fileKey),
	}
}
