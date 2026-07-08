package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/agent"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/security"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/trust"

	"github.com/spf13/cobra"
)

func newAgentCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Assemble, verify, and inject agent-context bundles",
		Long: `Agent-context bundles are signed JSON artifacts that package a Salesforce
Customer 360 slice for an agent to consume. The killer feature of this
CLI.

Commands:
  context  - build a signed bundle for one Account
  brief    - narrative + JSON summary for one Opportunity
  decay    - freshness score across activity, opps, cases, chatter
  verify   - confirm a bundle is signed, fresh, and untampered
  inject   - post a bundle summary to a Slack channel (FLS-aware)

Bundles are verifiable offline against a cached public key.`,
		Example: `  # Build a signed bundle for one Account
  salesforce-headless-360-pp-cli agent context acme-corp --since 90d

  # Verify a bundle another agent generated
  salesforce-headless-360-pp-cli agent verify bundle.json --strict

  # Freshness score for one account
  salesforce-headless-360-pp-cli agent decay --account 001xx000001 --json

  # Narrative brief for one opportunity
  salesforce-headless-360-pp-cli agent brief --opp 006xx000001`,
	}
	// v1 read subcommands are attached in root.go. v1.1 write primitives
	// live here so their shared command plumbing stays close together.
	cmd.AddCommand(newAgentUpdateCmd(flags))
	cmd.AddCommand(newAgentUpsertCmd(flags))
	cmd.AddCommand(newAgentCreateCmd(flags))
	cmd.AddCommand(newAgentLogActivityCmd(flags))
	cmd.AddCommand(newAgentAdvanceCmd(flags))
	cmd.AddCommand(newAgentCloseCaseCmd(flags))
	cmd.AddCommand(newAgentNoteCmd(flags))
	cmd.AddCommand(newAgentPlanCmd(flags))
	cmd.AddCommand(newAgentSignPlanCmd(flags))
	cmd.AddCommand(newAgentExecutePlanCmd(flags))
	cmd.AddCommand(newAgentWriteAuditCmd(flags))
	return cmd
}

func newAgentUpdateCmd(flags *rootFlags) *cobra.Command {
	var fieldFlags []string
	var ifLastModifiedRaw, idempotencyKey, runAsUser string
	var confirmBulk int
	var forceStale, dryRun, useMock bool
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Patch one Salesforce record with a signed, audited write intent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fields, err := agent.ParseFieldAssignments(fieldFlags)
			if err != nil {
				return err
			}
			if len(fields) == 0 {
				return fmt.Errorf("at least one --field NAME=VALUE is required")
			}
			var ifLastModified time.Time
			if ifLastModifiedRaw != "" {
				ifLastModified, err = time.Parse(time.RFC3339, ifLastModifiedRaw)
				if err != nil {
					return fmt.Errorf("--if-last-modified must be RFC3339: %w", err)
				}
			}
			opts := agent.NewUpdateWriteOptions(args[0], fields)
			opts.IdempotencyKey = idempotencyKey
			opts.IfLastModified = ifLastModified
			opts.ForceStale = forceStale
			opts.DryRun = dryRun || flags.dryRun
			opts.RunAsUser = runAsUser
			opts.ConfirmBulk = confirmBulk
			return runAgentWrite(cmd, flags, opts, useMock)
		},
	}
	cmd.Flags().StringArrayVar(&fieldFlags, "field", nil, "Field assignment NAME=VALUE; repeatable")
	cmd.Flags().StringVar(&ifLastModifiedRaw, "if-last-modified", "", "Expected LastModifiedDate in RFC3339")
	cmd.Flags().BoolVar(&forceStale, "force-stale", false, "Bypass optimistic concurrency")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview payload without DML or audit writes")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Optional idempotency key recorded in audit")
	cmd.Flags().StringVar(&runAsUser, "run-as-user", "", "SF User Id required in JWT mode")
	addConfirmBulkFlag(cmd, &confirmBulk)
	cmd.Flags().BoolVar(&useMock, "mock", false, "Run against the in-process Salesforce mock server")
	return cmd
}

func newAgentUpsertCmd(flags *rootFlags) *cobra.Command {
	var fieldFlags []string
	var sobject, idempotencyKey, runAsUser string
	var confirmBulk int
	var dryRun, useMock bool
	cmd := &cobra.Command{
		Use:   "upsert",
		Short: "Upsert one Salesforce record by SF360 idempotency key",
		RunE: func(cmd *cobra.Command, args []string) error {
			fields, err := agent.ParseFieldAssignments(fieldFlags)
			if err != nil {
				return err
			}
			if len(fields) == 0 {
				return fmt.Errorf("at least one --field NAME=VALUE is required")
			}
			opts := agent.NewUpsertWriteOptions(sobject, idempotencyKey, fields)
			opts.DryRun = dryRun || flags.dryRun
			opts.RunAsUser = runAsUser
			opts.ConfirmBulk = confirmBulk
			return runAgentWrite(cmd, flags, opts, useMock)
		},
	}
	cmd.Flags().StringVar(&sobject, "sobject", "", "Salesforce object API name")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Required idempotency key")
	cmd.Flags().StringArrayVar(&fieldFlags, "field", nil, "Field assignment NAME=VALUE; repeatable")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview payload without DML or audit writes")
	cmd.Flags().StringVar(&runAsUser, "run-as-user", "", "SF User Id required in JWT mode")
	addConfirmBulkFlag(cmd, &confirmBulk)
	cmd.Flags().BoolVar(&useMock, "mock", false, "Run against the in-process Salesforce mock server")
	_ = cmd.MarkFlagRequired("sobject")
	_ = cmd.MarkFlagRequired("idempotency-key")
	return cmd
}

