// Copyright 2026 grahac and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// newSiteConfigCmd prints the customer-types → personas → questions tree
// for a site, with the actual edit-command hints below so the user can
// copy-paste mutations to the spec-emitted CRUD commands. Non-interactive.
func newSiteConfigCmd(flags *rootFlags) *cobra.Command {
	var siteUUID string
	var depth string

	cmd := &cobra.Command{
		Use:   "site-config",
		Short: "Show the customer-types / personas / questions tree for a site, with copy-paste edit hints.",
		Long: `Print the full audit configuration for a site as a nested tree:

  customer-types → personas → questions

Each node shows its name + UUID. Below the tree, the command emits the
exact follow-up command for adding, updating, or removing nodes. JSON
output (--json or --agent) returns the same data as a structured tree
that agents can walk programmatically.

The site is read from $BOTSEE_SITE_UUID or ~/.botsee/config.json by
default; pass --site to override.`,
		Example: "  botsee-pp-cli site-config --site $SITE_UUID\n" +
			"  botsee-pp-cli site-config --site $SITE_UUID --agent\n" +
			"  botsee-pp-cli site-config --site $SITE_UUID --depth personas",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:novel":      "site-config",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			resolved := siteUUID
			if resolved == "" {
				resolved = strings.TrimSpace(os.Getenv("BOTSEE_SITE_UUID"))
			}
			if resolved == "" {
				return fmt.Errorf("--site is required (or set $BOTSEE_SITE_UUID)")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			tree, err := buildSiteConfigTree(ctx, c, resolved, depth)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if flags.asJSON || flags.agent || !isTerminal(out) {
				return printJSONFiltered(out, tree, flags)
			}

			renderSiteConfigText(out, tree, depth)
			return nil
		},
	}
	cmd.Flags().StringVar(&siteUUID, "site", "", "Site UUID (overrides $BOTSEE_SITE_UUID and config)")
	cmd.Flags().StringVar(&depth, "depth", "questions", "Tree depth: customer-types | personas | questions")
	return cmd
}

type siteConfigNode struct {
	UUID        string            `json:"uuid"`
	Name        string            `json:"name,omitempty"`
	Description string            `json:"description,omitempty"`
	Children    []*siteConfigNode `json:"children,omitempty"`
	Kind        string            `json:"kind"`           // customer_type | persona | question
	Text        string            `json:"text,omitempty"` // question body
}

type siteConfigTree struct {
	SiteUUID      string            `json:"site_uuid"`
	CustomerTypes []*siteConfigNode `json:"customer_types"`
	Counts        map[string]int    `json:"counts"`
	EditHints     []string          `json:"edit_hints"`
}

