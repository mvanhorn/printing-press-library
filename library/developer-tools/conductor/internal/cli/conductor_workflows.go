// Copyright 2026 Cole Grolmus and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored ENG-525 orchestration workflows preserved across reprints.
// pp:data-source live

package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/conductor/internal/client"
	"github.com/spf13/cobra"
)

type launchOptions struct {
	ProjectID     string
	RepositoryURL string
	Branch        string
	Name          string
	SessionName   string
	Agent         string
	Model         string
	Effort        string
	Channel       string
	Environment   []string
	Brief         string
	BriefFile     string
}

type launchReceipt struct {
	WorkspaceID  string `json:"workspace_id"`
	SessionID    string `json:"session_id"`
	DeepLink     string `json:"deep_link"`
	MessageID    string `json:"message_id"`
	MessageState string `json:"message_state"`
}

type monitorEvent struct {
	ID           string `json:"id"`
	SessionID    string `json:"session_id"`
	SessionIndex int64  `json:"session_index"`
	Type         string `json:"type"`
	Content      any    `json:"content"`
	ReceivedAt   string `json:"received_at"`
}

type monitorReceipt struct {
	SessionID       string         `json:"session_id"`
	WorkspaceID     string         `json:"workspace_id,omitempty"`
	Status          string         `json:"status"`
	StartedObserved bool           `json:"started_observed"`
	CompletionProof string         `json:"completion_proof"`
	LastMessageID   string         `json:"last_message_id,omitempty"`
	Events          []monitorEvent `json:"events"`
	Polls           int            `json:"polls"`
}

type sessionStatus struct {
	WorkspaceID  string `json:"workspaceId"`
	SessionID    string `json:"sessionId"`
	Status       string `json:"status"`
	UpdatedAt    string `json:"updatedAt"`
	ErrorMessage string `json:"errorMessage"`
	LastError    string `json:"lastError"`
}

type messageRecord struct {
	ID           string  `json:"id"`
	SessionID    string  `json:"sessionId"`
	SessionIndex float64 `json:"sessionIndex"`
	Type         string  `json:"type"`
	Content      any     `json:"content"`
	ReceivedAt   string  `json:"receivedAt"`
}

type messagePage struct {
	Data          []messageRecord `json:"data"`
	Offset        float64         `json:"offset"`
	HasMore       bool            `json:"hasMore"`
	CursorMissing bool            `json:"-"`
}

var errMonitorTimeout = errors.New("monitor timeout")

func newNovelLaunchCmd(flags *rootFlags) *cobra.Command {
	var opts launchOptions
	cmd := &cobra.Command{
		Use:         "launch",
		Short:       "Create a workspace and first session, send a brief, and return the Conductor deep link.",
		Long:        "Create one Conductor Cloud workspace and its first session, then send exactly one task brief. This command does not wait, merge, deploy, or archive anything.",
		Example:     "  conductor-pp-cli launch --repository-url https://github.com/example/acme --branch main --harness codex --model gpt-5.4 --effort high --brief-file issue.md --agent",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			brief, err := resolveBrief(opts.Brief, opts.BriefFile)
			if err != nil {
				return usageErr(err)
			}
			opts.Brief = brief
			if err := validateLaunchOptions(opts); err != nil {
				return usageErr(err)
			}
			if flags.dryRun {
				return flags.printJSON(cmd, launchDryRun(opts))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			receipt, err := launchConductor(cmd.Context(), c, opts)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return flags.printJSON(cmd, map[string]any{"action": "launch", "success": true, "results": receipt})
		},
	}
	bindLaunchFlags(cmd, &opts)
	return cmd
}