func newAgentCreateCmd(flags *rootFlags) *cobra.Command {
	var fieldFlags []string
	var sobject, idempotencyKey, runAsUser string
	var confirmBulk int
	var dryRun, useMock bool
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create one Salesforce record with a signed, audited write intent",
		RunE: func(cmd *cobra.Command, args []string) error {
			fields, err := agent.ParseFieldAssignments(fieldFlags)
			if err != nil {
				return err
			}
			if len(fields) == 0 {
				return fmt.Errorf("at least one --field NAME=VALUE is required")
			}
			opts := agent.NewCreateWriteOptions(sobject, idempotencyKey, fields)
			opts.DryRun = dryRun || flags.dryRun
			opts.RunAsUser = runAsUser
			opts.ConfirmBulk = confirmBulk
			return runAgentWrite(cmd, flags, opts, useMock)
		},
	}
	cmd.Flags().StringVar(&sobject, "sobject", "", "Salesforce object API name")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Required idempotency key")
	cmd.Flags().StringArrayVar(&fieldFlags, "field", nil, "Field assignment NAME=VALUE; repeatable")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview payload without DML or audit writes")
	cmd.Flags().StringVar(&runAsUser, "run-as-user", "", "SF User Id required in JWT mode")
	addConfirmBulkFlag(cmd, &confirmBulk)
	cmd.Flags().BoolVar(&useMock, "mock", false, "Run against the in-process Salesforce mock server")
	_ = cmd.MarkFlagRequired("sobject")
	_ = cmd.MarkFlagRequired("idempotency-key")
	return cmd
}

func newAgentLogActivityCmd(flags *rootFlags) *cobra.Command {
	var activityType, whatID, whoID, subject, description, startRaw, endRaw, idempotencyKey, runAsUser string
	var duration int
	var confirmBulk int
	var dryRun, useMock bool
	cmd := &cobra.Command{
		Use:   "log-activity",
		Short: "Log a completed call/email Task or meeting Event",
		RunE: func(cmd *cobra.Command, args []string) error {
			var start, end time.Time
			var err error
			if startRaw != "" {
				start, err = time.Parse(time.RFC3339, startRaw)
				if err != nil {
					return fmt.Errorf("INVALID_DATE: --start must be RFC3339")
				}
			}
			if endRaw != "" {
				end, err = time.Parse(time.RFC3339, endRaw)
				if err != nil {
					return fmt.Errorf("INVALID_DATE: --end must be RFC3339")
				}
			}
			opts, err := agent.NewLogActivityWriteOptions(agent.LogActivityOptions{
				Type:            activityType,
				WhatID:          whatID,
				WhoID:           whoID,
				Subject:         subject,
				Description:     description,
				DurationSeconds: duration,
				Start:           start,
				End:             end,
				IdempotencyKey:  idempotencyKey,
			})
			if err != nil {
				return err
			}
			opts.DryRun = dryRun || flags.dryRun
			opts.RunAsUser = runAsUser
			opts.ConfirmBulk = confirmBulk
			return runAgentWrite(cmd, flags, opts, useMock)
		},
	}
	cmd.Flags().StringVar(&activityType, "type", "", "Activity type: call, email, or meeting")
	cmd.Flags().StringVar(&whatID, "what", "", "Related WhatId such as an Account or Opportunity")
	cmd.Flags().StringVar(&whoID, "who", "", "Related WhoId such as a Contact")
	cmd.Flags().StringVar(&subject, "subject", "", "Activity subject")
	cmd.Flags().StringVar(&description, "description", "", "Optional activity description")
	cmd.Flags().IntVar(&duration, "duration", 0, "Optional duration in seconds for Task activities")
	cmd.Flags().StringVar(&startRaw, "start", "", "Meeting start time in RFC3339")
	cmd.Flags().StringVar(&endRaw, "end", "", "Meeting end time in RFC3339")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Required idempotency key")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview payload without DML or audit writes")
	cmd.Flags().StringVar(&runAsUser, "run-as-user", "", "SF User Id required in JWT mode")
	addConfirmBulkFlag(cmd, &confirmBulk)
	cmd.Flags().BoolVar(&useMock, "mock", false, "Run against the in-process Salesforce mock server")
	return cmd
}

