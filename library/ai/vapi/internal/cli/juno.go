// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// Private Juno house-manager call workflows.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newJunoCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "juno",
		Aliases: []string{"b"},
		Short:   "Personal house-manager call workflows",
		Long: `Personal assistant call workflows for reservations, ordering, quotes, and general phone calls.

Juno composes safe outbound-call payloads for a Vapi assistant. Every workflow supports --dry-run for preview and refuses live calls unless --yes is passed. The assistant receives structured variables: task type, goal, script, constraints, success criteria, and escalation rules.`,
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newJunoCallCmd(flags))
	cmd.AddCommand(newJunoReservationCmd(flags))
	cmd.AddCommand(newJunoOrderCmd(flags))
	cmd.AddCommand(newJunoQuoteCmd(flags))
	cmd.AddCommand(newJunoFollowupCmd(flags))
	cmd.AddCommand(newJunoStatusCmd(flags))
	cmd.AddCommand(newJunoSetupCmd(flags))
	cmd.AddCommand(newJunoTestCallCmd(flags))
	cmd.AddCommand(newJunoReportCmd(flags))
	cmd.AddCommand(newJunoPhoneNumbersCmd(flags))
	cmd.AddCommand(newJunoAssistantCmd(flags))
	return cmd
}

type junoCommonOptions struct {
	AssistantID        string
	AssistantJSON      string
	PhoneNumberID      string
	PhoneNumberJSON    string
	To                 []string
	ToFile             string
	SIPURI             string
	CustomerName       string
	CustomerEmail      string
	CustomerExternalID string
	ScheduleAt         string
	ScheduleLatestAt   string
	CallName           string
	VariablesJSON      string
	OverridesJSON      string
	NoRecord           bool
}

type junoWorkflowOptions struct {
	TaskType        string
	Goal            string
	Business        string
	Context         string
	Constraints     []string
	SuccessCriteria []string
	EscalateIf      []string
	MaxSpend        string
	Deadline        string
	PreferredWindow string
	Notes           []string
}

func addJunoCommonFlags(cmd *cobra.Command, opts *junoCommonOptions) {
	cmd.Flags().StringVar(&opts.AssistantID, "assistant-id", "", "Saved Vapi assistant ID for Juno")
	cmd.Flags().StringVar(&opts.AssistantJSON, "assistant-json", "", "Transient assistant JSON: inline JSON, @- for stdin, or path")
	cmd.Flags().StringVar(&opts.PhoneNumberID, "phone-number-id", "", "Saved Vapi phone number ID to call from")
	cmd.Flags().StringVar(&opts.PhoneNumberJSON, "phone-number-json", "", "Transient phoneNumber JSON: inline JSON, @- for stdin, or path")
	cmd.Flags().StringArrayVar(&opts.To, "to", nil, "Destination phone number; repeat for multiple calls or quote gathering")
	cmd.Flags().StringVar(&opts.ToFile, "to-file", "", "File with one destination phone number per line")
	cmd.Flags().StringVar(&opts.SIPURI, "sip-uri", "", "Destination SIP URI instead of --to")
	cmd.Flags().StringVar(&opts.CustomerName, "customer-name", "", "Customer/contact name for a single destination")
	cmd.Flags().StringVar(&opts.CustomerEmail, "customer-email", "", "Customer/contact email for a single destination")
	cmd.Flags().StringVar(&opts.CustomerExternalID, "customer-external-id", "", "Customer external ID for a single destination")
	cmd.Flags().StringVar(&opts.ScheduleAt, "schedule-at", "", "Earliest ISO-8601 time to place the call")
	cmd.Flags().StringVar(&opts.ScheduleLatestAt, "schedule-latest-at", "", "Latest ISO-8601 time for a scheduled call")
	cmd.Flags().StringVar(&opts.CallName, "name", "", "Optional Vapi call name")
	cmd.Flags().StringVar(&opts.VariablesJSON, "variables-json", "", "Extra assistantOverrides.variableValues JSON")
	cmd.Flags().StringVar(&opts.OverridesJSON, "assistant-overrides-json", "", "Raw assistantOverrides JSON")
	cmd.Flags().BoolVar(&opts.NoRecord, "no-record", false, "Disable Vapi call recording for this Juno call")
}