func buildSiteConfigTree(ctx context.Context, c httpClient, siteUUID, depth string) (*siteConfigTree, error) {
	tree := &siteConfigTree{
		SiteUUID: siteUUID,
		Counts:   map[string]int{"customer_types": 0, "personas": 0, "questions": 0},
	}

	ctData, err := c.Get(ctx, "/api/v1/sites/"+siteUUID+"/customer-types", nil)
	if err != nil {
		return nil, fmt.Errorf("listing customer types: %w", err)
	}
	cts := extractList(ctData, "customer_types")
	tree.Counts["customer_types"] = len(cts)

	for _, ct := range cts {
		ctNode := &siteConfigNode{
			UUID:        asString(ct["uuid"]),
			Name:        asString(ct["name"]),
			Description: asString(ct["description"]),
			Kind:        "customer_type",
		}
		if depth != "customer-types" {
			pData, perr := c.Get(ctx, "/api/v1/customer-types/"+ctNode.UUID+"/personas", nil)
			if perr != nil {
				// Surface per-CT failure to stderr so the user sees an
				// incomplete tree instead of silently missing personas.
				fmt.Fprintf(os.Stderr, "warning: listing personas for customer-type %s failed: %v\n", ctNode.UUID, perr)
			} else {
				personas := extractList(pData, "personas")
				tree.Counts["personas"] += len(personas)
				for _, p := range personas {
					pNode := &siteConfigNode{
						UUID:        asString(p["uuid"]),
						Name:        asString(p["name"]),
						Description: asString(p["description"]),
						Kind:        "persona",
					}
					if depth == "questions" {
						qData, qerr := c.Get(ctx, "/api/v1/personas/"+pNode.UUID+"/questions", nil)
						if qerr != nil {
							fmt.Fprintf(os.Stderr, "warning: listing questions for persona %s failed: %v\n", pNode.UUID, qerr)
						} else {
							questions := extractList(qData, "questions")
							tree.Counts["questions"] += len(questions)
							for _, q := range questions {
								pNode.Children = append(pNode.Children, &siteConfigNode{
									UUID: asString(q["uuid"]),
									Text: asString(q["question"]),
									Kind: "question",
								})
							}
						}
					}
					ctNode.Children = append(ctNode.Children, pNode)
				}
			}
		}
		tree.CustomerTypes = append(tree.CustomerTypes, ctNode)
	}

	tree.EditHints = []string{
		"# Add a customer type:    botsee-pp-cli customer-types create --site " + siteUUID + " --name \"...\" --description \"...\"",
		"# Add a persona:           botsee-pp-cli personas create --customer-type <ct-uuid> --name \"...\" --description \"...\"",
		"# Add a question:          botsee-pp-cli questions create --persona <persona-uuid> --text \"...\"",
		"# Update a node:           botsee-pp-cli {customer-types|personas|questions} update <uuid> --name \"...\"",
		"# Remove a node:           botsee-pp-cli {customer-types|personas|questions} delete <uuid>",
		"# LLM-generate more:       botsee-pp-cli {customer-types|personas|questions} generate <parent-uuid> --count N",
	}
	return tree, nil
}

func renderSiteConfigText(w interface{ Write([]byte) (int, error) }, tree *siteConfigTree, depth string) {
	fmt.Fprintf(w, "Site: %s\n", tree.SiteUUID)
	fmt.Fprintf(w, "  %d customer types  •  %d personas  •  %d questions\n\n",
		tree.Counts["customer_types"], tree.Counts["personas"], tree.Counts["questions"])
	for ctIdx, ct := range tree.CustomerTypes {
		ctLast := ctIdx == len(tree.CustomerTypes)-1
		ctPrefix := "├─"
		if ctLast {
			ctPrefix = "└─"
		}
		fmt.Fprintf(w, "%s %s (ct=%s)\n", ctPrefix, nameOr(ct.Name, "(unnamed)"), ct.UUID)
		// Vertical guide line under this customer-type's children: '│' if more
		// customer types follow, blank if this is the last customer-type.
		vbar := "│"
		if ctLast {
			vbar = " "
		}
		for i, p := range ct.Children {
			isLast := i == len(ct.Children)-1
			pre := vbar + "  ├─"
			if isLast {
				pre = vbar + "  └─"
			}
			fmt.Fprintf(w, "%s %s (persona=%s)\n", pre, nameOr(p.Name, "(unnamed)"), p.UUID)
			if depth == "questions" {
				// Vertical guide under the persona: '│' if more personas
				// follow, blank if this persona is the last child of the
				// current customer-type.
				pbar := "│"
				if isLast {
					pbar = " "
				}
				for j, q := range p.Children {
					qLast := j == len(p.Children)-1
					qConn := "├─"
					if qLast {
						qConn = "└─"
					}
					fmt.Fprintf(w, "%s  %s  %s \"%s\" (q=%s)\n", vbar, pbar, qConn, truncate(q.Text, 80), q.UUID)
				}
			}
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Actions:")
	for _, h := range tree.EditHints {
		fmt.Fprintln(w, "  "+h)
	}
}

func nameOr(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

// Compile-time check json package used
var _ = json.Unmarshal