func newAgentAdvanceCmd(flags *rootFlags) *cobra.Command {
	var oppID, stage, closeDate, idempotencyKey, runAsUser string
	var confirmBulk int
	var forceStale, dryRun, useMock bool
	cmd := &cobra.Command{
		Use:   "advance",
		Short: "Advance an Opportunity to a validated stage",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(oppID) == "" {
				return fmt.Errorf("MISSING_REQUIRED_FLAG: --opp is required")
			}
			if strings.TrimSpace(stage) == "" {
				return fmt.Errorf("MISSING_REQUIRED_FLAG: --stage is required")
			}
			if strings.TrimSpace(closeDate) != "" {
				if _, err := time.Parse("2006-01-02", closeDate); err != nil {
					return fmt.Errorf("INVALID_DATE: --close-date must be YYYY-MM-DD")
				}
			}
			opts := agent.WriteOptions{
				IdempotencyKey: idempotencyKey,
				ForceStale:     forceStale,
				DryRun:         dryRun || flags.dryRun,
				RunAsUser:      runAsUser,
				ConfirmBulk:    confirmBulk,
			}
			return runAgentWriteWithResolver(cmd, flags, opts, useMock, func(configured agent.WriteOptions) (agent.WriteOptions, error) {
				resolved, err := agent.NewAdvanceWriteOptions(cmd.Context(), agent.AdvanceOptions{
					OpportunityID: oppID,
					StageName:     stage,
					CloseDate:     closeDate,
					Client:        configured.Client,
				})
				if err != nil {
					return agent.WriteOptions{}, err
				}
				return resolved, nil
			}, agent.ExecuteWrite)
		},
	}
	cmd.Flags().StringVar(&oppID, "opp", "", "Opportunity Id")
	cmd.Flags().StringVar(&stage, "stage", "", "Target Opportunity StageName")
	cmd.Flags().StringVar(&closeDate, "close-date", "", "Optional CloseDate in YYYY-MM-DD")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Optional idempotency key recorded in audit")
	cmd.Flags().BoolVar(&forceStale, "force-stale", false, "Bypass optimistic concurrency")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview payload without DML or audit writes")
	cmd.Flags().StringVar(&runAsUser, "run-as-user", "", "SF User Id required in JWT mode")
	addConfirmBulkFlag(cmd, &confirmBulk)
	cmd.Flags().BoolVar(&useMock, "mock", false, "Run against the in-process Salesforce mock server")
	return cmd
}

func newAgentCloseCaseCmd(flags *rootFlags) *cobra.Command {
	var caseID, resolution, status, idempotencyKey, runAsUser string
	var confirmBulk int
	var forceStale, dryRun, useMock bool
	cmd := &cobra.Command{
		Use:   "close-case",
		Short: "Close a Case with a resolution",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := agent.NewCloseCaseWriteOptions(agent.CloseCaseOptions{
				CaseID:     caseID,
				Resolution: resolution,
				Status:     status,
			})
			if err != nil {
				return err
			}
			opts.IdempotencyKey = idempotencyKey
			opts.ForceStale = forceStale
			opts.DryRun = dryRun || flags.dryRun
			opts.RunAsUser = runAsUser
			opts.ConfirmBulk = confirmBulk
			return runAgentWriteWithExecutor(cmd, flags, opts, useMock, agent.ExecuteCloseCase)
		},
	}
	cmd.Flags().StringVar(&caseID, "case", "", "Case Id")
	cmd.Flags().StringVar(&resolution, "resolution", "", "Resolution text")
	cmd.Flags().StringVar(&status, "status", "Closed", "Case status to write")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Optional idempotency key recorded in audit")
	cmd.Flags().BoolVar(&forceStale, "force-stale", false, "Bypass optimistic concurrency")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview payload without DML or audit writes")
	cmd.Flags().StringVar(&runAsUser, "run-as-user", "", "SF User Id required in JWT mode")
	addConfirmBulkFlag(cmd, &confirmBulk)
	cmd.Flags().BoolVar(&useMock, "mock", false, "Run against the in-process Salesforce mock server")
	return cmd
}

func newAgentNoteCmd(flags *rootFlags) *cobra.Command {
	var entityID, text, runAsUser string
	var confirmBulk int
	var dryRun, useMock bool
	cmd := &cobra.Command{
		Use:   "note",
		Short: "Post a Chatter FeedItem note to one record",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := agent.NewNoteWriteOptions(entityID, text)
			opts.DryRun = dryRun || flags.dryRun
			opts.RunAsUser = runAsUser
			opts.ConfirmBulk = confirmBulk
			return runAgentWriteWithExecutor(cmd, flags, opts, useMock, agent.ExecuteNote)
		},
	}
	cmd.Flags().StringVar(&entityID, "entity", "", "Record Id to receive the Chatter note")
	cmd.Flags().StringVar(&text, "text", "", "Note body")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview payload without DML or audit writes")
	cmd.Flags().StringVar(&runAsUser, "run-as-user", "", "SF User Id required in JWT mode")
	addConfirmBulkFlag(cmd, &confirmBulk)
	cmd.Flags().BoolVar(&useMock, "mock", false, "Run against the in-process Salesforce mock server")
	return cmd
}

func addConfirmBulkFlag(cmd *cobra.Command, target *int) {
	cmd.Flags().IntVar(target, "confirm-bulk", 0, "Confirm intentional bulk writes by passing the exact record count")
}

func runAgentWrite(cmd *cobra.Command, flags *rootFlags, opts agent.WriteOptions, useMock bool) error {
	return runAgentWriteWithExecutor(cmd, flags, opts, useMock, agent.ExecuteWrite)
}

