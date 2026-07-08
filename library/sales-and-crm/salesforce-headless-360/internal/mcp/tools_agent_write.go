package mcp

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/agent"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/security"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/store"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/trust"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/schemas"
)

const confirmRequiredMessage = "This tool mutates Salesforce records. Pass confirm: true to proceed."

type agentWriteArgKind string

const (
	argString agentWriteArgKind = "string"
	argNumber agentWriteArgKind = "number"
	argBool   agentWriteArgKind = "boolean"
	argObject agentWriteArgKind = "object"
)

type agentWriteToolArg struct {
	name        string
	kind        agentWriteArgKind
	required    bool
	description string
}

type agentWriteToolSpec struct {
	name        string
	description string
	args        []agentWriteToolArg
	handler     server.ToolHandlerFunc
}

type mcpWriteConfigFunc func(includeAudit bool, useMock bool) (agent.WriteOptions, func(), error)

var configureMCPAgentWriteOptions = defaultConfigureMCPAgentWriteOptions

// RegisterAgentWriteTools adds the v1.1 write, plan, and write-audit tools.
func RegisterAgentWriteTools(s *server.MCPServer) {
	for _, spec := range agentWriteToolSpecs() {
		options := []mcplib.ToolOption{mcplib.WithDescription(spec.description)}
		for _, arg := range spec.args {
			options = append(options, arg.toolOption())
		}
		s.AddTool(mcplib.NewTool(spec.name, options...), spec.handler)
	}
}

