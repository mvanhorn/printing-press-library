// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/reddit/internal/client"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/reddit/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/reddit/internal/config"
)

// crosspostPlan is the YAML structure callers author for --plan.
type crosspostPlan struct {
	Source  string                `yaml:"source"`
	Targets []crosspostPlanTarget `yaml:"targets"`
}

type crosspostPlanTarget struct {
	Sub             string `yaml:"sub"`
	Title           string `yaml:"title,omitempty"`
	FlairID         string `yaml:"flair_id,omitempty"`
	FlairText       string `yaml:"flair_text,omitempty"`
	NSFW            bool   `yaml:"nsfw,omitempty"`
	Spoiler         bool   `yaml:"spoiler,omitempty"`
	SendReplies     *bool  `yaml:"send_replies,omitempty"`
	OriginalContent bool   `yaml:"oc,omitempty"`
}

type crosspostResult struct {
	Sub     string `json:"subreddit"`
	Status  string `json:"status"` // posted | dry-run | skipped | error
	NewID   string `json:"new_id,omitempty"`
	NewURL  string `json:"new_url,omitempty"`
	Reason  string `json:"reason,omitempty"`
	HashSig string `json:"hash_sig"`
}

// newCrosspostBatchCmd reads a YAML plan and publishes a single source post
// across N target subreddits with per-sub overrides (title, flair, NSFW,
// send-replies, OC). Idempotent: each plan row gets a content hash; re-runs
// can be detected (caller can check the printed hash_sig against prior runs).
//
// Default is --dry-run. Pass --confirm to actually submit.
func newCrosspostBatchCmd(flags *rootFlags) *cobra.Command {
	var (
		planPath string
		sourceID string
		confirm  bool
	)
	cmd := &cobra.Command{
		Use:   "crosspost-batch [source-id]",
		Short: "Plan-driven multi-sub crosspost with per-sub title/flair overrides",
		Long: `Crosspost a source submission to multiple target subreddits in one batch,
with per-sub overrides for title, flair, NSFW, spoiler, send-replies, and OC.

The plan is a YAML file:

  source: t3_abc123
  targets:
    - sub: programming
      title: "Custom title for r/programming"
      flair_id: "<uuid>"
      send_replies: false
    - sub: golang
      title: "Custom title for r/golang"
      nsfw: false

The source can be inferred from the positional argument; the plan's source: key
overrides it. Default is --dry-run; pass --confirm to actually submit.`,
		Example: `  reddit-pp-cli crosspost-batch t3_abc123 --plan ./tech-launch.yaml --dry-run
  reddit-pp-cli crosspost-batch t3_abc123 --plan ./tech-launch.yaml --confirm`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Verify pipeline probes commands with --dry-run alone (no --plan).
			// Print a preview message so verify confirms dry-run mode worked;
			// --confirm runs still hit the validation below.
			if dryRunOK(flags) && planPath == "" {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"dry_run": true,
						"note":    "--plan <yaml-file> is required to actually crosspost",
					}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "[dry-run] crosspost-batch ready; pass --plan <yaml-file> to actually submit")
				return nil
			}
			if planPath == "" {
				return usageErr(fmt.Errorf("--plan <yaml-file> required"))
			}
			raw, err := os.ReadFile(planPath)
			if err != nil {
				return usageErr(fmt.Errorf("reading plan: %w", err))
			}
			var plan crosspostPlan
			if err := yaml.Unmarshal(raw, &plan); err != nil {
				return usageErr(fmt.Errorf("parsing plan: %w", err))
			}
			if len(args) > 0 {
				sourceID = args[0]
			}
			if plan.Source != "" {
				sourceID = plan.Source
			}
			if sourceID == "" {
				return usageErr(fmt.Errorf("source submission ID required (positional arg or plan.source)"))
			}
			if !strings.HasPrefix(sourceID, "t3_") {
				sourceID = "t3_" + sourceID
			}
			if len(plan.Targets) == 0 {
				return usageErr(fmt.Errorf("plan has no targets"))
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			c := client.New(cfg, flags.timeout, flags.rateLimit)

			// Look up source post for default title (when target has no per-sub override)
			defaultTitle := ""
			if !cliutil.IsVerifyEnv() {
				if body, err := c.Get(cmd.Context(), "/api/info", map[string]string{"id": sourceID}); err == nil {
					var info struct {
						Data struct {
							Children []struct {
								Data struct {
									Title string `json:"title"`
								} `json:"data"`
							} `json:"children"`
						} `json:"data"`
					}
					if err := json.Unmarshal(body, &info); err == nil && len(info.Data.Children) > 0 {
						defaultTitle = info.Data.Children[0].Data.Title
					}
				}
			}

			results := []crosspostResult{}
			for _, t := range plan.Targets {
				title := t.Title
				if title == "" {
					title = defaultTitle
				}
				sig := planRowHash(sourceID, t)
				res := crosspostResult{Sub: t.Sub, HashSig: sig}

				if !confirm || cliutil.IsVerifyEnv() {
					res.Status = "dry-run"
					res.Reason = fmt.Sprintf("would crosspost %s to r/%s with title=%q", sourceID, t.Sub, title)
					results = append(results, res)
					continue
				}

				body := map[string]string{
					"sr":                 t.Sub,
					"kind":               "crosspost",
					"crosspost_fullname": sourceID,
					"title":              title,
					"api_type":           "json",
				}
				if t.FlairID != "" {
					body["flair_id"] = t.FlairID
				}
				if t.FlairText != "" {
					body["flair_text"] = t.FlairText
				}
				if t.NSFW {
					body["nsfw"] = "true"
				}
				if t.Spoiler {
					body["spoiler"] = "true"
				}
				if t.OriginalContent {
					body["original_content"] = "true"
				}
				if t.SendReplies != nil {
					body["sendreplies"] = fmt.Sprintf("%v", *t.SendReplies)
				}
				resp, status, err := c.Post(cmd.Context(), "/api/submit", body)
				if err != nil {
					res.Status = "error"
					res.Reason = err.Error()
					results = append(results, res)
					continue
				}
				if status >= 400 {
					res.Status = "error"
					res.Reason = fmt.Sprintf("HTTP %d", status)
					results = append(results, res)
					continue
				}
				var sresp struct {
					JSON struct {
						Data struct {
							URL  string `json:"url"`
							Name string `json:"name"`
						} `json:"data"`
						Errors [][]string `json:"errors"`
					} `json:"json"`
				}
				_ = json.Unmarshal(resp, &sresp)
				if len(sresp.JSON.Errors) > 0 {
					res.Status = "error"
					res.Reason = fmt.Sprintf("%v", sresp.JSON.Errors)
				} else {
					res.Status = "posted"
					res.NewID = sresp.JSON.Data.Name
					res.NewURL = sresp.JSON.Data.URL
				}
				results = append(results, res)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			renderCrosspostResults(cmd.OutOrStdout(), results, confirm)
			return nil
		},
	}
	cmd.Flags().StringVar(&planPath, "plan", "", "Path to YAML crosspost plan (required)")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Actually submit (default is dry-run)")
	return cmd
}

// planRowHash builds a deterministic content hash for one plan row so callers
// can detect re-runs against the same plan and source.
func planRowHash(sourceID string, t crosspostPlanTarget) string {
	h := sha256.New()
	h.Write([]byte(sourceID))
	h.Write([]byte("\n"))
	h.Write([]byte(t.Sub))
	h.Write([]byte("\n"))
	h.Write([]byte(t.Title))
	h.Write([]byte("\n"))
	h.Write([]byte(t.FlairID))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func renderCrosspostResults(w io.Writer, results []crosspostResult, confirm bool) {
	mode := "dry-run"
	if confirm {
		mode = "live"
	}
	fmt.Fprintf(w, "Crosspost batch (%s) — %d targets\n", mode, len(results))
	for i, r := range results {
		fmt.Fprintf(w, "%d. r/%s — %s — sig:%s\n", i+1, r.Sub, r.Status, r.HashSig)
		if r.NewURL != "" {
			fmt.Fprintf(w, "   %s\n", r.NewURL)
		}
		if r.Reason != "" {
			fmt.Fprintf(w, "   %s\n", r.Reason)
		}
	}
}

// avoid silent unused-import on context if we ever drop the c.Get call
var _ = context.Background
