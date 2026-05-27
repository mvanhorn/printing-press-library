// `apify-pp-cli workflow run <file.yaml>` — chain multiple Actor runs + a digest into one declarative pipeline.
//
// YAML shape:
//
//	name: weekly-newsletter
//	topic: AI dev tools
//	since: 7d
//	digest:
//	  template: default
//	  limit: 20
//	  output: file:./digest.md
//	steps:
//	  - actor: apidojo/twitter-scraper-lite
//	    input: { searchTerms: ["AI agents", "MCP"], maxItems: 100 }
//	    wait: true
//	    only_new: true
//	  - actor: trudax/reddit-scraper
//	    input: { subreddits: ["LocalLLaMA","ClaudeAI"], maxItems: 50 }
//	    wait: true
//	    only_new: true
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/apify/internal/normalize"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/apify/internal/store"
)

// WorkflowFile is the on-disk shape.
type WorkflowFile struct {
	Name   string         `yaml:"name"`
	Topic  string         `yaml:"topic,omitempty"`
	Since  string         `yaml:"since,omitempty"` // e.g. "7d", "24h"
	Steps  []WorkflowStep `yaml:"steps"`
	Digest *DigestStep    `yaml:"digest,omitempty"`
}

type WorkflowStep struct {
	Actor   string         `yaml:"actor"`
	Input   map[string]any `yaml:"input,omitempty"`
	Wait    bool           `yaml:"wait"`
	OnlyNew bool           `yaml:"only_new"`
	Timeout int            `yaml:"timeout_secs,omitempty"`
	Memory  int            `yaml:"memory,omitempty"`
}

type DigestStep struct {
	Template     string `yaml:"template,omitempty"`
	TemplateFile string `yaml:"template_file,omitempty"`
	Limit        int    `yaml:"limit,omitempty"`
	Output       string `yaml:"output,omitempty"` // "stdout" | "file:<path>"
}

// newWorkflowRunCmd is the `workflow run <file.yaml>` subcommand.
func newWorkflowRunCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <file.yaml>",
		Short: "Run a YAML-declared multi-Actor workflow (chains Actor runs + digest)",
		Long: strings.Trim(`
Read a workflow YAML file and execute each step in order: start the Actor,
wait for completion, fetch + normalize + persist the items (with --only-new
filtering if set). After all steps, render a digest if the YAML includes
a 'digest:' block.

Workflow YAML shape:

  name: weekly-newsletter
  topic: AI dev tools
  since: 7d
  digest:
    template: default
    limit: 20
    output: file:./digest.md
  steps:
    - actor: apidojo/twitter-scraper-lite
      input: { searchTerms: ["AI agents"], maxItems: 100 }
      wait: true
      only_new: true
    - actor: trudax/reddit-scraper
      input: { subreddits: ["LocalLLaMA"], maxItems: 50 }
      wait: true
      only_new: true

Examples:
  apify-pp-cli workflow run ./weekly-newsletter.yaml
  apify-pp-cli workflow run ./scrape-and-digest.yaml --json
`, "\n"),
		Example: strings.Trim(`
  apify-pp-cli workflow run ./weekly-newsletter.yaml --agent
`, "\n"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would run workflow %q (dry-run)\n", args[0])
				return nil
			}
			data, err := os.ReadFile(args[0])
			if err != nil {
				return configErr(fmt.Errorf("reading workflow file: %w", err))
			}
			wf := &WorkflowFile{}
			if err := yaml.Unmarshal(data, wf); err != nil {
				return usageErr(fmt.Errorf("parsing workflow YAML: %w", err))
			}
			if wf.Name == "" {
				wf.Name = strings.TrimSuffix(args[0], ".yaml")
			}
			return runWorkflow(cmd, flags, wf)
		},
	}
	return cmd
}

// WorkflowResult captures the per-step outcome for the final JSON envelope.
type WorkflowResult struct {
	Name        string               `json:"name"`
	StartedAt   time.Time            `json:"started_at"`
	FinishedAt  time.Time            `json:"finished_at"`
	Status      string               `json:"status"`
	StepResults []WorkflowStepResult `json:"steps"`
	DigestPath  string               `json:"digest_path,omitempty"`
	DigestBytes int                  `json:"digest_bytes,omitempty"`
	DigestError string               `json:"digest_error,omitempty"`
	Meta        map[string]any       `json:"meta,omitempty"`
}

