// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/ca7ai/pp-virustotal/internal/vtstore"
	"github.com/spf13/cobra"
)

func newPivotCmd(flags *rootFlags) *cobra.Command {
	var through string
	var to string
	var depth int
	var format string

	cmd := &cobra.Command{
		Use:   "pivot <type> <id>",
		Short: "Traverse VirusTotal relationship graph",
		Long: `Pivot through IOC relationships to discover connected threats.

Traverses file → domain → IP → file relationships using the VirusTotal API
and caches results for offline analysis.

Examples:
  # File to contacted domains
  virustotal-pp-cli pivot file <hash> --through domains

  # File to domains to resolved IPs
  virustotal-pp-cli pivot file <hash> --through domains --to ips --depth 2

  # IP to communicating files to contacted domains
  virustotal-pp-cli pivot ip 1.2.3.4 --through files --to domains --depth 3`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			startType := args[0]
			startID := args[1]

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			store, err := vtstore.Open()
			if err != nil {
				return fmt.Errorf("opening cache: %w", err)
			}
			defer store.Close()

			// Execute pivot traversal
			graph, err := executePivot(c, store, startType, startID, through, to, depth)
			if err != nil {
				return err
			}

			// Output results
			switch format {
			case "mermaid":
				return outputMermaid(cmd.OutOrStdout(), graph)
			case "json":
				return printOutput(cmd.OutOrStdout(), graph.ToJSON(), true)
			default:
				return outputPivotTable(cmd.OutOrStdout(), graph)
			}
		},
	}

	cmd.Flags().StringVar(&through, "through", "", "Relationship type to traverse (domains, ips, files, urls)")
	cmd.Flags().StringVar(&to, "to", "", "Final target type (domains, ips, files)")
	cmd.Flags().IntVar(&depth, "depth", 1, "Maximum traversal depth")
	cmd.Flags().StringVar(&format, "format", "table", "Output format: table, json, mermaid")

	return cmd
}

// PivotGraph represents the traversal result
type PivotGraph struct {
	StartType string                 `json:"start_type"`
	StartID   string                 `json:"start_id"`
	Depth     int                    `json:"depth"`
	Nodes     map[string]*PivotNode  `json:"nodes"`
	Edges     []PivotEdge            `json:"edges"`
}

type PivotNode struct {
	Type  string         `json:"type"`
	ID    string         `json:"id"`
	Data  json.RawMessage `json:"data,omitempty"`
	Depth int            `json:"depth"`
}

type PivotEdge struct {
	Source       string `json:"source"`
	Target       string `json:"target"`
	Relationship string `json:"relationship"`
}

func (g *PivotGraph) ToJSON() json.RawMessage {
	data, _ := json.MarshalIndent(g, "", "  ")
	return json.RawMessage(data)
}

func executePivot(c interface{}, store *vtstore.VTStore, startType, startID, through, to string, maxDepth int) (*PivotGraph, error) {
	graph := &PivotGraph{
		StartType: startType,
		StartID:   startID,
		Depth:     maxDepth,
		Nodes:     make(map[string]*PivotNode),
	}

	// BFS traversal
	type queueItem struct {
		typ   string
		id    string
		depth int
	}

	queue := []queueItem{{typ: startType, id: startID, depth: 0}}
	visited := make(map[string]bool)

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		nodeKey := current.typ + ":" + current.id
		if visited[nodeKey] || current.depth > maxDepth {
			continue
		}
		visited[nodeKey] = true

		// Fetch data for current node
		data, err := fetchIOCData(c, store, current.typ, current.id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to fetch %s/%s: %v\n", current.typ, current.id, err)
			continue
		}

		graph.Nodes[nodeKey] = &PivotNode{
			Type:  current.typ,
			ID:    current.id,
			Data:  data,
			Depth: current.depth,
		}

		if current.depth >= maxDepth {
			continue
		}

		// Determine which relationships to follow
		var relTypes []string
		if through != "" && current.depth == 0 {
			relTypes = []string{through}
		} else if to != "" && current.depth == maxDepth-1 {
			relTypes = []string{to}
		} else {
			relTypes = getDefaultRelationships(current.typ)
		}

		// Traverse relationships
		for _, relType := range relTypes {
			targets, err := extractRelationships(data, current.typ, relType)
			if err != nil {
				continue
			}

			for _, target := range targets {
				// Store relationship
				store.StoreRelationship(current.typ, current.id, relType, target.Type, target.ID)

				graph.Edges = append(graph.Edges, PivotEdge{
					Source:       nodeKey,
					Target:       target.Type + ":" + target.ID,
					Relationship: relType,
				})

				// Enqueue if not at max depth
				if current.depth+1 <= maxDepth {
					queue = append(queue, queueItem{
						typ:   target.Type,
						id:    target.ID,
						depth: current.depth + 1,
					})
				}
			}
		}
	}

	return graph, nil
}