func agentWriteToolSpecs() []agentWriteToolSpec {
	return []agentWriteToolSpec{
		{
			name:        "agent_update",
			description: "Patch one Salesforce record with a signed, audited write intent. Requires confirm:true unless dry_run:true.",
			args: append(writePrimitiveArgs(
				agentWriteToolArg{name: "record_id", kind: argString, required: true, description: "Record Id to update"},
				fieldsArg(),
			), updateExtraArgs()...),
			handler: handleAgentWrite("update"),
		},
		{
			name:        "agent_upsert",
			description: "Upsert one Salesforce record by SF360 idempotency key. Requires confirm:true unless dry_run:true.",
			args: writePrimitiveArgs(
				agentWriteToolArg{name: "sobject", kind: argString, required: true, description: "Salesforce object API name"},
				agentWriteToolArg{name: "idempotency_key", kind: argString, required: true, description: "Required idempotency key"},
				fieldsArg(),
			),
			handler: handleAgentWrite("upsert"),
		},
		{
			name:        "agent_create",
			description: "Create one Salesforce record with a signed, audited write intent. Requires confirm:true unless dry_run:true.",
			args: writePrimitiveArgs(
				agentWriteToolArg{name: "sobject", kind: argString, required: true, description: "Salesforce object API name"},
				agentWriteToolArg{name: "idempotency_key", kind: argString, required: true, description: "Required idempotency key"},
				fieldsArg(),
			),
			handler: handleAgentWrite("create"),
		},
		{
			name:        "agent_log_activity",
			description: "Log a completed call/email Task or meeting Event. Requires confirm:true unless dry_run:true.",
			args: writePrimitiveArgs(
				agentWriteToolArg{name: "type", kind: argString, required: true, description: "Activity type: call, email, or meeting"},
				agentWriteToolArg{name: "what", kind: argString, description: "Related WhatId such as an Account or Opportunity"},
				agentWriteToolArg{name: "who", kind: argString, description: "Related WhoId such as a Contact"},
				agentWriteToolArg{name: "subject", kind: argString, required: true, description: "Activity subject"},
				agentWriteToolArg{name: "description", kind: argString, description: "Optional activity description"},
				agentWriteToolArg{name: "duration", kind: argNumber, description: "Optional duration in seconds for Task activities"},
				agentWriteToolArg{name: "start", kind: argString, description: "Meeting start time in RFC3339"},
				agentWriteToolArg{name: "end", kind: argString, description: "Meeting end time in RFC3339"},
				agentWriteToolArg{name: "idempotency_key", kind: argString, required: true, description: "Required idempotency key"},
			),
			handler: handleAgentWrite("log_activity"),
		},
		{
			name:        "agent_advance",
			description: "Advance an Opportunity to a validated stage. Requires confirm:true unless dry_run:true.",
			args: append(writePrimitiveArgs(
				agentWriteToolArg{name: "opp", kind: argString, required: true, description: "Opportunity Id"},
				agentWriteToolArg{name: "stage", kind: argString, required: true, description: "Target Opportunity StageName"},
				agentWriteToolArg{name: "close_date", kind: argString, description: "Optional CloseDate in YYYY-MM-DD"},
				agentWriteToolArg{name: "idempotency_key", kind: argString, description: "Optional idempotency key recorded in audit"},
			), forceStaleArg()),
			handler: handleAgentWrite("advance"),
		},
		{
			name:        "agent_close_case",
			description: "Close a Case with a resolution. Requires confirm:true unless dry_run:true.",
			args: append(writePrimitiveArgs(
				agentWriteToolArg{name: "case", kind: argString, required: true, description: "Case Id"},
				agentWriteToolArg{name: "resolution", kind: argString, required: true, description: "Resolution text"},
				agentWriteToolArg{name: "status", kind: argString, description: "Case status to write (default: Closed)"},
				agentWriteToolArg{name: "idempotency_key", kind: argString, description: "Optional idempotency key recorded in audit"},
			), forceStaleArg()),
			handler: handleAgentWrite("close_case"),
		},
		{
			name:        "agent_note",
			description: "Post a Chatter FeedItem note to one record. Requires confirm:true unless dry_run:true.",
			args: writePrimitiveArgs(
				agentWriteToolArg{name: "entity", kind: argString, required: true, description: "Record Id to receive the Chatter note"},
				agentWriteToolArg{name: "text", kind: argString, required: true, description: "Note body"},
			),
			handler: handleAgentWrite("note"),
		},
		planToolSpec("agent_plan_update", "Plan an audited record update.", "update", append(planArgs(
			agentWriteToolArg{name: "record_id", kind: argString, required: true, description: "Record Id to update"},
			fieldsArg(),
		), updateExtraArgs()...)),
		planToolSpec("agent_plan_upsert", "Plan an upsert by SF360 idempotency key.", "upsert", planArgs(
			agentWriteToolArg{name: "sobject", kind: argString, required: true, description: "Salesforce object API name"},
			agentWriteToolArg{name: "idempotency_key", kind: argString, required: true, description: "Required idempotency key"},
			fieldsArg(),
		)),
		planToolSpec("agent_plan_create", "Plan a record create.", "create", planArgs(
			agentWriteToolArg{name: "sobject", kind: argString, required: true, description: "Salesforce object API name"},
			agentWriteToolArg{name: "idempotency_key", kind: argString, required: true, description: "Required idempotency key"},
			fieldsArg(),
		)),
		planToolSpec("agent_plan_log_activity", "Plan a completed Task or Event.", "log_activity", planArgs(
			agentWriteToolArg{name: "type", kind: argString, required: true, description: "Activity type: call, email, or meeting"},
			agentWriteToolArg{name: "what", kind: argString, description: "Related WhatId such as an Account or Opportunity"},
			agentWriteToolArg{name: "who", kind: argString, description: "Related WhoId such as a Contact"},
			agentWriteToolArg{name: "subject", kind: argString, required: true, description: "Activity subject"},
			agentWriteToolArg{name: "description", kind: argString, description: "Optional activity description"},
			agentWriteToolArg{name: "duration", kind: argNumber, description: "Optional duration in seconds for Task activities"},
			agentWriteToolArg{name: "start", kind: argString, description: "Meeting start time in RFC3339"},
			agentWriteToolArg{name: "end", kind: argString, description: "Meeting end time in RFC3339"},
			agentWriteToolArg{name: "idempotency_key", kind: argString, required: true, description: "Required idempotency key"},
		)),
		planToolSpec("agent_plan_advance", "Plan an Opportunity stage advance.", "advance", append(planArgs(
			agentWriteToolArg{name: "opp", kind: argString, required: true, description: "Opportunity Id"},
			agentWriteToolArg{name: "stage", kind: argString, required: true, description: "Target Opportunity StageName"},
			agentWriteToolArg{name: "close_date", kind: argString, description: "Optional CloseDate in YYYY-MM-DD"},
			agentWriteToolArg{name: "idempotency_key", kind: argString, description: "Optional idempotency key recorded in audit"},
		), forceStaleArg())),
		planToolSpec("agent_plan_close_case", "Plan a Case close.", "close_case", append(planArgs(
			agentWriteToolArg{name: "case", kind: argString, required: true, description: "Case Id"},
			agentWriteToolArg{name: "resolution", kind: argString, required: true, description: "Resolution text"},
			agentWriteToolArg{name: "status", kind: argString, description: "Case status to write (default: Closed)"},
			agentWriteToolArg{name: "idempotency_key", kind: argString, description: "Optional idempotency key recorded in audit"},
		), forceStaleArg())),
		planToolSpec("agent_plan_note", "Plan a Chatter FeedItem note.", "note", planArgs(
			agentWriteToolArg{name: "entity", kind: argString, required: true, description: "Record Id to receive the Chatter note"},
			agentWriteToolArg{name: "text", kind: argString, required: true, description: "Note body"},
		)),
		{
			name:        "agent_sign_plan",
			description: "Append a local countersignature to a write plan. Requires confirm:true.",
			args: []agentWriteToolArg{
				{name: "plan", kind: argObject, required: true, description: "Write plan JSON object or base64-encoded JSON string"},
				{name: "output", kind: argString, description: "Optional output path for the signed plan"},
				{name: "confirm", kind: argBool, description: "Pass true to confirm local plan mutation"},
			},
			handler: handleAgentSignPlan,
		},
		{
			name:        "agent_execute_plan",
			description: "Verify and execute a signed write plan. Requires confirm:true.",
			args: []agentWriteToolArg{
				{name: "plan", kind: argObject, required: true, description: "Write plan JSON object or base64-encoded JSON string"},
				{name: "require_countersignatures", kind: argNumber, description: "Minimum countersignatures required before execution"},
				{name: "force_stale", kind: argBool, description: "Bypass optimistic concurrency"},
				{name: "dry_run", kind: argBool, description: "Preview execution without DML or audit writes"},
				{name: "confirm_bulk", kind: argNumber, description: "Confirm intentional bulk writes by passing the exact record count"},
				{name: "mock", kind: argBool, description: "Run against the in-process Salesforce mock server when available"},
				{name: "confirm", kind: argBool, description: "Pass true to confirm execution"},
			},
			handler: handleAgentExecutePlan,
		},
		{
			name:        "agent_write_audit_list",
			description: "List local signed write audit rows.",
			args: []agentWriteToolArg{
				{name: "since", kind: argString, description: "Only include rows generated at or after this RFC3339 timestamp"},
				{name: "sobject", kind: argString, description: "Filter by Salesforce object API name"},
				{name: "status", kind: argString, description: "Filter by execution status"},
				{name: "kid", kind: argString, description: "Filter by signing key id"},
				{name: "limit", kind: argNumber, description: "Maximum rows to return"},
			},
			handler: handleAgentWriteAuditList,
		},
		{
			name:        "agent_write_audit_inspect",
			description: "Inspect one local write audit row.",
			args: []agentWriteToolArg{
				{name: "jti", kind: argString, required: true, description: "Write intent JTI"},
			},
			handler: handleAgentWriteAuditInspect,
		},
		{
			name:        "agent_write_audit_verify",
			description: "Verify one local write audit row's signed write intent.",
			args: []agentWriteToolArg{
				{name: "jti", kind: argString, required: true, description: "Write intent JTI"},
			},
			handler: handleAgentWriteAuditVerify,
		},
	}
}

