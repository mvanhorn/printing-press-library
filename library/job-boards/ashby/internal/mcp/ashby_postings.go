// Copyright 2026 klubieniecki and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/mvanhorn/printing-press-library/library/job-boards/ashby/internal/cli"
	"github.com/mvanhorn/printing-press-library/library/job-boards/ashby/internal/cliutil"
)

func makeAshbyPublicJobPostingsHandler() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		c, platformSession, err := newMCPClient(ctx)
		if err != nil {
			return mcpToolError(err.Error()), nil
		}
		if platformSession != nil {
			defer platformSession.ZeroCredentials()
		}
		args := req.GetArguments()
		if err := cli.AdoptMCPOutputSemantics(platformSession, args); err != nil {
			return mcpToolError(err.Error()), nil
		}
		board, _ := args["jobBoardName"].(string)
		board = strings.TrimSpace(board)
		if board == "" || strings.ContainsAny(board, "/?#") {
			return mcpToolError("jobBoardName must be a valid Ashby board slug"), nil
		}
		params := map[string]string{}
		if include, _ := args["includeCompensation"].(bool); include {
			params["includeCompensation"] = "true"
		}
		data, err := c.Get(ctx, "/posting-api/job-board/"+cliutil.EscapePathParam(board), params)
		if err != nil {
			return mcpToolError(err.Error()), nil
		}
		filtered, err := filterListedAshbyResponse(data)
		if err != nil {
			return mcpToolError(err.Error()), nil
		}
		return mcpToolResultTextWithPlatform("GET", filtered, platformSession), nil
	}
}

func filterListedAshbyResponse(data json.RawMessage) (json.RawMessage, error) {
	var response struct {
		APIVersion string            `json:"apiVersion"`
		Jobs       []json.RawMessage `json:"jobs"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	if response.Jobs == nil {
		return nil, errors.New("Ashby response is missing jobs")
	}
	listed := make([]json.RawMessage, 0, len(response.Jobs))
	for _, job := range response.Jobs {
		var visibility struct {
			IsListed bool `json:"isListed"`
		}
		if err := json.Unmarshal(job, &visibility); err != nil {
			return nil, err
		}
		if visibility.IsListed {
			listed = append(listed, job)
		}
	}
	return json.Marshal(map[string]any{"apiVersion": response.APIVersion, "jobs": listed})
}
