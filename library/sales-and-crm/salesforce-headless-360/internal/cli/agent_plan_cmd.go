package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/agent"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/security"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/trust"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/schemas"

	"github.com/spf13/cobra"
)

type planCommandFlags struct {
	output       string
	expiresIn    time.Duration
	humanSummary string
	useMock      bool
}

func newAgentPlanCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Create signed write plans without executing them",
	}
	cmd.AddCommand(newAgentPlanUpdateCmd(flags))
	cmd.AddCommand(newAgentPlanUpsertCmd(flags))
	cmd.AddCommand(newAgentPlanCreateCmd(flags))
	cmd.AddCommand(newAgentPlanLogActivityCmd(flags))
	cmd.AddCommand(newAgentPlanAdvanceCmd(flags))
	cmd.AddCommand(newAgentPlanCloseCaseCmd(flags))
	cmd.AddCommand(newAgentPlanNoteCmd(flags))
	return cmd
}

func newAgentPlanUpdateCmd(flags *rootFlags) *cobra.Command {
	var fieldFlags []string
	var ifLastModifiedRaw, idempotencyKey, runAsUser string
	var forceStale, dryRun bool
	common := planCommandFlags{expiresIn: time.Hour}
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Plan an audited record update",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fields, err := agent.ParseFieldAssignments(fieldFlags)
			if err != nil {
				return err
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
			return runAgentPlan(cmd, flags, opts, common, nil)
		},
	}
	addPlanCommonFlags(cmd, &common)
	cmd.Flags().StringArrayVar(&fieldFlags, "field", nil, "Field assignment NAME=VALUE; repeatable")
	cmd.Flags().StringVar(&ifLastModifiedRaw, "if-last-modified", "", "Expected LastModifiedDate in RFC3339")
	cmd.Flags().BoolVar(&forceStale, "force-stale", false, "Bypass optimistic concurrency when executed")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Persist dry-run execution intent in the plan")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Optional idempotency key recorded in audit")
	cmd.Flags().StringVar(&runAsUser, "run-as-user", "", "SF User Id required in JWT mode")
	return cmd
}

func newAgentPlanUpsertCmd(flags *rootFlags) *cobra.Command {
	var fieldFlags []string
	var sobject, idempotencyKey, runAsUser string
	var dryRun bool
	common := planCommandFlags{expiresIn: time.Hour}
	cmd := &cobra.Command{
		Use:   "upsert",
		Short: "Plan an upsert by SF360 idempotency key",
		RunE: func(cmd *cobra.Command, args []string) error {
			fields, err := agent.ParseFieldAssignments(fieldFlags)
			if err != nil {
				return err
			}
			opts := agent.NewUpsertWriteOptions(sobject, idempotencyKey, fields)
			opts.DryRun = dryRun || flags.dryRun
			opts.RunAsUser = runAsUser
			return runAgentPlan(cmd, flags, opts, common, nil)
		},
	}
	addPlanCommonFlags(cmd, &common)
	cmd.Flags().StringVar(&sobject, "sobject", "", "Salesforce object API name")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Required idempotency key")
	cmd.Flags().StringArrayVar(&fieldFlags, "field", nil, "Field assignment NAME=VALUE; repeatable")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Persist dry-run execution intent in the plan")
	cmd.Flags().StringVar(&runAsUser, "run-as-user", "", "SF User Id required in JWT mode")
	_ = cmd.MarkFlagRequired("sobject")
	_ = cmd.MarkFlagRequired("idempotency-key")
	return cmd
}