func addJunoWorkflowFlags(cmd *cobra.Command, opts *junoWorkflowOptions) {
	cmd.Flags().StringVar(&opts.Goal, "goal", "", "What Juno should accomplish on the call")
	cmd.Flags().StringVar(&opts.Business, "business", "", "Business/vendor/provider/place being called")
	cmd.Flags().StringVar(&opts.Context, "context", "", "Background context Juno should know")
	cmd.Flags().StringArrayVar(&opts.Constraints, "constraint", nil, "Hard constraint; repeatable")
	cmd.Flags().StringArrayVar(&opts.SuccessCriteria, "success", nil, "Success criterion Juno must satisfy; repeatable")
	cmd.Flags().StringArrayVar(&opts.EscalateIf, "escalate-if", nil, "Condition where Juno must stop and report back; repeatable")
	cmd.Flags().StringVar(&opts.MaxSpend, "max-spend", "", "Maximum spend/deposit/fee Juno may accept before escalating")
	cmd.Flags().StringVar(&opts.Deadline, "deadline", "", "Deadline or date window for the task")
	cmd.Flags().StringVar(&opts.PreferredWindow, "preferred-window", "", "Preferred time/date window")
	cmd.Flags().StringArrayVar(&opts.Notes, "note", nil, "Extra note for Juno; repeatable")
}

func newJunoCallCmd(flags *rootFlags) *cobra.Command {
	var common junoCommonOptions
	workflow := junoWorkflowOptions{TaskType: "general_call"}
	cmd := &cobra.Command{
		Use:   "call",
		Short: "Make a general delegated assistant call",
		Example: `  vapi-pp-cli juno call --assistant-id asst_123 --phone-number-id pn_123 --to +15551234567 \
    --business "Front desk" --goal "Ask whether they found my lost sunglasses" --dry-run --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if workflow.Goal == "" {
				return fmt.Errorf("--goal is required")
			}
			return runJunoWorkflow(cmd, flags, common, workflow)
		},
	}
	addJunoCommonFlags(cmd, &common)
	addJunoWorkflowFlags(cmd, &workflow)
	return cmd
}

func newJunoReservationCmd(flags *rootFlags) *cobra.Command {
	var common junoCommonOptions
	workflow := junoWorkflowOptions{TaskType: "reservation"}
	var partySize string
	var occasion string
	cmd := &cobra.Command{
		Use:     "reservation",
		Aliases: []string{"reserve"},
		Short:   "Call to make or modify a reservation",
		Example: `  vapi-pp-cli juno reservation --assistant-id asst_123 --phone-number-id pn_123 --to +15551234567 \
    --business "Via Carota" --party-size 4 --preferred-window "Friday 7-9pm" --occasion birthday --dry-run --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if workflow.Goal == "" {
				workflow.Goal = "Make a reservation"
			}
			if partySize != "" {
				workflow.Notes = append(workflow.Notes, "party size: "+partySize)
			}
			if occasion != "" {
				workflow.Notes = append(workflow.Notes, "occasion: "+occasion)
			}
			workflow.SuccessCriteria = append(workflow.SuccessCriteria,
				"Return confirmed reservation date, time, party size, name, cancellation policy, and confirmation number if available.",
			)
			workflow.EscalateIf = append(workflow.EscalateIf,
				"A deposit, card number, cancellation fee, or non-refundable commitment is required.",
				"The only available time is outside the requested window.",
			)
			return runJunoWorkflow(cmd, flags, common, workflow)
		},
	}
	addJunoCommonFlags(cmd, &common)
	addJunoWorkflowFlags(cmd, &workflow)
	cmd.Flags().StringVar(&partySize, "party-size", "", "Reservation party size")
	cmd.Flags().StringVar(&occasion, "occasion", "", "Optional occasion to mention")
	return cmd
}