func runAgentWriteWithExecutor(cmd *cobra.Command, flags *rootFlags, opts agent.WriteOptions, useMock bool, executor func(context.Context, agent.WriteOptions) (*agent.WriteResult, error)) error {
	return runAgentWriteWithResolver(cmd, flags, opts, useMock, nil, executor)
}

func runAgentWriteWithResolver(cmd *cobra.Command, flags *rootFlags, opts agent.WriteOptions, useMock bool, resolver func(agent.WriteOptions) (agent.WriteOptions, error), executor func(context.Context, agent.WriteOptions) (*agent.WriteResult, error)) error {
	cleanup, err := configureTrustMock(useMock)
	if err != nil {
		return err
	}
	defer cleanup()

	c, err := flags.newClient()
	if err != nil {
		return err
	}
	c.DryRun = false
	orgAlias := firstNonEmpty(ResolveOrgAlias(""), flags.profileName, "default")
	profile, _ := GetProfile(orgAlias)
	authMethod := strings.ToLower(os.Getenv("SF360_AUTH_METHOD"))
	if authMethod == "" && profile != nil {
		authMethod = strings.ToLower(profile.AuthMethod)
	}
	apexInstalled := false
	if profile != nil && profile.Values != nil {
		apexInstalled = strings.EqualFold(profile.Values["apex_safe_write_installed"], "true")
	}
	if useMock && authMethod != agent.AuthMethodJWT {
		apexInstalled = true
	}

	signer, err := trust.NewFileSigner(orgAlias)
	if err != nil {
		return fmt.Errorf("load signer for org=%s: %w (hint: run 'trust register --org %s')", orgAlias, err, orgAlias)
	}
	writer := trust.NewWriteAuditWriter(trust.WriteAuditOptions{
		Client:    c,
		DBPath:    defaultDBPath("salesforce-headless-360-pp-cli"),
		HIPAAMode: trust.HIPAAModeFromManifest(".printing-press.json"),
		LogWarn: func(format string, args ...any) {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: "+format+"\n", args...)
		},
	})
	defer writer.Wait()

	opts.Client = c
	opts.Filter = security.NewDefaultFilter(security.Options{Client: c, OrgAlias: orgAlias})
	if useMock && authMethod != agent.AuthMethodJWT {
		opts.Filter = mockWriteFilter{}
	}
	opts.Signer = signer
	opts.AuditWriter = writer
	opts.AuthMethod = authMethod
	opts.ApexCompanionInstalled = apexInstalled
	opts.OrgAlias = orgAlias
	opts.OrgID = firstNonEmpty(os.Getenv("SF360_ORG_ID"), orgAlias)
	opts.ActingUser = firstNonEmpty(os.Getenv("SF360_USER_ID"), opts.RunAsUser)
	opts.LogWarn = func(format string, args ...any) {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: "+format+"\n", args...)
	}
	if resolver != nil {
		resolved, err := resolver(opts)
		if err != nil {
			return err
		}
		opts = inheritConfiguredWriteOptions(resolved, opts)
	}

	result, err := executor(cmd.Context(), opts)
	if err != nil {
		var writeErr *agent.WriteError
		if errors.As(err, &writeErr) {
			envelope := agentWriteEnvelope(false, writeErr.Envelope.Code, writeErr.Envelope.HTTPStatus, writeErr.Envelope.Stage, writeErr.Envelope.Org, writeErr.Envelope.TraceID, nil)
			envelope["hint"] = writeErr.Envelope.Hint
			envelope["cause"] = writeErr.Envelope.Cause
			envelope["data"] = writeErr.Envelope.Data
			if flags.asJSON || flags.agent {
				_ = flags.printJSON(cmd, envelope)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "write failed: %s\n", writeErr.Envelope.Code)
				if writeErr.Envelope.Hint != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "hint: %s\n", writeErr.Envelope.Hint)
				}
			}
		}
		return err
	}

	envelope := agentWriteEnvelope(true, "", result.HTTPStatus, "salesforce_write", opts.OrgID, result.JTI, resultData(result))
	if flags.asJSON || flags.agent {
		return flags.printJSON(cmd, envelope)
	}
	printWriteSummary(cmd, result)
	return nil
}

func inheritConfiguredWriteOptions(resolved, configured agent.WriteOptions) agent.WriteOptions {
	if resolved.IdempotencyKey == "" {
		resolved.IdempotencyKey = configured.IdempotencyKey
	}
	if resolved.IfLastModified.IsZero() {
		resolved.IfLastModified = configured.IfLastModified
	}
	resolved.ForceStale = resolved.ForceStale || configured.ForceStale
	resolved.DryRun = resolved.DryRun || configured.DryRun
	if resolved.ConfirmBulk == 0 {
		resolved.ConfirmBulk = configured.ConfirmBulk
	}
	if resolved.RecordCount == 0 {
		resolved.RecordCount = configured.RecordCount
	}
	if resolved.RunAsUser == "" {
		resolved.RunAsUser = configured.RunAsUser
	}
	resolved.Client = configured.Client
	resolved.Filter = configured.Filter
	resolved.Signer = configured.Signer
	resolved.AuditWriter = configured.AuditWriter
	resolved.AuthMethod = configured.AuthMethod
	resolved.ApexCompanionInstalled = configured.ApexCompanionInstalled
	resolved.OrgAlias = configured.OrgAlias
	resolved.OrgID = configured.OrgID
	resolved.ActingUser = configured.ActingUser
	resolved.Now = configured.Now
	resolved.LogWarn = configured.LogWarn
	resolved.PlanExpiresIn = configured.PlanExpiresIn
	resolved.JTI = configured.JTI
	resolved.AuditMetadata = configured.AuditMetadata
	return resolved
}