func newAgentPlanCreateCmd(flags *rootFlags) *cobra.Command {
	var fieldFlags []string
	var sobject, idempotencyKey, runAsUser string
	var dryRun bool
	common := planCommandFlags{expiresIn: time.Hour}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Plan a record create",
		RunE: func(cmd *cobra.Command, args []string) error {
			fields, err := agent.ParseFieldAssignments(fieldFlags)
			if err != nil {
				return err
			}
			opts := agent.NewCreateWriteOptions(sobject, idempotencyKey, fields)
			opts.DryRun = dryRun || flags.dryRun
			opts.RunAsUser = runAsUser
			return runAgentPlan(cmd, flags, opts, common, nil)
		},
	}
	addPlanCommonFlags(cmd, &common)
	cmd.Flags().StringVar(&sobject, "sobject", "", "Salesforce object API name")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Required idempotency key")
	cmd.Flags().StringArrayVar(&fieldFlags, "field", nil, "Field assignment NAME=VALUE; repeatable")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Persist dry-run execution intent in the plan")
	cmd.Flags().StringVar(&runAsUser, "run-as-user", "", "SF User Id required in JWT mode")
	_ = cmd.MarkFlagRequired("sobject")
	_ = cmd.MarkFlagRequired("idempotency-key")
	return cmd
}

func newAgentPlanLogActivityCmd(flags *rootFlags) *cobra.Command {
	var activityType, whatID, whoID, subject, description, startRaw, endRaw, idempotencyKey, runAsUser string
	var duration int
	var dryRun bool
	common := planCommandFlags{expiresIn: time.Hour}
	cmd := &cobra.Command{
		Use:   "log-activity",
		Short: "Plan a completed Task or Event",
		RunE: func(cmd *cobra.Command, args []string) error {
			start, end, err := parsePlanActivityTimes(startRaw, endRaw)
			if err != nil {
				return err
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
			return runAgentPlan(cmd, flags, opts, common, nil)
		},
	}
	addPlanCommonFlags(cmd, &common)
	cmd.Flags().StringVar(&activityType, "type", "", "Activity type: call, email, or meeting")
	cmd.Flags().StringVar(&whatID, "what", "", "Related WhatId such as an Account or Opportunity")
	cmd.Flags().StringVar(&whoID, "who", "", "Related WhoId such as a Contact")
	cmd.Flags().StringVar(&subject, "subject", "", "Activity subject")
	cmd.Flags().StringVar(&description, "description", "", "Optional activity description")
	cmd.Flags().IntVar(&duration, "duration", 0, "Optional duration in seconds for Task activities")
	cmd.Flags().StringVar(&startRaw, "start", "", "Meeting start time in RFC3339")
	cmd.Flags().StringVar(&endRaw, "end", "", "Meeting end time in RFC3339")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Required idempotency key")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Persist dry-run execution intent in the plan")
	cmd.Flags().StringVar(&runAsUser, "run-as-user", "", "SF User Id required in JWT mode")
	return cmd
}

func newAgentPlanAdvanceCmd(flags *rootFlags) *cobra.Command {
	var oppID, stage, closeDate, idempotencyKey, runAsUser string
	var forceStale, dryRun bool
	common := planCommandFlags{expiresIn: time.Hour}
	cmd := &cobra.Command{
		Use:   "advance",
		Short: "Plan an Opportunity stage advance",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := agent.WriteOptions{
				IdempotencyKey: idempotencyKey,
				ForceStale:     forceStale,
				DryRun:         dryRun || flags.dryRun,
				RunAsUser:      runAsUser,
			}
			return runAgentPlan(cmd, flags, opts, common, func(configured agent.WriteOptions) (agent.WriteOptions, error) {
				return agent.NewAdvanceWriteOptions(cmd.Context(), agent.AdvanceOptions{
					OpportunityID: oppID,
					StageName:     stage,
					CloseDate:     closeDate,
					Client:        configured.Client,
				})
			})
		},
	}
	addPlanCommonFlags(cmd, &common)
	cmd.Flags().StringVar(&oppID, "opp", "", "Opportunity Id")
	cmd.Flags().StringVar(&stage, "stage", "", "Target Opportunity StageName")
	cmd.Flags().StringVar(&closeDate, "close-date", "", "Optional CloseDate in YYYY-MM-DD")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Optional idempotency key recorded in audit")
	cmd.Flags().BoolVar(&forceStale, "force-stale", false, "Bypass optimistic concurrency when executed")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Persist dry-run execution intent in the plan")
	cmd.Flags().StringVar(&runAsUser, "run-as-user", "", "SF User Id required in JWT mode")
	return cmd
}

