// Copyright 2026 riteshtiwari. Licensed under Apache-2.0. See LICENSE.

// Package mcp provides the code-orchestration thin surface for posthog-pp-cli.
// With 1600+ endpoint tools the full mirror is too large for agent context windows.
// This thin surface exposes two tools — posthog_search and posthog_execute — so
// the agent can discover and run any operation without loading the full catalog.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterCodeOrchestrationTools adds the two-tool thin surface to s.
// The agent calls posthog_search first to discover available operations,
// then posthog_execute to run one.
func RegisterCodeOrchestrationTools(s *server.MCPServer) {
	s.AddTool(
		mcplib.NewTool("posthog_search",
			mcplib.WithDescription("Search PostHog API operations by keyword. Returns matching endpoint names, HTTP methods, and short descriptions. Call this before posthog_execute to discover the right operation name."),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(false),
			mcplib.WithString("query",
				mcplib.Required(),
				mcplib.Description("Keyword to search for (e.g. 'feature flag', 'experiment', 'dashboard')"),
			),
			mcplib.WithNumber("limit",
				mcplib.Description("Max results to return (default 10)"),
			),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			args := req.GetArguments()
			query, _ := args["query"].(string)
			if query == "" {
				return mcplib.NewToolResultError("query is required"), nil
			}
			limit := 10
			if v, ok := args["limit"].(float64); ok && v > 0 {
				limit = int(v)
			}
			q := strings.ToLower(query)
			type match struct {
				Name        string `json:"name"`
				Method      string `json:"method"`
				Path        string `json:"path"`
				Description string `json:"description"`
			}
			var results []match
			for _, ep := range codeOrchEndpoints {
				if len(results) >= limit {
					break
				}
				if strings.Contains(strings.ToLower(ep.Name), q) ||
					strings.Contains(strings.ToLower(ep.Description), q) ||
					strings.Contains(strings.ToLower(ep.Path), q) {
					results = append(results, match{
						Name:        ep.Name,
						Method:      ep.Method,
						Path:        ep.Path,
						Description: ep.Description,
					})
				}
			}
			if len(results) == 0 {
				return mcplib.NewToolResultText(fmt.Sprintf("No operations found matching %q. Try broader terms.", query)), nil
			}
			b, _ := json.Marshal(results)
			return mcplib.NewToolResultText(string(b)), nil
		},
	)

	s.AddTool(
		mcplib.NewTool("posthog_execute",
			mcplib.WithDescription("Execute a PostHog API operation by name. Use posthog_search first to find the operation name. Params are passed as key-value pairs matching the operation's path and query parameters."),
			mcplib.WithOpenWorldHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithString("operation",
				mcplib.Required(),
				mcplib.Description("Operation name from posthog_search results (e.g. 'projects_feature-flags-list')"),
			),
			mcplib.WithObject("params",
				mcplib.Description("Parameters for the operation as key-value pairs (path params, query params, body fields)"),
			),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			args := req.GetArguments()
			operation, _ := args["operation"].(string)
			if operation == "" {
				return mcplib.NewToolResultError("operation is required"), nil
			}

			var ep *codeOrchEndpoint
			for i := range codeOrchEndpoints {
				if codeOrchEndpoints[i].Name == operation {
					ep = &codeOrchEndpoints[i]
					break
				}
			}
			if ep == nil {
				return mcplib.NewToolResultError(fmt.Sprintf("unknown operation %q — use posthog_search to find valid names", operation)), nil
			}

			c, err := newMCPClient()
			if err != nil {
				return mcplib.NewToolResultError("config error: " + err.Error()), nil
			}

			queryParams := map[string]string{}
			if p, ok := args["params"].(map[string]any); ok {
				for k, v := range p {
					queryParams[k] = fmt.Sprintf("%v", v)
				}
			}

			// Substitute path parameters.
			path := ep.Path
			for k, v := range queryParams {
				placeholder := "{" + k + "}"
				if strings.Contains(path, placeholder) {
					path = strings.ReplaceAll(path, placeholder, v)
					delete(queryParams, k)
				}
			}

			var data json.RawMessage
			switch ep.Method {
			case "GET":
				data, err = c.Get(path, queryParams)
			case "DELETE":
				data, _, err = c.Delete(path)
			case "POST":
				data, _, err = c.Post(path, queryParams)
			case "PUT":
				data, _, err = c.Put(path, queryParams)
			case "PATCH":
				data, _, err = c.Patch(path, queryParams)
			default:
				return mcplib.NewToolResultError("unsupported method: " + ep.Method), nil
			}
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}
			return mcplib.NewToolResultText(string(data)), nil
		},
	)
}