func agentWriteEnvelope(success bool, code string, status int, stage string, org string, traceID string, data any) map[string]any {
	return map[string]any{
		"success":     success,
		"code":        code,
		"http_status": status,
		"stage":       stage,
		"org":         org,
		"trace_id":    traceID,
		"data":        data,
	}
}

func resultData(result *agent.WriteResult) map[string]any {
	return map[string]any{
		"jti":             result.JTI,
		"operation":       result.Operation,
		"sobject":         result.SObject,
		"record_id":       result.RecordID,
		"dry_run":         result.DryRun,
		"apex_used":       result.ApexUsed,
		"after_state":     result.AfterState,
		"filter_dropped":  result.FilterDropped,
		"diff":            result.Diff,
		"no_change":       result.NoChange,
		"http_status":     result.HTTPStatus,
		"write_path":      result.WritePath,
		"idempotency_key": result.IdempotencyKey,
	}
}

func printWriteSummary(cmd *cobra.Command, result *agent.WriteResult) {
	status := "executed"
	if result.DryRun {
		status = "dry-run"
	} else if result.NoChange {
		status = "no change"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s (%s)\n", result.Operation, result.SObject, firstNonEmpty(result.RecordID, "(new record)"), status)
	fmt.Fprintf(cmd.OutOrStdout(), "jti: %s\n", result.JTI)
	if len(result.FilterDropped) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "filtered: %s\n", strings.Join(result.FilterDropped, ", "))
	}
	if len(result.Diff) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "diff:")
		keys := make([]string, 0, len(result.Diff))
		for key := range result.Diff {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			entry := result.Diff[key]
			fmt.Fprintf(cmd.OutOrStdout(), "  %s: %v -> %v\n", key, entry["before"], entry["after"])
		}
	}
}

type mockWriteFilter struct{}

func (mockWriteFilter) AllowFieldWrite(user, sobject, field string) bool {
	_ = user
	_ = sobject
	return field != "Salary__c"
}