func newAgentPlanCloseCaseCmd(flags *rootFlags) *cobra.Command {
	var caseID, resolution, status, idempotencyKey, runAsUser string
	var forceStale, dryRun bool
	common := planCommandFlags{expiresIn: time.Hour}
	cmd := &cobra.Command{
		Use:   "close-case",
		Short: "Plan a Case close",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := agent.NewCloseCaseWriteOptions(agent.CloseCaseOptions{CaseID: caseID, Resolution: resolution, Status: status})
			if err != nil {
				return err
			}
			opts.IdempotencyKey = idempotencyKey
			opts.ForceStale = forceStale
			opts.DryRun = dryRun || flags.dryRun
			opts.RunAsUser = runAsUser
			return runAgentPlan(cmd, flags, opts, common, nil)
		},
	}
	addPlanCommonFlags(cmd, &common)
	cmd.Flags().StringVar(&caseID, "case", "", "Case Id")
	cmd.Flags().StringVar(&resolution, "resolution", "", "Resolution text")
	cmd.Flags().StringVar(&status, "status", "Closed", "Case status to write")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Optional idempotency key recorded in audit")
	cmd.Flags().BoolVar(&forceStale, "force-stale", false, "Bypass optimistic concurrency when executed")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Persist dry-run execution intent in the plan")
	cmd.Flags().StringVar(&runAsUser, "run-as-user", "", "SF User Id required in JWT mode")
	return cmd
}

func newAgentPlanNoteCmd(flags *rootFlags) *cobra.Command {
	var entityID, text, runAsUser string
	var dryRun bool
	common := planCommandFlags{expiresIn: time.Hour}
	cmd := &cobra.Command{
		Use:   "note",
		Short: "Plan a Chatter FeedItem note",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := agent.NewNoteWriteOptions(entityID, text)
			opts.DryRun = dryRun || flags.dryRun
			opts.RunAsUser = runAsUser
			return runAgentPlan(cmd, flags, opts, common, nil)
		},
	}
	addPlanCommonFlags(cmd, &common)
	cmd.Flags().StringVar(&entityID, "entity", "", "Record Id to receive the Chatter note")
	cmd.Flags().StringVar(&text, "text", "", "Note body")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Persist dry-run execution intent in the plan")
	cmd.Flags().StringVar(&runAsUser, "run-as-user", "", "SF User Id required in JWT mode")
	return cmd
}

func newAgentSignPlanCmd(flags *rootFlags) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "sign-plan <plan.json>",
		Short: "Append a local countersignature to a write plan",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			plan, err := readPlanFile(args[0])
			if err != nil {
				return err
			}
			orgAlias := firstNonEmpty(ResolveOrgAlias(""), flags.profileName, "default")
			signer, err := trust.NewFileSigner(orgAlias)
			if err != nil {
				return fmt.Errorf("load signer for org=%s: %w (hint: run 'trust register --org %s')", orgAlias, err, orgAlias)
			}
			if err := ensureLocalPlanKeyRecord(orgAlias, signer); err != nil {
				return err
			}
			if err := agent.AppendCountersignature(plan, signer); err != nil {
				return err
			}
			if output == "" {
				output = args[0]
			}
			return writePlanOutput(cmd, plan, output)
		},
	}
	cmd.Flags().StringVar(&output, "output", "", "Write signed plan to path (default: update input file)")
	return cmd
}