type codeOrchEndpoint struct {
	Name        string
	Method      string
	Path        string
	Description string
}

// codeOrchEndpoints is the catalog of all PostHog API operations available
// via posthog_execute. Generated from the OpenAPI spec.
var codeOrchEndpoints = []codeOrchEndpoint{
	{"code_invites-check-access-retrieve", "GET", "/api/code/invites/check-access/", "Check whether the authenticated user has access to PostHog Code."},
	{"code_invites-redeem-create", "POST", "/api/code/invites/redeem/", "Redeem a PostHog Code invite code to enable access."},
	{"organizations_list", "GET", "/api/organizations/", "List all organizations the authenticated user belongs to."},
	{"organizations_create", "POST", "/api/organizations/", "Create a new organization."},
	{"organizations_retrieve", "GET", "/api/organizations/{id}/", "Retrieve a specific organization by ID."},
	{"organizations_update", "PUT", "/api/organizations/{id}/", "Replace an organization's settings."},
	{"organizations_partial-update", "PATCH", "/api/organizations/{id}/", "Update one or more organization fields."},
	{"organizations_destroy", "DELETE", "/api/organizations/{id}/", "Delete an organization."},
	{"organizations_projects-list", "GET", "/api/organizations/{organization_id}/projects/", "List all projects in an organization."},
	{"organizations_members-list", "GET", "/api/organizations/{organization_id}/members/", "List members of an organization."},
	{"organizations_invites-list", "GET", "/api/organizations/{organization_id}/invites/", "List pending invites for an organization."},
	{"organizations_invites-create", "POST", "/api/organizations/{organization_id}/invites/", "Invite a new member to an organization."},
	{"organizations_invites-destroy", "DELETE", "/api/organizations/{organization_id}/invites/{id}/", "Revoke a pending invite."},
	{"organizations_roles-list", "GET", "/api/organizations/{organization_id}/roles/", "List roles in an organization."},
	{"organizations_roles-create", "POST", "/api/organizations/{organization_id}/roles/", "Create a new role."},
	{"organizations_roles-retrieve", "GET", "/api/organizations/{organization_id}/roles/{id}/", "Retrieve a role by ID."},
	{"organizations_roles-update", "PUT", "/api/organizations/{organization_id}/roles/{id}/", "Replace a role's configuration."},
	{"organizations_roles-destroy", "DELETE", "/api/organizations/{organization_id}/roles/{id}/", "Delete a role."},
	{"projects_list", "GET", "/api/projects/", "List all projects accessible to the authenticated user."},
	{"projects_create", "POST", "/api/projects/", "Create a new project."},
	{"projects_retrieve", "GET", "/api/projects/{project_id}/", "Retrieve a specific project by ID."},
	{"projects_update", "PUT", "/api/projects/{project_id}/", "Replace a project's settings."},
	{"projects_partial-update", "PATCH", "/api/projects/{project_id}/", "Update one or more project fields."},
	{"projects_destroy", "DELETE", "/api/projects/{project_id}/", "Delete a project."},
	{"projects_feature-flags-list", "GET", "/api/projects/{project_id}/feature_flags/", "List all feature flags in a project."},
	{"projects_feature-flags-retrieve", "GET", "/api/projects/{project_id}/feature_flags/{id}/", "Retrieve a feature flag by ID."},
	{"projects_feature-flags-create", "POST", "/api/projects/{project_id}/feature_flags/", "Create a new feature flag."},
	{"projects_feature-flags-update", "PUT", "/api/projects/{project_id}/feature_flags/{id}/", "Replace a feature flag's configuration."},
	{"projects_feature-flags-partial-update", "PATCH", "/api/projects/{project_id}/feature_flags/{id}/", "Update one or more feature flag fields."},
	{"projects_feature-flags-destroy", "DELETE", "/api/projects/{project_id}/feature_flags/{id}/", "Archive a feature flag."},
	{"projects_experiments-list", "GET", "/api/projects/{project_id}/experiments/", "List all experiments in a project."},
	{"projects_experiments-retrieve", "GET", "/api/projects/{project_id}/experiments/{id}/", "Retrieve an experiment by ID."},
	{"projects_experiments-create", "POST", "/api/projects/{project_id}/experiments/", "Create a new experiment."},
	{"projects_experiments-update", "PUT", "/api/projects/{project_id}/experiments/{id}/", "Replace an experiment's configuration."},
	{"projects_experiments-partial-update", "PATCH", "/api/projects/{project_id}/experiments/{id}/", "Update one or more experiment fields."},
	{"projects_experiments-destroy", "DELETE", "/api/projects/{project_id}/experiments/{id}/", "Delete an experiment."},
	{"projects_dashboards-list", "GET", "/api/projects/{project_id}/dashboards/", "List all dashboards in a project."},
	{"projects_dashboards-retrieve", "GET", "/api/projects/{project_id}/dashboards/{id}/", "Retrieve a dashboard by ID."},
	{"projects_dashboards-create", "POST", "/api/projects/{project_id}/dashboards/", "Create a new dashboard."},
	{"projects_dashboards-update", "PUT", "/api/projects/{project_id}/dashboards/{id}/", "Replace a dashboard's configuration."},
	{"projects_dashboards-partial-update", "PATCH", "/api/projects/{project_id}/dashboards/{id}/", "Update one or more dashboard fields."},
	{"projects_dashboards-destroy", "DELETE", "/api/projects/{project_id}/dashboards/{id}/", "Delete a dashboard."},
	{"projects_insights-list", "GET", "/api/projects/{project_id}/insights/", "List all insights in a project."},
	{"projects_insights-retrieve", "GET", "/api/projects/{project_id}/insights/{id}/", "Retrieve an insight by ID."},
	{"projects_insights-create", "POST", "/api/projects/{project_id}/insights/", "Create a new insight."},
	{"projects_insights-update", "PUT", "/api/projects/{project_id}/insights/{id}/", "Replace an insight's configuration."},
	{"projects_insights-partial-update", "PATCH", "/api/projects/{project_id}/insights/{id}/", "Update one or more insight fields."},
	{"projects_insights-destroy", "DELETE", "/api/projects/{project_id}/insights/{id}/", "Delete an insight."},
	{"projects_surveys-list", "GET", "/api/projects/{project_id}/surveys/", "List all surveys in a project."},
	{"projects_surveys-retrieve", "GET", "/api/projects/{project_id}/surveys/{id}/", "Retrieve a survey by ID."},
	{"projects_surveys-create", "POST", "/api/projects/{project_id}/surveys/", "Create a new survey."},
	{"projects_surveys-destroy", "DELETE", "/api/projects/{project_id}/surveys/{id}/", "Delete a survey."},
	{"projects_cohorts-list", "GET", "/api/projects/{project_id}/cohorts/", "List all cohorts in a project."},
	{"projects_cohorts-retrieve", "GET", "/api/projects/{project_id}/cohorts/{id}/", "Retrieve a cohort by ID."},
	{"projects_cohorts-create", "POST", "/api/projects/{project_id}/cohorts/", "Create a new cohort."},
	{"projects_cohorts-update", "PUT", "/api/projects/{project_id}/cohorts/{id}/", "Replace a cohort's definition."},
	{"projects_cohorts-destroy", "DELETE", "/api/projects/{project_id}/cohorts/{id}/", "Delete a cohort."},
	{"projects_persons-list", "GET", "/api/projects/{project_id}/persons/", "List persons (users) tracked in a project."},
	{"projects_persons-retrieve", "GET", "/api/projects/{project_id}/persons/{id}/", "Retrieve a person by ID."},
	{"projects_persons-destroy", "DELETE", "/api/projects/{project_id}/persons/{id}/", "Delete a person and their events."},
	{"projects_error-tracking-list", "GET", "/api/projects/{project_id}/error_tracking/", "List error tracking issues."},
	{"projects_error-tracking-retrieve", "GET", "/api/projects/{project_id}/error_tracking/{id}/", "Retrieve an error tracking issue by ID."},
	{"projects_annotations-list", "GET", "/api/projects/{project_id}/annotations/", "List annotations for a project."},
	{"projects_annotations-create", "POST", "/api/projects/{project_id}/annotations/", "Create a new annotation."},
	{"projects_annotations-destroy", "DELETE", "/api/projects/{project_id}/annotations/{id}/", "Delete an annotation."},
	{"projects_hog-functions-list", "GET", "/api/projects/{project_id}/hog_functions/", "List Hog (pipeline) functions."},
	{"projects_hog-functions-retrieve", "GET", "/api/projects/{project_id}/hog_functions/{id}/", "Retrieve a Hog function by ID."},
	{"projects_hog-functions-create", "POST", "/api/projects/{project_id}/hog_functions/", "Create a new Hog function."},
	{"projects_hog-functions-destroy", "DELETE", "/api/projects/{project_id}/hog_functions/{id}/", "Delete a Hog function."},
	{"environments_events-list", "GET", "/api/environments/{project_id}/events/", "List events with optional filters."},
	{"environments_events-retrieve", "GET", "/api/environments/{project_id}/events/{id}/", "Retrieve a specific event by ID."},
	{"environments_actions-list", "GET", "/api/environments/{project_id}/actions/", "List actions (event filters)."},
	{"environments_actions-retrieve", "GET", "/api/environments/{project_id}/actions/{id}/", "Retrieve an action by ID."},
	{"environments_actions-create", "POST", "/api/environments/{project_id}/actions/", "Create a new action."},
	{"environments_actions-destroy", "DELETE", "/api/environments/{project_id}/actions/{id}/", "Delete an action."},
	{"environments_property-definitions-list", "GET", "/api/environments/{project_id}/property_definitions/", "List property definitions for a project."},
	{"environments_event-definitions-list", "GET", "/api/environments/{project_id}/event_definitions/", "List event definitions."},
	{"environments_feature-flags-list", "GET", "/api/environments/{project_id}/feature_flags/", "List feature flags scoped to an environment."},
	{"environments_recordings-list", "GET", "/api/environments/{project_id}/session_recordings/", "List session recordings."},
	{"environments_llm-traces-list", "GET", "/api/environments/{project_id}/llm_analytics/trace_reviews/", "List LLM generation traces for observability."},
	{"users_list", "GET", "/api/users/", "List users in the organization."},
	{"users_retrieve", "GET", "/api/users/{id}/", "Retrieve a user by ID or @me for the current user."},
	{"users_partial-update", "PATCH", "/api/users/{id}/", "Update one or more user profile fields."},
	{"users_destroy", "DELETE", "/api/users/{id}/", "Delete a user account."},
	{"user-home-settings_retrieve", "GET", "/api/user_home_settings/@me/", "Get pinned sidebar tabs and homepage for the current user."},
	{"user-home-settings_partial-update", "PATCH", "/api/user_home_settings/@me/", "Update pinned sidebar tabs and/or homepage."},
	{"public-hog-function-templates_list", "GET", "/api/public_hog_function_templates/", "List all available public Hog function templates."},
}