func newAgentContextActionCmd(flags *rootFlags) *cobra.Command {
	var orgAlias, window, output, runAsUser string
	var dryRun, live, useMock bool
	cmd := &cobra.Command{
		Use:   "context <account>",
		Short: "Build a signed agent-context bundle for one Account",
		Long: `Assembles Account + Contacts + Opportunities + Cases + Tasks + Events +
ContentDocumentLinks + Chatter for the given Account into a signed JSON
bundle any agent can consume. In v1 without a live Salesforce org the
bundle reads from the local SQLite store populated by 'sync'; without
synced data the command returns a clear error.

The bundle is signed by the local key registered via 'trust register'.`,
		Args: cobra.ExactArgs(1),
		Example: `  # Happy path (requires synced local data + registered trust key)
  salesforce-headless-360-pp-cli agent context acme-corp --since 90d --agent

  # Compliance preview without persisting anything
  salesforce-headless-360-pp-cli agent context acme-corp --dry-run

  # CI mode using JWT Bearer auth
  SF360_ORG=prod salesforce-headless-360-pp-cli agent context acme-corp --run-as-user 005xx00000ABCDE`,
		RunE: func(cmd *cobra.Command, args []string) error {
			accountHint := args[0]
			if window == "" {
				window = "P90D"
			}
			if orgAlias == "" {
				orgAlias = firstNonEmpty(ResolveOrgAlias(""), "default")
			}
			if useMock {
				live = true
			}

			var m agent.Manifest
			sourcesUsed := []string{"local"}
			sourcesMissing := []string{"rest", "data_cloud", "slack_linkage"}
			var auditClient trust.BundleAuditClient
			if live {
				cleanup, err := configureTrustMock(useMock)
				if err != nil {
					return err
				}
				defer cleanup()
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				auditClient = c
				secFilter := security.NewDefaultFilter(security.Options{Client: c, OrgAlias: orgAlias})
				since, err := parseAgentSince(window)
				if err != nil {
					return err
				}
				m, _, err = agent.AssembleLiveManifest(cmd.Context(), c, agent.LiveAssemblyOptions{
					AccountID:   accountHint,
					Since:       since,
					Filter:      secFilter,
					FileFetcher: agent.NewSalesforceFileFetcher(c),
				})
				if err != nil {
					return err
				}
				sourcesUsed = []string{"composite_graph"}
				sourcesMissing = []string{"data_cloud", "slack_linkage"}
			} else {
				// Preserve the existing offline/local path. A synced local store can
				// replace this manifest construction without changing the live path.
				m = agent.Manifest{Account: &agent.Account{ID: accountHint, Name: accountHint}}
			}
			hipaaMode := trust.HIPAAModeFromManifest(".printing-press.json")
			if auditClient == nil {
				c, err := flags.newClient()
				if err == nil {
					auditClient = c
				} else if hipaaMode {
					return fmt.Errorf("HIPAA mode requires Salesforce auth for bundle audit: %w", err)
				}
			}
			opts := agent.AssembleOptions{
				OrgAlias:       orgAlias,
				OrgID:          firstNonEmpty(os.Getenv("SF360_ORG_ID"), "unknown-org"),
				InstanceURL:    os.Getenv("SALESFORCE_INSTANCE_URL"),
				UserID:         firstNonEmpty(os.Getenv("SF360_USER_ID"), runAsUser),
				AccountHint:    accountHint,
				QueryWindow:    window,
				SourcesUsed:    sourcesUsed,
				SourcesMissing: sourcesMissing,
				TraceID:        fmt.Sprintf("01J%d", time.Now().UnixNano()),
				AuditClient:    auditClient,
				AuditDBPath:    defaultDBPath("salesforce-headless-360-pp-cli"),
				HIPAAMode:      hipaaMode,
				AuditLogger: func(format string, args ...any) {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: "+format+"\n", args...)
				},
			}

			if dryRun || flags.dryRun {
				env := agent.Envelope{
					OrgID: opts.OrgID, QueryWindow: window,
					SourcesUsed: opts.SourcesUsed, SourcesUnavailable: opts.SourcesMissing,
					Redactions: map[string]int{},
				}
				if flags.asJSON || flags.agent {
					return flags.printJSON(cmd, map[string]any{
						"dry_run":          true,
						"manifest_preview": m,
						"envelope_preview": env,
					})
				}
				fmt.Fprint(cmd.OutOrStdout(), agent.DryRunSummary(m, env))
				return nil
			}

			// JWT mode guard: when running under JWT auth, require --run-as-user
			// to re-scope FLS. Integration users typically see everything.
			if os.Getenv("SF360_AUTH_METHOD") == "jwt" && runAsUser == "" {
				return fmt.Errorf("SF360.AUTH.JWT_NO_RUN_AS_USER: JWT auth mode requires --run-as-user <Id> to enforce FLS")
			}

			signer, err := trust.NewFileSigner(orgAlias)
			if err != nil {
				return fmt.Errorf("load signer for org=%s: %w (hint: run 'trust register --org %s')", orgAlias, err, orgAlias)
			}

			bundle, err := agent.Assemble(m, opts, signerShim{signer})
			if err != nil {
				return err
			}

			if output == "" {
				output = "-"
			}
			if output == "-" {
				if flags.asJSON || flags.agent {
					return flags.printJSON(cmd, bundle)
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(bundle)
			}
			data, err := json.MarshalIndent(bundle, "", "  ")
			if err != nil {
				return err
			}
			if err := os.WriteFile(output, data, 0o600); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", output)
			return nil
		},
	}
	cmd.Flags().StringVar(&orgAlias, "org", "", "Org alias (default: SF360_ORG env or 'default')")
	cmd.Flags().StringVar(&window, "since", "P90D", "Query window (P<N>D, P<N>W) or explicit ISO-8601 duration")
	cmd.Flags().StringVar(&output, "output", "", "Write bundle to path (default: stdout)")
	cmd.Flags().StringVar(&runAsUser, "run-as-user", "", "SF User Id to scope reads against (required in JWT mode)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview bundle contents without signing or persisting")
	cmd.Flags().BoolVar(&live, "live", false, "Assemble directly from Salesforce Composite Graph instead of local store")
	cmd.Flags().BoolVar(&useMock, "mock", false, "Run live assembly against the in-process Salesforce mock server")
	return cmd
}

func newAgentVerifyCmd(flags *rootFlags) *cobra.Command {
	var live, deep, strict, useMock bool
	cmd := &cobra.Command{
		Use:   "verify <bundle-file>",
		Short: "Verify a previously-generated bundle",
		Long: `Verifies the bundle's JWS signature against a cached public key in the
local keystore, and optionally re-fetches the org key (--live) or the
original ContentVersion bytes (--deep). --strict combines both and also
fails if the bundle is past its exp claim.`,
		Args: cobra.ExactArgs(1),
		Example: `  salesforce-headless-360-pp-cli agent verify bundle.json
  salesforce-headless-360-pp-cli agent verify bundle.json --strict`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cleanup, err := configureTrustMock(useMock)
			if err != nil {
				return err
			}
			defer cleanup()

			var c any
			if live || deep || strict {
				client, err := flags.newClient()
				if err != nil {
					return err
				}
				c = client
			}
			var fetcher agent.ContentVersionFetcher
			var keyClient trust.OrgClient
			if client, ok := c.(interface {
				GetStream(string, map[string]string) (io.ReadCloser, error)
				Get(string, map[string]string) (json.RawMessage, error)
				Post(string, any) (json.RawMessage, int, error)
				Patch(string, any) (json.RawMessage, int, error)
			}); ok {
				fetcher = agent.NewSalesforceFileFetcher(client)
				keyClient = client
			}
			result, err := agent.VerifyBundle(args[0], agent.VerifyOptions{
				Live:        live,
				Deep:        deep,
				Strict:      strict,
				FileFetcher: fetcher,
				KeyClient:   keyClient,
			})
			if err != nil {
				if result != nil {
					flags.printJSON(cmd, result)
				}
				return err
			}
			return flags.printJSON(cmd, result)
		},
	}
	cmd.Flags().BoolVar(&live, "live", false, "Re-fetch the org's key collection and check retirement")
	cmd.Flags().BoolVar(&deep, "deep", false, "Re-fetch ContentVersion bytes and rehash")
	cmd.Flags().BoolVar(&strict, "strict", false, "Combine --live + --deep and fail on expired exp")
	cmd.Flags().BoolVar(&useMock, "mock", false, "Run live/deep verification against the in-process Salesforce mock server")
	return cmd
}