func newAgentExecutePlanCmd(flags *rootFlags) *cobra.Command {
	var requireCountersigs int
	var confirmBulk int
	var useMock, forceStale bool
	cmd := &cobra.Command{
		Use:   "execute-plan <plan.json>",
		Short: "Verify and execute a signed write plan",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			plan, err := readPlanFile(args[0])
			if err != nil {
				return err
			}
			configured, cleanup, err := configureAgentPlanOptions(cmd, flags, useMock, true)
			if err != nil {
				return err
			}
			defer cleanup()
			result, err := agent.ExecutePlan(cmd.Context(), plan, agent.ExecuteOptions{
				MinCountersignatures:   requireCountersigs,
				Client:                 configured.Client,
				Filter:                 configured.Filter,
				Signer:                 configured.Signer,
				AuditWriter:            configured.AuditWriter,
				AuthMethod:             configured.AuthMethod,
				ApexCompanionInstalled: configured.ApexCompanionInstalled,
				OrgAlias:               configured.OrgAlias,
				OrgID:                  configured.OrgID,
				ActingUser:             configured.ActingUser,
				RunAsUser:              configured.RunAsUser,
				ForceStale:             forceStale,
				DryRun:                 flags.dryRun,
				ConfirmBulk:            confirmBulk,
				Now:                    configured.Now,
				LogWarn:                configured.LogWarn,
			})
			if err != nil {
				return err
			}
			envelope := agentWriteEnvelope(true, "", result.HTTPStatus, "salesforce_write", configured.OrgID, result.JTI, resultData(result))
			if flags.asJSON || flags.agent {
				return flags.printJSON(cmd, envelope)
			}
			printWriteSummary(cmd, result)
			return nil
		},
	}
	cmd.Flags().IntVar(&requireCountersigs, "require-countersignatures", 0, "Minimum countersignatures required before execution")
	addConfirmBulkFlag(cmd, &confirmBulk)
	cmd.Flags().BoolVar(&useMock, "mock", false, "Run against the in-process Salesforce mock server")
	cmd.Flags().BoolVar(&forceStale, "force-stale", false, "Bypass optimistic concurrency")
	return cmd
}

func addPlanCommonFlags(cmd *cobra.Command, flags *planCommandFlags) {
	cmd.Flags().StringVar(&flags.output, "output", "-", "Write plan JSON to path (default: stdout)")
	cmd.Flags().DurationVar(&flags.expiresIn, "expires-in", time.Hour, "Plan expiration duration")
	cmd.Flags().StringVar(&flags.humanSummary, "human-summary", "", "Human-readable plan summary")
	cmd.Flags().BoolVar(&flags.useMock, "mock", false, "Run plan construction against the in-process Salesforce mock server")
}

func runAgentPlan(cmd *cobra.Command, flags *rootFlags, opts agent.WriteOptions, planFlags planCommandFlags, resolver func(agent.WriteOptions) (agent.WriteOptions, error)) error {
	configured, cleanup, err := configureAgentPlanOptions(cmd, flags, planFlags.useMock, false)
	if err != nil {
		return err
	}
	defer cleanup()
	opts = inheritConfiguredWriteOptions(opts, configured)
	opts.PlanExpiresIn = planFlags.expiresIn
	if resolver != nil {
		resolved, err := resolver(opts)
		if err != nil {
			return err
		}
		opts = inheritConfiguredWriteOptions(resolved, opts)
		opts.PlanExpiresIn = planFlags.expiresIn
	}
	executePath := "ui_api"
	if opts.AuthMethod == agent.AuthMethodJWT {
		executePath = "apex"
	} else if opts.SObject == "FeedItem" {
		executePath = "chatter"
	}
	plan, err := agent.BuildPlan(cmd.Context(), opts, firstNonEmpty(opts.AuthMethod, "sf_fallthrough"), executePath, planFlags.humanSummary)
	if err != nil {
		return err
	}
	return writePlanOutput(cmd, plan, planFlags.output)
}