func (arg agentWriteToolArg) toolOption() mcplib.ToolOption {
	opts := []mcplib.PropertyOption{mcplib.Description(arg.description)}
	if arg.required {
		opts = append(opts, mcplib.Required())
	}
	switch arg.kind {
	case argBool:
		return mcplib.WithBoolean(arg.name, opts...)
	case argNumber:
		return mcplib.WithNumber(arg.name, opts...)
	case argObject:
		opts = append(opts, mcplib.AdditionalProperties(true))
		return mcplib.WithObject(arg.name, opts...)
	default:
		return mcplib.WithString(arg.name, opts...)
	}
}

func fieldsArg() agentWriteToolArg {
	return agentWriteToolArg{name: "fields", kind: argObject, required: true, description: "Field values to write as a JSON object"}
}

func forceStaleArg() agentWriteToolArg {
	return agentWriteToolArg{name: "force_stale", kind: argBool, description: "Bypass optimistic concurrency"}
}

func updateExtraArgs() []agentWriteToolArg {
	return []agentWriteToolArg{
		{name: "idempotency_key", kind: argString, description: "Optional idempotency key recorded in audit"},
		{name: "if_last_modified", kind: argString, description: "Expected LastModifiedDate in RFC3339"},
		forceStaleArg(),
	}
}

func writePrimitiveArgs(args ...agentWriteToolArg) []agentWriteToolArg {
	common := []agentWriteToolArg{
		{name: "dry_run", kind: argBool, description: "Preview payload without DML or audit writes"},
		{name: "run_as_user", kind: argString, description: "SF User Id required in JWT mode"},
		{name: "confirm_bulk", kind: argNumber, description: "Confirm intentional bulk writes by passing the exact record count"},
		{name: "mock", kind: argBool, description: "Run against the in-process Salesforce mock server when available"},
		{name: "confirm", kind: argBool, description: "Pass true to confirm mutation"},
	}
	return append(args, common...)
}

func planArgs(args ...agentWriteToolArg) []agentWriteToolArg {
	common := []agentWriteToolArg{
		{name: "dry_run", kind: argBool, description: "Persist dry-run execution intent in the plan"},
		{name: "run_as_user", kind: argString, description: "SF User Id required in JWT mode"},
		{name: "output", kind: argString, description: "Optional output path; default returns JSON in the tool response"},
		{name: "expires_in", kind: argString, description: "Plan expiration duration, e.g. 1h"},
		{name: "human_summary", kind: argString, description: "Human-readable plan summary"},
		{name: "mock", kind: argBool, description: "Run plan construction against the in-process Salesforce mock server when available"},
	}
	return append(args, common...)
}

