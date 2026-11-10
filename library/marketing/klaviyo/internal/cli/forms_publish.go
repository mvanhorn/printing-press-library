// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/client"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		for _, parent := range root.Commands() {
			if parent.Name() == "forms" {
				addNovelCommandIfAbsent(parent, newFormsPublishCmd(flags))
				addNovelCommandIfAbsent(parent, newFormsUnpublishCmd(flags))
			}
		}
	})
}

func newFormsPublishCmd(flags *rootFlags) *cobra.Command {
	return newFormPublicationCmd(flags, "publish", &cobra.Command{Use: "publish <form-id>"})
}

func newFormsUnpublishCmd(flags *rootFlags) *cobra.Command {
	return newFormPublicationCmd(flags, "unpublish", &cobra.Command{Use: "unpublish <form-id>"})
}

func newFormPublicationCmd(flags *rootFlags, action string, cmd *cobra.Command) *cobra.Command {
	var apply bool
	cmd.Short = "Preview or " + action + " a form, with its last 30 days of performance"
	cmd.Args = cobra.ExactArgs(1)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		id := strings.TrimSpace(args[0])
		if id == "" || strings.ContainsAny(id, "/?#\\") || id == "." || id == ".." {
			return usageErr(fmt.Errorf("a valid form id is required"))
		}
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		// Preview still needs live reads and the read-only report POST.
		c.DryRun = false
		c.NoCache = true
		return runFormPublication(cmd, flags, c, id, action, apply && !flags.dryRun)
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply the change after previewing it (--dry-run always previews)")
	return cmd
}

type formPublicationState struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Definition *struct {
		Versions []map[string]any `json:"versions"`
	} `json:"definition"`
}

