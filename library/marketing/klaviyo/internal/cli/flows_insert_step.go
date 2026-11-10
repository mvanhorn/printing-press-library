// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/client"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		for _, parent := range root.Commands() {
			if parent.Name() == "flows" {
				addNovelCommandIfAbsent(parent, newFlowsInsertStepCmd(flags))
			}
		}
	})
}

type flowInsertStepAction struct {
	ID    string             `json:"id"`
	Type  string             `json:"type"`
	Links map[string]*string `json:"links"`
	Data  map[string]any     `json:"data"`
}

type flowInsertStepDefinition struct {
	EntryActionID string                 `json:"entry_action_id"`
	Actions       []flowInsertStepAction `json:"actions"`
}

type flowInsertStepFlow struct {
	Name       string                    `json:"name"`
	Status     string                    `json:"status"`
	Definition *flowInsertStepDefinition `json:"definition"`
}

type flowInsertStepNode struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Label     string                 `json:"label,omitempty"`
	New       bool                   `json:"new,omitempty"`
	Reference bool                   `json:"reference,omitempty"`
	Branches  []flowInsertStepBranch `json:"branches,omitempty"`
}

type flowInsertStepBranch struct {
	Link    string               `json:"link"`
	Label   string               `json:"label,omitempty"`
	Actions []flowInsertStepNode `json:"actions"`
}

type flowInsertStepView struct {
	EntryActionID string               `json:"entry_action_id"`
	Actions       []flowInsertStepNode `json:"actions"`
	Unconnected   []flowInsertStepNode `json:"unconnected,omitempty"`
}

type flowInsertStepResult struct {
	FlowID      string             `json:"flow_id"`
	Name        string             `json:"name"`
	Status      string             `json:"status"`
	AfterAction string             `json:"after_action"`
	Link        string             `json:"link"`
	TemporaryID string             `json:"temporary_id"`
	ActionID    string             `json:"action_id,omitempty"`
	Preview     bool               `json:"preview"`
	DryRun      bool               `json:"dry_run"`
	Applied     bool               `json:"applied"`
	Verified    bool               `json:"verified"`
	Warnings    []string           `json:"warnings,omitempty"`
	Request     map[string]any     `json:"request"`
	Before      flowInsertStepView `json:"before"`
	After       flowInsertStepView `json:"after"`
	AfterSource string             `json:"after_source"`
	Error       string             `json:"error,omitempty"`
}

