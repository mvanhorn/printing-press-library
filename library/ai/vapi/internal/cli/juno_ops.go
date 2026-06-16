// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// Private Juno setup, reporting, and readiness commands.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const defaultJunoProfileName = "juno-default"

func newJunoSetupCmd(flags *rootFlags) *cobra.Command {
	var assistantID string
	var phoneNumberID string
	var profileName string
	var createAssistant bool
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Verify Juno assistant/phone readiness and save a reusable profile",
		Example: `  vapi-pp-cli juno setup --agent
  vapi-pp-cli juno setup --assistant-id asst_123 --phone-number-id pn_123 --profile juno-default --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.DryRun = false
			created := false
			if assistantID == "" {
				assistants, err := fetchJunoAssistants(cmd, c)
				if err != nil {
					return err
				}
				assistantID = findNamedJunoAssistantID(assistants)
				if assistantID == "" && createAssistant {
					if flags.dryRun {
						created = true
						assistantID = "<new-juno-assistant-id>"
					} else {
						if !flags.yes {
							return fmt.Errorf("refusing to create a Juno assistant without --yes; rerun with --dry-run to preview")
						}
						createdAssistant, err := createJunoAssistant(cmd, c, defaultJunoAssistantPayload())
						if err != nil {
							return err
						}
						assistantID = stringValue(createdAssistant["id"])
						created = true
					}
				}
			}
			phoneNumbers, err := fetchJunoPhoneNumbers(cmd, c)
			if err != nil {
				return err
			}
			if phoneNumberID == "" {
				phoneNumberID = firstJunoReadyPhoneNumberID(phoneNumbers)
			}
			out := map[string]any{
				"profile":           profileName,
				"assistant_id":      assistantID,
				"phone_number_id":   phoneNumberID,
				"assistant_created": created,
				"phone_numbers":     summarizeJunoPhoneNumbers(phoneNumbers),
			}
			ready := assistantID != "" && phoneNumberID != ""
			out["ready"] = ready
			if !ready {
				out["next_action"] = "Create a Juno assistant and provision a real outbound-capable Vapi phone number, then rerun setup."
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			values := map[string]string{
				"assistant-id":    assistantID,
				"phone-number-id": phoneNumberID,
			}
			if flags.dryRun {
				out["dry_run"] = true
				out["profile_values"] = values
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if err := saveJunoProfile(profileName, values); err != nil {
				return err
			}
			out["saved"] = true
			out["profile_values"] = values
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&assistantID, "assistant-id", "", "Juno assistant ID to save")
	cmd.Flags().StringVar(&phoneNumberID, "phone-number-id", "", "Outbound-capable Vapi phone number ID to save")
	cmd.Flags().StringVar(&profileName, "profile-name", defaultJunoProfileName, "Profile name to save")
	cmd.Flags().BoolVar(&createAssistant, "create-assistant", false, "Create the default Juno assistant if one does not exist")
	return cmd
}

func newJunoTestCallCmd(flags *rootFlags) *cobra.Command {
	var common junoCommonOptions
	workflow := junoWorkflowOptions{
		TaskType: "test_call",
		Goal:     "Make a short test call to confirm Juno can place recorded outbound calls.",
		Business: "Juno test recipient",
	}
	var downloadPath string
	var wait bool
	cmd := &cobra.Command{
		Use:   "test-call",
		Short: "Place a guarded recorded Juno smoke-test call",
		Example: `  vapi-pp-cli --profile juno-default juno test-call --to +15551234567 --dry-run --agent
  vapi-pp-cli --profile juno-default juno test-call --to +15551234567 --yes --watch --download ./juno-test.wav --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if workflow.Goal == "" {
				workflow.Goal = "Make a short test call to confirm Juno can place recorded outbound calls."
			}
			if workflow.Business == "" {
				workflow.Business = "Juno test recipient"
			}
			if len(common.To) == 0 && common.ToFile == "" && common.SIPURI == "" {
				return fmt.Errorf("missing destination: pass --to, --to-file, or --sip-uri")
			}
			workflow.SuccessCriteria = append(workflow.SuccessCriteria, "Confirm the call connected and ended cleanly.")
			workflow.Notes = append(workflow.Notes, "This is a short Juno CLI smoke test.")
			body, err := buildJunoWorkflowBody(common, workflow)
			if err != nil {
				return err
			}
			if !flags.dryRun && !flags.yes {
				return fmt.Errorf("refusing to place a live Juno test call without --yes; rerun with --dry-run to preview")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			envelope := map[string]any{
				"action":   "post",
				"resource": "call",
				"workflow": workflow.TaskType,
				"path":     "/call",
			}
			if flags.dryRun {
				envelope["dry_run"] = true
				envelope["request"] = map[string]any{
					"method": "POST",
					"url":    c.RequestBaseURL() + "/call",
					"body":   body,
				}
				return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
			}
			data, statusCode, err := c.PostWithParams(cmd.Context(), "/call", map[string]string{}, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			envelope["status"] = statusCode
			envelope["success"] = statusCode >= 200 && statusCode < 300
			var call map[string]any
			if len(data) > 0 && json.Unmarshal(data, &call) == nil {
				envelope["data"] = call
			} else if len(data) > 0 {
				envelope["raw"] = string(data)
			}
			callID := stringValue(call["id"])
			if wait && callID != "" {
				call, err := waitForJunoCallStatus(cmd, c, callID, true, 5*time.Second, 10*time.Minute)
				if err != nil {
					return err
				}
				envelope["result"] = summarizeJunoCall(callID, call, true)
				if cmd.Flags().Changed("download") {
					saved, err := downloadJunoRecording(cmd, c, callID, downloadPath)
					if err != nil {
						return err
					}
					envelope["recording_saved"] = saved
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
		},
	}
	addJunoCommonFlags(cmd, &common)
	addJunoWorkflowFlags(cmd, &workflow)
	cmd.Flags().BoolVar(&wait, "watch", false, "Wait for the test call to end and summarize it")
	cmd.Flags().StringVar(&downloadPath, "download", "", "With --watch, download the recording to an optional path")
	if flag := cmd.Flags().Lookup("download"); flag != nil {
		flag.NoOptDefVal = "."
	}
	return cmd
}

func newJunoReportCmd(flags *rootFlags) *cobra.Command {
	var callIDs []string
	var recent int
	var includeTranscript bool
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Summarize recent Juno calls or explicit call IDs",
		Example: `  vapi-pp-cli juno report --recent 10 --agent
  vapi-pp-cli juno report --call-id call_123 --call-id call_456 --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.DryRun = false
			if len(callIDs) == 0 {
				calls, err := fetchJunoCalls(cmd, c, recent)
				if err != nil {
					return err
				}
				for _, call := range calls {
					if id := stringValue(call["id"]); id != "" {
						callIDs = append(callIDs, id)
					}
				}
			}
			reports := make([]map[string]any, 0, len(callIDs))
			for _, callID := range callIDs {
				call, err := fetchJunoCall(cmd, c, callID)
				if err != nil {
					return err
				}
				reports = append(reports, summarizeJunoCall(callID, call, includeTranscript))
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"count":   len(reports),
				"reports": reports,
			}, flags)
		},
	}
	cmd.Flags().StringArrayVar(&callIDs, "call-id", nil, "Call ID to include; repeatable")
	cmd.Flags().IntVar(&recent, "recent", 10, "When no --call-id is passed, include this many recent calls")
	cmd.Flags().BoolVar(&includeTranscript, "transcript", false, "Include full transcript text")
	return cmd
}

func newJunoPhoneNumbersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "phone-numbers",
		Short:   "List Vapi phone numbers with Juno outbound readiness hints",
		Example: `  vapi-pp-cli juno phone-numbers --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.DryRun = false
			phoneNumbers, err := fetchJunoPhoneNumbers(cmd, c)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"count":         len(phoneNumbers),
				"phone_numbers": summarizeJunoPhoneNumbers(phoneNumbers),
			}, flags)
		},
	}
	return cmd
}

func newJunoAssistantCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "assistant",
		Short: "Create, update, or preview the default Juno assistant",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newJunoAssistantPayloadCmd(flags))
	cmd.AddCommand(newJunoAssistantCreateCmd(flags))
	cmd.AddCommand(newJunoAssistantUpdateCmd(flags))
	return cmd
}

func newJunoAssistantPayloadCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "payload",
		Short: "Print the default Juno assistant JSON payload",
		RunE: func(cmd *cobra.Command, args []string) error {
			return printJSONFiltered(cmd.OutOrStdout(), defaultJunoAssistantPayload(), flags)
		},
	}
}

func newJunoAssistantCreateCmd(flags *rootFlags) *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create the default Juno assistant",
		Example: `  vapi-pp-cli juno assistant create --dry-run --agent
  vapi-pp-cli juno assistant create --yes --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			payload := defaultJunoAssistantPayload()
			if name != "" {
				payload["name"] = name
			}
			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run": true,
					"request": map[string]any{
						"method": "POST",
						"path":   "/assistant",
						"body":   payload,
					},
				}, flags)
			}
			if !flags.yes {
				return fmt.Errorf("refusing to create a Juno assistant without --yes; rerun with --dry-run to preview")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			assistant, err := createJunoAssistant(cmd, c, payload)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), assistant, flags)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Assistant name override")
	return cmd
}

func newJunoAssistantUpdateCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <assistant-id>",
		Short: "Update an existing assistant with the default Juno prompt/settings",
		Example: `  vapi-pp-cli juno assistant update asst_123 --dry-run --agent
  vapi-pp-cli juno assistant update asst_123 --yes --agent`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			payload := defaultJunoAssistantPayload()
			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run": true,
					"request": map[string]any{
						"method": "PATCH",
						"path":   "/assistant/" + args[0],
						"body":   payload,
					},
				}, flags)
			}
			if !flags.yes {
				return fmt.Errorf("refusing to update a Juno assistant without --yes; rerun with --dry-run to preview")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, statusCode, err := c.PatchWithParams(cmd.Context(), replacePathParam("/assistant/{id}", "id", args[0]), map[string]string{}, payload)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var assistant map[string]any
			if err := json.Unmarshal(data, &assistant); err != nil {
				return fmt.Errorf("parsing assistant update response: %w", err)
			}
			assistant["status"] = statusCode
			return printJSONFiltered(cmd.OutOrStdout(), assistant, flags)
		},
	}
	return cmd
}

func buildJunoWorkflowBody(common junoCommonOptions, workflow junoWorkflowOptions) (map[string]any, error) {
	variables, err := buildJunoVariables(common, workflow)
	if err != nil {
		return nil, err
	}
	variablesJSON, err := json.Marshal(variables)
	if err != nil {
		return nil, err
	}
	return buildDialBody(dialBodyOptions{
		AssistantID:            common.AssistantID,
		AssistantJSON:          common.AssistantJSON,
		PhoneNumberID:          common.PhoneNumberID,
		PhoneNumberJSON:        common.PhoneNumberJSON,
		Numbers:                common.To,
		NumbersFile:            common.ToFile,
		CustomerName:           common.CustomerName,
		CustomerEmail:          common.CustomerEmail,
		CustomerExternalID:     common.CustomerExternalID,
		CustomerSIPURI:         common.SIPURI,
		ScheduleAt:             common.ScheduleAt,
		ScheduleLatestAt:       common.ScheduleLatestAt,
		CallName:               firstNonEmpty(common.CallName, "juno-"+workflow.TaskType),
		AssistantOverridesJSON: common.OverridesJSON,
		VariableValuesJSON:     string(variablesJSON),
		NoRecord:               common.NoRecord,
	})
}

func fetchJunoAssistants(cmd *cobra.Command, c interface {
	GetNoCache(ctx context.Context, path string, params map[string]string) (json.RawMessage, error)
}) ([]map[string]any, error) {
	data, err := c.GetNoCache(cmd.Context(), "/assistant", map[string]string{"limit": "100"})
	if err != nil {
		return nil, classifyAPIError(err, nil)
	}
	var assistants []map[string]any
	if err := json.Unmarshal(data, &assistants); err != nil {
		return nil, fmt.Errorf("parsing assistant list: %w", err)
	}
	return assistants, nil
}

func fetchJunoPhoneNumbers(cmd *cobra.Command, c interface {
	GetNoCache(ctx context.Context, path string, params map[string]string) (json.RawMessage, error)
}) ([]map[string]any, error) {
	data, err := c.GetNoCache(cmd.Context(), "/phone-number", map[string]string{"limit": "100"})
	if err != nil {
		return nil, classifyAPIError(err, nil)
	}
	var numbers []map[string]any
	if err := json.Unmarshal(data, &numbers); err != nil {
		return nil, fmt.Errorf("parsing phone number list: %w", err)
	}
	return numbers, nil
}

func fetchJunoCalls(cmd *cobra.Command, c interface {
	GetNoCache(ctx context.Context, path string, params map[string]string) (json.RawMessage, error)
}, limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 10
	}
	data, err := c.GetNoCache(cmd.Context(), "/call", map[string]string{"limit": fmt.Sprintf("%d", limit)})
	if err != nil {
		return nil, classifyAPIError(err, nil)
	}
	var calls []map[string]any
	if err := json.Unmarshal(data, &calls); err != nil {
		return nil, fmt.Errorf("parsing call list: %w", err)
	}
	return calls, nil
}

func createJunoAssistant(cmd *cobra.Command, c interface {
	PostWithParams(ctx context.Context, path string, params map[string]string, body any) (json.RawMessage, int, error)
}, payload map[string]any) (map[string]any, error) {
	data, statusCode, err := c.PostWithParams(cmd.Context(), "/assistant", map[string]string{}, payload)
	if err != nil {
		return nil, classifyAPIError(err, nil)
	}
	var assistant map[string]any
	if err := json.Unmarshal(data, &assistant); err != nil {
		return nil, fmt.Errorf("parsing assistant create response: %w", err)
	}
	assistant["status"] = statusCode
	return assistant, nil
}

func defaultJunoAssistantPayload() map[string]any {
	return map[string]any{
		"name":               "Juno",
		"firstMessage":       "Hi, this is Juno calling for Cathryn.",
		"endCallMessage":     "Thanks, I will pass that along to Cathryn.",
		"voicemailMessage":   "Hi, this is Juno calling for Cathryn. I will follow up another time. Thank you.",
		"maxDurationSeconds": 600,
		"model": map[string]any{
			"provider":    "openai",
			"model":       "gpt-4.1",
			"temperature": 0.4,
			"messages": []map[string]any{
				{
					"role":    "system",
					"content": "You are Juno, Cathryn's house-manager assistant. You make concise, polite phone calls on Cathryn's behalf for reservations, ordering, quote gathering, follow-ups, and general errands. Follow the provided assistantOverrides.variableValues exactly, especially juno_goal, juno_script, juno_constraints, juno_success_criteria, juno_escalate_if, juno_max_spend, and juno_guardrails. Be transparent that you are an assistant calling for Cathryn when asked. Do not provide payment card details, passwords, SSNs, account credentials, legal consent, medical consent, financial consent, deposits, cancellation fees, irreversible purchases, recurring commitments, or terms outside the supplied constraints. If a representative asks for anything outside the instructions, stop and say you need to confirm with Cathryn. Keep responses natural and brief, ask one question at a time, confirm names/dates/times/prices clearly, and end by summarizing the exact outcome, representative name if available, reference or confirmation number, cost, timing, and any next action.",
				},
			},
		},
		"voice": map[string]any{
			"provider": "vapi",
			"voiceId":  "Elliot",
		},
		"artifactPlan": map[string]any{
			"recordingEnabled": true,
			"recordingFormat":  "wav;l16",
			"transcriptPlan": map[string]any{
				"enabled":       true,
				"assistantName": "Juno",
				"userName":      "Representative",
			},
		},
		"analysisPlan": map[string]any{
			"summaryPlan": map[string]any{
				"enabled": true,
			},
		},
	}
}

func saveJunoProfile(name string, values map[string]string) error {
	if strings.ContainsAny(name, `/\: `) {
		return fmt.Errorf("profile name %q contains reserved characters", name)
	}
	s, err := loadProfileStore()
	if err != nil {
		return err
	}
	s.Profiles[name] = Profile{
		Name:        name,
		Description: "Default Juno assistant and outbound phone number",
		Values:      values,
	}
	return saveProfileStore(s)
}

func findNamedJunoAssistantID(assistants []map[string]any) string {
	for _, assistant := range assistants {
		if strings.EqualFold(stringValue(assistant["name"]), "Juno") {
			return stringValue(assistant["id"])
		}
	}
	return ""
}

func firstJunoReadyPhoneNumberID(numbers []map[string]any) string {
	for _, number := range numbers {
		if junoPhoneNumberReady(number) {
			return stringValue(number["id"])
		}
	}
	return ""
}

func summarizeJunoPhoneNumbers(numbers []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(numbers))
	for _, number := range numbers {
		ready := junoPhoneNumberReady(number)
		item := map[string]any{
			"id":             stringValue(number["id"]),
			"name":           stringValue(number["name"]),
			"number":         stringValue(number["number"]),
			"provider":       stringValue(number["provider"]),
			"outbound_ready": ready,
		}
		if !ready {
			item["reason"] = "missing id, number, or provider in Vapi phone-number record"
		}
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return stringValue(out[i]["name"]) < stringValue(out[j]["name"])
	})
	return out
}

func junoPhoneNumberReady(number map[string]any) bool {
	return stringValue(number["id"]) != "" && stringValue(number["number"]) != "" && stringValue(number["provider"]) != ""
}
