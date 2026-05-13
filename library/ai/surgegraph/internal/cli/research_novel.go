package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// newResearchNovelCmd parents three transcendence-tier research commands:
// drift, gaps publish, and domain diff. The flat spec-derived
// `get-topic-research`, `get-domain-research`, `get-topic-map`,
// `create-topic-research-expansion`, etc. commands remain available.
func newResearchNovelCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "research",
		Short: "Topic / domain research transcendence commands (drift, gaps publish, diff)",
		Long: strings.TrimSpace(`
  research drift           Diff a topic_research's tree against shipped docs
  research gaps publish    Topic-gap -> bulk-create -> publish to WordPress (one command)
  research domain diff     Set-diff own topic_research vs competitor domain_research

The compound 'gaps publish' workflow respects the API's 50-doc bulk cap
and is idempotent on retry — the underlying create_bulk_documents call
de-duplicates by external_id when present.
`),
	}
	cmd.AddCommand(newResearchDriftCmd(flags))
	cmd.AddCommand(newResearchGapsCmd(flags))
	cmd.AddCommand(newResearchDomainDiffCmd(flags))
	return cmd
}

// ---------- research drift ----------

func newResearchDriftCmd(flags *rootFlags) *cobra.Command {
	var researchID, projectID string
	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Diff a topic_research's hierarchical map against shipped writer/optimized docs",
		Long: strings.TrimSpace(`
Fetches the topic_research (with its hierarchical tree) and the project's
writer/optimized documents in the same call. For each micro-topic, marks
"covered" when a document title fuzzy-matches the topic, or "gap" when no
document covers it. Useful for planning a content sprint.
`),
		Example: strings.Trim(`
  surgegraph-pp-cli research drift --research-id res_xyz789 --project proj_abc123 --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if researchID == "" || projectID == "" {
				if dryRunOK(flags) {
					return nil
				}
				return errors.New("--research-id and --project are required")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			topicData, _, err := c.Post("/v1/get_topic_research", map[string]any{
				"projectId":  projectID,
				"researchId": researchID,
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			writerDocs, _, err := c.Post("/v1/get_writer_documents", map[string]any{"projectId": projectID})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			// PATCH(greptile-7): also fetch optimized documents. The command's
			// short/long descriptions promise "writer/optimized" coverage, so
			// a topic covered exclusively by a Content Optimizer article must
			// not be classified as a gap. classifyAPIError is intentionally
			// not used for the optimizer leg: a project without the optimizer
			// add-on still has a valid writer-doc result; we don't want to
			// fail-loud and force users to know which add-ons are active.
			optimizedDocs, _, optErr := c.Post("/v1/get_optimized_documents", map[string]any{"projectId": projectID})
			docTitles := collectDocTitles(writerDocs)
			if optErr == nil {
				docTitles = append(docTitles, collectDocTitles(optimizedDocs)...)
			}
			topics := extractTopicTreeLeaves(topicData)
			type row struct {
				Topic    string `json:"topic"`
				Parent   string `json:"parent,omitempty"`
				Covered  bool   `json:"covered"`
				MatchDoc string `json:"match_doc,omitempty"`
			}
			result := make([]row, 0, len(topics))
			for _, t := range topics {
				r := row{Topic: t.name, Parent: t.parent}
				lc := strings.ToLower(t.name)
				for _, title := range docTitles {
					if strings.Contains(title, lc) || strings.Contains(lc, title) {
						r.Covered = true
						r.MatchDoc = title
						break
					}
				}
				result = append(result, r)
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&researchID, "research-id", "", "Topic research ID")
	cmd.Flags().StringVar(&projectID, "project", "", "Project ID")
	return cmd
}

// ---------- research gaps (parent + publish leaf) ----------

func newResearchGapsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gaps",
		Short: "Topic-gap subcommands",
	}
	cmd.AddCommand(newResearchGapsPublishCmd(flags))
	return cmd
}

func newResearchGapsPublishCmd(flags *rootFlags) *cobra.Command {
	var researchID, integration, projectID string
	var batchSize int
	cmd := &cobra.Command{
		Use:   "publish",
		Short: "Take topic-gaps -> bulk-create documents -> publish to WordPress (one command, idempotent)",
		Long: strings.TrimSpace(`
Compound pipeline:
  1. get_topic_map(projectId) -> enumerate gap topics
  2. create_bulk_documents in batches of --batch-size (cap 50)
  3. publish_document_to_cms(documentId, integrationId) per resulting doc

Respects --dry-run: prints the request bodies without calling the API.
Idempotent on retry via the upstream create_bulk_documents external_id
behavior; safe to re-run after a partial failure.
`),
		Example: strings.Trim(`
  surgegraph-pp-cli research gaps publish --research-id res_xyz --integration wp_int_456 --project proj_abc --dry-run
  surgegraph-pp-cli research gaps publish --research-id res_xyz --integration wp_int_456 --project proj_abc
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if researchID == "" || integration == "" || projectID == "" {
				if dryRunOK(flags) {
					return nil
				}
				return errors.New("--research-id, --integration, and --project are required")
			}
			if batchSize <= 0 || batchSize > 50 {
				batchSize = 50
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			mapData, _, err := c.Post("/v1/get_topic_map", map[string]any{"projectId": projectID})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			gaps := extractTopicMapGaps(mapData)
			if len(gaps) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"note":    "no gap topics found for project — nothing to publish",
					"project": projectID,
				}, flags)
			}

			type stage struct {
				Stage       string          `json:"stage"`
				Action      string          `json:"action"`
				BatchSize   int             `json:"batch_size,omitempty"`
				DocumentIDs []string        `json:"document_ids,omitempty"`
				Integration string          `json:"integration,omitempty"`
				Topics      []string        `json:"topics,omitempty"`
				Result      json.RawMessage `json:"result,omitempty"`
			}
			steps := []stage{}

			for i := 0; i < len(gaps); i += batchSize {
				end := i + batchSize
				if end > len(gaps) {
					end = len(gaps)
				}
				batch := gaps[i:end]
				bodyDocs := make([]map[string]any, 0, len(batch))
				for _, g := range batch {
					bodyDocs = append(bodyDocs, map[string]any{
						"projectId":  projectID,
						"prompt":     g,
						"externalId": fmt.Sprintf("gap:%s:%s", researchID, slugifyTopic(g)),
					})
				}
				step := stage{Stage: "bulk-create", BatchSize: len(batch), Topics: batch}
				if flags.dryRun {
					step.Action = "would call POST /v1/create_bulk_documents"
				} else {
					data, _, err := c.Post("/v1/create_bulk_documents", map[string]any{
						"projectId": projectID,
						"documents": bodyDocs,
					})
					if err != nil {
						return classifyAPIError(err, flags)
					}
					step.Result = data
					// Try to extract created document IDs for the publish stage.
					step.DocumentIDs = extractDocumentIDs(data)
				}
				steps = append(steps, step)

				// Publish stage per resulting doc.
				for _, docID := range step.DocumentIDs {
					ps := stage{Stage: "publish", Action: "POST /v1/publish_document_to_cms", Integration: integration, DocumentIDs: []string{docID}}
					if !flags.dryRun {
						data, _, err := c.Post("/v1/publish_document_to_cms", map[string]any{
							"documentId":    docID,
							"integrationId": integration,
						})
						if err != nil {
							return classifyAPIError(err, flags)
						}
						ps.Result = data
					} else {
						ps.Action = "would call POST /v1/publish_document_to_cms"
					}
					steps = append(steps, ps)
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"research_id": researchID,
				"project":     projectID,
				"integration": integration,
				"gaps":        len(gaps),
				"steps":       steps,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&researchID, "research-id", "", "Topic research ID")
	cmd.Flags().StringVar(&integration, "integration", "", "WordPress integration ID")
	cmd.Flags().StringVar(&projectID, "project", "", "Project ID")
	cmd.Flags().IntVar(&batchSize, "batch-size", 50, "Documents per bulk-create batch (max 50)")
	return cmd
}

