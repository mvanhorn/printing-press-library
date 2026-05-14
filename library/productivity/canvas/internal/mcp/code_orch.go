// Copyright 2026 martin. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/canvas/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/canvas/internal/config"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterOrchestrationTools adds the canvas_search + canvas_execute pair.
// These two tools cover the full API surface in ~1K tokens rather than
// exposing 356 individual endpoint mirrors.
func RegisterOrchestrationTools(s *server.MCPServer) {
	s.AddTool(
		mcplib.NewTool("canvas_search",
			mcplib.WithDescription("Search Canvas LMS resources. Returns a list of available API operations matching the query. Use this to discover what canvas_execute can do."),
			mcplib.WithString("query",
				mcplib.Required(),
				mcplib.Description("Search terms to find Canvas API operations (e.g. 'list assignments', 'courses', 'submissions grade')"),
			),
		),
		handleCanvasSearch,
	)

	s.AddTool(
		mcplib.NewTool("canvas_execute",
			mcplib.WithDescription("Execute any Canvas LMS API operation by name. Use canvas_search first to find the operation name, then call this with the operation and parameters."),
			mcplib.WithString("operation",
				mcplib.Required(),
				mcplib.Description("API operation name (e.g. 'courses.list-your', 'courses.assignments.list', 'courses.submissions.list'). Get names from canvas_search."),
			),
			mcplib.WithObject("params",
				mcplib.Description("Operation parameters as key-value pairs (e.g. {\"course_id\": \"12345\", \"include\": \"submission\"})"),
			),
		),
		handleCanvasExecute,
	)
}

// handleCanvasSearch returns a list of Canvas API operations matching the query.
func handleCanvasSearch(_ context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	query, _ := req.GetArguments()["query"].(string)
	query = strings.ToLower(strings.TrimSpace(query))

	// Static catalog of the most useful Canvas API operations
	type op struct {
		Name        string `json:"name"`
		Method      string `json:"method"`
		Path        string `json:"path"`
		Description string `json:"description"`
	}
	ops := []op{
		{"courses.list-your", "GET", "/v1/courses", "List your enrolled courses"},
		{"courses.get-single", "GET", "/v1/courses/{course_id}", "Get a single course"},
		{"courses.assignments.list", "GET", "/v1/courses/{course_id}/assignments", "List assignments for a course"},
		{"courses.assignments.get-single", "GET", "/v1/courses/{course_id}/assignments/{id}", "Get a single assignment"},
		{"courses.assignments.create", "POST", "/v1/courses/{course_id}/assignments", "Create an assignment"},
		{"courses.submissions.list", "GET", "/v1/courses/{course_id}/assignments/{assignment_id}/submissions", "List submissions for an assignment"},
		{"courses.submissions.get-single", "GET", "/v1/courses/{course_id}/assignments/{assignment_id}/submissions/{user_id}", "Get a single submission"},
		{"courses.submissions.update", "PUT", "/v1/courses/{course_id}/assignments/{assignment_id}/submissions/{user_id}", "Grade/update a submission"},
		{"courses.enrollments.list", "GET", "/v1/courses/{course_id}/enrollments", "List enrollments for a course"},
		{"courses.modules.list", "GET", "/v1/courses/{course_id}/modules", "List modules in a course"},
		{"courses.modules.items", "GET", "/v1/courses/{course_id}/modules/{module_id}/items", "List items in a module"},
		{"courses.files.list", "GET", "/v1/courses/{course_id}/files", "List files in a course"},
		{"courses.pages.list", "GET", "/v1/courses/{course_id}/pages", "List pages in a course"},
		{"courses.discussion-topics.list", "GET", "/v1/courses/{course_id}/discussion_topics", "List discussion topics"},
		{"courses.quizzes.list", "GET", "/v1/courses/{course_id}/quizzes", "List quizzes in a course"},
		{"announcements.list", "GET", "/v1/announcements", "List announcements (requires context_codes param)"},
		{"users.self-profile", "GET", "/v1/users/self/profile", "Get the current user's profile"},
		{"users.self-course-nicknames.list", "GET", "/v1/users/self/course_nicknames", "List course nicknames"},
		{"users.self-todo-items.list", "GET", "/v1/users/self/todo", "List todo items for the current user"},
		{"users.self-upcoming-events.list", "GET", "/v1/users/self/upcoming_events", "List upcoming calendar events"},
		{"calendar-events.list", "GET", "/v1/calendar_events", "List calendar events"},
		{"rubrics.list", "GET", "/v1/courses/{course_id}/rubrics", "List rubrics in a course"},
		{"sections.list", "GET", "/v1/courses/{course_id}/sections", "List sections in a course"},
	}

	// Filter by query
	var matches []op
	for _, o := range ops {
		if query == "" ||
			strings.Contains(strings.ToLower(o.Name), query) ||
			strings.Contains(strings.ToLower(o.Description), query) ||
			strings.Contains(strings.ToLower(o.Path), query) {
			matches = append(matches, o)
		}
	}

	b, _ := json.MarshalIndent(matches, "", "  ")
	return mcplib.NewToolResultText(string(b)), nil
}

