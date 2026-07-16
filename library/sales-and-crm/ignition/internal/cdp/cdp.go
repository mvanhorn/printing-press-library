package cdp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/ignition/internal/cliutil"
)

const requestTimeout = 30 * time.Second

// apiLimiter paces the in-page GraphQL fetches PostGraphQL evaluates inside
// the authed browser tab. The CDP transport short-circuits the generated
// client's limiter and 429-retry loop (client.go returns the CDP result before
// that loop runs), so throttling protection for the real go.ignitionapp.com
// calls lives here. Auto mode: conservative 2 req/s start; the adaptive
// 429-probe raises or lowers it from observed responses.
var apiLimiter = cliutil.NewAdaptiveLimiterAuto(2.0)

// rateLimitBodyPreview bounds how much of a 429 response body is carried into
// the typed error, keeping messages readable without dropping the evidence.
const rateLimitBodyPreview = 200

var defaultPorts = []int{18800, 9223}

type versionResponse struct {
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

type target struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	URL  string `json:"url"`
}

type protocolError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type protocolMessage struct {
	ID        int             `json:"id"`
	Method    string          `json:"method"`
	Result    json.RawMessage `json:"result"`
	Params    json.RawMessage `json:"params"`
	Error     *protocolError  `json:"error"`
	SessionID string          `json:"sessionId"`
}

// PostGraphQL executes an existing GraphQL request inside an authenticated
// Ignition page. The same-origin page fetch supplies the browser session and a
// fresh CSRF token without replaying browser credentials in the CLI process.
func PostGraphQL(ctx context.Context, path string, bodyBytes []byte) (json.RawMessage, int, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	ports, err := configuredPorts()
	if err != nil {
		return nil, 0, err
	}

	port, browserWebSocketURL, ok := discoverBrowser(ctx, ports)
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	if !ok {
		return nil, 0, fmt.Errorf("no CDP-enabled Chrome on ports %s; open your authed Ignition tab in that Chrome", formatPorts(ports))
	}

	var targets []target
	if err := getJSON(ctx, "http://127.0.0.1:"+strconv.Itoa(port)+"/json", &targets); err != nil {
		return nil, 0, fmt.Errorf("listing CDP targets on port %d: %w", port, err)
	}

	var pageTarget target
	for _, candidate := range targets {
		if candidate.Type == "page" && strings.Contains(candidate.URL, "ignitionapp.com") {
			pageTarget = candidate
			break
		}
	}
	if pageTarget.ID == "" {
		return nil, 0, errors.New("no authed Ignition tab open (open https://go.ignitionapp.com in the CDP Chrome and sign in)")
	}

	dialer := websocket.Dialer{HandshakeTimeout: requestTimeout}
	conn, _, err := dialer.DialContext(ctx, browserWebSocketURL, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("dialing CDP browser websocket: %w", err)
	}
	defer conn.Close()

	deadline, _ := ctx.Deadline()
	if err := conn.SetReadDeadline(deadline); err != nil {
		return nil, 0, fmt.Errorf("setting CDP read deadline: %w", err)
	}
	if err := conn.SetWriteDeadline(deadline); err != nil {
		return nil, 0, fmt.Errorf("setting CDP write deadline: %w", err)
	}

	stopCancellationWatch := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-stopCancellationWatch:
		}
	}()
	defer close(stopCancellationWatch)

	nextID := 0
	attachID := nextProtocolID(&nextID)
	if err := conn.WriteJSON(map[string]any{
		"id":     attachID,
		"method": "Target.attachToTarget",
		"params": map[string]any{
			"targetId": pageTarget.ID,
			"flatten":  true,
		},
	}); err != nil {
		return nil, 0, contextAwareError(ctx, "sending Target.attachToTarget", err)
	}

	sessionID, err := waitForSession(ctx, conn, attachID, pageTarget.ID)
	if err != nil {
		return nil, 0, err
	}

	expression, err := graphqlExpression(path, bodyBytes)
	if err != nil {
		return nil, 0, err
	}
	// Pace the real API call. The in-page fetch hits go.ignitionapp.com with
	// the browser session; without this the CDP transport would issue requests
	// as fast as callers loop (e.g. the search-index pagination in the novel
	// analytics commands).
	apiLimiter.Wait()
	evaluateID := nextProtocolID(&nextID)
	if err := conn.WriteJSON(map[string]any{
		"id":        evaluateID,
		"method":    "Runtime.evaluate",
		"sessionId": sessionID,
		"params": map[string]any{
			"expression":    expression,
			"awaitPromise":  true,
			"returnByValue": true,
		},
	}); err != nil {
		return nil, 0, contextAwareError(ctx, "sending Runtime.evaluate", err)
	}

	raw, status, err := waitForEvaluation(ctx, conn, evaluateID)
	if err != nil {
		return nil, 0, err
	}
	if status == http.StatusTooManyRequests {
		// Typed 429: without this, throttled responses would flow onward as
		// GraphQL payloads and decode into empty results, silently swallowing
		// the throttle signal.
		apiLimiter.OnRateLimit()
		return nil, status, &cliutil.RateLimitError{
			URL:  path,
			Body: truncateForError(string(raw), rateLimitBodyPreview),
		}
	}
	apiLimiter.OnSuccess()
	return raw, status, nil
}

