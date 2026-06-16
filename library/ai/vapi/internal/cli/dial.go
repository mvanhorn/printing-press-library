// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// Private command extension for Vapi outbound calls.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newDialCmd(flags *rootFlags) *cobra.Command {
	var assistantID string
	var assistantJSON string
	var phoneNumberID string
	var phoneNumberJSON string
	var numbers []string
	var numbersFile string
	var customerName string
	var customerEmail string
	var customerExternalID string
	var customerExtension string
	var customerSIPURI string
	var e164CheckDisabled bool
	var scheduleAt string
	var scheduleLatestAt string
	var callName string
	var assistantOverridesJSON string
	var variableValuesJSON string
	var bodyJSON string
	var noRecord bool

	cmd := &cobra.Command{
		Use:   "dial",
		Short: "Place or schedule an outbound Vapi assistant call",
		Long: `Place or schedule outbound Vapi assistant calls without hand-building the /call JSON payload.

The command maps the workflow from Vapi's outbound-calling docs:
  1. choose a saved assistant (--assistant-id) or transient assistant (--assistant-json)
  2. choose the phone number to call from (--phone-number-id or --phone-number-json)
  3. provide one or more destinations (--to, --to-file, or --sip-uri)

Live calls are external side effects. Use --dry-run to preview the exact request, and pass --yes for a real POST.`,
		Example: `  # Preview a single outbound call payload
  vapi-pp-cli dial --assistant-id asst_123 --phone-number-id pn_123 --to +15551234567 --dry-run --agent

  # Place the call after review
  VAPI_TOKEN=... vapi-pp-cli dial --assistant-id asst_123 --phone-number-id pn_123 --to +15551234567 --yes --agent

  # Schedule a batch call from a newline-delimited file
  vapi-pp-cli dial --assistant-id asst_123 --phone-number-id pn_123 --to-file ./numbers.txt --schedule-at 2026-06-10T15:00:00Z --dry-run --agent

  # Provide a fully custom body, while still getting guardrails/output
  vapi-pp-cli dial --body-json ./call.json --dry-run --agent`,
		Annotations: map[string]string{"pp:endpoint": "call.create", "pp:method": "POST", "pp:path": "/call", "pp:novel": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !flags.dryRun && !flags.yes {
				return fmt.Errorf("refusing to place a live call without --yes; rerun with --dry-run to preview")
			}

			body, err := buildDialBody(dialBodyOptions{
				AssistantID:            assistantID,
				AssistantJSON:          assistantJSON,
				PhoneNumberID:          phoneNumberID,
				PhoneNumberJSON:        phoneNumberJSON,
				Numbers:                numbers,
				NumbersFile:            numbersFile,
				CustomerName:           customerName,
				CustomerEmail:          customerEmail,
				CustomerExternalID:     customerExternalID,
				CustomerExtension:      customerExtension,
				CustomerSIPURI:         customerSIPURI,
				E164CheckDisabled:      e164CheckDisabled,
				ScheduleAt:             scheduleAt,
				ScheduleLatestAt:       scheduleLatestAt,
				CallName:               callName,
				AssistantOverridesJSON: assistantOverridesJSON,
				VariableValuesJSON:     variableValuesJSON,
				BodyJSON:               bodyJSON,
				NoRecord:               noRecord,
			})
			if err != nil {
				return err
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			envelope := map[string]any{
				"action":   "post",
				"resource": "call",
				"path":     "/call",
			}
			if flags.dryRun {
				envelope["dry_run"] = true
				envelope["status"] = 0
				envelope["success"] = false
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
			if len(data) > 0 {
				var parsed any
				if err := json.Unmarshal(data, &parsed); err == nil {
					envelope["data"] = parsed
				} else {
					envelope["raw"] = string(data)
				}
			}

			return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
		},
	}

	cmd.Flags().StringVar(&assistantID, "assistant-id", "", "Saved Vapi assistant ID to use for the call")
	cmd.Flags().StringVar(&assistantJSON, "assistant-json", "", "Transient assistant JSON: inline JSON, @- for stdin, or path to a JSON file")
	cmd.Flags().StringVar(&phoneNumberID, "phone-number-id", "", "Saved Vapi phone number ID to call from")
	cmd.Flags().StringVar(&phoneNumberJSON, "phone-number-json", "", "Transient phoneNumber JSON: inline JSON, @- for stdin, or path to a JSON file")
	cmd.Flags().StringArrayVar(&numbers, "to", nil, "Destination customer phone number; repeat for batch calls")
	cmd.Flags().StringVar(&numbersFile, "to-file", "", "File with one destination phone number per line; # comments and blank lines are ignored")
	cmd.Flags().StringVar(&customerSIPURI, "sip-uri", "", "Destination SIP URI instead of --to")
	cmd.Flags().StringVar(&customerName, "customer-name", "", "Customer name for a single destination")
	cmd.Flags().StringVar(&customerEmail, "customer-email", "", "Customer email for a single destination")
	cmd.Flags().StringVar(&customerExternalID, "customer-external-id", "", "Customer external ID for a single destination")
	cmd.Flags().StringVar(&customerExtension, "extension", "", "Extension to dial after the destination answers")
	cmd.Flags().BoolVar(&e164CheckDisabled, "disable-e164-check", false, "Set customer.numberE164CheckEnabled=false for non-E164 trunk/SIP numbers")
	cmd.Flags().StringVar(&scheduleAt, "schedule-at", "", "Earliest ISO-8601 time to place the call")
	cmd.Flags().StringVar(&scheduleLatestAt, "schedule-latest-at", "", "Latest ISO-8601 time to place the scheduled call")
	cmd.Flags().StringVar(&callName, "name", "", "Optional Vapi call name, max 40 chars")
	cmd.Flags().StringVar(&assistantOverridesJSON, "assistant-overrides-json", "", "assistantOverrides JSON: inline JSON, @- for stdin, or path to a JSON file")
	cmd.Flags().StringVar(&variableValuesJSON, "variables-json", "", "Shortcut for assistantOverrides.variableValues JSON")
	cmd.Flags().StringVar(&bodyJSON, "body-json", "", "Full /call request body JSON; bypasses ergonomic flag composition")
	cmd.Flags().BoolVar(&noRecord, "no-record", false, "Disable Vapi call recording for this outbound call")
	return cmd
}

type dialBodyOptions struct {
	AssistantID            string
	AssistantJSON          string
	PhoneNumberID          string
	PhoneNumberJSON        string
	Numbers                []string
	NumbersFile            string
	CustomerName           string
	CustomerEmail          string
	CustomerExternalID     string
	CustomerExtension      string
	CustomerSIPURI         string
	E164CheckDisabled      bool
	ScheduleAt             string
	ScheduleLatestAt       string
	CallName               string
	AssistantOverridesJSON string
	VariableValuesJSON     string
	BodyJSON               string
	NoRecord               bool
}

func buildDialBody(opts dialBodyOptions) (map[string]any, error) {
	if opts.BodyJSON != "" {
		var body map[string]any
		if err := readJSONValue(opts.BodyJSON, &body); err != nil {
			return nil, fmt.Errorf("parsing --body-json: %w", err)
		}
		applyDialRecordingDefault(body, opts.NoRecord)
		return body, nil
	}

	body := map[string]any{}
	if opts.CallName != "" {
		body["name"] = opts.CallName
	}

	switch {
	case opts.AssistantID != "" && opts.AssistantJSON != "":
		return nil, fmt.Errorf("use either --assistant-id or --assistant-json, not both")
	case opts.AssistantID != "":
		body["assistantId"] = opts.AssistantID
	case opts.AssistantJSON != "":
		var assistant any
		if err := readJSONValue(opts.AssistantJSON, &assistant); err != nil {
			return nil, fmt.Errorf("parsing --assistant-json: %w", err)
		}
		body["assistant"] = assistant
	default:
		return nil, fmt.Errorf("missing assistant: pass --assistant-id or --assistant-json")
	}

	switch {
	case opts.PhoneNumberID != "" && opts.PhoneNumberJSON != "":
		return nil, fmt.Errorf("use either --phone-number-id or --phone-number-json, not both")
	case opts.PhoneNumberID != "":
		body["phoneNumberId"] = opts.PhoneNumberID
	case opts.PhoneNumberJSON != "":
		var phoneNumber any
		if err := readJSONValue(opts.PhoneNumberJSON, &phoneNumber); err != nil {
			return nil, fmt.Errorf("parsing --phone-number-json: %w", err)
		}
		body["phoneNumber"] = phoneNumber
	default:
		return nil, fmt.Errorf("missing phone number: pass --phone-number-id or --phone-number-json")
	}

	allNumbers := append([]string{}, opts.Numbers...)
	if opts.NumbersFile != "" {
		fromFile, err := readLines(opts.NumbersFile)
		if err != nil {
			return nil, err
		}
		allNumbers = append(allNumbers, fromFile...)
	}
	if opts.CustomerSIPURI != "" && len(allNumbers) > 0 {
		return nil, fmt.Errorf("use --sip-uri or --to/--to-file, not both")
	}
	if opts.CustomerSIPURI == "" && len(allNumbers) == 0 {
		return nil, fmt.Errorf("missing destination: pass --to, --to-file, or --sip-uri")
	}

	if opts.ScheduleAt != "" || opts.ScheduleLatestAt != "" {
		if opts.ScheduleAt == "" {
			return nil, fmt.Errorf("--schedule-latest-at requires --schedule-at")
		}
		schedule := map[string]any{"earliestAt": opts.ScheduleAt}
		if opts.ScheduleLatestAt != "" {
			schedule["latestAt"] = opts.ScheduleLatestAt
		}
		body["schedulePlan"] = schedule
	}

	if opts.AssistantOverridesJSON != "" {
		var overrides map[string]any
		if err := readJSONValue(opts.AssistantOverridesJSON, &overrides); err != nil {
			return nil, fmt.Errorf("parsing --assistant-overrides-json: %w", err)
		}
		body["assistantOverrides"] = overrides
	}
	if opts.VariableValuesJSON != "" {
		var variables map[string]any
		if err := readJSONValue(opts.VariableValuesJSON, &variables); err != nil {
			return nil, fmt.Errorf("parsing --variables-json: %w", err)
		}
		overrides, _ := body["assistantOverrides"].(map[string]any)
		if overrides == nil {
			overrides = map[string]any{}
		}
		overrides["variableValues"] = variables
		body["assistantOverrides"] = overrides
	}

	if opts.CustomerSIPURI != "" {
		body["customer"] = buildDialCustomer("", opts)
		applyDialRecordingDefault(body, opts.NoRecord)
		return body, nil
	}
	customers := make([]map[string]any, 0, len(allNumbers))
	for _, number := range allNumbers {
		number = strings.TrimSpace(number)
		if number == "" {
			continue
		}
		customers = append(customers, buildDialCustomer(number, opts))
	}
	if len(customers) == 0 {
		return nil, fmt.Errorf("no destination numbers found")
	}
	if len(customers) == 1 {
		body["customer"] = customers[0]
	} else {
		if opts.CustomerName != "" || opts.CustomerEmail != "" || opts.CustomerExternalID != "" || opts.CustomerExtension != "" {
			return nil, fmt.Errorf("customer name/email/external-id/extension apply only to a single destination; use --body-json for per-customer batch metadata")
		}
		body["customers"] = customers
	}
	applyDialRecordingDefault(body, opts.NoRecord)
	return body, nil
}

func applyDialRecordingDefault(body map[string]any, noRecord bool) {
	if body == nil {
		return
	}
	overrides, _ := body["assistantOverrides"].(map[string]any)
	if overrides == nil {
		overrides = map[string]any{}
		body["assistantOverrides"] = overrides
	}
	artifactPlan, _ := overrides["artifactPlan"].(map[string]any)
	if artifactPlan == nil {
		artifactPlan = map[string]any{}
		overrides["artifactPlan"] = artifactPlan
	}
	if noRecord {
		artifactPlan["recordingEnabled"] = false
		return
	}
	artifactPlan["recordingEnabled"] = true
	if _, ok := artifactPlan["recordingFormat"]; !ok {
		artifactPlan["recordingFormat"] = "wav;l16"
	}
}

func buildDialCustomer(number string, opts dialBodyOptions) map[string]any {
	customer := map[string]any{}
	if number != "" {
		customer["number"] = number
	}
	if opts.CustomerSIPURI != "" {
		customer["sipUri"] = opts.CustomerSIPURI
	}
	if opts.CustomerName != "" {
		customer["name"] = opts.CustomerName
	}
	if opts.CustomerEmail != "" {
		customer["email"] = opts.CustomerEmail
	}
	if opts.CustomerExternalID != "" {
		customer["externalId"] = opts.CustomerExternalID
	}
	if opts.CustomerExtension != "" {
		customer["extension"] = opts.CustomerExtension
	}
	if opts.E164CheckDisabled {
		customer["numberE164CheckEnabled"] = false
	}
	return customer
}

func readJSONValue(spec string, dest any) error {
	data, err := readValue(spec)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return err
	}
	return nil
}

func readValue(spec string) ([]byte, error) {
	trimmed := strings.TrimSpace(spec)
	if trimmed == "@-" || trimmed == "-" {
		return io.ReadAll(os.Stdin)
	}
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return []byte(trimmed), nil
	}
	return os.ReadFile(spec)
}

func readLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading --to-file: %w", err)
	}
	lines := strings.Split(string(data), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out, nil
}