func newNovelMonitorCmd(flags *rootFlags) *cobra.Command {
	var interval time.Duration
	var afterMessageID string
	var cancelOnTimeout bool
	cmd := &cobra.Command{
		Use:         "monitor <session-id>",
		Short:       "Poll a session until real completion while collecting only new transcript events.",
		Long:        "Monitor requires evidence that work started before accepting idle as completion. Pass --after-message-id with the receipt from launch or steer so a fast turn that starts and finishes between polls still counts as started when its transcript changes.",
		Args:        cobra.ExactArgs(1),
		Example:     "  conductor-pp-cli monitor sess_123 --after-message-id msg_456 --timeout 30m --interval 5s --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if interval <= 0 {
				return usageErr(errors.New("--interval must be greater than zero"))
			}
			if flags.dryRun {
				return flags.printJSON(cmd, map[string]any{"dry_run": true, "session_id": args[0], "after_message_id": afterMessageID, "interval": interval.String(), "timeout": flags.timeout.String(), "cancel_on_timeout": cancelOnTimeout})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			receipt, err := monitorConductor(cmd.Context(), c, args[0], afterMessageID, interval, flags.timeout)
			if errors.Is(err, errMonitorTimeout) && cancelOnTimeout {
				cancelErr := cancelAndConfirm(cmd.Context(), c, args[0], interval, time.Minute)
				if cancelErr != nil {
					return classifyAPIError(fmt.Errorf("%w; cancellation confirmation failed: %v", err, cancelErr), flags)
				}
				return flags.printJSON(cmd, map[string]any{"action": "monitor", "success": false, "timed_out": true, "canceled": true, "results": receipt})
			}
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return flags.printJSON(cmd, map[string]any{"action": "monitor", "success": true, "results": receipt})
		},
	}
	cmd.Flags().DurationVar(&interval, "interval", 5*time.Second, "Polling interval")
	cmd.Flags().StringVar(&afterMessageID, "after-message-id", "", "Message receipt that starts the monitored turn")
	cmd.Flags().BoolVar(&cancelOnTimeout, "cancel-on-timeout", false, "Cancel the running turn and poll until idle when the monitor deadline expires")
	return cmd
}

