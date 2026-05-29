package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type storySearchRun struct {
	Subject  string                 `json:"subject"`
	DryRun   bool                   `json:"dry_run,omitempty"`
	Requests []storySearchRequest   `json:"requests"`
	Results  []storyEvidence        `json:"results,omitempty"`
	Summary  storySearchRunSummary  `json:"summary"`
	Meta     map[string]interface{} `json:"meta,omitempty"`
}

type storySearchRequest struct {
	Cluster string            `json:"cluster"`
	Query   string            `json:"query"`
	Path    string            `json:"path"`
	Params  map[string]string `json:"params"`
}

type storySearchRunSummary struct {
	QueriesPlanned  int `json:"queries_planned"`
	QueriesExecuted int `json:"queries_executed"`
	ItemsFound      int `json:"items_found"`
	ItemsUnique     int `json:"items_unique"`
}

type workspaceDocument struct {
	Name      string              `json:"name"`
	CreatedAt string              `json:"created_at"`
	UpdatedAt string              `json:"updated_at"`
	Runs      []storySearchRun    `json:"runs,omitempty"`
	Items     []storyEvidenceItem `json:"items"`
}

type rankedStoryEvidenceItem struct {
	storyEvidenceItem
	Score int `json:"score"`
}

func newSearchRunPlanCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var publicDomainOnly bool
	var workspaceName string
	var workspaceDir string
	cmd := &cobra.Command{
		Use:         "run-plan <plan.json>",
		Short:       "Execute or preview a story discovery plan against NYPL item search",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if limit < 1 {
				return usageErr(fmt.Errorf("--limit must be at least 1"))
			}
			plan, err := readStoryPlanFile(args[0])
			if err != nil {
				return err
			}
			run := buildStorySearchRunRequests(plan, limit, publicDomainOnly)
			if flags.dryRun {
				run.DryRun = true
				return writeJSON(cmd, run)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			for _, req := range run.Requests {
				data, err := c.Get(cmd.Context(), req.Path, req.Params)
				evidence := storyEvidence{Cluster: req.Cluster, Query: req.Query}
				if err != nil {
					evidence.Error = err.Error()
				} else {
					evidence.Items = extractStoryEvidenceItems(data, limit)
				}
				run.Results = append(run.Results, evidence)
			}
			run.Summary.QueriesExecuted = len(run.Results)
			run.Summary.ItemsFound = countStoryItems(run.Results)
			run.Summary.ItemsUnique = len(dedupeStoryEvidenceItems(flattenStoryItems(run.Results)))
			if workspaceName != "" {
				if _, err := addRunToWorkspace(workspaceDir, workspaceName, run); err != nil {
					return err
				}
			}
			return writeJSON(cmd, run)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum results per planned query")
	cmd.Flags().BoolVar(&publicDomainOnly, "public-domain-only", false, "Restrict NYPL searches to public-domain results")
	cmd.Flags().StringVar(&workspaceName, "workspace", "", "Optional workspace name to save executed results")
	cmd.Flags().StringVar(&workspaceDir, "workspace-dir", "", "Workspace directory (default: app data workspaces directory)")
	return cmd
}

func readStoryPlanFile(path string) (storyDiscoveryPlan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return storyDiscoveryPlan{}, fmt.Errorf("reading story plan: %w", err)
	}
	var plan storyDiscoveryPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return storyDiscoveryPlan{}, fmt.Errorf("parsing story plan JSON: %w", err)
	}
	if strings.TrimSpace(plan.Subject) == "" {
		return storyDiscoveryPlan{}, fmt.Errorf("story plan missing subject")
	}
	return plan, nil
}

func buildStorySearchRunRequests(plan storyDiscoveryPlan, limit int, publicDomainOnly bool) storySearchRun {
	run := storySearchRun{Subject: plan.Subject, Requests: []storySearchRequest{}, Summary: storySearchRunSummary{}}
	for _, cluster := range plan.Clusters {
		for _, query := range cluster.Searches {
			params := map[string]string{"q": query, "per_page": fmt.Sprintf("%d", limit)}
			if publicDomainOnly {
				params["publicDomainOnly"] = "true"
			}
			run.Requests = append(run.Requests, storySearchRequest{Cluster: cluster.ID, Query: query, Path: "/items/search", Params: params})
		}
	}
	run.Summary.QueriesPlanned = len(run.Requests)
	return run
}

