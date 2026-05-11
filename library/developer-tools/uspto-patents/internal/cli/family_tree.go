package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// familyNode represents a single node in a patent family tree.
type familyNode struct {
	Depth             int    `json:"depth"`
	Relationship      string `json:"relationship"`
	ApplicationNumber string `json:"applicationNumber"`
	PatentNumber      string `json:"patentNumber,omitempty"`
	FilingDate        string `json:"filingDate,omitempty"`
	Status            string `json:"status,omitempty"`
}

func newFamilyTreeCmd(flags *rootFlags) *cobra.Command {
	var maxDepth int

	cmd := &cobra.Command{
		Use:   "family <applicationNumber>",
		Short: "Walk the full patent family tree via continuity data",
		Long: `Recursively walks the continuity endpoint to build the complete
patent family tree — continuations, divisionals, and CIPs.
Renders as an indented tree (terminal) or flat JSON array.`,
		Example: strings.Trim(`
  uspto-patents-pp-cli patent family 14412875
  uspto-patents-pp-cli patent family 14412875 --json
  uspto-patents-pp-cli patent family 14412875 --max-depth 3`, "\n"),
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

			appNum := args[0]
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			visited := map[string]bool{}
			var nodes []familyNode

			// BFS-style recursive walk
			type queueItem struct {
				appNum       string
				depth        int
				relationship string
			}
			queue := []queueItem{{appNum: appNum, depth: 0, relationship: "root"}}

			for len(queue) > 0 {
				item := queue[0]
				queue = queue[1:]

				if visited[item.appNum] {
					continue
				}
				if maxDepth > 0 && item.depth > maxDepth {
					continue
				}
				visited[item.appNum] = true

				node := familyNode{
					Depth:             item.depth,
					Relationship:      item.relationship,
					ApplicationNumber: item.appNum,
				}

				// Fetch continuity for this node
				contPath := replacePathParam("/api/v1/patent/applications/{num}/continuity", "num", item.appNum)
				contData, err := c.Get(contPath, nil)
				if err != nil {
					// Still record the node even if we can't fetch its continuity
					nodes = append(nodes, node)
					continue
				}

				// Extract family members from the continuity response
				relatives := extractFamilyMembers(contData)
				for _, rel := range relatives {
					if rel.ApplicationNumber == item.appNum {
						// Fill in details for current node from continuity data
						node.PatentNumber = rel.PatentNumber
						node.FilingDate = rel.FilingDate
						node.Status = rel.Status
						continue
					}
					if !visited[rel.ApplicationNumber] && rel.ApplicationNumber != "" {
						queue = append(queue, queueItem{
							appNum:       rel.ApplicationNumber,
							depth:        item.depth + 1,
							relationship: rel.Relationship,
						})
					}
				}

				nodes = append(nodes, node)
			}

			if len(nodes) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no family members found for application %s\n", appNum)
				return nil
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), nodes, flags)
			}

			// Indented tree output
			w := cmd.OutOrStdout()
			for _, n := range nodes {
				indent := strings.Repeat("  ", n.Depth)
				label := n.ApplicationNumber
				if n.PatentNumber != "" {
					label += " (US " + n.PatentNumber + ")"
				}
				if n.FilingDate != "" {
					label += " filed:" + n.FilingDate
				}
				if n.Status != "" {
					label += " [" + n.Status + "]"
				}
				fmt.Fprintf(w, "%s%s %s\n", indent, n.Relationship, label)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&maxDepth, "max-depth", 10, "Maximum recursion depth for family tree walk")
	return cmd
}

