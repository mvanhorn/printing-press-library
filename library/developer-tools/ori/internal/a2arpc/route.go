// Copyright 2026 error. Licensed under Apache-2.0.
// JSON-RPC adapter: the OpenAPI spec describes a REST veneer over the A2A
// server's underlying JSON-RPC protocol. This package translates REST-shaped
// requests into JSON-RPC calls at POST /a2a/{agent}/v1. The client/client.go
// interceptor delegates here for any path matching /a2a/{agent}/{anything}
// except /a2a/{agent}/v1 itself.

package a2arpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Routes reports whether a path matches the REST veneer the adapter should
// intercept. The actual JSON-RPC endpoint (.../v1) is not intercepted —
// it passes through to plain HTTP transport for forward compatibility.
func Routes(path string) bool {
	if !strings.HasPrefix(path, "/a2a/") {
		return false
	}
	if strings.HasSuffix(path, "/v1") {
		return false
	}
	return true
}

// Route translates REST → JSON-RPC and returns the JSON-RPC `result` field
// as if it were a plain HTTP response. Errors are surfaced as if they were
// HTTP errors so the caller's existing classifyAPIError logic still works.
func Route(httpClient *http.Client, baseURL, method, path string, params map[string]string, body []byte, authHeader string) (json.RawMessage, int, error) {
	parts := strings.Split(strings.TrimPrefix(path, "/a2a/"), "/")
	if len(parts) < 2 {
		return nil, 0, fmt.Errorf("a2a path missing agent name: %s", path)
	}
	agent := parts[0]
	rest := parts[1:]

	rpcMethod, rpcParams, err := mapRESTtoRPC(method, rest, params, body)
	if err != nil {
		return nil, 404, err
	}
	if rpcMethod == "" {
		return nil, 404, fmt.Errorf("no JSON-RPC mapping for %s %s", method, path)
	}

	envelope := map[string]any{
		"jsonrpc": "2.0",
		"id":      fmt.Sprintf("ori-%d", time.Now().UnixNano()),
		"method":  rpcMethod,
		"params":  rpcParams,
	}
	envBytes, mErr := json.Marshal(envelope)
	if mErr != nil {
		return nil, 0, fmt.Errorf("a2a rpc marshal: %w", mErr)
	}

	rpcURL := strings.TrimRight(baseURL, "/") + "/a2a/" + agent + "/v1"
	req, rErr := http.NewRequest("POST", rpcURL, bytes.NewReader(envBytes))
	if rErr != nil {
		return nil, 0, fmt.Errorf("a2a rpc request: %w", rErr)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("a2a-version", "1.0")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, dErr := httpClient.Do(req)
	if dErr != nil {
		return nil, 0, fmt.Errorf("a2a rpc transport: %w", dErr)
	}
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return respBytes, resp.StatusCode, fmt.Errorf("a2a rpc HTTP %d: %s", resp.StatusCode, truncate(string(respBytes), 200))
	}

	var rpcResp struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      any             `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   *rpcError       `json:"error,omitempty"`
	}
	if jErr := json.Unmarshal(respBytes, &rpcResp); jErr != nil {
		return respBytes, resp.StatusCode, fmt.Errorf("a2a rpc parse: %w", jErr)
	}
	if rpcResp.Error != nil {
		// JSON-RPC application error; map to a synthetic non-2xx so the caller's
		// classifyAPIError path treats it as an API error rather than a network failure.
		return respBytes, 502, fmt.Errorf("a2a rpc method=%s code=%d: %s", rpcMethod, rpcResp.Error.Code, rpcResp.Error.Message)
	}
	if len(rpcResp.Result) == 0 {
		// Successful call with empty result — return an empty JSON object so
		// downstream JSON-output paths don't break on nil.
		return json.RawMessage(`{}`), resp.StatusCode, nil
	}
	return rpcResp.Result, resp.StatusCode, nil
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func mapRESTtoRPC(method string, rest []string, params map[string]string, body []byte) (string, map[string]any, error) {
	// /a2a/{agent}/tasks
	if len(rest) == 1 && rest[0] == "tasks" {
		switch method {
		case "GET":
			p := map[string]any{}
			if v := params["page_size"]; v != "" {
				if n, ok := atoi(v); ok {
					p["pageSize"] = n
				}
			} else {
				p["pageSize"] = 50
			}
			if v := params["context_id"]; v != "" {
				p["contextId"] = v
			}
			if v := params["page_token"]; v != "" {
				p["pageToken"] = v
			}
			return "ListTasks", p, nil
		case "POST":
			// Wait is *bool so we can distinguish "explicitly false" from "omitted"
			// — the documented default is true (block until terminal state).
			var sendReq struct {
				Message   string `json:"message"`
				ContextID string `json:"context_id,omitempty"`
				Wait      *bool  `json:"wait,omitempty"`
			}
			if len(body) > 0 {
				if err := json.Unmarshal(body, &sendReq); err != nil {
					return "", nil, fmt.Errorf("SendMessage body parse: %w", err)
				}
			}
			if strings.TrimSpace(sendReq.Message) == "" {
				return "", nil, fmt.Errorf("SendMessage requires 'message' field in body")
			}
			// A2A `lf.a2a.v1.Message` JSON shape: messageId, role (ROLE_USER /
			// ROLE_AGENT enum string), parts (text parts are oneof, serialized
			// as `{text: ...}` with no discriminator), contextId/taskId optional.
			// Confirmed from the server's own error response listing available fields.
			//
			// When the client omits context_id, mint a fresh UUID rather than
			// letting the server fall back to its shared "main" context — that
			// fallback pools every no-context call into one polluted session.
			ctxID := sendReq.ContextID
			if ctxID == "" {
				ctxID = uuid.NewString()
			}
			msg := map[string]any{
				"messageId": uuid.NewString(),
				"role":      "ROLE_USER",
				"contextId": ctxID,
				"parts": []map[string]any{
					{"text": sendReq.Message},
				},
			}
			// lf.a2a.v1.SendMessageConfiguration uses `returnImmediately` (the
			// inverse of "wait"), confirmed from the server's own error response
			// listing available fields: acceptedOutputModes, taskPushNotificationConfig,
			// historyLength, returnImmediately. Default wait=true → returnImmediately=false
			// (block until terminal state) when the caller didn't say otherwise.
			wait := true
			if sendReq.Wait != nil {
				wait = *sendReq.Wait
			}
			return "SendMessage", map[string]any{
				"message":       msg,
				"configuration": map[string]any{"returnImmediately": !wait},
			}, nil
		}
	}
	// /a2a/{agent}/tasks/<id>
	// A2A protobuf GetTaskRequest / CancelTaskRequest field set is {tenant, id,
	// historyLength}. contextId is intentionally not accepted by these request
	// types — task ids are globally unique, no disambiguation needed.
	if len(rest) == 2 && rest[0] == "tasks" {
		taskID := rest[1]
		switch method {
		case "GET":
			p := map[string]any{"id": taskID}
			if v := params["history_length"]; v != "" {
				if n, ok := atoi(v); ok {
					p["historyLength"] = n
				}
			}
			return "GetTask", p, nil
		case "DELETE":
			return "CancelTask", map[string]any{"id": taskID}, nil
		}
	}
	// /a2a/{agent}/tasks/<id>/resume → GetTask (snapshot of current state +
	// accumulated artifacts). The A2A protocol's `SubscribeToTask` is a
	// server-streaming RPC that emits SSE chunks, which a single-response MCP
	// tool / sync CLI can't consume — the JSON parser chokes on the `data: ...`
	// stream prefix. For the realistic "did this task finish, what did it
	// produce" use case, GetTask provides the same observable result without
	// the streaming-protocol mismatch. Callers that need wait-until-terminal
	// semantics should poll GetTask, or use SendMessage with wait=true.
	if len(rest) == 3 && rest[0] == "tasks" && rest[2] == "resume" {
		taskID := rest[1]
		if method == "POST" {
			return "GetTask", map[string]any{"id": taskID}, nil
		}
	}
	// /a2a/{agent}/approvals* → no JSON-RPC mapping. Approvals are bridge-side
	// state, not part of the A2A protocol. Return a structured error the user
	// can read.
	if len(rest) > 0 && rest[0] == "approvals" {
		return "", nil, fmt.Errorf("the A2A server does not expose approvals over the protocol; they are bridge-side state. See README \"Known Gaps\".")
	}

	return "", nil, fmt.Errorf("no JSON-RPC mapping for %s /%s", method, strings.Join(rest, "/"))
}

func atoi(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return n, true
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