func newNovelSteerCmd(flags *rootFlags) *cobra.Command {
	var message, messageFile string
	cmd := &cobra.Command{
		Use:         "steer <session-id>",
		Short:       "Send follow-up guidance to an existing Conductor session with a delivery receipt.",
		Args:        cobra.ExactArgs(1),
		Example:     "  conductor-pp-cli steer sess_123 --message 'Run the focused tests before changing the schema' --agent",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := resolveBrief(message, messageFile)
			if err != nil {
				return usageErr(err)
			}
			if flags.dryRun {
				return flags.printJSON(cmd, briefDryRun("steer", body, map[string]any{"session_id": args[0]}))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			receipt, err := sendMessage(cmd.Context(), c, args[0], body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return flags.printJSON(cmd, map[string]any{"action": "steer", "success": true, "session_id": args[0], "results": receipt})
		},
	}
	cmd.Flags().StringVar(&message, "message", "", "Follow-up guidance text")
	cmd.Flags().StringVar(&messageFile, "message-file", "", "Read follow-up guidance from a file")
	return cmd
}

func newNovelRunCmd(flags *rootFlags) *cobra.Command {
	var opts launchOptions
	var interval time.Duration
	var cancelOnTimeout bool
	cmd := &cobra.Command{
		Use:         "run",
		Short:       "Launch, monitor, and collect the final transcript and deep link with explicit timeout rules.",
		Long:        "Run creates a workspace, sends one brief, and waits for proven completion. It never merges, deploys, archives, or cancels unless --cancel-on-timeout is set.",
		Example:     "  conductor-pp-cli run --repository-url https://github.com/example/acme --branch main --harness codex --model gpt-5.4 --effort high --brief-file issue.md --timeout 30m --agent",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			brief, err := resolveBrief(opts.Brief, opts.BriefFile)
			if err != nil {
				return usageErr(err)
			}
			opts.Brief = brief
			if err := validateLaunchOptions(opts); err != nil {
				return usageErr(err)
			}
			if interval <= 0 {
				return usageErr(errors.New("--interval must be greater than zero"))
			}
			if flags.dryRun {
				plan := launchDryRun(opts)
				plan["action"] = "run"
				plan["interval"] = interval.String()
				plan["timeout"] = flags.timeout.String()
				plan["cancel_on_timeout"] = cancelOnTimeout
				return flags.printJSON(cmd, plan)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			launch, err := launchConductor(cmd.Context(), c, opts)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			mon, err := monitorConductor(cmd.Context(), c, launch.SessionID, launch.MessageID, interval, flags.timeout)
			if errors.Is(err, errMonitorTimeout) && cancelOnTimeout {
				if cancelErr := cancelAndConfirm(cmd.Context(), c, launch.SessionID, interval, time.Minute); cancelErr != nil {
					return classifyAPIError(fmt.Errorf("%w; cancellation confirmation failed: %v", err, cancelErr), flags)
				}
				return flags.printJSON(cmd, map[string]any{"action": "run", "success": false, "timed_out": true, "canceled": true, "launch": launch, "monitor": mon})
			}
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return flags.printJSON(cmd, map[string]any{"action": "run", "success": true, "launch": launch, "monitor": mon})
		},
	}
	bindLaunchFlags(cmd, &opts)
	cmd.Flags().DurationVar(&interval, "interval", 5*time.Second, "Polling interval")
	cmd.Flags().BoolVar(&cancelOnTimeout, "cancel-on-timeout", false, "Cancel and confirm idle when the run deadline expires")
	return cmd
}

func newNovelPlanImplementCmd(flags *rootFlags) *cobra.Command {
	var opts launchOptions
	var plannerAgent, plannerModel, plannerEffort string
	var implementerAgent, implementerModel, implementerEffort string
	var interval time.Duration
	cmd := &cobra.Command{
		Use:         "plan-implement",
		Short:       "Keep planner and implementation sessions separate in one workspace.",
		Long:        "Create a planner session, wait for its transcript, then create a second implementation session in the same workspace using the original brief and planner evidence. No merge or deployment occurs.",
		Example:     "  conductor-pp-cli plan-implement --repository-url https://github.com/example/acme --branch main --planner-agent claude --implementer-agent codex --brief-file issue.md --timeout 30m --agent",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			brief, err := resolveBrief(opts.Brief, opts.BriefFile)
			if err != nil {
				return usageErr(err)
			}
			opts.Brief = "Produce an implementation plan only. Do not modify files.\n\n" + brief
			opts.Agent, opts.Model, opts.Effort = plannerAgent, plannerModel, plannerEffort
			if err := validateLaunchOptions(opts); err != nil {
				return usageErr(fmt.Errorf("planner: %w", err))
			}
			if err := validateAgentModelEffort(implementerAgent, implementerModel, implementerEffort); err != nil {
				return usageErr(fmt.Errorf("implementer: %w", err))
			}
			if interval <= 0 {
				return usageErr(errors.New("--interval must be greater than zero"))
			}
			if flags.dryRun {
				plan := launchDryRun(opts)
				plan["action"] = "plan-implement"
				plan["implementer"] = map[string]any{"agent": implementerAgent, "model": implementerModel, "effort": implementerEffort}
				plan["timeout_per_session"] = flags.timeout.String()
				return flags.printJSON(cmd, plan)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			plannerLaunch, err := launchConductor(cmd.Context(), c, opts)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			plannerMonitor, err := monitorConductor(cmd.Context(), c, plannerLaunch.SessionID, plannerLaunch.MessageID, interval, flags.timeout)
			if err != nil {
				return classifyAPIError(fmt.Errorf("planner session: %w", err), flags)
			}
			planEvidence, err := json.Marshal(plannerMonitor.Events)
			if err != nil {
				return err
			}
			createBody := map[string]any{"workspaceId": plannerLaunch.WorkspaceID, "agent": implementerAgent, "name": "Implementation"}
			if implementerModel != "" {
				createBody["model"] = implementerModel
			}
			if implementerEffort != "" {
				createBody["effort"] = implementerEffort
			}
			createdRaw, _, err := c.PostWithParams(cmd.Context(), "/v0/sessions", nil, createBody)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var created struct {
				ID       string `json:"id"`
				DeepLink string `json:"deepLink"`
			}
			if err := json.Unmarshal(createdRaw, &created); err != nil {
				return fmt.Errorf("decode implementation session: %w", err)
			}
			if created.ID == "" {
				return errors.New("Conductor returned no implementation session id")
			}
			implementationBrief := "Implement the task below. Use the planner evidence as guidance, verify the result, and do not merge or deploy.\n\nOriginal brief:\n" + brief + "\n\nPlanner evidence (JSON):\n" + string(planEvidence)
			messageReceipt, err := sendMessage(cmd.Context(), c, created.ID, implementationBrief)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			implementationMonitor, err := monitorConductor(cmd.Context(), c, created.ID, messageReceipt["message_id"].(string), interval, flags.timeout)
			if err != nil {
				return classifyAPIError(fmt.Errorf("implementation session: %w", err), flags)
			}
			return flags.printJSON(cmd, map[string]any{
				"action": "plan-implement", "success": true,
				"workspace_id":   plannerLaunch.WorkspaceID,
				"planner":        map[string]any{"launch": plannerLaunch, "monitor": plannerMonitor},
				"implementation": map[string]any{"session_id": created.ID, "deep_link": created.DeepLink, "message": messageReceipt, "monitor": implementationMonitor},
			})
		},
	}
	bindLaunchFlags(cmd, &opts)
	_ = cmd.Flags().MarkHidden("harness")
	_ = cmd.Flags().MarkHidden("model")
	_ = cmd.Flags().MarkHidden("effort")
	cmd.Flags().StringVar(&plannerAgent, "planner-agent", "claude", "Planner harness: claude, codex, cursor, or acp")
	cmd.Flags().StringVar(&plannerModel, "planner-model", "", "Planner model id from the pinned Conductor contract")
	cmd.Flags().StringVar(&plannerEffort, "planner-effort", "", "Planner effort from the pinned Conductor contract")
	cmd.Flags().StringVar(&implementerAgent, "implementer-agent", "codex", "Implementation harness: claude, codex, cursor, or acp")
	cmd.Flags().StringVar(&implementerModel, "implementer-model", "", "Implementation model id from the pinned Conductor contract")
	cmd.Flags().StringVar(&implementerEffort, "implementer-effort", "", "Implementation effort from the pinned Conductor contract")
	cmd.Flags().DurationVar(&interval, "interval", 5*time.Second, "Polling interval")
	return cmd
}

func newNovelDailyReportCmd(flags *rootFlags) *cobra.Command {
	var since string
	var limit int
	cmd := &cobra.Command{
		Use:         "daily-report",
		Short:       "Return recent Conductor session rows and mechanical activity totals from transcript search.",
		Long:        "Query only session_transcripts_view and return deterministic recent-session rows. The command does not summarize or call an LLM.",
		Example:     "  conductor-pp-cli daily-report --since 24h --limit 50 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			d, err := parseReportDuration(since)
			if err != nil {
				return usageErr(err)
			}
			if limit < 1 || limit > 500 {
				return usageErr(errors.New("--limit must be between 1 and 500"))
			}
			cutoff := time.Now().UTC().Add(-d).Format(time.RFC3339)
			query := fmt.Sprintf("SELECT session_id, workspace_id, session_title, agent_type, model, workspace_name, workspace_state, repo_url, session_created_at, transcript_updated_at FROM session_transcripts_view WHERE transcript_updated_at >= '%s' ORDER BY transcript_updated_at DESC LIMIT %d", cutoff, limit)
			if flags.dryRun {
				return flags.printJSON(cmd, map[string]any{"dry_run": true, "since": since, "limit": limit, "query": query})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, _, err := c.PostQueryWithParams(cmd.Context(), "/v0/sql", nil, map[string]any{"query": query})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var response struct {
				Rows      []map[string]any `json:"rows"`
				RowCount  int              `json:"rowCount"`
				Truncated bool             `json:"truncated"`
			}
			if err := json.Unmarshal(raw, &response); err != nil {
				return fmt.Errorf("decode daily report: %w", err)
			}
			byAgent := map[string]int{}
			byState := map[string]int{}
			for _, row := range response.Rows {
				byAgent[fmt.Sprint(row["agent_type"])]++
				byState[fmt.Sprint(row["workspace_state"])]++
			}
			return flags.printJSON(cmd, map[string]any{"action": "daily-report", "success": true, "since": since, "row_count": response.RowCount, "truncated": response.Truncated, "by_agent": byAgent, "by_workspace_state": byState, "rows": response.Rows})
		},
	}
	cmd.Flags().StringVar(&since, "since", "24h", "Lookback duration such as 24h, 7d, or 1w")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum transcript rows, 1-500")
	return cmd
}

func bindLaunchFlags(cmd *cobra.Command, opts *launchOptions) {
	cmd.Flags().StringVar(&opts.ProjectID, "project-id", "", "Existing Conductor project id (mutually exclusive with --repository-url)")
	cmd.Flags().StringVar(&opts.RepositoryURL, "repository-url", "", "Repository URL (mutually exclusive with --project-id)")
	cmd.Flags().StringVar(&opts.Branch, "branch", "", "Repository branch")
	cmd.Flags().StringVar(&opts.Name, "name", "", "Workspace name")
	cmd.Flags().StringVar(&opts.SessionName, "session-name", "", "First session name")
	cmd.Flags().StringVar(&opts.Agent, "harness", "", "Agent harness: claude, codex, cursor, or acp")
	cmd.Flags().StringVar(&opts.Model, "model", "", "Model id from the pinned Conductor contract")
	cmd.Flags().StringVar(&opts.Effort, "effort", "", "Effort from the pinned Conductor contract")
	cmd.Flags().StringVar(&opts.Channel, "channel", "", "Desktop channel for the returned deep link")
	cmd.Flags().StringSliceVar(&opts.Environment, "env", nil, "Workspace environment entry KEY=VALUE (repeatable; values are never printed)")
	cmd.Flags().StringVar(&opts.Brief, "brief", "", "Task brief text")
	cmd.Flags().StringVar(&opts.BriefFile, "brief-file", "", "Read the task brief from a file")
}

func resolveBrief(inline, path string) (string, error) {
	if strings.TrimSpace(inline) != "" && strings.TrimSpace(path) != "" {
		return "", errors.New("use exactly one of --brief/--message and --brief-file/--message-file")
	}
	if strings.TrimSpace(path) != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read brief file: %w", err)
		}
		inline = string(data)
	}
	inline = strings.TrimSpace(inline)
	if inline == "" {
		return "", errors.New("brief or message text is required")
	}
	return inline, nil
}