func readFormPublicationState(c *client.Client, id string) (*formPublicationState, error) {
	raw, err := c.GetNoCache(klaviyoRequestContext(c), "/api/forms/"+url.PathEscape(id), map[string]string{
		"fields[form]": "definition,name,status,ab_test",
	})
	if err != nil {
		return nil, classifyAPIErrorOnly(err)
	}
	var payload struct {
		Data *struct {
			Attributes *formPublicationState `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("parsing form response: %w", err)
	}
	if payload.Data == nil || payload.Data.Attributes == nil {
		return nil, fmt.Errorf("form %s response has no attributes", id)
	}
	state := payload.Data.Attributes
	if state.Status == "" && (state.Definition == nil || state.Definition.Versions == nil) {
		return nil, fmt.Errorf("form %s response has neither status nor versions", id)
	}
	return state, nil
}

func (s *formPublicationState) versions(status string) []map[string]any {
	var versions []map[string]any
	if s.Definition != nil {
		for _, version := range s.Definition.Versions {
			if version["status"] == status {
				versions = append(versions, version)
			}
		}
	}
	return versions
}

func (s *formPublicationState) live() bool {
	return s.Status == "live" || len(s.versions("live")) > 0
}

type formPublicationChange struct {
	Path string `json:"path"`
	Old  string `json:"old"`
	New  string `json:"new"`
}

type formPublicationNumbers struct {
	Views      float64 `json:"views"`
	Submits    float64 `json:"submits"`
	SubmitRate float64 `json:"submit_rate"`
}

type formPublicationResult struct {
	FormID     string                  `json:"form_id"`
	Name       string                  `json:"name"`
	Action     string                  `json:"action"`
	Status     string                  `json:"status"`
	Preview    bool                    `json:"preview"`
	Applied    bool                    `json:"applied"`
	Verified   bool                    `json:"verified"`
	Message    string                  `json:"message"`
	NoLive     bool                    `json:"no_live_version,omitempty"`
	Changes    []formPublicationChange `json:"changes"`
	Last30Days *formPublicationNumbers `json:"last_30_days,omitempty"`
}

func runFormPublication(cmd *cobra.Command, flags *rootFlags, c *client.Client, id, action string, apply bool) error {
	state, err := readFormPublicationState(c, id)
	if err != nil {
		return err
	}
	result := formPublicationResult{
		FormID: id, Name: state.Name, Status: state.Status, Action: action,
		Preview: !apply, Changes: []formPublicationChange{},
	}
	var draft map[string]any
	if action == "publish" {
		if state.Definition == nil || state.Definition.Versions == nil {
			return fmt.Errorf("form %s response has no definition.versions; cannot inspect the draft", id)
		}
		drafts := state.versions("draft")
		if len(drafts) == 0 {
			result.Message = "no draft version; nothing to publish"
			return printFormPublication(cmd, flags, result)
		}
		for _, version := range drafts {
			if test := version["ab_test"]; test != nil && test != false {
				result.Message = "publish this one in the Klaviyo form editor"
				if err := printFormPublication(cmd, flags, result); err != nil {
					return err
				}
				return fmt.Errorf("%s: draft has an A/B test", result.Message)
			}
		}
		if len(drafts) != 1 {
			return fmt.Errorf("form %s has multiple draft versions; publish this one in the Klaviyo form editor", id)
		}
		draft = drafts[0]
		live := state.versions("live")
		if len(live) > 1 {
			return fmt.Errorf("form %s has multiple live versions; publish this one in the Klaviyo form editor", id)
		}
		var before map[string]any
		if len(live) == 0 {
			result.NoLive = true
		} else {
			before = live[0]
		}
		result.Changes = diffFormVersions(before, draft)
	}
	result.Last30Days, err = fetchFormPublicationNumbers(c, id)
	if action == "unpublish" && !state.live() {
		result.Message = "form is not live; nothing to unpublish"
		if err != nil {
			result.Message += "; last 30 days unavailable: " + err.Error()
		}
		return printFormPublication(cmd, flags, result)
	}
	if err != nil {
		return err
	}
	result.Message = "preview only; use --apply to " + action
	if apply {
		result.Message = "ready to " + action
	}
	// Human output shows the review before the mutation; JSON stays one document.
	if !flags.asJSON || !apply {
		if err := printFormPublication(cmd, flags, result); err != nil {
			return err
		}
	}
	if !apply {
		return nil
	}
	jobType := "form-" + action + "-job"
	body := map[string]any{"data": map[string]any{
		"type": jobType,
		"relationships": map[string]any{"form": map[string]any{
			"data": map[string]any{"type": "form", "id": id},
		}},
	}}
	_, status, err := c.PostFormPublicationJob(klaviyoRequestContext(c), "/api/"+jobType+"s", body)
	if err != nil || status != 202 {
		result.Message = action + " request failed; outcome unverified; job was not retried"
		if printErr := printFormPublication(cmd, flags, result); printErr != nil {
			return printErr
		}
		if err != nil {
			return classifyAPIErrorOnly(err)
		}
		return fmt.Errorf("%s: HTTP %d, expected 202", result.Message, status)
	}
	result.Applied = true
	after, err := readFormPublicationState(c, id)
	if err == nil {
		result.Status = after.Status
		if action == "publish" {
			for _, live := range after.versions("live") {
				if reflect.DeepEqual(formVersionContent(live), formVersionContent(draft)) {
					result.Verified = true
				}
			}
			if after.Definition == nil || after.Definition.Versions == nil {
				result.Verified = after.Status == "live"
			}
		} else {
			result.Verified = !after.live()
		}
	}
	if err != nil || !result.Verified {
		result.Message = action + " verification failed; job was not retried"
		if printErr := printFormPublication(cmd, flags, result); printErr != nil {
			return printErr
		}
		if err != nil {
			return fmt.Errorf("%s: %w", result.Message, err)
		}
		return fmt.Errorf("%s", result.Message)
	}
	result.Message = action + " verified"
	if flags.asJSON {
		return printFormPublication(cmd, flags, result)
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), result.Message)
	return err
}

func fetchFormPublicationNumbers(c *client.Client, id string) (*formPublicationNumbers, error) {
	body, _, err := buildKlaviyoReportBody(c, klaviyoReportRequest{
		Type: "form-values-report", Statistics: []string{"viewed_form", "submits", "submit_rate"},
		Timeframe: map[string]any{"key": "last_30_days"},
	}, false)
	if err != nil {
		return nil, err
	}
	attrs := body["data"].(map[string]any)["attributes"].(map[string]any)
	attrs["filter"] = "equals(form_id," + strconv.Quote(id) + ")"
	attrs["group_by"] = []string{"form_id"}
	raw, _, err := postKlaviyoReport(c, "/api/form-values-reports", body)
	if err != nil {
		return nil, classifyAPIErrorOnly(err)
	}
	var report struct {
		Data struct {
			Attributes struct {
				Results []map[string]any `json:"results"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &report); err != nil {
		return nil, fmt.Errorf("parsing form report: %w", err)
	}
	rows := report.Data.Attributes.Results
	if rows == nil {
		return nil, fmt.Errorf("form report did not contain data.attributes.results")
	}
	numbers := &formPublicationNumbers{}
	if len(rows) == 0 {
		return numbers, nil
	}
	if len(rows) != 1 || anyPath(rows[0], "groupings.form_id") != id {
		return nil, fmt.Errorf("form report did not return a single grouping for form %s", id)
	}
	for name, destination := range map[string]*float64{
		"viewed_form": &numbers.Views, "submits": &numbers.Submits, "submit_rate": &numbers.SubmitRate,
	} {
		value, ok := anyPath(rows[0], "statistics."+name).(float64)
		if !ok || value < 0 {
			return nil, fmt.Errorf("form report has no valid %s statistic", name)
		}
		*destination = value
	}
	return numbers, nil
}

func printFormPublication(cmd *cobra.Command, flags *rootFlags, result formPublicationResult) error {
	if flags.asJSON {
		return flags.printJSON(cmd, result)
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Form %s (%s): status %s\n", terminalText(out, result.Name), terminalText(out, result.FormID), terminalText(out, result.Status))
	if result.NoLive {
		fmt.Fprintln(out, "no live version yet")
	}
	for _, change := range result.Changes {
		fmt.Fprintf(out, "  %s: %s -> %s\n", terminalText(out, change.Path), terminalText(out, change.Old), terminalText(out, change.New))
	}
	if result.Last30Days != nil {
		n := result.Last30Days
		fmt.Fprintf(out, "Last 30 days: views %g, submits %g, submit rate %.2f%%\n", n.Views, n.Submits, n.SubmitRate*100)
	}
	_, err := fmt.Fprintln(out, result.Message)
	return err
}

func formVersionContent(value any) any {
	switch value := value.(type) {
	case map[string]any:
		content := make(map[string]any, len(value))
		for key, child := range value {
			switch key {
			case "id", "status", "created_at", "updated_at", "published_at", "archived_at", "timestamp", "timestamps":
				continue
			}
			content[key] = formVersionContent(child)
		}
		return content
	case []any:
		content := make([]any, len(value))
		for i, child := range value {
			content[i] = formVersionContent(child)
		}
		return content
	default:
		return value
	}
}

func diffFormVersions(live, draft map[string]any) []formPublicationChange {
	changes := []formPublicationChange{}
	var walk func(string, any, any, bool, bool)
	walk = func(path string, old, next any, oldExists, nextExists bool) {
		if oldExists == nextExists && reflect.DeepEqual(old, next) {
			return
		}
		oldMap, oldIsMap := old.(map[string]any)
		nextMap, nextIsMap := next.(map[string]any)
		if oldIsMap && nextIsMap {
			keys := map[string]bool{}
			for key := range oldMap {
				keys[key] = true
			}
			for key := range nextMap {
				keys[key] = true
			}
			ordered := make([]string, 0, len(keys))
			for key := range keys {
				ordered = append(ordered, key)
			}
			sort.Strings(ordered)
			for _, key := range ordered {
				before, had := oldMap[key]
				after, has := nextMap[key]
				walk(path+"["+strconv.Quote(key)+"]", before, after, had, has)
			}
			return
		}
		oldArray, oldIsArray := old.([]any)
		nextArray, nextIsArray := next.([]any)
		if oldIsArray && nextIsArray {
			for i := 0; i < max(len(oldArray), len(nextArray)); i++ {
				var before, after any
				if i < len(oldArray) {
					before = oldArray[i]
				}
				if i < len(nextArray) {
					after = nextArray[i]
				}
				walk(fmt.Sprintf("%s[%d]", path, i), before, after, i < len(oldArray), i < len(nextArray))
			}
			return
		}
		changes = append(changes, formPublicationChange{Path: path, Old: formPublicationValue(old, oldExists), New: formPublicationValue(next, nextExists)})
	}
	walk("$", formVersionContent(live), formVersionContent(draft), live != nil, draft != nil)
	return changes
}

func formPublicationValue(value any, exists bool) string {
	if !exists {
		return "<absent>"
	}
	raw, _ := json.Marshal(value)
	text := []rune(string(raw))
	if len(text) > 160 {
		return string(text[:157]) + "..."
	}
	return string(text)
}