func newAgentInjectCmd(flags *rootFlags) *cobra.Command {
	var slackChannel, bundlePath string
	var allowExternal, ephemeral, useMock, attach bool
	cmd := &cobra.Command{
		Use:   "inject [bundle-file]",
		Short: "Post a bundle summary to a Slack channel with channel-audience FLS intersection",
		Long: `Reads a signed bundle, enumerates the Slack channel members, maps each
to a Salesforce User by email, intersects FLS across the full audience,
and posts a field-gated markdown summary. Raw JSON is never posted.

Requires SLACK_BOT_TOKEN. Aborts when any channel member cannot be
mapped to a Salesforce User unless --allow-external-channel-members is
passed with explicit acknowledgment.`,
		Example: `  salesforce-headless-360-pp-cli agent inject --slack '#acme-deal' --bundle bundle.json`,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if bundlePath == "" && len(args) == 1 {
				bundlePath = args[0]
			}
			if slackChannel == "" || bundlePath == "" {
				return fmt.Errorf("--slack and --bundle are both required")
			}
			if os.Getenv("SLACK_BOT_TOKEN") == "" && !useMock {
				return fmt.Errorf("SF360.SLACK.NO_TOKEN: SLACK_BOT_TOKEN not set. Export it to enable posting")
			}
			var poster agent.SlackPoster
			var members []agent.AudienceMember
			if useMock {
				poster = &recordingSlackPoster{}
				members = mockAudienceMembers()
			} else {
				slack := newSlackWebClient(os.Getenv("SLACK_BOT_TOKEN"))
				poster = slack
				slackMembers, err := slack.Audience(slackChannel)
				if err != nil {
					return err
				}
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				members, err = salesforceAudienceMembers(c, slackMembers)
				if err != nil {
					return err
				}
			}
			result, err := agent.InjectBundle(agent.InjectOptions{
				BundlePath:  bundlePath,
				Channel:     slackChannel,
				Ephemeral:   ephemeral,
				AllowWaiver: allowExternal,
				Attach:      attach,
				Members:     members,
				Slack:       poster,
			})
			if err != nil {
				if result != nil && (flags.asJSON || flags.agent) {
					_ = flags.printJSON(cmd, result)
				}
				return err
			}
			return flags.printJSON(cmd, result)
		},
	}
	cmd.Flags().StringVar(&slackChannel, "slack", "", "Slack channel (required)")
	cmd.Flags().StringVar(&bundlePath, "bundle", "", "Path to a signed bundle (required)")
	cmd.Flags().BoolVar(&allowExternal, "allow-external-channel-members", false, "Acknowledge external/unmapped channel members and proceed")
	cmd.Flags().BoolVar(&ephemeral, "slack-ephemeral", false, "Use chat.postEphemeral (no channel retention)")
	cmd.Flags().BoolVar(&attach, "attach", false, "Upload the signed bundle as a Slack file")
	cmd.Flags().BoolVar(&useMock, "mock", false, "Run inject against deterministic mock Slack/Salesforce audience")
	return cmd
}

func newAgentRefreshCmd(flags *rootFlags) *cobra.Command {
	var accountID string
	cmd := &cobra.Command{
		Use:   "refresh",
		Short: "Force a fresh sync before the next bundle assembly",
		Long: `Invalidates sync cursors and re-runs sync for an account (or all
accounts). Exposed as an MCP tool (agent_refresh) so agents can ensure
currency without the user intervening.`,
		Example: `  salesforce-headless-360-pp-cli agent refresh --account 001xx000001`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return flags.printJSON(cmd, map[string]any{
				"status":  "accepted",
				"account": accountID,
				"note":    "v1 resets the sync cursor; the next 'sync' call will do a full pull",
			})
		},
	}
	cmd.Flags().StringVar(&accountID, "account", "", "Account Id (default: all accounts)")
	return cmd
}

// signerShim adapts *trust.FileSigner to agent.Signer.
type signerShim struct{ s *trust.FileSigner }

func (a signerShim) Sign(payload []byte) ([]byte, error) { return a.s.Sign(payload) }
func (a signerShim) KID() string                         { return a.s.KID() }

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func parseAgentSince(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == "P90D" {
		return time.Now().UTC().Add(-90 * 24 * time.Hour), nil
	}
	if strings.HasPrefix(value, "P") && strings.HasSuffix(value, "D") {
		days := strings.TrimSuffix(strings.TrimPrefix(value, "P"), "D")
		var n int
		if _, err := fmt.Sscanf(days, "%d", &n); err != nil {
			return time.Time{}, fmt.Errorf("invalid --since value %q", value)
		}
		return time.Now().UTC().Add(-time.Duration(n) * 24 * time.Hour), nil
	}
	if d, err := time.ParseDuration(value); err == nil {
		return time.Now().UTC().Add(-d), nil
	}
	return time.Time{}, fmt.Errorf("invalid --since value %q", value)
}