func validateLaunchOptions(opts launchOptions) error {
	if (opts.ProjectID == "") == (opts.RepositoryURL == "") {
		return errors.New("set exactly one of --project-id or --repository-url")
	}
	if strings.TrimSpace(opts.Agent) == "" {
		return errors.New("--harness is required")
	}
	if err := validateAgentModelEffort(opts.Agent, opts.Model, opts.Effort); err != nil {
		return err
	}
	if opts.Channel != "" && !contains([]string{"prod", "alpha", "alpha-chromium", "beta", "patch", "dev"}, opts.Channel) {
		return fmt.Errorf("invalid --channel %q", opts.Channel)
	}
	_, err := parseEnvironment(opts.Environment)
	return err
}

func validateAgentModelEffort(agent, model, effort string) error {
	models := map[string][]string{
		"claude": {"fable-5", "opus-5-1m", "opus-4-8-1m", "opus-4-8", "opus-4-7-1m", "opus-4-7", "opus-1m", "opus", "opus-4-6-1m", "sonnet-5-1m", "sonnet-4-6-1m", "sonnet", "haiku"},
		"codex":  {"gpt-5.5", "gpt-5.4", "gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna", "gpt-5.3-codex-spark", "gpt-5.3-codex", "gpt-5.2-codex"},
		"cursor": {"auto", "composer-2.5", "grok-4.5"},
		"acp":    {},
	}
	allowedModels, ok := models[agent]
	if !ok {
		return fmt.Errorf("invalid agent %q", agent)
	}
	if model != "" && !contains(allowedModels, model) {
		return fmt.Errorf("model %q is not valid for agent %q", model, agent)
	}
	if agent == "claude" && effort != "" && !contains([]string{"low", "medium", "high", "xhigh", "max"}, effort) {
		return fmt.Errorf("effort %q is not valid for claude", effort)
	}
	if agent == "codex" && effort != "" && !contains([]string{"none", "low", "medium", "high", "xhigh", "max", "ultra"}, effort) {
		return fmt.Errorf("effort %q is not valid for codex", effort)
	}
	if (agent == "cursor" || agent == "acp") && effort != "" {
		return fmt.Errorf("effort is not supported for agent %q", agent)
	}
	if agent == "acp" && model != "" {
		return errors.New("model is not supported for agent \"acp\"")
	}
	if agent == "codex" && effort == "max" && !strings.HasPrefix(model, "gpt-5.6-") {
		return errors.New("codex max effort requires a GPT-5.6 model")
	}
	if agent == "codex" && effort == "ultra" && model != "gpt-5.6-sol" && model != "gpt-5.6-terra" {
		return errors.New("codex ultra effort requires gpt-5.6-sol or gpt-5.6-terra")
	}
	return nil
}