func configureAgentPlanOptions(cmd *cobra.Command, flags *rootFlags, useMock bool, includeAudit bool) (agent.WriteOptions, func(), error) {
	trustCleanup, err := configureTrustMock(useMock)
	if err != nil {
		return agent.WriteOptions{}, nil, err
	}
	cleanup := trustCleanup
	c, err := flags.newClient()
	if err != nil {
		cleanup()
		return agent.WriteOptions{}, nil, err
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
		cleanup()
		return agent.WriteOptions{}, nil, fmt.Errorf("load signer for org=%s: %w (hint: run 'trust register --org %s')", orgAlias, err, orgAlias)
	}
	if err := ensureLocalPlanKeyRecord(orgAlias, signer); err != nil {
		cleanup()
		return agent.WriteOptions{}, nil, err
	}
	var writer *trust.WriteAuditWriter
	if includeAudit {
		writer = trust.NewWriteAuditWriter(trust.WriteAuditOptions{
			Client:    c,
			DBPath:    defaultDBPath("salesforce-headless-360-pp-cli"),
			HIPAAMode: trust.HIPAAModeFromManifest(".printing-press.json"),
			LogWarn: func(format string, args ...any) {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: "+format+"\n", args...)
			},
		})
		previousCleanup := cleanup
		cleanup = func() {
			writer.Wait()
			previousCleanup()
		}
	}
	var writeFilter security.WriteFilter = security.NewDefaultFilter(security.Options{Client: c, OrgAlias: orgAlias})
	if useMock && authMethod != agent.AuthMethodJWT {
		writeFilter = mockWriteFilter{}
	}
	return agent.WriteOptions{
		Client:                 c,
		Filter:                 writeFilter,
		Signer:                 signer,
		AuditWriter:            writer,
		AuthMethod:             authMethod,
		ApexCompanionInstalled: apexInstalled,
		OrgAlias:               orgAlias,
		OrgID:                  firstNonEmpty(os.Getenv("SF360_ORG_ID"), orgAlias),
		ActingUser:             os.Getenv("SF360_USER_ID"),
		Now:                    time.Now,
		LogWarn: func(format string, args ...any) {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: "+format+"\n", args...)
		},
	}, cleanup, nil
}

func ensureLocalPlanKeyRecord(orgAlias string, signer *trust.FileSigner) error {
	if _, err := trust.LoadKeyRecord(signer.KID()); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("load local key record for kid=%s: %w", signer.KID(), err)
	}
	if err := trust.SaveKeyRecord(trust.KeyRecord{
		KID:          signer.KID(),
		OrgAlias:     orgAlias,
		OrgID:        firstNonEmpty(os.Getenv("SF360_ORG_ID"), orgAlias),
		Algorithm:    "Ed25519",
		PublicKeyPEM: signer.PublicKeyPEM(),
		IssuerUserID: os.Getenv("SF360_USER_ID"),
		RegisteredAt: time.Now().UTC(),
		Source:       "local-generated",
	}); err != nil {
		return fmt.Errorf("save local key record for kid=%s: %w", signer.KID(), err)
	}
	return nil
}

func readPlanFile(path string) (*agent.WritePlan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := schemas.ValidateWritePlan(data); err != nil {
		return nil, err
	}
	var plan agent.WritePlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, err
	}
	return &plan, nil
}

func writePlanOutput(cmd *cobra.Command, plan *agent.WritePlan, output string) error {
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if output == "" || output == "-" {
		_, err = cmd.OutOrStdout().Write(data)
		return err
	}
	if err := os.WriteFile(output, data, 0o600); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", output)
	return nil
}

func parsePlanActivityTimes(startRaw, endRaw string) (time.Time, time.Time, error) {
	var start, end time.Time
	var err error
	if startRaw != "" {
		start, err = time.Parse(time.RFC3339, startRaw)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("INVALID_DATE: --start must be RFC3339")
		}
	}
	if endRaw != "" {
		end, err = time.Parse(time.RFC3339, endRaw)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("INVALID_DATE: --end must be RFC3339")
		}
	}
	return start, end, nil
}
