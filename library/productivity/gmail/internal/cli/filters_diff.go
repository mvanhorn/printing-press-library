// Copyright 2026 Rahul Bansal and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: filters as code. A flat YAML file describes the desired
// filter set; diff compares it against live settings.filters and apply
// executes the create/delete plan. Absorbs gmailctl's niche without the
// Jsonnet complexity.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source live
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/cliutil"
)

// filterSpec is one declarative filter in the YAML file.
type filterSpec struct {
	From         string   `yaml:"from,omitempty" json:"from,omitempty"`
	To           string   `yaml:"to,omitempty" json:"to,omitempty"`
	Subject      string   `yaml:"subject,omitempty" json:"subject,omitempty"`
	Query        string   `yaml:"query,omitempty" json:"query,omitempty"`
	AddLabels    []string `yaml:"add_labels,omitempty" json:"add_labels,omitempty"`
	RemoveLabels []string `yaml:"remove_labels,omitempty" json:"remove_labels,omitempty"`
	Forward      string   `yaml:"forward,omitempty" json:"forward,omitempty"`
}

type filterFile struct {
	Filters []filterSpec `yaml:"filters"`
}

// liveFilter mirrors the settings.filters resource shape.
type liveFilter struct {
	ID       string `json:"id"`
	Criteria struct {
		From    string `json:"from"`
		To      string `json:"to"`
		Subject string `json:"subject"`
		Query   string `json:"query"`
	} `json:"criteria"`
	Action struct {
		AddLabelIDs    []string `json:"addLabelIds"`
		RemoveLabelIDs []string `json:"removeLabelIds"`
		Forward        string   `json:"forward"`
	} `json:"action"`
}