func planToolSpec(name, description, kind string, args []agentWriteToolArg) agentWriteToolSpec {
	return agentWriteToolSpec{name: name, description: description, args: args, handler: handleAgentPlan(kind)}
}

func handleAgentWrite(kind string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		if !boolArg(req.Params.Arguments, "dry_run") && !boolArg(req.Params.Arguments, "confirm") {
			return mcpStructuredError(confirmErrorPayload()), nil
		}
		configured, cleanup, err := configureMCPAgentWriteOptions(true, boolArg(req.Params.Arguments, "mock"))
		if err != nil {
			return mcpError(err), nil
		}
		defer cleanup()

		opts, executor, err := writeOptionsFromMCP(ctx, kind, req.Params.Arguments, configured)
		if err != nil {
			return mcpError(err), nil
		}
		opts = inheritMCPWriteOptions(opts, configured)
		result, err := executor(ctx, opts)
		if err != nil {
			return mcpError(err), nil
		}
		return mcpJSON(agentWriteResultEnvelope(result, opts.OrgID)), nil
	}
}

func handleAgentPlan(kind string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		configured, cleanup, err := configureMCPAgentWriteOptions(false, boolArg(req.Params.Arguments, "mock"))
		if err != nil {
			return mcpError(err), nil
		}
		defer cleanup()

		opts, _, err := writeOptionsFromMCP(ctx, kind, req.Params.Arguments, configured)
		if err != nil {
			return mcpError(err), nil
		}
		opts = inheritMCPWriteOptions(opts, configured)
		if expiresRaw := stringArg(req.Params.Arguments, "expires_in"); expiresRaw != "" {
			expiresIn, err := time.ParseDuration(expiresRaw)
			if err != nil {
				return mcpError(fmt.Errorf("INVALID_DURATION: expires_in must parse as a Go duration: %w", err)), nil
			}
			opts.PlanExpiresIn = expiresIn
		} else {
			opts.PlanExpiresIn = time.Hour
		}
		executePath := "ui_api"
		if opts.AuthMethod == agent.AuthMethodJWT {
			executePath = "apex"
		} else if opts.SObject == "FeedItem" {
			executePath = "chatter"
		}
		plan, err := agent.BuildPlan(ctx, opts, firstNonEmpty(opts.AuthMethod, "sf_fallthrough"), executePath, stringArg(req.Params.Arguments, "human_summary"))
		if err != nil {
			return mcpError(err), nil
		}
		return writeOptionalPlanOutput(plan, stringArg(req.Params.Arguments, "output"))
	}
}

func handleAgentSignPlan(_ context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if !boolArg(req.Params.Arguments, "confirm") {
		return mcpStructuredError(confirmErrorPayload()), nil
	}
	plan, err := planFromMCPArg(req.Params.Arguments["plan"])
	if err != nil {
		return mcpError(err), nil
	}
	orgAlias := firstNonEmpty(os.Getenv("SF360_ORG"), "default")
	signer, err := trust.NewFileSigner(orgAlias)
	if err != nil {
		return mcpError(fmt.Errorf("load signer for org=%s: %w", orgAlias, err)), nil
	}
	if err := ensureMCPLocalPlanKeyRecord(orgAlias, signer); err != nil {
		return mcpError(err), nil
	}
	if err := agent.AppendCountersignature(plan, signer); err != nil {
		return mcpError(err), nil
	}
	return writeOptionalPlanOutput(plan, stringArg(req.Params.Arguments, "output"))
}

func handleAgentExecutePlan(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if !boolArg(req.Params.Arguments, "confirm") {
		return mcpStructuredError(confirmErrorPayload()), nil
	}
	plan, err := planFromMCPArg(req.Params.Arguments["plan"])
	if err != nil {
		return mcpError(err), nil
	}
	configured, cleanup, err := configureMCPAgentWriteOptions(true, boolArg(req.Params.Arguments, "mock"))
	if err != nil {
		return mcpError(err), nil
	}
	defer cleanup()

	result, err := agent.ExecutePlan(ctx, plan, agent.ExecuteOptions{
		MinCountersignatures:   intArg(req.Params.Arguments, "require_countersignatures", 0),
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
		ForceStale:             boolArg(req.Params.Arguments, "force_stale"),
		DryRun:                 boolArg(req.Params.Arguments, "dry_run"),
		ConfirmBulk:            intArg(req.Params.Arguments, "confirm_bulk", 0),
		Now:                    configured.Now,
		LogWarn:                configured.LogWarn,
	})
	if err != nil {
		return mcpError(err), nil
	}
	return mcpJSON(agentWriteResultEnvelope(result, configured.OrgID)), nil
}