type WorkflowStepResult struct {
	Actor      string `json:"actor"`
	RunID      string `json:"run_id,omitempty"`
	RunStatus  string `json:"run_status,omitempty"`
	ItemsTotal int    `json:"items_total"`
	ItemsNovel int    `json:"items_novel"`
	Error      string `json:"error,omitempty"`
}

func runWorkflow(cmd *cobra.Command, flags *rootFlags, wf *WorkflowFile) error {
	ctx := cmd.Context()
	result := WorkflowResult{
		Name:      wf.Name,
		StartedAt: time.Now().UTC(),
		Status:    "running",
	}

	db, err := store.OpenWithContext(ctx, defaultDBPath("apify-pp-cli"))
	if err != nil {
		return configErr(fmt.Errorf("opening local store: %w", err))
	}
	defer db.Close()
	if err := db.EnsureExtensions(ctx); err != nil {
		return configErr(fmt.Errorf("ensuring extensions: %w", err))
	}

	c, err := flags.newClient()
	if err != nil {
		return configErr(err)
	}
	reg, _ := normalize.NewRegistry()

	for _, step := range wf.Steps {
		stepRes := executeWorkflowStep(ctx, c, db, reg, step)
		result.StepResults = append(result.StepResults, stepRes)
	}

	// Optional digest pass
	if wf.Digest != nil && wf.Topic != "" {
		since := parseSinceWindow(wf.Since)
		if since == 0 {
			since = 7 * 24 * time.Hour
		}
		// Each digest error is captured into result.DigestError so a failed
		// digest is never silently reported as a successful workflow.
		items, err := loadDigestItems(ctx, db, wf.Topic, since, nil, wf.Digest.Limit)
		if err != nil {
			result.DigestError = fmt.Sprintf("loading items: %v", err)
		} else {
			ranked := dedupeAndRank(items)
			if wf.Digest.Limit > 0 && len(ranked) > wf.Digest.Limit {
				ranked = ranked[:wf.Digest.Limit]
			}
			tmplStr, terr := resolveDigestTemplate(wf.Digest.Template, wf.Digest.TemplateFile)
			if terr != nil {
				result.DigestError = fmt.Sprintf("resolving template: %v", terr)
			} else {
				path, bytes, derr := writeDigestOutput(wf.Digest.Output,
					tmplStr, wf.Topic, since, ranked, len(items))
				if derr != nil {
					result.DigestError = fmt.Sprintf("writing digest: %v", derr)
				} else {
					result.DigestPath = path
					result.DigestBytes = bytes
				}
			}
		}
	}

	result.FinishedAt = time.Now().UTC()
	allOK := result.DigestError == ""
	for _, r := range result.StepResults {
		if r.Error != "" {
			allOK = false
			break
		}
	}
	if allOK {
		result.Status = "succeeded"
	} else {
		result.Status = "partial"
	}

	// A workflow run hydrates the local store from every step; record sync
	// state so doctor and external tooling see when fresh data last landed.
	totalItems := 0
	for _, r := range result.StepResults {
		totalItems += r.ItemsTotal
	}
	writeSyncState(wf.Name, allOK, totalItems)

	// Persist
	resJSON, _ := json.Marshal(result)
	_, _ = db.DB().ExecContext(ctx, `
		INSERT OR REPLACE INTO pp_workflow_runs
		(id, workflow_name, started_at, finished_at, status, result_json)
		VALUES (?, ?, ?, ?, ?, ?)`,
		fmt.Sprintf("%s-%d", wf.Name, result.StartedAt.UnixMilli()),
		wf.Name,
		result.StartedAt.Format(time.RFC3339),
		result.FinishedAt.Format(time.RFC3339),
		result.Status,
		string(resJSON))

	return printJSONFiltered(cmd.OutOrStdout(), result, flags)
}