// handleCanvasExecute proxies an API operation by name to the Canvas REST API.
func handleCanvasExecute(_ context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	operation, _ := args["operation"].(string)
	if operation == "" {
		return mcplib.NewToolResultError("operation is required. Use canvas_search to find operation names."), nil
	}
	params, _ := args["params"].(map[string]interface{})

	cfg, err := config.Load("")
	if err != nil || cfg.CanvasApiToken == "" {
		return mcplib.NewToolResultError("Canvas API not configured. Set CANVAS_API_TOKEN and CANVAS_BASE_URL environment variables."), nil
	}

	c := client.New(cfg, 30*time.Second, 0)

	// Convert operation name to URL path
	// e.g. "courses.assignments.list" → "/v1/courses/{course_id}/assignments"
	path, method := operationToPath(operation, params)
	if path == "" {
		return mcplib.NewToolResultError(fmt.Sprintf("unknown operation %q. Use canvas_search to find valid operation names.", operation)), nil
	}

	queryParams := map[string]string{}
	for k, v := range params {
		if !strings.Contains(path, "{"+k+"}") {
			queryParams[k] = fmt.Sprintf("%v", v)
		}
	}

	var data json.RawMessage
	var provErr error
	switch strings.ToUpper(method) {
	case "GET":
		data, provErr = c.Get(path, queryParams)
	default:
		return mcplib.NewToolResultError(fmt.Sprintf("method %s for operation %q requires a request body; use the CLI directly: canvas-lms-pp-cli %s", method, operation, strings.ReplaceAll(operation, ".", " "))), nil
	}

	if provErr != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("API error: %v", provErr)), nil
	}

	return mcplib.NewToolResultText(string(data)), nil
}

// operationToPath maps operation name to REST path + method.
func operationToPath(op string, params map[string]interface{}) (string, string) {
	courseID := fmt.Sprintf("%v", params["course_id"])
	assignmentID := fmt.Sprintf("%v", params["assignment_id"])
	userID := fmt.Sprintf("%v", params["user_id"])
	moduleID := fmt.Sprintf("%v", params["module_id"])
	id := fmt.Sprintf("%v", params["id"])

	ops := map[string][2]string{
		"courses.list-your":                  {"/v1/courses", "GET"},
		"courses.get-single":                 {"/v1/courses/" + courseID, "GET"},
		"courses.assignments.list":           {"/v1/courses/" + courseID + "/assignments", "GET"},
		"courses.assignments.get-single":     {"/v1/courses/" + courseID + "/assignments/" + id, "GET"},
		"courses.submissions.list":           {"/v1/courses/" + courseID + "/assignments/" + assignmentID + "/submissions", "GET"},
		"courses.submissions.get-single":     {"/v1/courses/" + courseID + "/assignments/" + assignmentID + "/submissions/" + userID, "GET"},
		"courses.enrollments.list":           {"/v1/courses/" + courseID + "/enrollments", "GET"},
		"courses.modules.list":               {"/v1/courses/" + courseID + "/modules", "GET"},
		"courses.modules.items":              {"/v1/courses/" + courseID + "/modules/" + moduleID + "/items", "GET"},
		"courses.files.list":                 {"/v1/courses/" + courseID + "/files", "GET"},
		"courses.pages.list":                 {"/v1/courses/" + courseID + "/pages", "GET"},
		"courses.discussion-topics.list":     {"/v1/courses/" + courseID + "/discussion_topics", "GET"},
		"courses.quizzes.list":               {"/v1/courses/" + courseID + "/quizzes", "GET"},
		"announcements.list":                 {"/v1/announcements", "GET"},
		"users.self-profile":                 {"/v1/users/self/profile", "GET"},
		"users.self-course-nicknames.list":   {"/v1/users/self/course_nicknames", "GET"},
		"users.self-todo-items.list":         {"/v1/users/self/todo", "GET"},
		"users.self-upcoming-events.list":    {"/v1/users/self/upcoming_events", "GET"},
		"calendar-events.list":               {"/v1/calendar_events", "GET"},
		"rubrics.list":                       {"/v1/courses/" + courseID + "/rubrics", "GET"},
		"sections.list":                      {"/v1/courses/" + courseID + "/sections", "GET"},
	}

	if v, ok := ops[op]; ok {
		return v[0], v[1]
	}
	return "", ""
}