func writeOptionsFromMCP(ctx context.Context, kind string, args map[string]any, configured agent.WriteOptions) (agent.WriteOptions, func(context.Context, agent.WriteOptions) (*agent.WriteResult, error), error) {
	switch kind {
	case "update":
		recordID, err := requiredStringArg(args, "record_id")
		if err != nil {
			return agent.WriteOptions{}, nil, err
		}
		fields, err := requiredFieldsArg(args)
		if err != nil {
			return agent.WriteOptions{}, nil, err
		}
		opts := agent.NewUpdateWriteOptions(recordID, fields)
		opts.IdempotencyKey = stringArg(args, "idempotency_key")
		opts.IfLastModified, err = optionalRFC3339(args, "if_last_modified")
		if err != nil {
			return agent.WriteOptions{}, nil, err
		}
		applyCommonWriteArgs(&opts, args)
		return opts, agent.ExecuteWrite, nil
	case "upsert":
		sobject, err := requiredStringArg(args, "sobject")
		if err != nil {
			return agent.WriteOptions{}, nil, err
		}
		idempotencyKey, err := requiredStringArg(args, "idempotency_key")
		if err != nil {
			return agent.WriteOptions{}, nil, err
		}
		fields, err := requiredFieldsArg(args)
		if err != nil {
			return agent.WriteOptions{}, nil, err
		}
		opts := agent.NewUpsertWriteOptions(sobject, idempotencyKey, fields)
		applyCommonWriteArgs(&opts, args)
		return opts, agent.ExecuteWrite, nil
	case "create":
		sobject, err := requiredStringArg(args, "sobject")
		if err != nil {
			return agent.WriteOptions{}, nil, err
		}
		idempotencyKey, err := requiredStringArg(args, "idempotency_key")
		if err != nil {
			return agent.WriteOptions{}, nil, err
		}
		fields, err := requiredFieldsArg(args)
		if err != nil {
			return agent.WriteOptions{}, nil, err
		}
		opts := agent.NewCreateWriteOptions(sobject, idempotencyKey, fields)
		applyCommonWriteArgs(&opts, args)
		return opts, agent.ExecuteWrite, nil
	case "log_activity":
		opts, err := logActivityOptionsFromMCP(args)
		if err != nil {
			return agent.WriteOptions{}, nil, err
		}
		applyCommonWriteArgs(&opts, args)
		return opts, agent.ExecuteWrite, nil
	case "advance":
		opts, err := agent.NewAdvanceWriteOptions(ctx, agent.AdvanceOptions{
			OpportunityID: stringArg(args, "opp"),
			StageName:     stringArg(args, "stage"),
			CloseDate:     stringArg(args, "close_date"),
			Client:        configured.Client,
		})
		if err != nil {
			return agent.WriteOptions{}, nil, err
		}
		opts.IdempotencyKey = stringArg(args, "idempotency_key")
		applyCommonWriteArgs(&opts, args)
		return opts, agent.ExecuteWrite, nil
	case "close_case":
		opts, err := agent.NewCloseCaseWriteOptions(agent.CloseCaseOptions{
			CaseID:     stringArg(args, "case"),
			Resolution: stringArg(args, "resolution"),
			Status:     stringArg(args, "status"),
		})
		if err != nil {
			return agent.WriteOptions{}, nil, err
		}
		opts.IdempotencyKey = stringArg(args, "idempotency_key")
		applyCommonWriteArgs(&opts, args)
		return opts, agent.ExecuteCloseCase, nil
	case "note":
		entity, err := requiredStringArg(args, "entity")
		if err != nil {
			return agent.WriteOptions{}, nil, err
		}
		text, err := requiredStringArg(args, "text")
		if err != nil {
			return agent.WriteOptions{}, nil, err
		}
		opts := agent.NewNoteWriteOptions(entity, text)
		applyCommonWriteArgs(&opts, args)
		return opts, agent.ExecuteNote, nil
	default:
		return agent.WriteOptions{}, nil, fmt.Errorf("unsupported agent write kind %q", kind)
	}
}

func logActivityOptionsFromMCP(args map[string]any) (agent.WriteOptions, error) {
	start, err := optionalRFC3339(args, "start")
	if err != nil {
		return agent.WriteOptions{}, err
	}
	end, err := optionalRFC3339(args, "end")
	if err != nil {
		return agent.WriteOptions{}, err
	}
	return agent.NewLogActivityWriteOptions(agent.LogActivityOptions{
		Type:            stringArg(args, "type"),
		WhatID:          stringArg(args, "what"),
		WhoID:           stringArg(args, "who"),
		Subject:         stringArg(args, "subject"),
		Description:     stringArg(args, "description"),
		DurationSeconds: intArg(args, "duration", 0),
		Start:           start,
		End:             end,
		IdempotencyKey:  stringArg(args, "idempotency_key"),
	})
}