type slackWebClient struct {
	token string
	http  *http.Client
}

func newSlackWebClient(token string) *slackWebClient {
	return &slackWebClient{token: token, http: &http.Client{Timeout: 30 * time.Second}}
}

func (s *slackWebClient) Audience(channel string) ([]agent.AudienceMember, error) {
	raw, err := s.call("conversations.members", url.Values{"channel": {channel}})
	if err != nil {
		return nil, err
	}
	var payload struct {
		OK      bool     `json:"ok"`
		Error   string   `json:"error"`
		Members []string `json:"members"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	if !payload.OK {
		return nil, fmt.Errorf("Slack conversations.members failed: %s", payload.Error)
	}
	out := make([]agent.AudienceMember, 0, len(payload.Members))
	for _, id := range payload.Members {
		email, external, err := s.userEmail(id)
		if err != nil {
			return nil, err
		}
		out = append(out, agent.AudienceMember{SlackUserID: id, Email: email, External: external})
	}
	return out, nil
}

func (s *slackWebClient) userEmail(userID string) (string, bool, error) {
	raw, err := s.call("users.info", url.Values{"user": {userID}})
	if err != nil {
		return "", false, err
	}
	var payload struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
		User  struct {
			IsStranger bool `json:"is_stranger"`
			IsBot      bool `json:"is_bot"`
			Profile    struct {
				Email string `json:"email"`
			} `json:"profile"`
		} `json:"user"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", false, err
	}
	if !payload.OK {
		return "", false, fmt.Errorf("Slack users.info failed for %s: %s", userID, payload.Error)
	}
	return payload.User.Profile.Email, payload.User.IsStranger, nil
}

func (s *slackWebClient) PostMessage(channel, text string) error {
	_, err := s.post("chat.postMessage", map[string]any{"channel": channel, "text": text, "mrkdwn": true})
	return err
}

func (s *slackWebClient) PostEphemeral(channel, user, text string) error {
	_, err := s.post("chat.postEphemeral", map[string]any{"channel": channel, "user": user, "text": text, "mrkdwn": true})
	return err
}

func (s *slackWebClient) UploadFile(channel, path string) error {
	_, err := s.post("files.upload", map[string]any{"channels": channel, "file": path})
	return err
}

func (s *slackWebClient) call(method string, values url.Values) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, "https://slack.com/api/"+method+"?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (s *slackWebClient) post(method string, body any) ([]byte, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, "https://slack.com/api/"+method, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var payload struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if json.Unmarshal(raw, &payload) == nil && !payload.OK {
		return raw, fmt.Errorf("Slack %s failed: %s", method, payload.Error)
	}
	return raw, nil
}

type recordingSlackPoster struct{}

func (r *recordingSlackPoster) PostMessage(_, _ string) error      { return nil }
func (r *recordingSlackPoster) PostEphemeral(_, _, _ string) error { return nil }
func (r *recordingSlackPoster) UploadFile(_, _ string) error       { return nil }

func mockAudienceMembers() []agent.AudienceMember {
	fields := defaultAudienceFields()
	return []agent.AudienceMember{
		{SlackUserID: "UACME1", Email: "avery.morgan@example.com", SalesforceUserID: "005ACME1", Fields: fields},
		{SlackUserID: "UACME2", Email: "jordan.lee@example.com", SalesforceUserID: "005ACME2", Fields: fields},
	}
}

type soqlGetter interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}

func salesforceAudienceMembers(c soqlGetter, slackMembers []agent.AudienceMember) ([]agent.AudienceMember, error) {
	emails := make([]string, 0, len(slackMembers))
	for _, member := range slackMembers {
		if member.Email != "" && !member.External {
			emails = append(emails, member.Email)
		}
	}
	userByEmail := map[string]string{}
	if len(emails) > 0 {
		q := "SELECT Id, Email FROM User WHERE Email IN (" + soqlQuotedList(emails) + ")"
		raw, err := c.Get("/services/data/v63.0/query", map[string]string{"q": q})
		if err != nil {
			return nil, err
		}
		var payload struct {
			Records []struct {
				ID    string `json:"Id"`
				Email string `json:"Email"`
			} `json:"records"`
		}
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, err
		}
		for _, record := range payload.Records {
			userByEmail[strings.ToLower(record.Email)] = record.ID
		}
	}
	fields := defaultAudienceFields()
	for i := range slackMembers {
		slackMembers[i].SalesforceUserID = userByEmail[strings.ToLower(slackMembers[i].Email)]
		slackMembers[i].Fields = fields
	}
	return slackMembers, nil
}

func defaultAudienceFields() map[string][]string {
	return map[string][]string{
		"Account":     {"Name", "Industry", "Website", "Type"},
		"Opportunity": {"Name", "StageName", "Amount", "CloseDate"},
	}
}

func soqlQuotedList(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, "'"+strings.ReplaceAll(value, "'", "\\'")+"'")
	}
	return strings.Join(quoted, ",")
}