func writeJSON(cmd *cobra.Command, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = cmd.OutOrStdout().Write(append(data, '\n'))
	return err
}

func countStoryItems(results []storyEvidence) int {
	n := 0
	for _, result := range results {
		n += len(result.Items)
	}
	return n
}

func flattenStoryItems(results []storyEvidence) []storyEvidenceItem {
	var items []storyEvidenceItem
	for _, result := range results {
		items = append(items, result.Items...)
	}
	return items
}

func dedupeStoryEvidenceItems(items []storyEvidenceItem) []storyEvidenceItem {
	seen := map[string]bool{}
	out := make([]storyEvidenceItem, 0, len(items))
	for _, item := range items {
		key := storyItemIdentity(item)
		if key == "" {
			key = strings.ToLower(strings.TrimSpace(item.Title))
		}
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}

func storyItemIdentity(item storyEvidenceItem) string {
	for _, value := range []string{item.UUID, item.ImageID, item.ItemLink} {
		if strings.TrimSpace(value) != "" {
			return strings.ToLower(strings.TrimSpace(value))
		}
	}
	return ""
}

func rankStoryEvidenceItems(items []storyEvidenceItem, terms []string) []rankedStoryEvidenceItem {
	ranked := make([]rankedStoryEvidenceItem, 0, len(items))
	for _, item := range items {
		score := 0
		title := strings.ToLower(item.Title)
		for _, term := range terms {
			term = strings.ToLower(strings.TrimSpace(term))
			if term != "" && strings.Contains(title, term) {
				score += 10
			}
		}
		if item.ImageID != "" {
			score += 8
		}
		if item.ItemLink != "" {
			score += 4
		}
		if item.UUID != "" {
			score += 3
		}
		if strings.Contains(strings.ToLower(item.TypeOfResource), "image") {
			score += 3
		}
		ranked = append(ranked, rankedStoryEvidenceItem{storyEvidenceItem: item, Score: score})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].Score == ranked[j].Score {
			return ranked[i].Title < ranked[j].Title
		}
		return ranked[i].Score > ranked[j].Score
	})
	return ranked
}

func newWorkspaceCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "workspace", Short: "Manage local NYPL research workspaces"}
	cmd.AddCommand(newWorkspaceInitCmd(flags))
	cmd.AddCommand(newWorkspaceAddRunCmd(flags))
	cmd.AddCommand(newWorkspaceListItemsCmd(flags))
	return cmd
}

func newWorkspaceInitCmd(flags *rootFlags) *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "init <name>",
		Short: "Create a local research workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			now := time.Now().UTC().Format(time.RFC3339)
			doc := workspaceDocument{Name: args[0], CreatedAt: now, UpdatedAt: now, Items: []storyEvidenceItem{}}
			if err := saveWorkspace(dir, doc); err != nil {
				return err
			}
			return writeJSON(cmd, doc)
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "Workspace directory")
	return cmd
}

func newWorkspaceAddRunCmd(flags *rootFlags) *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "add-run <workspace> <run.json>",
		Short: "Import a search run JSON file into a workspace",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[1])
			if err != nil {
				return err
			}
			var run storySearchRun
			if err := json.Unmarshal(data, &run); err != nil {
				return err
			}
			doc, err := addRunToWorkspace(dir, args[0], run)
			if err != nil {
				return err
			}
			return writeJSON(cmd, doc)
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "Workspace directory")
	return cmd
}

func newWorkspaceListItemsCmd(flags *rootFlags) *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "list-items <workspace>",
		Short: "List deduped items saved in a workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			doc, err := loadWorkspace(dir, args[0])
			if err != nil {
				return err
			}
			return writeJSON(cmd, doc.Items)
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "Workspace directory")
	return cmd
}