func parseEnvironment(entries []string) (map[string]string, error) {
	out := map[string]string{}
	for _, entry := range entries {
		key, value, ok := strings.Cut(entry, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return nil, fmt.Errorf("invalid --env %q; expected KEY=VALUE", entry)
		}
		out[key] = value
	}
	return out, nil
}

func launchConductor(ctx context.Context, c *client.Client, opts launchOptions) (launchReceipt, error) {
	env, err := parseEnvironment(opts.Environment)
	if err != nil {
		return launchReceipt{}, err
	}
	body := map[string]any{"agent": opts.Agent}
	if opts.ProjectID != "" {
		body["projectId"] = opts.ProjectID
	} else {
		body["repositoryUrl"] = opts.RepositoryURL
	}
	if opts.Branch != "" {
		body["branch"] = opts.Branch
	}
	if opts.Name != "" {
		body["name"] = opts.Name
	}
	if opts.SessionName != "" {
		body["sessionName"] = opts.SessionName
	}
	if opts.Model != "" {
		body["model"] = opts.Model
	}
	if opts.Effort != "" {
		body["effort"] = opts.Effort
	}
	if len(env) > 0 {
		body["env"] = env
	}
	params := map[string]string{}
	if opts.Channel != "" {
		params["channel"] = opts.Channel
	}
	raw, _, err := c.PostWithParams(ctx, "/v0/workspaces", params, body)
	if err != nil {
		return launchReceipt{}, err
	}
	var receipt launchReceipt
	var created struct {
		WorkspaceID string `json:"workspaceId"`
		SessionID   string `json:"sessionId"`
		DeepLink    string `json:"deepLink"`
	}
	if err := json.Unmarshal(raw, &created); err != nil {
		return receipt, fmt.Errorf("decode workspace launch: %w", err)
	}
	if created.WorkspaceID == "" || created.SessionID == "" {
		return receipt, errors.New("Conductor launch response omitted workspaceId or sessionId")
	}
	messageReceipt, err := sendMessage(ctx, c, created.SessionID, opts.Brief)
	if err != nil {
		return receipt, err
	}
	receipt.WorkspaceID, receipt.SessionID, receipt.DeepLink = created.WorkspaceID, created.SessionID, created.DeepLink
	receipt.MessageID, _ = messageReceipt["message_id"].(string)
	receipt.MessageState, _ = messageReceipt["state"].(string)
	return receipt, nil
}