func fetchIOCData(c interface{}, store *vtstore.VTStore, iocType, iocID string) (json.RawMessage, error) {
	// Try cache first
	switch iocType {
	case "file", "files":
		if report, _ := store.GetFile(iocID); report != nil {
			return report.Data, nil
		}
		// Fetch from API
		client := c.(interface{ Get(string, map[string]string) (json.RawMessage, error) })
		data, err := client.Get("/files/"+url.PathEscape(iocID), nil)
		if err == nil {
			// Store in cache
			storeFileData(store, iocID, data)
		}
		return data, err

	case "domain", "domains":
		if data, _ := store.GetDomain(iocID); data != nil {
			return data, nil
		}
		client := c.(interface{ Get(string, map[string]string) (json.RawMessage, error) })
		data, err := client.Get("/domains/"+url.PathEscape(iocID), nil)
		if err == nil {
			store.StoreDomain(iocID, data)
		}
		return data, err

	case "ip", "ips", "ip-address", "ip_addresses":
		if data, _ := store.GetIP(iocID); data != nil {
			return data, nil
		}
		client := c.(interface{ Get(string, map[string]string) (json.RawMessage, error) })
		data, err := client.Get("/ip_addresses/"+url.PathEscape(iocID), nil)
		if err == nil {
			store.StoreIP(iocID, data)
		}
		return data, err
	}

	return nil, fmt.Errorf("unsupported IOC type: %s", iocType)
}

func getDefaultRelationships(iocType string) []string {
	switch iocType {
	case "file", "files":
		return []string{"domains", "ips"}
	case "domain", "domains":
		return []string{"ips", "files"}
	case "ip", "ips", "ip-address", "ip_addresses":
		return []string{"domains", "files"}
	default:
		return []string{}
	}
}

func extractRelationships(data json.RawMessage, sourceType, targetType string) ([]struct{ Type, ID string }, error) {
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, err
	}

	// Navigate to data.attributes.* for relationship fields
	attributes, ok := getNestedMap(parsed, "data", "attributes")
	if !ok {
		return nil, fmt.Errorf("no attributes found")
	}

	var results []struct{ Type, ID string }

	// Map relationship field names
	var fieldName string
	switch sourceType {
	case "file", "files":
		switch targetType {
		case "domains":
			fieldName = "contacted_domains"
		case "ips":
			fieldName = "contacted_ips"
		}
	case "domain", "domains":
		switch targetType {
		case "ips":
			fieldName = "last_dns_records"
		case "files":
			fieldName = "communicating_files"
		}
	case "ip", "ips", "ip-address", "ip_addresses":
		switch targetType {
		case "domains":
			fieldName = "resolutions"
		case "files":
			fieldName = "communicating_files"
		}
	}

	if fieldName == "" {
		return nil, fmt.Errorf("no relationship mapping for %s -> %s", sourceType, targetType)
	}

	// Extract relationship data
	relData, ok := attributes[fieldName]
	if !ok {
		return nil, fmt.Errorf("field %s not found", fieldName)
	}

	// Handle different response structures
	switch v := relData.(type) {
	case []interface{}:
		for _, item := range v {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if id, ok := itemMap["id"].(string); ok {
					results = append(results, struct{ Type, ID string }{
						Type: normalizeIOCType(targetType),
						ID:   id,
					})
				}
			} else if str, ok := item.(string); ok {
				results = append(results, struct{ Type, ID string }{
					Type: normalizeIOCType(targetType),
					ID:   str,
				})
			}
		}
	case map[string]interface{}:
		// Handle DNS records structure
		if records, ok := v["data"].([]interface{}); ok {
			for _, rec := range records {
				if recMap, ok := rec.(map[string]interface{}); ok {
					if val, ok := recMap["value"].(string); ok {
						results = append(results, struct{ Type, ID string }{
							Type: normalizeIOCType(targetType),
							ID:   val,
						})
					}
				}
			}
		}
	}

	return results, nil
}