func addRunToWorkspace(dir, name string, run storySearchRun) (workspaceDocument, error) {
	doc, err := loadWorkspace(dir, name)
	if err != nil {
		return workspaceDocument{}, err
	}
	doc.Runs = append(doc.Runs, run)
	doc.Items = dedupeStoryEvidenceItems(append(doc.Items, flattenStoryItems(run.Results)...))
	doc.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return doc, saveWorkspace(dir, doc)
}

func loadWorkspace(dir, name string) (workspaceDocument, error) {
	path, err := workspacePath(dir, name)
	if err != nil {
		return workspaceDocument{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return workspaceDocument{}, fmt.Errorf("reading workspace %q: %w", name, err)
	}
	var doc workspaceDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return workspaceDocument{}, err
	}
	return doc, nil
}

func saveWorkspace(dir string, doc workspaceDocument) error {
	path, err := workspacePath(dir, doc.Name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func workspacePath(dir, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsAny(name, `/\\:`) {
		return "", fmt.Errorf("invalid workspace name %q", name)
	}
	if dir == "" {
		dir = filepath.Join(filepath.Dir(defaultDBPath("nypl-digital-collections-pp-cli")), "workspaces")
	}
	return filepath.Join(dir, name+".json"), nil
}

func newStoriesDossierCmd(flags *rootFlags) *cobra.Command {
	var perCluster int
	var fromRun string
	var markdown bool
	cmd := &cobra.Command{
		Use:   "dossier <character-or-topic>",
		Short: "Create a compact research dossier from a story plan or search run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			plan := buildStoryDiscoveryPlan(args[0], perCluster)
			var run *storySearchRun
			if fromRun != "" {
				data, err := os.ReadFile(fromRun)
				if err != nil {
					return err
				}
				var parsed storySearchRun
				if err := json.Unmarshal(data, &parsed); err != nil {
					return err
				}
				run = &parsed
			}
			if markdown || !flags.asJSON {
				return printDossierMarkdown(cmd, plan, run)
			}
			return writeJSON(cmd, map[string]any{"subject": plan.Subject, "plan": plan, "run": run})
		},
	}
	cmd.Flags().IntVar(&perCluster, "per-cluster", 3, "Maximum planned searches per narrative cluster")
	cmd.Flags().StringVar(&fromRun, "from-run", "", "Optional search run JSON to include item evidence")
	cmd.Flags().BoolVar(&markdown, "markdown", false, "Emit Markdown dossier")
	return cmd
}

func printDossierMarkdown(cmd *cobra.Command, plan storyDiscoveryPlan, run *storySearchRun) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s dossier\n\n", plan.Subject)
	fmt.Fprintf(&b, "%s\n\n", plan.Intent)
	fmt.Fprintf(&b, "## Narrative clusters\n\n")
	for _, cluster := range plan.Clusters {
		fmt.Fprintf(&b, "### %s\n\n%s\n\n", cluster.Label, cluster.Why)
		for _, query := range cluster.Searches {
			fmt.Fprintf(&b, "- NYPL search: `%s`\n", query)
		}
		for _, char := range cluster.SimilarCharacters {
			fmt.Fprintf(&b, "- Similar character: **%s** — %s\n", char.Name, char.Why)
		}
		b.WriteString("\n")
	}
	if run != nil {
		terms := strings.Fields(plan.Subject)
		items := rankStoryEvidenceItems(dedupeStoryEvidenceItems(flattenStoryItems(run.Results)), terms)
		fmt.Fprintf(&b, "## Ranked item candidates\n\n")
		for i, item := range items {
			if i >= 10 {
				break
			}
			fmt.Fprintf(&b, "- %s", item.Title)
			if item.ItemLink != "" {
				fmt.Fprintf(&b, " — %s", item.ItemLink)
			}
			fmt.Fprintf(&b, " _(score %d)_\n", item.Score)
		}
	}
	_, err := cmd.OutOrStdout().Write([]byte(b.String()))
	return err
}