func newFlowsInsertStepCmd(flags *rootFlags) *cobra.Command {
	var after, link, stepFile string
	var stdin, apply, allowLive bool
	cmd := &cobra.Command{
		Use:     "insert-step <flow-id>",
		Short:   "Preview or insert one step after an existing flow action",
		Example: "  klaviyo-pp-cli flows insert-step FLOW_ID --after ACTION_ID --step-file step.json --json\n  klaviyo-pp-cli flows insert-step FLOW_ID --after ACTION_ID --stdin --apply",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(args[0]) == "" || strings.TrimSpace(after) == "" || strings.TrimSpace(link) == "" {
				return usageErr(fmt.Errorf("flow ID, --after, and a non-empty --link are required"))
			}
			if (stepFile != "") == stdin {
				return usageErr(fmt.Errorf("provide exactly one of --step-file or --stdin"))
			}
			input := cmd.InOrStdin()
			if !stdin {
				file, err := os.Open(stepFile)
				if err != nil {
					return fmt.Errorf("reading step file: %w", err)
				}
				defer file.Close()
				input = file
			}
			step, alwaysLive, err := readFlowInsertStep(input)
			if err != nil {
				return usageErr(err)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// Preview needs the real definition; the command guards the only write.
			c.DryRun = false
			flow, err := fetchFlowInsertStep(c, args[0])
			if err != nil {
				return err
			}
			temporaryID := "insert-" + rand.Text()
			step["temporary_id"] = temporaryID
			anchor, downstream, err := flowInsertStepAnchor(flow.Definition, after, link, temporaryID)
			if err != nil {
				return usageErr(err)
			}
			anchorBody := jsonAPIBody("flow-action", map[string]any{"definition": anchor}, nil)
			anchorBody["data"].(map[string]any)["id"] = after
			result := flowInsertStepResult{
				FlowID: args[0], Name: flow.Name, Status: flow.Status, AfterAction: after, Link: link,
				TemporaryID: temporaryID, Preview: !apply || flags.dryRun, DryRun: flags.dryRun,
				Request: jsonAPIBody("flow-action", map[string]any{
					"definition": step, "allow_live_action": allowLive,
					"anchor-action": anchorBody,
				}, nil),
				Before: flowInsertStepGraph(flow.Definition, ""), AfterSource: "preview",
			}
			result.After = flowInsertStepPreview(flow.Definition, after, link, step, downstream)
			liveRisk := strings.EqualFold(flow.Status, "live") && alwaysLive
			if liveRisk {
				result.Warnings = append(result.Warnings, "This always-live step immediately affects the live flow; profiles already in the flow will hit the step.")
				if !allowLive {
					result.Error = "live flow requires --allow-live-action for this step type"
					return errors.Join(usageErr(errors.New(result.Error)), printFlowInsertStep(cmd, flags, result))
				}
			} else if !alwaysLive {
				result.Warnings = append(result.Warnings, "Communication actions are created in Draft and may need content or template configuration before activation.")
			}
			if result.Preview {
				return printFlowInsertStep(cmd, flags, result)
			}
			resp, status, err := postFlowInsertStepOnce(c, result.Request)
			if err != nil || status != http.StatusCreated {
				if err == nil {
					err = fmt.Errorf("expected HTTP 201, got HTTP %d", status)
				}
				if status == 0 || status >= 500 || status < 400 {
					err = fmt.Errorf("insert result unknown, re-read the flow before trying again: %w", err)
				}
				result.Error = err.Error()
				return errors.Join(classifyAPIErrorOnly(err), printFlowInsertStep(cmd, flags, result))
			}
			result.Applied = true
			result.ActionID = jsonAPIID(resp)
			updated, err := fetchFlowInsertStep(c, args[0])
			if err == nil {
				result.After = flowInsertStepGraph(updated.Definition, result.ActionID)
				result.AfterSource = "re-read"
				result.Status = updated.Status
				_, target, placementErr := flowInsertStepAnchor(updated.Definition, after, link, temporaryID)
				err = placementErr
				found := false
				for _, action := range updated.Definition.Actions {
					if action.ID == result.ActionID && action.Type == step["type"] {
						found = true
						if action.Type != "multi-branch-split" {
							next, exists := action.Links["next"]
							if !exists || (downstream == "" && next != nil) || (downstream != "" && (next == nil || *next != downstream)) {
								err = fmt.Errorf("created action %q did not preserve downstream link %q", result.ActionID, downstream)
							}
						}
					}
				}
				if err == nil && (result.ActionID == "" || target != result.ActionID || !found) {
					err = fmt.Errorf("created action %q was not found after anchor %s on link %s", result.ActionID, after, link)
				}
			}
			if err != nil {
				result.Error = "insertion verification failed; re-read the flow before trying again: " + err.Error()
				return errors.Join(errors.New(result.Error), printFlowInsertStep(cmd, flags, result))
			}
			result.Verified = true
			return printFlowInsertStep(cmd, flags, result)
		},
	}
	cmd.Flags().StringVar(&after, "after", "", "Existing action ID to insert after (required)")
	cmd.Flags().StringVar(&link, "link", "next", "Outgoing link: next, next_if_true, next_if_false, or a multi-branch branch ID")
	cmd.Flags().StringVar(&stepFile, "step-file", "", "JSON file containing the new action definition without links")
	cmd.Flags().BoolVar(&stdin, "stdin", false, "Read the new action definition from stdin")
	cmd.Flags().BoolVar(&allowLive, "allow-live-action", false, "Allow timing or logic steps to affect profiles already in a live flow")
	cmd.Flags().BoolVar(&apply, "apply", false, "Insert the step once and verify its placement (default: preview)")
	return cmd
}

