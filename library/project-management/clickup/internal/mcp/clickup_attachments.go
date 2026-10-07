// Hand-authored. Do not regenerate over this file with `printing-press generate`
// without merging — it owns the multipart upload handlers for the typed
// `task_attachment_create-task` and `workspaces_attachments_post-entity` tools.
//
// PATCH(multipart-attachment-upload): the generated handlers sent the upload
// as a JSON body, which ClickUp rejects. These handlers take a local
// `file_path` and POST it as multipart/form-data via client.PostMultipart.

package mcp

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/mvanhorn/printing-press-library/library/project-management/clickup/internal/client"
)

// multipartUploadSpec describes how MCP arguments map onto one upload.
type multipartUploadSpec struct {
	PathTemplate string
	PathParams   []string          // required path placeholders, by MCP arg name
	PathDefaults map[string]string // default values for optional path params
	QueryParams  []string          // MCP args forwarded as query params (same wire name)
	FormFields   []string          // MCP args forwarded as extra form fields (same wire name)
}

func makeMultipartUploadHandler(spec multipartUploadSpec) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		args := req.GetArguments()
		str := func(k string) string {
			v, ok := args[k]
			if !ok || v == nil {
				return ""
			}
			return strings.TrimSpace(fmt.Sprintf("%v", v))
		}

		filePath := str("file_path")
		if filePath == "" {
			return mcplib.NewToolResultError("file_path is required: absolute path to a local file to upload"), nil
		}
		upload, err := client.StatUploadFile(filePath)
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}

		path := spec.PathTemplate
		for _, p := range spec.PathParams {
			v := str(p)
			if v == "" {
				v = spec.PathDefaults[p]
			}
			if v == "" {
				return mcplib.NewToolResultError(p + " is required"), nil
			}
			path = strings.Replace(path, "{"+p+"}", escapePathSegment(v), 1)
		}
		params := map[string]string{}
		for _, q := range spec.QueryParams {
			if v := str(q); v != "" {
				params[q] = v
			}
		}
		fields := map[string]string{}
		for _, f := range spec.FormFields {
			if v := str(f); v != "" {
				fields[f] = v
			}
		}

		c, err := newMCPClient()
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		data, _, err := c.PostMultipart(path, params, "attachment", upload, fields)
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		return mcplib.NewToolResultText(string(data)), nil
	}
}

// escapePathSegment percent-encodes a path parameter as a single URL segment,
// matching the CLI's replacePathParam (PATCH path-parameters-percent-encoded).
func escapePathSegment(v string) string {
	if v == "." || v == ".." {
		return strings.Repeat("%2E", len(v))
	}
	return url.PathEscape(v)
}