func getNestedMap(m map[string]any, keys ...string) (map[string]any, bool) {
	current := m
	for _, key := range keys {
		next, ok := current[key].(map[string]any)
		if !ok {
			return nil, false
		}
		current = next
	}
	return current, true
}

func normalizeIOCType(t string) string {
	t = strings.ToLower(t)
	switch t {
	case "files", "file":
		return "file"
	case "domains", "domain":
		return "domain"
	case "ips", "ip", "ip-address", "ip_addresses":
		return "ip"
	default:
		return t
	}
}

func storeFileData(store *vtstore.VTStore, hash string, data json.RawMessage) {
	var parsed map[string]any
	json.Unmarshal(data, &parsed)

	attributes, ok := getNestedMap(parsed, "data", "attributes")
	if !ok {
		return
	}

	report := &vtstore.FileReport{
		SHA256: hash,
		Data:   data,
	}

	if md5, ok := attributes["md5"].(string); ok {
		report.MD5 = md5
	}
	if sha1, ok := attributes["sha1"].(string); ok {
		report.SHA1 = sha1
	}
	if size, ok := attributes["size"].(float64); ok {
		report.Size = int64(size)
	}
	if lastStats, ok := attributes["last_analysis_stats"].(map[string]interface{}); ok {
		if mal, ok := lastStats["malicious"].(float64); ok {
			report.MaliciousVotes = int(mal)
		}
		if harm, ok := lastStats["harmless"].(float64); ok {
			report.HarmlessVotes = int(harm)
		}
		total := report.MaliciousVotes + report.HarmlessVotes
		if total > 0 {
			report.DetectionRatio = fmt.Sprintf("%d/%d", report.MaliciousVotes, total)
		}
	}

	store.StoreFile(report)
}

func outputPivotTable(w interface{ Write([]byte) (int, error) }, graph *PivotGraph) error {
	fmt.Fprintf(w, "Pivot Graph: %s %s (depth: %d)\n\n", graph.StartType, graph.StartID, graph.Depth)
	fmt.Fprintf(w, "Nodes: %d\n", len(graph.Nodes))
	fmt.Fprintf(w, "Edges: %d\n\n", len(graph.Edges))

	fmt.Fprintf(w, "Relationships:\n")
	for _, edge := range graph.Edges {
		fmt.Fprintf(w, "  %s -[%s]-> %s\n", edge.Source, edge.Relationship, edge.Target)
	}

	return nil
}

func outputMermaid(w interface{ Write([]byte) (int, error) }, graph *PivotGraph) error {
	fmt.Fprintf(w, "graph TD\n")

	// Output nodes
	for key, node := range graph.Nodes {
		safeKey := strings.ReplaceAll(key, ":", "_")
		label := fmt.Sprintf("%s\\n%s", node.Type, truncateString(node.ID, 20))
		fmt.Fprintf(w, "  %s[\"%s\"]\n", safeKey, label)
	}

	// Output edges
	for _, edge := range graph.Edges {
		source := strings.ReplaceAll(edge.Source, ":", "_")
		target := strings.ReplaceAll(edge.Target, ":", "_")
		fmt.Fprintf(w, "  %s -->|%s| %s\n", source, edge.Relationship, target)
	}

	return nil
}

func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