func readFlowInsertStep(input io.Reader) (map[string]any, bool, error) {
	var step map[string]any
	decoder := json.NewDecoder(input)
	if err := decoder.Decode(&step); err != nil {
		return nil, false, fmt.Errorf("parsing step definition: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, false, fmt.Errorf("step input must contain exactly one JSON object")
	}
	kind, _ := step["type"].(string)
	alwaysLive := false
	// FlowActionInsertQuery at revision 2026-10-15 excludes experiments and AC.
	switch kind {
	case "time-delay", "countdown-delay", "target-date", "back-in-stock-delay", "update-profile", "send-webhook", "list-update", "multi-branch-split", "code":
		alwaysLive = true
	case "send-email", "send-sms", "send-mobile-push", "send-whatsapp", "send-internal-alert", "internal-service":
	default:
		return nil, false, fmt.Errorf("unsupported step type %q: conditional-split, trigger-split, action-output-split, ab-test, content-experiment, and Automated Conversations cannot be inserted; use a supported linear action, send-whatsapp, or multi-branch-split", kind)
	}
	for key := range step {
		switch key {
		case "type", "data", "temporary_id":
		case "links":
			return nil, false, fmt.Errorf("new step definition must not contain links when inserting after an anchor")
		default:
			return nil, false, fmt.Errorf("unexpected step field %q; supply an action definition with type and data, without an id or placement", key)
		}
	}
	if data, exists := step["data"]; exists && data != nil {
		if _, ok := data.(map[string]any); !ok {
			return nil, false, fmt.Errorf("step data must be a JSON object")
		}
	}
	for _, branch := range flowInsertStepBranches(step["data"]) {
		if _, exists := branch["links"]; exists {
			return nil, false, fmt.Errorf("new step definition must not contain branch links when inserting after an anchor")
		}
	}
	return step, alwaysLive, nil
}

func fetchFlowInsertStep(c *client.Client, flowID string) (*flowInsertStepFlow, error) {
	raw, err := c.GetNoCache(klaviyoRequestContext(c), "/api/flows/"+url.PathEscape(flowID), map[string]string{"additional-fields[flow]": "definition"})
	if err != nil {
		return nil, classifyAPIErrorOnly(err)
	}
	var response struct {
		Data *struct {
			Attributes flowInsertStepFlow `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, fmt.Errorf("parsing flow definition: %w", err)
	}
	if response.Data == nil || response.Data.Attributes.Definition == nil || response.Data.Attributes.Status == "" {
		return nil, fmt.Errorf("flow %s did not return a definition and status", flowID)
	}
	flow := &response.Data.Attributes
	seen := map[string]bool{}
	for _, action := range flow.Definition.Actions {
		if action.ID == "" || seen[action.ID] {
			return nil, fmt.Errorf("flow definition contains a missing or duplicate action id %q", action.ID)
		}
		seen[action.ID] = true
	}
	if !seen[flow.Definition.EntryActionID] {
		return nil, fmt.Errorf("flow definition has no valid entry action")
	}
	return flow, nil
}

func flowInsertStepBranches(data any) []map[string]any {
	object, _ := data.(map[string]any)
	branches, _ := object["branches"].([]any)
	var out []map[string]any
	for _, raw := range branches {
		if branch, ok := raw.(map[string]any); ok {
			out = append(out, branch)
		}
	}
	return out
}

func flowInsertStepAnchor(def *flowInsertStepDefinition, id, link, temporaryID string) (map[string]any, string, error) {
	for _, action := range def.Actions {
		if action.ID != id {
			continue
		}
		anchor := map[string]any{"type": action.Type}
		switch action.Type {
		case "multi-branch-split":
			for _, branch := range flowInsertStepBranches(action.Data) {
				if branch["branch_id"] == link {
					links, _ := branch["links"].(map[string]any)
					target, _ := links["next"].(string)
					anchor["data"] = map[string]any{"branches": []any{map[string]any{"branch_id": link, "links": map[string]string{"next": temporaryID}}}}
					return anchor, target, nil
				}
			}
			return nil, "", fmt.Errorf("link %q is not a branch ID on anchor %s (multi-branch-split)", link, id)
		case "conditional-split", "trigger-split", "action-output-split":
			if link != "next_if_true" && link != "next_if_false" {
				return nil, "", fmt.Errorf("link %q is invalid for anchor type %s; use next_if_true or next_if_false", link, action.Type)
			}
		case "back-in-stock-delay", "content-experiment", "send-email", "send-mobile-push", "send-sms", "send-webhook", "send-internal-alert", "send-whatsapp", "send-whatsapp-with-ac", "time-delay", "update-profile", "target-date", "countdown-delay", "ab-test", "internal-service", "code", "list-update", "enroll-form", "enroll-customer-hub-block":
			if link != "next" {
				return nil, "", fmt.Errorf("link %q is invalid for anchor type %s; use next", link, action.Type)
			}
		default:
			return nil, "", fmt.Errorf("unsupported anchor type %q", action.Type)
		}
		anchor["links"] = map[string]string{link: temporaryID}
		target := ""
		if action.Links[link] != nil {
			target = *action.Links[link]
		}
		return anchor, target, nil
	}
	return nil, "", fmt.Errorf("anchor action %q does not exist in this flow", id)
}

func flowInsertStepPreview(def *flowInsertStepDefinition, after, link string, step map[string]any, downstream string) flowInsertStepView {
	preview := *def
	preview.Actions = append([]flowInsertStepAction(nil), def.Actions...)
	for i := range preview.Actions {
		action := &preview.Actions[i]
		if action.ID != after {
			continue
		}
		temporaryID := step["temporary_id"].(string)
		if action.Type == "multi-branch-split" {
			action.Data = deepCopyMap(action.Data)
			for _, branch := range flowInsertStepBranches(action.Data) {
				if branch["branch_id"] == link {
					branch["links"] = map[string]any{"next": temporaryID}
				}
			}
		} else {
			action.Links = maps.Clone(action.Links)
			if action.Links == nil {
				action.Links = map[string]*string{}
			}
			action.Links[link] = &temporaryID
		}
	}
	data, _ := step["data"].(map[string]any)
	preview.Actions = append(preview.Actions, flowInsertStepAction{
		ID: step["temporary_id"].(string), Type: step["type"].(string), Data: data, Links: map[string]*string{"next": &downstream},
	})
	return flowInsertStepGraph(&preview, step["temporary_id"].(string))
}

func flowInsertStepGraph(def *flowInsertStepDefinition, newID string) flowInsertStepView {
	actions := map[string]flowInsertStepAction{}
	for _, action := range def.Actions {
		actions[action.ID] = action
	}
	seen := map[string]bool{}
	var walk func(string) []flowInsertStepNode
	walk = func(id string) []flowInsertStepNode {
		nodes := []flowInsertStepNode{}
		for id != "" {
			action, exists := actions[id]
			node := flowInsertStepNode{ID: id, Type: action.Type, Label: flowInsertStepLabel(action), New: id == newID}
			if seen[id] || !exists {
				node.Reference = true
				nodes = append(nodes, node)
				break
			}
			seen[id] = true
			keys := make([]string, 0, len(action.Links))
			for key := range action.Links {
				if key != "next" {
					keys = append(keys, key)
				}
			}
			sort.Strings(keys)
			for _, key := range keys {
				target := ""
				if action.Links[key] != nil {
					target = *action.Links[key]
				}
				node.Branches = append(node.Branches, flowInsertStepBranch{Link: key, Actions: walk(target)})
			}
			for _, branch := range flowInsertStepBranches(action.Data) {
				links, _ := branch["links"].(map[string]any)
				target, _ := links["next"].(string)
				branchID, _ := branch["branch_id"].(string)
				label, _ := branch["name"].(string)
				node.Branches = append(node.Branches, flowInsertStepBranch{Link: branchID, Label: label, Actions: walk(target)})
			}
			nodes = append(nodes, node)
			id = ""
			if action.Links["next"] != nil {
				id = *action.Links["next"]
			}
		}
		return nodes
	}
	view := flowInsertStepView{EntryActionID: def.EntryActionID, Actions: walk(def.EntryActionID)}
	for _, action := range def.Actions {
		if !seen[action.ID] {
			view.Unconnected = append(view.Unconnected, walk(action.ID)...)
		}
	}
	return view
}

func flowInsertStepLabel(action flowInsertStepAction) string {
	label := stringFromMapPath(action.Data, "message.subject_line")
	if label == "" {
		label = stringFromMapPath(action.Data, "message.name")
	}
	if label == "" {
		label = stringFromMapPath(action.Data, "name")
	}
	if label == "" && (action.Type == "time-delay" || action.Type == "countdown-delay") {
		if value, ok := action.Data["value"]; ok {
			unit, _ := action.Data["unit"].(string)
			if unit == "" {
				unit = "days"
			}
			label = fmt.Sprintf("%v %s", value, unit)
		}
	}
	if label == "" {
		label = stringFromMapPath(action.Data, "target_time")
	}
	label = strings.Join(strings.Fields(label), " ")
	if runes := []rune(label); len(runes) > 80 {
		label = string(runes[:77]) + "..."
	}
	return label
}

type flowInsertStepOnceTransport struct {
	base http.RoundTripper
	sent bool
}

func (t *flowInsertStepOnceTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.sent {
		return nil, fmt.Errorf("flow action insertion was already attempted; refusing to send it again")
	}
	t.sent = true
	// temporary_id and configured idempotency headers cannot make this endpoint replayable.
	req.Header.Del("Idempotency-Key")
	req.Header.Del("X-Idempotency-Key")
	return t.base.RoundTrip(req)
}

func postFlowInsertStepOnce(c *client.Client, body any) (json.RawMessage, int, error) {
	previous := c.HTTPClient
	httpClient := *previous
	transport := previous.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	httpClient.Transport = &flowInsertStepOnceTransport{base: transport}
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	c.HTTPClient = &httpClient
	defer func() { c.HTTPClient = previous }()
	return c.PostWithHeaders(klaviyoRequestContext(c), "/api/flow-actions", body, map[string]string{
		client.ReplaySafeHeader: "false", "Idempotency-Key": "", "X-Idempotency-Key": "",
	})
}

func printFlowInsertStep(cmd *cobra.Command, flags *rootFlags, result flowInsertStepResult) error {
	if platformStructuredOutputSelected(cmd.OutOrStdout(), flags) || wantsMachineOutput(flags) {
		return printJSONFiltered(cmd.OutOrStdout(), result, flags)
	}
	var out strings.Builder
	fmt.Fprintf(&out, "Flow %s: %s (%s)\n", result.FlowID, result.Name, result.Status)
	if result.Preview {
		out.WriteString("Preview only; use --apply to insert the step.\n")
	}
	for _, warning := range result.Warnings {
		fmt.Fprintf(&out, "Warning: %s\n", warning)
	}
	var render func([]flowInsertStepNode, int)
	render = func(nodes []flowInsertStepNode, indent int) {
		for _, node := range nodes {
			fmt.Fprintf(&out, "%s- %s (%s)", strings.Repeat("  ", indent), node.ID, node.Type)
			if node.Label != "" {
				fmt.Fprintf(&out, " %s", node.Label)
			}
			if node.New {
				out.WriteString(" [NEW]")
			}
			if node.Reference {
				out.WriteString(" [reference]")
			}
			out.WriteByte('\n')
			for _, branch := range node.Branches {
				fmt.Fprintf(&out, "%s- %s %s\n", strings.Repeat("  ", indent+1), branch.Link, branch.Label)
				render(branch.Actions, indent+2)
			}
		}
	}
	for _, view := range []struct {
		name string
		data flowInsertStepView
	}{{"Before", result.Before}, {"After (" + result.AfterSource + ")", result.After}} {
		fmt.Fprintf(&out, "%s:\n", view.name)
		render(view.data.Actions, 0)
		if len(view.data.Unconnected) > 0 {
			out.WriteString("Unconnected actions:\n")
			render(view.data.Unconnected, 0)
		}
	}
	if result.Verified {
		fmt.Fprintf(&out, "Verified action %s after %s on %s.\n", result.ActionID, result.AfterAction, result.Link)
	}
	if result.Error != "" {
		fmt.Fprintf(&out, "Error: %s\n", result.Error)
	}
	_, err := io.WriteString(cmd.OutOrStdout(), terminalText(cmd.OutOrStdout(), out.String()))
	return err
}
