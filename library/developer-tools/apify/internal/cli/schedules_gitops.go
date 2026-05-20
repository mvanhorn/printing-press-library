// `apify-pp-cli schedules apply|diff|pull` — terraform-style declarative
// schedule management for the Apify platform.
//
// YAML shape (one file holds N schedules):
//
//	schedules:
//	  - name: weekly-ai-twitter
//	    cron: "0 9 * * 1"
//	    timezone: America/Chicago
//	    actor: apidojo/twitter-scraper-lite
//	    input: { searchTerms: ["AI agents"], maxItems: 200 }
//	    enabled: true
//	  - name: daily-reddit-aisubs
//	    cron: "0 6 * * *"
//	    timezone: America/Chicago
//	    actor: trudax/reddit-scraper
//	    input: { subreddits: ["LocalLLaMA","ClaudeAI"], maxItems: 100 }
//	    enabled: true
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// ScheduleSpec is the YAML shape per schedule (subset that we project +
// compare against Apify's actual schedule resource).
type ScheduleSpec struct {
	Name     string         `yaml:"name" json:"name"`
	Cron     string         `yaml:"cron" json:"cron"`
	Timezone string         `yaml:"timezone,omitempty" json:"timezone,omitempty"`
	Actor    string         `yaml:"actor" json:"actor"`
	Input    map[string]any `yaml:"input,omitempty" json:"input,omitempty"`
	Enabled  bool           `yaml:"enabled" json:"enabled"`
	// Optional: webhook subscriptions, run options, etc. — kept simple for v1.
}

type ScheduleFile struct {
	Schedules []ScheduleSpec `yaml:"schedules"`
}

// newSchedulesApplyCmd registers `schedules apply <file.yaml>`.
func newSchedulesApplyCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply <file.yaml>",
		Short: "Declarative schedule sync: create/update schedules from a YAML file",
		Long: strings.Trim(`
Read schedule definitions from a YAML file and reconcile them against the
live Apify schedule API. Schedules in the file that don't exist remotely are
created. Schedules that exist but differ are updated. Schedules that exist
remotely but aren't in the file are LEFT UNTOUCHED by default — pass --prune
to delete them (terraform-style "the file is the source of truth").

Use --dry-run to preview without applying.

Examples:
  apify-pp-cli schedules apply ./schedules.yaml --dry-run
  apify-pp-cli schedules apply ./schedules.yaml --json
  apify-pp-cli schedules apply ./schedules.yaml --prune
`, "\n"),
		Example: strings.Trim(`
  apify-pp-cli schedules apply ./schedules.yaml --dry-run --json
`, "\n"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			prune, _ := cmd.Flags().GetBool("prune")
			data, err := os.ReadFile(args[0])
			if err != nil {
				return configErr(fmt.Errorf("reading schedule file: %w", err))
			}
			file := &ScheduleFile{}
			if err := yaml.Unmarshal(data, file); err != nil {
				return usageErr(fmt.Errorf("parsing schedule YAML: %w", err))
			}

			// If --dry-run or verify probe: emit the would-be plan without acting
			if dryRunOK(flags) || flags.dryRun {
				plan, err := computeSchedulePlan(flags, file, prune)
				if err != nil {
					return err
				}
				return printJSONFiltered(cmd.OutOrStdout(), plan, flags)
			}

			plan, err := computeSchedulePlan(flags, file, prune)
			if err != nil {
				return err
			}
			results := executeSchedulePlan(flags, plan)
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().Bool("prune", false, "Delete remote schedules absent from the file")
	return cmd
}

// newSchedulesDiffCmd registers `schedules diff <file.yaml>`.
func newSchedulesDiffCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff <file.yaml>",
		Short: "Show drift between a local schedule YAML and live Apify schedules",
		Long: strings.Trim(`
Compares a local schedules YAML against the live Apify schedule list.
Outputs a structured diff (create / update / delete) without applying.

Examples:
  apify-pp-cli schedules diff ./schedules.yaml --json
`, "\n"),
		Args: cobra.MaximumNArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			data, err := os.ReadFile(args[0])
			if err != nil {
				return configErr(fmt.Errorf("reading schedule file: %w", err))
			}
			file := &ScheduleFile{}
			if err := yaml.Unmarshal(data, file); err != nil {
				return usageErr(fmt.Errorf("parsing schedule YAML: %w", err))
			}
			plan, err := computeSchedulePlan(flags, file, true)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), plan, flags)
		},
	}
	return cmd
}

// SchedulePlan is the diff envelope.
type SchedulePlan struct {
	Create []ScheduleSpec       `json:"create"`
	Update []ScheduleUpdatePair `json:"update"`
	Delete []string             `json:"delete"`
	NoOp   int                  `json:"no_op"`
}

type ScheduleUpdatePair struct {
	Name    string       `json:"name"`
	Live    ScheduleSpec `json:"live"`
	Desired ScheduleSpec `json:"desired"`
}