// truncateForError bounds a response body for inclusion in an error message.
func truncateForError(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func configuredPorts() ([]int, error) {
	raw := strings.TrimSpace(os.Getenv("IGNITION_CDP_PORTS"))
	if raw == "" {
		return append([]int(nil), defaultPorts...), nil
	}

	parts := strings.Split(raw, ",")
	ports := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		port, err := strconv.Atoi(part)
		if err != nil || port < 1 || port > 65535 {
			return nil, fmt.Errorf("invalid IGNITION_CDP_PORTS port %q", part)
		}
		ports = append(ports, port)
	}
	return ports, nil
}

func formatPorts(ports []int) string {
	values := make([]string, 0, len(ports))
	for _, port := range ports {
		values = append(values, strconv.Itoa(port))
	}
	return strings.Join(values, ",")
}

func discoverBrowser(ctx context.Context, ports []int) (int, string, bool) {
	for _, port := range ports {
		var version versionResponse
		url := "http://127.0.0.1:" + strconv.Itoa(port) + "/json/version"
		if err := getJSON(ctx, url, &version); err != nil || version.WebSocketDebuggerURL == "" {
			continue
		}
		return port, version.WebSocketDebuggerURL, true
	}
	return 0, "", false
}

func getJSON(ctx context.Context, url string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("GET %s returned %s", url, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("decoding %s: %w", url, err)
	}
	return nil
}

func nextProtocolID(id *int) int {
	(*id)++
	return *id
}

func waitForSession(ctx context.Context, conn *websocket.Conn, attachID int, targetID string) (string, error) {
	for {
		msg, err := readProtocolMessage(ctx, conn)
		if err != nil {
			return "", contextAwareError(ctx, "waiting for Target.attachToTarget", err)
		}

		if msg.ID == attachID {
			if msg.Error != nil {
				return "", protocolResponseError("Target.attachToTarget", msg.Error)
			}
			var result struct {
				SessionID string `json:"sessionId"`
			}
			if err := json.Unmarshal(msg.Result, &result); err != nil {
				return "", fmt.Errorf("parsing Target.attachToTarget result: %w", err)
			}
			if result.SessionID != "" {
				return result.SessionID, nil
			}
		}

		if msg.Method == "Target.attachedToTarget" {
			var params struct {
				SessionID  string `json:"sessionId"`
				TargetInfo struct {
					TargetID string `json:"targetId"`
				} `json:"targetInfo"`
			}
			if err := json.Unmarshal(msg.Params, &params); err != nil {
				return "", fmt.Errorf("parsing Target.attachedToTarget event: %w", err)
			}
			if params.SessionID != "" && (params.TargetInfo.TargetID == "" || params.TargetInfo.TargetID == targetID) {
				return params.SessionID, nil
			}
		}
	}
}

func graphqlExpression(path string, bodyBytes []byte) (string, error) {
	pathJSON, err := json.Marshal(path)
	if err != nil {
		return "", fmt.Errorf("encoding GraphQL path: %w", err)
	}
	bodyJSON, err := json.Marshal(string(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("encoding GraphQL body: %w", err)
	}

	return `(async () => {
  const csrf = document.querySelector('meta[name=csrf-token]')?.content || '';
  const resp = await fetch(location.origin + ` + string(pathJSON) + `, {
    method: 'POST', credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf,
               'accept': 'application/graphql-response+json,application/json;q=0.9' },
    body: ` + string(bodyJSON) + `
  });
  const text = await resp.text();
  return JSON.stringify({ status: resp.status, body: text });
})()`, nil
}

func waitForEvaluation(ctx context.Context, conn *websocket.Conn, evaluateID int) (json.RawMessage, int, error) {
	for {
		msg, err := readProtocolMessage(ctx, conn)
		if err != nil {
			return nil, 0, contextAwareError(ctx, "waiting for Runtime.evaluate", err)
		}
		if msg.ID != evaluateID {
			continue
		}
		if msg.Error != nil {
			return nil, 0, protocolResponseError("Runtime.evaluate", msg.Error)
		}

		var evaluation struct {
			Result struct {
				Value json.RawMessage `json:"value"`
			} `json:"result"`
			ExceptionDetails json.RawMessage `json:"exceptionDetails"`
		}
		if err := json.Unmarshal(msg.Result, &evaluation); err != nil {
			return nil, 0, fmt.Errorf("parsing Runtime.evaluate result: %w", err)
		}
		if rawJSONPresent(evaluation.ExceptionDetails) {
			return nil, 0, fmt.Errorf("Runtime.evaluate exceptionDetails: %s", evaluation.ExceptionDetails)
		}

		var value string
		if err := json.Unmarshal(evaluation.Result.Value, &value); err != nil {
			return nil, 0, fmt.Errorf("parsing Runtime.evaluate return value: %w", err)
		}
		var response struct {
			Status int    `json:"status"`
			Body   string `json:"body"`
		}
		if err := json.Unmarshal([]byte(value), &response); err != nil {
			return nil, 0, fmt.Errorf("parsing in-page GraphQL response: %w", err)
		}
		return json.RawMessage([]byte(response.Body)), response.Status, nil
	}
}

func readProtocolMessage(ctx context.Context, conn *websocket.Conn) (protocolMessage, error) {
	var msg protocolMessage
	if err := conn.ReadJSON(&msg); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return protocolMessage{}, ctxErr
		}
		return protocolMessage{}, err
	}
	return msg, nil
}

func protocolResponseError(method string, protocolErr *protocolError) error {
	return fmt.Errorf("%s CDP error %d: %s", method, protocolErr.Code, protocolErr.Message)
}

func contextAwareError(ctx context.Context, operation string, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func rawJSONPresent(value json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(value))
	return trimmed != "" && trimmed != "null"
}