// ---------- research domain diff ----------

func newResearchDomainDiffCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "domain",
		Short: "Domain-research subcommands (set-diff vs own topic_research)",
	}
	cmd.AddCommand(newResearchDomainDiffLeaf(flags))
	return cmd
}

func newResearchDomainDiffLeaf(flags *rootFlags) *cobra.Command {
	var mineID, theirsID, projectID string
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Set-diff between own topic_research and competitor domain_research",
		Example: strings.Trim(`
  surgegraph-pp-cli research domain diff --project proj_abc123 --mine res_xyz789 --theirs dom_aaa111 --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if mineID == "" || theirsID == "" || projectID == "" {
				if dryRunOK(flags) {
					return nil
				}
				return errors.New("--project, --mine (topic_research id) and --theirs (domain_research id) are required")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			mineData, _, err := c.Post("/v1/get_topic_research", map[string]any{
				"projectId":  projectID,
				"researchId": mineID,
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			theirsData, _, err := c.Post("/v1/get_domain_research", map[string]any{
				"projectId":  projectID,
				"researchId": theirsID,
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			mine := setOf(extractTopicTreeLeavesNames(mineData))
			theirs := setOf(extractTopicTreeLeavesNames(theirsData))
			onlyMine := setDifference(mine, theirs)
			onlyTheirs := setDifference(theirs, mine)
			intersection := setIntersection(mine, theirs)
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"only_mine":    sortedSet(onlyMine),
				"only_theirs":  sortedSet(onlyTheirs),
				"intersection": sortedSet(intersection),
				"counts": map[string]int{
					"only_mine":    len(onlyMine),
					"only_theirs":  len(onlyTheirs),
					"intersection": len(intersection),
				},
			}, flags)
		},
	}
	cmd.Flags().StringVar(&projectID, "project", "", "Project ID that owns both researches")
	cmd.Flags().StringVar(&mineID, "mine", "", "My topic_research id")
	cmd.Flags().StringVar(&theirsID, "theirs", "", "Competitor domain_research id")
	return cmd
}

// ---------- topic tree helpers ----------

type topicNode struct {
	name   string
	parent string
}

// extractTopicTreeLeaves walks common shapes the SurgeGraph topic / domain
// research payloads use and emits the leaf topic names with their parents.
// Defensive: many response keys are unknown until live runs land, so this
// accepts a permissive set of node-array names.
func extractTopicTreeLeaves(data json.RawMessage) []topicNode {
	var env map[string]json.RawMessage
	if err := json.Unmarshal(data, &env); err != nil {
		return nil
	}
	// Try several common keys.
	for _, k := range []string{"topics", "macroTopics", "tree", "items", "data"} {
		if v, ok := env[k]; ok {
			return walkTopicArray(v, "")
		}
	}
	return nil
}

func extractTopicTreeLeavesNames(data json.RawMessage) []string {
	leaves := extractTopicTreeLeaves(data)
	out := make([]string, 0, len(leaves))
	for _, l := range leaves {
		out = append(out, l.name)
	}
	return out
}

func walkTopicArray(raw json.RawMessage, parent string) []topicNode {
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil
	}
	var out []topicNode
	for _, it := range items {
		var name string
		for _, k := range []string{"topic", "name", "title", "label"} {
			if v, ok := it[k]; ok {
				_ = json.Unmarshal(v, &name)
				if name != "" {
					break
				}
			}
		}
		if name == "" {
			continue
		}
		out = append(out, topicNode{name: name, parent: parent})
		for _, k := range []string{"children", "microTopics", "subTopics"} {
			if v, ok := it[k]; ok {
				out = append(out, walkTopicArray(v, name)...)
			}
		}
	}
	return out
}

// extractTopicMapGaps reads a topic map response and returns topics flagged
// as gaps (covered=false / status=gap / etc.). Permissive across schema
// shapes; treats any topic without an explicit covered/coverage signal as
// a candidate gap so the user can decide via --dry-run.
func extractTopicMapGaps(data json.RawMessage) []string {
	var env map[string]json.RawMessage
	_ = json.Unmarshal(data, &env)
	out := []string{}
	for _, k := range []string{"gaps", "uncoveredTopics", "missingTopics"} {
		if v, ok := env[k]; ok {
			var rows []string
			if err := json.Unmarshal(v, &rows); err == nil && len(rows) > 0 {
				return rows
			}
			var objs []map[string]json.RawMessage
			if err := json.Unmarshal(v, &objs); err == nil {
				for _, o := range objs {
					var name string
					for _, kk := range []string{"topic", "name", "title"} {
						if vv, ok := o[kk]; ok {
							_ = json.Unmarshal(vv, &name)
							if name != "" {
								break
							}
						}
					}
					if name != "" {
						out = append(out, name)
					}
				}
				if len(out) > 0 {
					return out
				}
			}
		}
	}
	// Fall back: walk the tree and treat every leaf as a candidate.
	leaves := extractTopicTreeLeaves(data)
	for _, l := range leaves {
		out = append(out, l.name)
	}
	return out
}

func extractDocumentIDs(data json.RawMessage) []string {
	var env map[string]json.RawMessage
	if err := json.Unmarshal(data, &env); err == nil {
		for _, k := range []string{"documents", "data", "results", "ids"} {
			if v, ok := env[k]; ok {
				var ids []string
				if json.Unmarshal(v, &ids) == nil && len(ids) > 0 {
					return ids
				}
				var objs []map[string]json.RawMessage
				if json.Unmarshal(v, &objs) == nil {
					out := []string{}
					for _, o := range objs {
						for _, kk := range []string{"id", "documentId"} {
							if vv, ok := o[kk]; ok {
								var s string
								if json.Unmarshal(vv, &s) == nil && s != "" {
									out = append(out, s)
									break
								}
							}
						}
					}
					if len(out) > 0 {
						return out
					}
				}
			}
		}
	}
	return nil
}

// collectDocTitles decodes the writer/optimizer documents envelope and
// returns lowercased non-empty titles. Permissive across schema shapes:
// supports bare arrays and the common envelope keys handled by
// decodeListArray.
func collectDocTitles(data json.RawMessage) []string {
	rows, _ := decodeListArray(data)
	out := make([]string, 0, len(rows))
	for _, d := range rows {
		var t struct {
			Title string `json:"title"`
		}
		if json.Unmarshal(d, &t) == nil && t.Title != "" {
			out = append(out, strings.ToLower(t.Title))
		}
	}
	return out
}

func slugifyTopic(s string) string {
	out := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' {
			return r
		}
		if r >= 'A' && r <= 'Z' {
			return r + 32
		}
		if r >= '0' && r <= '9' {
			return r
		}
		return '-'
	}, s)
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	return strings.Trim(out, "-")
}

// ---------- set helpers ----------

func setOf(items []string) map[string]struct{} {
	out := make(map[string]struct{}, len(items))
	for _, s := range items {
		s = strings.ToLower(strings.TrimSpace(s))
		if s != "" {
			out[s] = struct{}{}
		}
	}
	return out
}

func setDifference(a, b map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{}, len(a))
	for k := range a {
		if _, ok := b[k]; !ok {
			out[k] = struct{}{}
		}
	}
	return out
}

func setIntersection(a, b map[string]struct{}) map[string]struct{} {
	out := map[string]struct{}{}
	for k := range a {
		if _, ok := b[k]; ok {
			out[k] = struct{}{}
		}
	}
	return out
}

func sortedSet(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}