func newJunoOrderCmd(flags *rootFlags) *cobra.Command {
	var common junoCommonOptions
	workflow := junoWorkflowOptions{TaskType: "order"}
	var items []string
	var deliveryAddress string
	var pickup bool
	cmd := &cobra.Command{
		Use:   "order",
		Short: "Call to order items or gather an order-ready quote",
		Example: `  vapi-pp-cli juno order --assistant-id asst_123 --phone-number-id pn_123 --to +15551234567 \
    --business "Local florist" --item "seasonal birthday bouquet" --max-spend "$100" --dry-run --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if workflow.Goal == "" {
				workflow.Goal = "Get an order-ready quote; do not purchase unless explicitly allowed."
			}
			for _, item := range items {
				workflow.Notes = append(workflow.Notes, "item: "+item)
			}
			if deliveryAddress != "" {
				workflow.Notes = append(workflow.Notes, "delivery address: "+deliveryAddress)
			}
			if pickup {
				workflow.Notes = append(workflow.Notes, "pickup requested")
			}
			workflow.SuccessCriteria = append(workflow.SuccessCriteria,
				"Return exact items, price, taxes/fees, pickup/delivery timing, cancellation policy, and payment requirement.",
			)
			workflow.EscalateIf = append(workflow.EscalateIf,
				"Payment card, deposit, irreversible purchase, substitution approval, or price above max spend is required.",
			)
			return runJunoWorkflow(cmd, flags, common, workflow)
		},
	}
	addJunoCommonFlags(cmd, &common)
	addJunoWorkflowFlags(cmd, &workflow)
	cmd.Flags().StringArrayVar(&items, "item", nil, "Item to order or quote; repeatable")
	cmd.Flags().StringVar(&deliveryAddress, "delivery-address", "", "Delivery address to provide if appropriate")
	cmd.Flags().BoolVar(&pickup, "pickup", false, "Ask for pickup instead of delivery")
	return cmd
}

func newJunoQuoteCmd(flags *rootFlags) *cobra.Command {
	var common junoCommonOptions
	workflow := junoWorkflowOptions{TaskType: "quote"}
	var service string
	cmd := &cobra.Command{
		Use:   "quote",
		Short: "Call one or more vendors to get comparable quotes",
		Example: `  vapi-pp-cli juno quote --assistant-id asst_123 --phone-number-id pn_123 --to-file plumbers.txt \
    --service "fix leak under kitchen sink" --preferred-window "tomorrow after 10am" --max-spend "$150 diagnostic" --dry-run --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if service == "" && workflow.Goal == "" {
				return fmt.Errorf("--service or --goal is required")
			}
			if workflow.Goal == "" {
				workflow.Goal = "Get a quote for: " + service
			}
			if service != "" {
				workflow.Notes = append(workflow.Notes, "service: "+service)
			}
			workflow.SuccessCriteria = append(workflow.SuccessCriteria,
				"Return price, availability, service fee, deposit requirement, license/insurance status if relevant, and callback/reference details.",
			)
			workflow.EscalateIf = append(workflow.EscalateIf,
				"A deposit, payment card, contract, recurring commitment, or price above max spend is required.",
			)
			return runJunoWorkflow(cmd, flags, common, workflow)
		},
	}
	addJunoCommonFlags(cmd, &common)
	addJunoWorkflowFlags(cmd, &workflow)
	cmd.Flags().StringVar(&service, "service", "", "Service or job to quote")
	return cmd
}