func applyCommonWriteArgs(opts *agent.WriteOptions, args map[string]any) {
	opts.ForceStale = boolArg(args, "force_stale")
	opts.DryRun = boolArg(args, "dry_run")
	opts.RunAsUser = stringArg(args, "run_as_user")
	opts.ConfirmBulk = intArg(args, "confirm_bulk", 0)
}

func defaultConfigureMCPAgentWriteOptions(includeAudit bool, _ bool) (agent.WriteOptions, func(), error) {
	c, err := newMCPClient()
	if err != nil {
		return agent.WriteOptions{}, nil, err
	}
	c.DryRun = false
	orgAlias := firstNonEmpty(os.Getenv("SF360_ORG"), "default")
	authMethod := strings.ToLower(os.Getenv("SF360_AUTH_METHOD"))
	apexInstalled := strings.EqualFold(os.Getenv("SF360_APEX_SAFE_WRITE_INSTALLED"), "true")
	signer, err := trust.NewFileSigner(orgAlias)
	if err != nil {
		return agent.WriteOptions{}, nil, fmt.Errorf("load signer for org=%s: %w (hint: run 'trust register --org %s')", orgAlias, err, orgAlias)
	}
	if err := ensureMCPLocalPlanKeyRecord(orgAlias, signer); err != nil {
		return agent.WriteOptions{}, nil, err
	}
	var writer *trust.WriteAuditWriter
	cleanup := func() {}
	if includeAudit {
		writer = trust.NewWriteAuditWriter(trust.WriteAuditOptions{
			Client:    c,
			DBPath:    dbPath(),
			HIPAAMode: trust.HIPAAModeFromManifest(".printing-press.json"),
			LogWarn:   func(format string, args ...any) { fmt.Fprintf(os.Stderr, "warning: "+format+"\n", args...) },
		})
		cleanup = writer.Wait
	}
	return agent.WriteOptions{
		Client:                 c,
		Filter:                 security.NewDefaultFilter(security.Options{Client: c, OrgAlias: orgAlias}),
		Signer:                 signer,
		AuditWriter:            writer,
		AuthMethod:             authMethod,
		ApexCompanionInstalled: apexInstalled,
		OrgAlias:               orgAlias,
		OrgID:                  firstNonEmpty(os.Getenv("SF360_ORG_ID"), orgAlias),
		ActingUser:             os.Getenv("SF360_USER_ID"),
		Now:                    time.Now,
		LogWarn:                func(format string, args ...any) { fmt.Fprintf(os.Stderr, "warning: "+format+"\n", args...) },
	}, cleanup, nil
}

func inheritMCPWriteOptions(resolved, configured agent.WriteOptions) agent.WriteOptions {
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

func ensureMCPLocalPlanKeyRecord(orgAlias string, signer *trust.FileSigner) error {
	if _, err := trust.LoadKeyRecord(signer.KID()); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("load local key record for kid=%s: %w", signer.KID(), err)
	}
	return trust.SaveKeyRecord(trust.KeyRecord{
		KID:          signer.KID(),
		OrgAlias:     orgAlias,
		OrgID:        firstNonEmpty(os.Getenv("SF360_ORG_ID"), orgAlias),
		Algorithm:    "Ed25519",
		PublicKeyPEM: signer.PublicKeyPEM(),
		IssuerUserID: os.Getenv("SF360_USER_ID"),
		RegisteredAt: time.Now().UTC(),
		Source:       "local-generated",
	})
}

func writeOptionalPlanOutput(plan *agent.WritePlan, output string) (*mcplib.CallToolResult, error) {
	if output == "" || output == "-" {
		return mcpJSON(plan), nil
	}
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return mcpError(err), nil
	}
	data = append(data, '\n')
	if err := os.WriteFile(output, data, 0o600); err != nil {
		return mcpError(err), nil
	}
	return mcpJSON(map[string]any{"output": output, "plan": plan}), nil
}

func planFromMCPArg(value any) (*agent.WritePlan, error) {
	if value == nil {
		return nil, missingArg("plan")
	}
	var raw []byte
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if strings.HasPrefix(trimmed, "{") {
			raw = []byte(trimmed)
			break
		}
		decoded, err := decodePlanString(trimmed)
		if err != nil {
			return nil, fmt.Errorf("INVALID_PLAN: plan must be a JSON object or base64-encoded JSON: %w", err)
		}
		raw = decoded
	default:
		var err error
		raw, err = json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("INVALID_PLAN: marshal plan: %w", err)
		}
	}
	if err := schemas.ValidateWritePlan(raw); err != nil {
		return nil, fmt.Errorf("INVALID_PLAN: %w", err)
	}
	var plan agent.WritePlan
	if err := json.Unmarshal(raw, &plan); err != nil {
		return nil, fmt.Errorf("INVALID_PLAN: %w", err)
	}
	return &plan, nil
}