// filterKey produces a comparable identity for a filter, with label IDs
// resolved to lowercase names so file-side names and live-side IDs meet in
// the middle.
func filterKey(from, to, subject, query string, addLabels, removeLabels []string, forward string) string {
	norm := func(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
	addN := make([]string, 0, len(addLabels))
	for _, l := range addLabels {
		addN = append(addN, norm(l))
	}
	removeN := make([]string, 0, len(removeLabels))
	for _, l := range removeLabels {
		removeN = append(removeN, norm(l))
	}
	sort.Strings(addN)
	sort.Strings(removeN)
	return strings.Join([]string{
		norm(from), norm(to), norm(subject), norm(query),
		strings.Join(addN, "+"), strings.Join(removeN, "+"), norm(forward),
	}, "|")
}

// labelNameMaps fetches id->name and name->id label mappings.
func labelNameMaps(cmd *cobra.Command, c *client.Client) (idToName, nameToID map[string]string, err error) {
	data, err := c.Get(cmd.Context(), "/gmail/v1/users/me/labels", nil)
	if err != nil {
		return nil, nil, err
	}
	var page struct {
		Labels []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"labels"`
	}
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, nil, fmt.Errorf("parsing labels.list: %w", err)
	}
	idToName = map[string]string{}
	nameToID = map[string]string{}
	for _, l := range page.Labels {
		idToName[l.ID] = l.Name
		nameToID[strings.ToLower(l.Name)] = l.ID
	}
	return idToName, nameToID, nil
}

type filterPlan struct {
	Create []filterSpec `json:"create"`
	Delete []liveFilter `json:"delete"`
	Keep   int          `json:"keep"`
}

// buildFilterPlan diffs the file against live filters.
func buildFilterPlan(specs []filterSpec, live []liveFilter, idToName map[string]string) filterPlan {
	labelNames := func(ids []string) []string {
		out := make([]string, 0, len(ids))
		for _, id := range ids {
			if n, ok := idToName[id]; ok {
				out = append(out, n)
			} else {
				out = append(out, id)
			}
		}
		return out
	}
	liveKeys := map[string]liveFilter{}
	for _, f := range live {
		k := filterKey(f.Criteria.From, f.Criteria.To, f.Criteria.Subject, f.Criteria.Query,
			labelNames(f.Action.AddLabelIDs), labelNames(f.Action.RemoveLabelIDs), f.Action.Forward)
		liveKeys[k] = f
	}
	fileKeys := map[string]bool{}
	plan := filterPlan{}
	for _, s := range specs {
		k := filterKey(s.From, s.To, s.Subject, s.Query, s.AddLabels, s.RemoveLabels, s.Forward)
		fileKeys[k] = true
		if _, ok := liveKeys[k]; ok {
			plan.Keep++
		} else {
			plan.Create = append(plan.Create, s)
		}
	}
	for k, f := range liveKeys {
		if !fileKeys[k] {
			plan.Delete = append(plan.Delete, f)
		}
	}
	sort.Slice(plan.Delete, func(i, j int) bool { return plan.Delete[i].ID < plan.Delete[j].ID })
	return plan
}

func loadFilterFile(path string) ([]filterSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading filter file: %w", err)
	}
	var f filterFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if len(f.Filters) == 0 {
		return nil, fmt.Errorf("%s declares no filters (expected a top-level 'filters:' list)", path)
	}
	for i, s := range f.Filters {
		if s.From == "" && s.To == "" && s.Subject == "" && s.Query == "" {
			return nil, fmt.Errorf("filter %d has no criteria (need from, to, subject, or query)", i+1)
		}
		if len(s.AddLabels) == 0 && len(s.RemoveLabels) == 0 && s.Forward == "" {
			return nil, fmt.Errorf("filter %d has no action (need add_labels, remove_labels, or forward)", i+1)
		}
	}
	return f.Filters, nil
}

func fetchLiveFilters(cmd *cobra.Command, c *client.Client) ([]liveFilter, error) {
	data, err := c.Get(cmd.Context(), "/gmail/v1/users/me/settings/filters", nil)
	if err != nil {
		return nil, err
	}
	var page struct {
		Filter []liveFilter `json:"filter"`
	}
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, fmt.Errorf("parsing settings.filters list: %w", err)
	}
	return page.Filter, nil
}

func newNovelFiltersDiffCmd(flags *rootFlags) *cobra.Command {
	var flagFile string

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Diff a declarative filters.yaml against live Gmail filters",
		Long: `Compares the filters declared in a YAML file with the account's live
filters and prints the reconciliation plan: filters to create, filters to
delete, filters already in sync. Nothing is changed; run "filters apply"
to execute the plan.

File shape:

  filters:
    - from: notifications@github.com
      add_labels: [GitHub]
      remove_labels: [INBOX]
    - query: "list:golang-nuts older_than:30d"
      remove_labels: [INBOX]`,
		Example: "  gmail-pp-cli filters diff --file filters.yaml",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--file=filters.yaml",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !hasChangedLocalFlags(cmd) {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "diff declared filters against live settings")
			}
			if flagFile == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--file is required"))
			}
			specs, err := loadFilterFile(flagFile)
			if err != nil {
				return usageErr(err)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			live, err := fetchLiveFilters(cmd, c)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			idToName, _, err := labelNameMaps(cmd, c)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			plan := buildFilterPlan(specs, live, idToName)
			if wantsJSONOutput(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), plan, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%d in sync, %d to create, %d to delete\n", plan.Keep, len(plan.Create), len(plan.Delete))
			for _, s := range plan.Create {
				fmt.Fprintf(w, "+ create %+v\n", s)
			}
			for _, f := range plan.Delete {
				fmt.Fprintf(w, "- delete %s (from=%q query=%q)\n", f.ID, f.Criteria.From, f.Criteria.Query)
			}
			if len(plan.Create) > 0 || len(plan.Delete) > 0 {
				fmt.Fprintln(w, "\napply with: gmail-pp-cli filters apply --file "+flagFile)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagFile, "file", "", "Declarative filter file (YAML)")
	return cmd
}

func newNovelFiltersApplyCmd(flags *rootFlags) *cobra.Command {
	var flagFile string
	var deleteMissing bool

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply the filters.yaml reconciliation plan to the live account",
		Long: `Creates every filter declared in the file that is missing live. With
--delete-missing, also deletes live filters absent from the file (full
reconciliation). Label names are resolved to IDs, creating labels that do
not exist yet.`,
		Example: "  gmail-pp-cli filters apply --file filters.yaml\n  gmail-pp-cli filters apply --file filters.yaml --delete-missing",
		Annotations: map[string]string{
			"pp:happy-args": "--file=filters.yaml",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !hasChangedLocalFlags(cmd) {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "reconcile live Gmail filters with the declared file")
			}
			if flagFile == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--file is required"))
			}
			if cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would apply filter plan from:", flagFile)
				return nil
			}
			specs, err := loadFilterFile(flagFile)
			if err != nil {
				return usageErr(err)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			live, err := fetchLiveFilters(cmd, c)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			idToName, _, err := labelNameMaps(cmd, c)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			plan := buildFilterPlan(specs, live, idToName)
			created, deleted := 0, 0
			for _, s := range plan.Create {
				addIDs, err := resolveLabelIDs(cmd, c, s.AddLabels, true)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				removeIDs, err := resolveLabelIDs(cmd, c, s.RemoveLabels, false)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				body := map[string]any{
					"criteria": map[string]string{
						"from": s.From, "to": s.To, "subject": s.Subject, "query": s.Query,
					},
					"action": map[string]any{},
				}
				action := body["action"].(map[string]any)
				if len(addIDs) > 0 {
					action["addLabelIds"] = addIDs
				}
				if len(removeIDs) > 0 {
					action["removeLabelIds"] = removeIDs
				}
				if s.Forward != "" {
					action["forward"] = s.Forward
				}
				if _, _, err := c.Post(cmd.Context(), "/gmail/v1/users/me/settings/filters", body); err != nil {
					return classifyAPIError(err, flags)
				}
				created++
			}
			if deleteMissing {
				for _, f := range plan.Delete {
					if _, _, err := c.Delete(cmd.Context(), "/gmail/v1/users/me/settings/filters/"+f.ID); err != nil {
						return classifyAPIError(err, flags)
					}
					deleted++
				}
			}
			out := map[string]any{"created": created, "deleted": deleted, "kept": plan.Keep}
			if wantsJSONOutput(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "created %d, deleted %d, %d already in sync\n", created, deleted, plan.Keep)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagFile, "file", "", "Declarative filter file (YAML)")
	cmd.Flags().BoolVar(&deleteMissing, "delete-missing", false, "Also delete live filters absent from the file")
	return cmd
}