// extractFamilyMembers parses continuity response into family member records.
// Handles the USPTO patentFileWrapperDataBag envelope and both parent/child bags.
func extractFamilyMembers(data json.RawMessage) []familyNode {
	var members []familyNode

	var items []map[string]interface{}

	// USPTO envelope: patentFileWrapperDataBag[0].{parentContinuityBag, childContinuityBag}
	var envelope struct {
		Bags []map[string]json.RawMessage `json:"patentFileWrapperDataBag"`
	}
	if json.Unmarshal(data, &envelope) == nil && len(envelope.Bags) > 0 {
		for _, bag := range envelope.Bags {
			for _, key := range []string{"parentContinuityBag", "childContinuityBag"} {
				if raw, ok := bag[key]; ok {
					var arr []map[string]interface{}
					if json.Unmarshal(raw, &arr) == nil {
						items = append(items, arr...)
					}
				}
			}
		}
	}

	// Generic fallback
	if len(items) == 0 {
		if err := json.Unmarshal(data, &items); err != nil {
			var wrapper map[string]json.RawMessage
			if json.Unmarshal(data, &wrapper) == nil {
				for _, key := range []string{"continuityData", "parentContinuity", "childContinuity", "data", "results", "items"} {
					if raw, ok := wrapper[key]; ok {
						if json.Unmarshal(raw, &items) == nil && len(items) > 0 {
							break
						}
					}
				}
				if items == nil {
					for _, key := range []string{"parentContinuity", "childContinuity"} {
						if raw, ok := wrapper[key]; ok {
							var sub []map[string]interface{}
							if json.Unmarshal(raw, &sub) == nil {
								items = append(items, sub...)
							}
						}
					}
				}
			}
		}
	}

	for _, item := range items {
		node := familyNode{}

		// Application number — USPTO uses parentApplicationNumberText/childApplicationNumberText
		if v, ok := item["parentApplicationNumberText"]; ok && v != nil {
			node.ApplicationNumber = fmt.Sprintf("%v", v)
		} else if v, ok := item["childApplicationNumberText"]; ok && v != nil {
			node.ApplicationNumber = fmt.Sprintf("%v", v)
		} else if v, ok := item["applicationNumberText"]; ok && v != nil {
			node.ApplicationNumber = fmt.Sprintf("%v", v)
		} else if v, ok := item["applicationNumber"]; ok && v != nil {
			node.ApplicationNumber = fmt.Sprintf("%v", v)
		} else if v, ok := item["parentApplicationNumber"]; ok && v != nil {
			node.ApplicationNumber = fmt.Sprintf("%v", v)
		} else if v, ok := item["childApplicationNumber"]; ok && v != nil {
			node.ApplicationNumber = fmt.Sprintf("%v", v)
		}

		// Patent number
		if v, ok := item["childPatentNumber"]; ok && v != nil {
			node.PatentNumber = fmt.Sprintf("%v", v)
		} else if v, ok := item["patentNumber"]; ok && v != nil {
			node.PatentNumber = fmt.Sprintf("%v", v)
		}

		// Filing date
		for _, f := range []string{"parentApplicationFilingDate", "childApplicationFilingDate", "filingDate"} {
			if v, ok := item[f]; ok && v != nil {
				s := fmt.Sprintf("%v", v)
				if len(s) > 10 {
					s = s[:10]
				}
				node.FilingDate = s
				break
			}
		}

		// Status
		if v, ok := item["parentApplicationStatusDescriptionText"]; ok && v != nil {
			node.Status = fmt.Sprintf("%v", v)
		} else if v, ok := item["childApplicationStatusDescriptionText"]; ok && v != nil {
			node.Status = fmt.Sprintf("%v", v)
		} else if v, ok := item["status"]; ok && v != nil {
			node.Status = fmt.Sprintf("%v", v)
		} else if v, ok := item["applicationStatus"]; ok && v != nil {
			node.Status = fmt.Sprintf("%v", v)
		}

		// Relationship type
		if v, ok := item["claimParentageTypeCodeDescriptionText"]; ok && v != nil {
			node.Relationship = fmt.Sprintf("%v", v)
		} else if v, ok := item["claimType"]; ok && v != nil {
			node.Relationship = fmt.Sprintf("%v", v)
		} else if v, ok := item["relationshipType"]; ok && v != nil {
			node.Relationship = fmt.Sprintf("%v", v)
		} else if v, ok := item["type"]; ok && v != nil {
			node.Relationship = fmt.Sprintf("%v", v)
		}

		if node.ApplicationNumber != "" {
			members = append(members, node)
		}
	}

	return members
}