func decodePlanString(value string) ([]byte, error) {
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	var lastErr error
	for _, enc := range encodings {
		decoded, err := enc.DecodeString(value)
		if err == nil {
			return decoded, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func handleAgentWriteAuditList(_ context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	since, err := optionalRFC3339(req.Params.Arguments, "since")
	if err != nil {
		return mcpError(err), nil
	}
	limit := intArg(req.Params.Arguments, "limit", 50)
	kid := stringArg(req.Params.Arguments, "kid")
	db, err := store.Open(dbPath())
	if err != nil {
		return mcpError(err), nil
	}
	defer db.Close()
	rows, err := db.ListWriteAudit(store.WriteAuditFilter{
		TargetSObject:   strings.TrimSpace(stringArg(req.Params.Arguments, "sobject")),
		ExecutionStatus: strings.TrimSpace(stringArg(req.Params.Arguments, "status")),
		Limit:           writeAuditFetchLimit(limit, !since.IsZero(), kid),
	})
	if err != nil {
		return mcpError(err), nil
	}
	out := make([]store.WriteAuditRow, 0, len(rows))
	for _, row := range rows {
		if kid != "" && row.ActingKID != kid {
			continue
		}
		if !since.IsZero() {
			generatedAt, err := time.Parse(time.RFC3339, row.GeneratedAt)
			if err != nil || generatedAt.Before(since) {
				continue
			}
		}
		out = append(out, row)
		if len(out) >= normalizedLimit(limit) {
			break
		}
	}
	return mcpJSON(out), nil
}

func handleAgentWriteAuditInspect(_ context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	jti, err := requiredStringArg(req.Params.Arguments, "jti")
	if err != nil {
		return mcpError(err), nil
	}
	row, err := mcpWriteAuditRow(jti)
	if err != nil {
		return mcpError(err), nil
	}
	return mcpJSON(row), nil
}

func handleAgentWriteAuditVerify(_ context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	jti, err := requiredStringArg(req.Params.Arguments, "jti")
	if err != nil {
		return mcpError(err), nil
	}
	row, err := mcpWriteAuditRow(jti)
	if err != nil {
		return mcpError(err), nil
	}
	return mcpJSON(verifyMCPWriteAuditRow(row, time.Now().UTC())), nil
}

func mcpWriteAuditRow(jti string) (store.WriteAuditRow, error) {
	db, err := store.Open(dbPath())
	if err != nil {
		return store.WriteAuditRow{}, err
	}
	defer db.Close()
	row, err := db.GetWriteAudit(jti)
	if errors.Is(err, sql.ErrNoRows) {
		return store.WriteAuditRow{}, fmt.Errorf("WRITE_AUDIT_NOT_FOUND: write audit row %q not found", jti)
	}
	return row, err
}

type mcpWriteAuditVerifyResult struct {
	JTI                string `json:"jti"`
	KID                string `json:"kid,omitempty"`
	SignatureValid     bool   `json:"signature_valid"`
	Audience           string `json:"audience,omitempty"`
	AudienceValid      bool   `json:"audience_valid"`
	Expired            bool   `json:"expired"`
	NotExpired         bool   `json:"not_expired"`
	PlanJTI            string `json:"plan_jti,omitempty"`
	PlanSignatureValid *bool  `json:"plan_signature_valid,omitempty"`
	Error              string `json:"error,omitempty"`
}

func verifyMCPWriteAuditRow(row store.WriteAuditRow, now time.Time) mcpWriteAuditVerifyResult {
	result := mcpWriteAuditVerifyResult{JTI: row.JTI, KID: row.ActingKID}
	claims, err := trust.VerifyWriteIntent([]byte(row.IntentJWS))
	if err == nil || errors.Is(err, trust.ErrIntentExpired) || errors.Is(err, trust.ErrWrongAudience) {
		result.SignatureValid = true
		result.Audience = claims.Aud
		result.AudienceValid = claims.Aud == trust.WriteIntentAudience
		result.Expired = claims.Exp <= now.Unix()
		result.NotExpired = !result.Expired
	} else {
		result.Error = err.Error()
	}
	if result.KID == "" {
		if kid, err := trust.ExtractKIDUnsafe(row.IntentJWS); err == nil {
			result.KID = kid
		}
	}
	var fieldDiff map[string]any
	_ = json.Unmarshal([]byte(row.FieldDiff), &fieldDiff)
	if planJTI, _ := fieldDiff["plan_jti"].(string); planJTI != "" {
		result.PlanJTI = planJTI
		valid := result.SignatureValid && planJTI == row.JTI
		result.PlanSignatureValid = &valid
	}
	return result
}

func writeAuditFetchLimit(limit int, hasSince bool, kid string) int {
	limit = normalizedLimit(limit)
	if hasSince || strings.TrimSpace(kid) != "" {
		return max(limit*10, 1000)
	}
	return limit
}

func normalizedLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	return limit
}

func agentWriteResultEnvelope(result *agent.WriteResult, orgID string) map[string]any {
	return map[string]any{
		"success":     true,
		"code":        "",
		"http_status": result.HTTPStatus,
		"stage":       "salesforce_write",
		"org":         orgID,
		"trace_id":    result.JTI,
		"data": map[string]any{
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
		},
	}
}

func mcpError(err error) *mcplib.CallToolResult {
	if err == nil {
		return nil
	}
	var writeErr *agent.WriteError
	if errors.As(err, &writeErr) {
		envelope := writeErr.Envelope
		cause := ""
		if envelope.Cause != nil {
			cause = fmt.Sprintf("%v", envelope.Cause)
		}
		payload := map[string]any{
			"code":        envelope.Code,
			"message":     firstNonEmpty(envelope.Hint, cause, writeErr.Error()),
			"http_status": envelope.HTTPStatus,
			"stage":       envelope.Stage,
			"org":         envelope.Org,
			"trace_id":    envelope.TraceID,
			"hint":        envelope.Hint,
			"cause":       envelope.Cause,
			"data":        envelope.Data,
		}
		return mcpStructuredError(payload)
	}
	code := codeFromError(err)
	status := http.StatusBadRequest
	if code == "INTERNAL_ERROR" {
		status = http.StatusInternalServerError
	}
	return mcpStructuredError(map[string]any{
		"code":        code,
		"message":     err.Error(),
		"http_status": status,
	})
}

func mcpStructuredError(payload map[string]any) *mcplib.CallToolResult {
	data, _ := json.Marshal(payload)
	return mcplib.NewToolResultError(string(data))
}

func confirmErrorPayload() map[string]any {
	return map[string]any{
		"code":        "MCP_CONFIRM_REQUIRED",
		"message":     confirmRequiredMessage,
		"http_status": http.StatusBadRequest,
	}
}

func codeFromError(err error) string {
	msg := err.Error()
	for _, code := range []string{
		"MISSING_REQUIRED_ARG",
		"MISSING_REQUIRED_FLAG",
		"MISSING_RELATED_RECORD",
		"INVALID_TYPE",
		"INVALID_DATE",
		"INVALID_DURATION",
		"INVALID_PLAN",
		"WRITE_AUDIT_NOT_FOUND",
		"INSUFFICIENT_COUNTERSIGNATURES",
		"COUNTERSIGNATURE_INVALID",
		"PLAN_SIGNATURE_INVALID",
		"PLAN_EXPIRED",
	} {
		if strings.Contains(msg, code) {
			if code == "MISSING_REQUIRED_FLAG" {
				return "MISSING_REQUIRED_ARG"
			}
			return code
		}
	}
	return "INTERNAL_ERROR"
}

func mcpJSON(value any) *mcplib.CallToolResult {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return mcpError(err)
	}
	return mcplib.NewToolResultText(string(data))
}

func requiredStringArg(args map[string]any, name string) (string, error) {
	value := strings.TrimSpace(stringArg(args, name))
	if value == "" {
		return "", missingArg(name)
	}
	return value, nil
}

func missingArg(name string) error {
	return fmt.Errorf("MISSING_REQUIRED_ARG: %s is required", name)
}

func requiredFieldsArg(args map[string]any) (map[string]any, error) {
	raw, ok := args["fields"]
	if !ok || raw == nil {
		return nil, missingArg("fields")
	}
	fields, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("INVALID_ARG: fields must be an object")
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("MISSING_REQUIRED_ARG: fields must not be empty")
	}
	return fields, nil
}

func optionalRFC3339(args map[string]any, name string) (time.Time, error) {
	raw := stringArg(args, name)
	if raw == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("INVALID_DATE: %s must be RFC3339: %w", name, err)
	}
	return parsed, nil
}

func stringArg(args map[string]any, name string) string {
	if value, ok := args[name]; ok {
		switch typed := value.(type) {
		case string:
			return typed
		case fmt.Stringer:
			return typed.String()
		default:
			return fmt.Sprintf("%v", value)
		}
	}
	return ""
}

func boolArg(args map[string]any, name string) bool {
	if value, ok := args[name]; ok {
		if typed, ok := value.(bool); ok {
			return typed
		}
	}
	return false
}

func intArg(args map[string]any, name string, fallback int) int {
	value, ok := args[name]
	if !ok {
		return fallback
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		i, err := typed.Int64()
		if err == nil {
			return int(i)
		}
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
