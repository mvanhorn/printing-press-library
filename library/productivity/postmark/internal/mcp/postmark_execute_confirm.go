// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"encoding/json"
	"net/url"
	"path"
	"regexp"
	"strings"
	"sync"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

// postmarkConfirmEndpoints run through postmark_execute only with
// confirm: true, in addition to every DELETE. Each one destroys, overwrites,
// or cannot undo data. Email sends need no entry: the client transport refuses
// them unless the CLI's --send gate allowed the request.
var postmarkConfirmEndpoints = map[string]string{
	"data_removals.create":           "permanently erases the recipient's data from Postmark",
	"streams.archive":                "stops the message stream, and Postmark deletes archived streams after a grace period",
	"suppressions.delete":            "lets mail reach addresses that were suppressed after bounces, complaints, or unsubscribes",
	"templates.push-between-servers": "overwrites matching templates on the destination server",
}

// postmarkExecuteConfirmReason reports why ep needs explicit confirmation.
func postmarkExecuteConfirmReason(ep *codeOrchEndpoint) (string, bool) {
	if ep.Method == "DELETE" {
		return "deletes the resource", true
	}
	reason, ok := postmarkConfirmEndpoints[ep.ID]
	return reason, ok
}

type postmarkConfirmPath struct {
	method string
	re     *regexp.Regexp
	reason string
}

var (
	postmarkConfirmPathsOnce sync.Once
	postmarkConfirmPaths     []postmarkConfirmPath
)

// confirmPaths turns the policy endpoints' path templates into patterns, so an
// endpoint whose path parameter resolves onto one of them (templates.update
// with the alias "push" lands on PUT /templates/push) is gated as well.
func confirmPaths() []postmarkConfirmPath {
	postmarkConfirmPathsOnce.Do(func() {
		for id, reason := range postmarkConfirmEndpoints {
			ep := findCodeOrchEndpoint(id)
			if ep == nil {
				continue
			}
			segments := strings.Split(ep.Path, "/")
			for i, seg := range segments {
				if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
					segments[i] = `[^/]+`
				} else {
					segments[i] = regexp.QuoteMeta(seg)
				}
			}
			postmarkConfirmPaths = append(postmarkConfirmPaths, postmarkConfirmPath{
				method: ep.Method,
				re:     regexp.MustCompile("(?i)^" + strings.Join(segments, "/") + "$"),
				reason: reason,
			})
		}
	})
	return postmarkConfirmPaths
}

// postmarkExecuteResolvedGate checks the request about to be sent: the
// method and the resolved path, raw and decoded, against the policy paths.
func postmarkExecuteResolvedGate(ep *codeOrchEndpoint, resolved string, args, params map[string]any) *mcplib.CallToolResult {
	if _, needs := postmarkExecuteConfirmReason(ep); needs {
		return nil // already gated by endpoint before the path was resolved
	}
	raw, _, _ := strings.Cut(resolved, "?")
	candidates := []string{raw}
	if decoded, err := url.PathUnescape(raw); err == nil {
		candidates = append(candidates, path.Clean(decoded))
	}
	for _, cp := range confirmPaths() {
		if cp.method != ep.Method {
			continue
		}
		for _, candidate := range candidates {
			if cp.re.MatchString(candidate) {
				return postmarkConfirmPreview(ep, args, params, cp.reason, raw)
			}
		}
	}
	return nil
}

// postmarkExecuteConfirmGate returns a preview, without calling the API, when
// ep needs confirmation and the call does not carry a top-level JSON boolean
// confirm: true. A confirm key inside params does not count.
func postmarkExecuteConfirmGate(ep *codeOrchEndpoint, args, params map[string]any) *mcplib.CallToolResult {
	reason, needs := postmarkExecuteConfirmReason(ep)
	if !needs {
		return nil
	}
	return postmarkConfirmPreview(ep, args, params, reason, "")
}

// postmarkConfirmPreview returns nil when args carries a top-level JSON
// boolean confirm: true, and otherwise the preview that replaces the call.
func postmarkConfirmPreview(ep *codeOrchEndpoint, args, params map[string]any, reason, resolvedPath string) *mcplib.CallToolResult {
	if confirmed, ok := args["confirm"].(bool); ok && confirmed {
		return nil
	}
	fields := map[string]any{
		"status":      "confirmation_required",
		"endpoint_id": ep.ID,
		"method":      ep.Method,
		"path":        ep.Path,
		"params":      params,
		"reason":      "This call " + reason + ".",
		"next":        "Nothing was sent. Show this to the user; after they approve, call postmark_execute again with the same arguments plus confirm: true.",
	}
	if resolvedPath != "" {
		fields["resolved_path"] = resolvedPath
	}
	preview, _ := json.Marshal(fields)
	return mcplib.NewToolResultError(string(preview))
}
