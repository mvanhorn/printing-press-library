// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cobratree

import (
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"unicode/utf8"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mvanhorn/printing-press-library/library/travel/nap-camp/internal/mcp/bound"
)

const failurePreviewBytes = 4000

// ToolResultFromFailedCLICommand preserves bounded output as partial evidence,
// never a successful MCP result. Oversized or opaque output is a labeled text
// preview; it is not parsed from a truncated JSON prefix.
func ToolResultFromFailedCLICommand(result CLICommandResult, err error) *mcplib.CallToolResult {
	if err == nil {
		return ToolResultFromCLICommand(result)
	}
	stdout := strings.TrimSpace(result.Stdout)
	if stdout == "" {
		return boundedToolResultError(err.Error())
	}
	envelope := map[string]any{"error": failurePreview(err.Error()), "partial": true}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		envelope["exit_code"] = exit.ExitCode()
	}
	captureOverflow := len(result.Stdout) > bound.MaxBytes
	if !captureOverflow && utf8.ValidString(stdout) && json.Valid([]byte(stdout)) {
		envelope["partial_output"] = json.RawMessage(stdout)
		data, marshalErr := json.Marshal(envelope)
		if marshalErr == nil && len(data) <= bound.MaxBytes {
			return mcplib.NewToolResultError(string(data))
		}
		delete(envelope, "partial_output")
	}
	preview := failurePreview(stdout)
	envelope["partial_stdout_preview"] = preview
	envelope["partial_stdout_encoding"] = "utf-8"
	envelope["partial_stdout_is_preview"] = true
	envelope["partial_stdout_truncated"] = captureOverflow || len(strings.ToValidUTF8(stdout, "�")) > failurePreviewBytes
	envelope["partial_stdout_bytes_captured"] = len(result.Stdout)
	data, marshalErr := json.Marshal(envelope)
	if marshalErr != nil {
		return boundedToolResultError(err.Error())
	}
	return mcplib.NewToolResultError(bound.Text(string(data)))
}

func failurePreview(text string) string {
	text = strings.ToValidUTF8(text, "�")
	if len(text) <= failurePreviewBytes {
		return text
	}
	end := failurePreviewBytes
	for end > 0 && !utf8.ValidString(text[:end]) {
		end--
	}
	return text[:end] + "…"
}