func executeWorkflowStep(ctx context.Context, c interface {
	PostWithParams(string, map[string]string, any) (json.RawMessage, int, error)
	Get(string, map[string]string) (json.RawMessage, error)
	GetNoCache(string, map[string]string) (json.RawMessage, error)
}, db *store.Store, reg *normalize.Registry, step WorkflowStep) WorkflowStepResult {
	res := WorkflowStepResult{Actor: step.Actor}

	// Start run
	params := map[string]string{}
	if step.Timeout > 0 {
		params["timeout"] = fmt.Sprintf("%d", step.Timeout)
	}
	if step.Memory > 0 {
		params["memory"] = fmt.Sprintf("%d", step.Memory)
	}
	if step.Wait {
		params["waitForFinish"] = "60"
	}

	inputBytes, _ := json.Marshal(step.Input)
	if len(inputBytes) == 0 || string(inputBytes) == "null" {
		inputBytes = []byte("{}")
	}

	body, status, err := c.PostWithParams(
		fmt.Sprintf("/v2/acts/%s/runs", actorPathSegment(step.Actor)), params, json.RawMessage(inputBytes))
	if err != nil {
		res.Error = err.Error()
		return res
	}
	if status >= 400 {
		res.Error = fmt.Sprintf("actor run start failed: HTTP %d", status)
		return res
	}

	var runResp struct {
		Data RunData `json:"data"`
	}
	if err := json.Unmarshal(body, &runResp); err != nil {
		res.Error = fmt.Sprintf("parsing run start response: %v", err)
		return res
	}
	run := runResp.Data
	res.RunID = run.ID
	res.RunStatus = run.Status

	// Poll if needed
	if step.Wait && !isTerminalStatus(run.Status) {
		deadline := time.Now().Add(15 * time.Minute)
		polled, perr := pollRunUntilTerminal(ctx, c, run.ID, deadline)
		if perr != nil {
			res.Error = perr.Error()
			res.RunStatus = run.Status
			return res
		}
		run = polled
		res.RunStatus = run.Status
	}

	// Persist run history regardless of outcome
	_ = db.RecordActorRun(ctx, run.ID, run.ActID, step.Actor, run.Status,
		run.Stats.ComputeUnits, run.Options.MemoryMbytes,
		secondsBetween(run.StartedAt, run.FinishedAt),
		run.DefaultDatasetID, run.StartedAt, run.FinishedAt, inputBytes)

	if run.Status != "SUCCEEDED" || run.DefaultDatasetID == "" {
		return res
	}

	// Fetch + normalize + dedupe + persist
	raws, err := fetchDatasetItems(c, run.DefaultDatasetID)
	if err != nil {
		res.Error = fmt.Sprintf("fetching dataset items: %v", err)
		return res
	}
	items := reg.NormalizeBatch(step.Actor, raws)
	res.ItemsTotal = len(items)

	hashes := make([]string, len(items))
	for i, it := range items {
		hashes[i] = it.Hash
	}
	seen, _ := db.HashesSeen(ctx, hashes)
	for _, it := range items {
		isNew := !seen[it.Hash]
		if isNew {
			res.ItemsNovel++
		}
		if step.OnlyNew && !isNew {
			continue
		}
		_, _ = db.UpsertNormalizedItem(ctx,
			it.Hash, it.SourceActor, run.ID, run.DefaultDatasetID,
			it.URL, it.Title, it.Body, it.Author,
			timeStrOrEmptyHelper(it.PublishedAt), it.EngagementScore,
			it.FetchedAt, it.Raw)
	}
	return res
}

func parseSinceWindow(s string) time.Duration {
	if s == "" {
		return 0
	}
	d, err := time.ParseDuration(s)
	if err == nil {
		return d
	}
	// Also accept "7d", "30d" forms
	if strings.HasSuffix(s, "d") {
		var n int
		if _, err := fmt.Sscanf(s, "%dd", &n); err == nil {
			return time.Duration(n) * 24 * time.Hour
		}
	}
	return 0
}

// writeDigestOutput renders the digest and routes it to stdout or a file.
// Returns (path-or-empty, byte-count, err).
func writeDigestOutput(spec, tmplStr, topic string, since time.Duration,
	items []DigestItem, sourceCount int) (string, int, error) {
	if spec == "" || spec == "stdout" {
		var buf strings.Builder
		if err := renderDigest(&buf, tmplStr, topic, since, items, sourceCount); err != nil {
			return "", 0, err
		}
		os.Stdout.WriteString(buf.String())
		return "stdout", buf.Len(), nil
	}
	if strings.HasPrefix(spec, "file:") {
		path := strings.TrimPrefix(spec, "file:")
		var buf strings.Builder
		if err := renderDigest(&buf, tmplStr, topic, since, items, sourceCount); err != nil {
			return "", 0, err
		}
		if err := os.WriteFile(path, []byte(buf.String()), 0o644); err != nil {
			return "", 0, fmt.Errorf("writing digest to %s: %w", path, err)
		}
		return path, buf.Len(), nil
	}
	return "", 0, fmt.Errorf("unsupported output spec %q (use 'stdout' or 'file:<path>')", spec)
}