func sendMessage(ctx context.Context, c *client.Client, sessionID, message string) (map[string]any, error) {
	path := replacePathParam("/v0/sessions/{sessionId}/messages", "sessionId", sessionID)
	raw, _, err := c.PostWithParams(ctx, path, nil, map[string]any{"message": message})
	if err != nil {
		return nil, err
	}
	var response struct {
		MessageID string `json:"messageId"`
		State     string `json:"state"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, fmt.Errorf("decode message receipt: %w", err)
	}
	if response.MessageID == "" {
		return nil, errors.New("Conductor returned no message id")
	}
	return map[string]any{"message_id": response.MessageID, "state": response.State}, nil
}

func monitorConductor(parent context.Context, c *client.Client, sessionID, afterMessageID string, interval, timeout time.Duration) (monitorReceipt, error) {
	// Lifecycle polling must never reuse GET response-cache entries. A cached
	// idle status is indistinguishable from the queued false-idle race this
	// workflow exists to prevent.
	previousNoCache := c.NoCache
	c.NoCache = true
	defer func() { c.NoCache = previousNoCache }()
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	receipt := monitorReceipt{SessionID: sessionID, LastMessageID: afterMessageID, Events: []monitorEvent{}}
	cursor := afterMessageID
	if cursor == "" {
		page, err := fetchMessages(ctx, c, sessionID, "")
		if err != nil {
			return receipt, err
		}
		if n := len(page.Data); n > 0 {
			cursor = page.Data[n-1].ID
			receipt.LastMessageID = cursor
		}
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		receipt.Polls++
		statusRaw, err := c.Get(ctx, replacePathParam("/v0/sessions/{sessionId}/status", "sessionId", sessionID), nil)
		if err != nil {
			return receipt, err
		}
		var status sessionStatus
		if err := json.Unmarshal(statusRaw, &status); err != nil {
			return receipt, fmt.Errorf("decode session status: %w", err)
		}
		receipt.Status, receipt.WorkspaceID = status.Status, status.WorkspaceID
		if status.Status == "working" {
			receipt.StartedObserved = true
		}
		page, err := fetchMessages(ctx, c, sessionID, cursor)
		if err != nil {
			return receipt, err
		}
		if page.CursorMissing {
			// The message id returned by POST /messages is a queue receipt, not
			// necessarily the id assigned when the user message reaches the
			// transcript. Resolve it to the newest transcript user message and
			// collect only events that follow it.
			all, err := fetchMessages(ctx, c, sessionID, "")
			if err != nil {
				return receipt, err
			}
			lastUser := -1
			for i := range all.Data {
				if all.Data[i].Type == "userMessage" {
					lastUser = i
				}
			}
			if lastUser >= 0 {
				cursor, receipt.LastMessageID = all.Data[lastUser].ID, all.Data[lastUser].ID
				page.Data = all.Data[lastUser+1:]
			} else {
				page.Data = nil
			}
		}
		for _, item := range page.Data {
			event := monitorEvent{ID: item.ID, SessionID: item.SessionID, SessionIndex: int64(item.SessionIndex), Type: item.Type, Content: item.Content, ReceivedAt: item.ReceivedAt}
			receipt.Events = append(receipt.Events, event)
			cursor, receipt.LastMessageID = item.ID, item.ID
			receipt.StartedObserved = true
		}
		if status.Status == "error" {
			return receipt, fmt.Errorf("session %s entered error state: %s %s", sessionID, status.ErrorMessage, status.LastError)
		}
		if receipt.StartedObserved && status.Status == "idle" {
			if len(receipt.Events) > 0 {
				receipt.CompletionProof = "transcript-change-then-idle"
			} else {
				receipt.CompletionProof = "working-then-idle"
			}
			return receipt, nil
		}
		select {
		case <-ctx.Done():
			return receipt, errMonitorTimeout
		case <-ticker.C:
		}
	}
}

func fetchMessages(ctx context.Context, c *client.Client, sessionID, after string) (messagePage, error) {
	params := map[string]string{"limit": "100"}
	if after != "" {
		params["after"] = after
	}
	raw, err := c.Get(ctx, replacePathParam("/v0/sessions/{sessionId}/messages", "sessionId", sessionID), params)
	if err != nil {
		var apiErr *client.APIError
		if after != "" && errors.As(err, &apiErr) && apiErr.StatusCode == 404 && strings.Contains(apiErr.Body, "Cursor message not found") {
			// A newly queued message receipt may not be visible in transcript
			// storage yet. Keep the same cursor and poll again; once the message
			// is indexed, `after` becomes valid and returns only later events.
			return messagePage{Data: []messageRecord{}, HasMore: false, CursorMissing: true}, nil
		}
		return messagePage{}, err
	}
	var page messagePage
	if err := json.Unmarshal(raw, &page); err != nil {
		return page, fmt.Errorf("decode session messages: %w", err)
	}
	return page, nil
}

func cancelAndConfirm(parent context.Context, c *client.Client, sessionID string, interval, timeout time.Duration) error {
	previousNoCache := c.NoCache
	c.NoCache = true
	defer func() { c.NoCache = previousNoCache }()
	if _, _, err := c.PostWithParams(parent, replacePathParam("/v0/sessions/{sessionId}/cancel", "sessionId", sessionID), nil, nil); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		raw, err := c.Get(ctx, replacePathParam("/v0/sessions/{sessionId}/status", "sessionId", sessionID), nil)
		if err != nil {
			return err
		}
		var status sessionStatus
		if err := json.Unmarshal(raw, &status); err != nil {
			return err
		}
		if status.Status == "idle" {
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.New("cancellation did not return session to idle before timeout")
		case <-ticker.C:
		}
	}
}

func launchDryRun(opts launchOptions) map[string]any {
	digest := sha256.Sum256([]byte(opts.Brief))
	envKeys := make([]string, 0, len(opts.Environment))
	for _, entry := range opts.Environment {
		key, _, _ := strings.Cut(entry, "=")
		envKeys = append(envKeys, key)
	}
	return map[string]any{"dry_run": true, "action": "launch", "project_id": opts.ProjectID, "repository_url": opts.RepositoryURL, "branch": opts.Branch, "name": opts.Name, "session_name": opts.SessionName, "agent": opts.Agent, "model": opts.Model, "effort": opts.Effort, "channel": opts.Channel, "env_keys": envKeys, "brief_bytes": len(opts.Brief), "brief_sha256": hex.EncodeToString(digest[:])}
}

func briefDryRun(action, brief string, extra map[string]any) map[string]any {
	digest := sha256.Sum256([]byte(brief))
	out := map[string]any{"dry_run": true, "action": action, "message_bytes": len(brief), "message_sha256": hex.EncodeToString(digest[:])}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func parseReportDuration(value string) (time.Duration, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if strings.HasSuffix(value, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(value, "d"))
		if err != nil || n < 1 {
			return 0, fmt.Errorf("invalid --since %q", value)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	if strings.HasSuffix(value, "w") {
		n, err := strconv.Atoi(strings.TrimSuffix(value, "w"))
		if err != nil || n < 1 {
			return 0, fmt.Errorf("invalid --since %q", value)
		}
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(value)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("invalid --since %q", value)
	}
	return d, nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