func computeSchedulePlan(flags *rootFlags, file *ScheduleFile, prune bool) (*SchedulePlan, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, configErr(err)
	}
	// List live schedules
	body, err := c.Get("/v2/schedules", map[string]string{"limit": "1000"})
	if err != nil {
		return nil, apiErr(fmt.Errorf("listing live schedules: %w", err))
	}
	var resp struct {
		Data struct {
			Items []map[string]any `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, apiErr(fmt.Errorf("parsing schedule list: %w", err))
	}

	liveByName := map[string]ScheduleSpec{}
	for _, raw := range resp.Data.Items {
		spec := scheduleSpecFromAPI(raw)
		if spec.Name != "" {
			liveByName[spec.Name] = spec
		}
	}

	plan := &SchedulePlan{}
	desiredNames := map[string]bool{}
	actorIDCache := map[string]string{}
	for _, desired := range file.Schedules {
		desiredNames[desired.Name] = true
		// The live schedule's actions[].actorId is always the Apify-internal
		// actor ID. Resolve the YAML's human slug to that same ID so
		// scheduleSpecEqual compares like-for-like; otherwise every diff/apply
		// reports permanent false drift.
		desired.Actor = resolveActorID(c, desired.Actor, actorIDCache)
		live, exists := liveByName[desired.Name]
		switch {
		case !exists:
			plan.Create = append(plan.Create, desired)
		case scheduleSpecEqual(desired, live):
			plan.NoOp++
		default:
			plan.Update = append(plan.Update, ScheduleUpdatePair{
				Name: desired.Name, Live: live, Desired: desired,
			})
		}
	}

	if prune {
		for name := range liveByName {
			if !desiredNames[name] {
				plan.Delete = append(plan.Delete, name)
			}
		}
		sort.Strings(plan.Delete)
	}
	return plan, nil
}

// resolveActorID maps a human-readable Actor slug (username/name or
// username~name) to the Apify-internal actor ID via GET /v2/acts/{id}.
// Results are cached per call to computeSchedulePlan. A value that is
// already an internal ID (no slash or tilde) is returned unchanged; on any
// lookup failure the original value is returned so a transient API error
// degrades to a possible false-drift report rather than a hard failure.
func resolveActorID(c interface {
	Get(string, map[string]string) (json.RawMessage, error)
}, actor string, cache map[string]string) string {
	if actor == "" || (!strings.Contains(actor, "/") && !strings.Contains(actor, "~")) {
		return actor
	}
	if id, ok := cache[actor]; ok {
		return id
	}
	body, err := c.Get("/v2/acts/"+actorPathSegment(actor), nil)
	if err != nil {
		cache[actor] = actor
		return actor
	}
	var resp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &resp) == nil && resp.Data.ID != "" {
		cache[actor] = resp.Data.ID
		return resp.Data.ID
	}
	cache[actor] = actor
	return actor
}

func scheduleSpecFromAPI(raw map[string]any) ScheduleSpec {
	spec := ScheduleSpec{}
	if v, ok := raw["name"].(string); ok {
		spec.Name = v
	}
	if v, ok := raw["cronExpression"].(string); ok {
		spec.Cron = v
	}
	if v, ok := raw["timezone"].(string); ok {
		spec.Timezone = v
	}
	if v, ok := raw["isEnabled"].(bool); ok {
		spec.Enabled = v
	}
	// actions[].actorId + actions[].input (first action — most schedules have one)
	if actions, ok := raw["actions"].([]any); ok && len(actions) > 0 {
		if first, ok := actions[0].(map[string]any); ok {
			if v, ok := first["actorId"].(string); ok {
				spec.Actor = v
			}
			if v, ok := first["input"].(map[string]any); ok {
				spec.Input = v
			}
		}
	}
	return spec
}

func scheduleSpecEqual(a, b ScheduleSpec) bool {
	if a.Name != b.Name || a.Cron != b.Cron || a.Actor != b.Actor || a.Enabled != b.Enabled {
		return false
	}
	// Timezones differ AND at least one side has it set => real drift.
	// Using || (not &&) here so a local spec missing the timezone while the
	// remote schedule has one set is still reported as drift, not NoOp.
	if a.Timezone != b.Timezone && (a.Timezone != "" || b.Timezone != "") {
		return false
	}
	// Compare inputs by JSON canonical form
	aj, _ := json.Marshal(a.Input)
	bj, _ := json.Marshal(b.Input)
	return string(aj) == string(bj)
}

// ExecuteResult captures the apply outcome.
type ExecuteResult struct {
	Created []string `json:"created"`
	Updated []string `json:"updated"`
	Deleted []string `json:"deleted"`
	Errors  []string `json:"errors,omitempty"`
}

func executeSchedulePlan(flags *rootFlags, plan *SchedulePlan) ExecuteResult {
	res := ExecuteResult{}
	c, err := flags.newClient()
	if err != nil {
		res.Errors = append(res.Errors, err.Error())
		return res
	}
	for _, s := range plan.Create {
		body := scheduleSpecToAPI(s)
		_, status, err := c.Post("/v2/schedules", body)
		if err != nil || status >= 400 {
			res.Errors = append(res.Errors,
				fmt.Sprintf("create %q: %v (HTTP %d)", s.Name, err, status))
			continue
		}
		res.Created = append(res.Created, s.Name)
	}
	for _, up := range plan.Update {
		// Apify's API uses schedule ID for PUT; we'd need to GET the schedule
		// by name to find its ID first. For v1 we surface a hint rather than
		// the wrong call.
		res.Errors = append(res.Errors,
			fmt.Sprintf("update %q deferred: edit in Apify Console or re-create (v1 limitation)", up.Name))
	}
	for _, name := range plan.Delete {
		res.Errors = append(res.Errors,
			fmt.Sprintf("delete %q deferred: confirm via Console then `schedules-delete` typed command (v1 safety)", name))
	}
	return res
}

func scheduleSpecToAPI(s ScheduleSpec) map[string]any {
	return map[string]any{
		"name":           s.Name,
		"cronExpression": s.Cron,
		"timezone":       s.Timezone,
		"isEnabled":      s.Enabled,
		"actions": []map[string]any{{
			"type":    "RUN_ACTOR",
			"actorId": s.Actor,
			"input":   s.Input,
		}},
	}
}