func runJunoWorkflow(cmd *cobra.Command, flags *rootFlags, common junoCommonOptions, workflow junoWorkflowOptions) error {
	if !flags.dryRun && !flags.yes {
		return fmt.Errorf("refusing to place a live Juno call without --yes; rerun with --dry-run to preview")
	}
	variables, err := buildJunoVariables(common, workflow)
	if err != nil {
		return err
	}
	variablesJSON, err := json.Marshal(variables)
	if err != nil {
		return err
	}
	body, err := buildDialBody(dialBodyOptions{
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
	if err != nil {
		return err
	}
	return executeJunoCall(cmd, flags, body, workflow.TaskType)
}

func buildJunoVariables(common junoCommonOptions, workflow junoWorkflowOptions) (map[string]any, error) {
	vars := map[string]any{
		"juno_task_type":        workflow.TaskType,
		"juno_business":         workflow.Business,
		"juno_goal":             workflow.Goal,
		"juno_context":          workflow.Context,
		"juno_constraints":      nonNilStrings(workflow.Constraints),
		"juno_success_criteria": nonNilStrings(workflow.SuccessCriteria),
		"juno_escalate_if":      nonNilStrings(workflow.EscalateIf),
		"juno_max_spend":        workflow.MaxSpend,
		"juno_deadline":         workflow.Deadline,
		"juno_preferred_window": workflow.PreferredWindow,
		"juno_notes":            nonNilStrings(workflow.Notes),
		"juno_script":           buildJunoScript(workflow),
		"juno_guardrails": []string{
			"Be transparent that you are an assistant calling on behalf of Cathryn when asked.",
			"Do not provide SSN, payment card details, account passwords, or legal/medical/financial consent.",
			"Do not agree to deposits, cancellation fees, purchases, recurring commitments, or terms outside constraints without escalation.",
			"Capture the representative name, reference/confirmation number, exact outcome, and follow-up required.",
		},
	}
	if common.VariablesJSON != "" {
		var extra map[string]any
		if err := readJSONValue(common.VariablesJSON, &extra); err != nil {
			return nil, fmt.Errorf("parsing --variables-json: %w", err)
		}
		for k, v := range extra {
			vars[k] = v
		}
	}
	return vars, nil
}

func buildJunoScript(workflow junoWorkflowOptions) string {
	var b strings.Builder
	b.WriteString("You are Juno, Cathryn's house-manager assistant. ")
	b.WriteString("Call to complete this task: ")
	b.WriteString(workflow.Goal)
	b.WriteString(". ")
	if workflow.Business != "" {
		b.WriteString("You are calling ")
		b.WriteString(workflow.Business)
		b.WriteString(". ")
	}
	if workflow.Context != "" {
		b.WriteString("Context: ")
		b.WriteString(workflow.Context)
		b.WriteString(". ")
	}
	if workflow.PreferredWindow != "" {
		b.WriteString("Preferred timing: ")
		b.WriteString(workflow.PreferredWindow)
		b.WriteString(". ")
	}
	if workflow.MaxSpend != "" {
		b.WriteString("Budget/max spend: ")
		b.WriteString(workflow.MaxSpend)
		b.WriteString(". ")
	}
	if len(workflow.Constraints) > 0 {
		b.WriteString("Hard constraints: ")
		b.WriteString(strings.Join(workflow.Constraints, "; "))
		b.WriteString(". ")
	}
	if len(workflow.SuccessCriteria) > 0 {
		b.WriteString("A successful call returns: ")
		b.WriteString(strings.Join(workflow.SuccessCriteria, "; "))
		b.WriteString(". ")
	}
	if len(workflow.EscalateIf) > 0 {
		b.WriteString("Stop and escalate if: ")
		b.WriteString(strings.Join(workflow.EscalateIf, "; "))
		b.WriteString(". ")
	}
	b.WriteString("End by confirming the outcome and any next action.")
	return b.String()
}

func executeJunoCall(cmd *cobra.Command, flags *rootFlags, body map[string]any, workflow string) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	envelope := map[string]any{
		"action":   "post",
		"resource": "call",
		"workflow": workflow,
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
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
