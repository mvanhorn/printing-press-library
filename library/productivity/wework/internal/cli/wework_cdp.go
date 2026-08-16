// Copyright 2026 Paul Byrne and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored (markerless) CDP live-read: reads the CURRENT in-memory WeWork
// auth0 session (access token + refresh token + uuid + member type) from a
// Chrome-family browser running with --remote-debugging-port, via the Chrome
// DevTools Protocol. Unlike the on-disk scan, this gets the LIVE token (Chrome
// flushes to disk lazily), so an agent runtime that keeps a logged-in Chrome
// with a debug port can seed the CLI autonomously.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type cdpTarget struct {
	Type                 string `json:"type"`
	URL                  string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

type weworkSession struct {
	Token   string `json:"token"`
	Refresh string `json:"refresh"`
	UUID    string `json:"uuid"`
	Member  string `json:"member"`
	Error   string `json:"error"`
}

// jsEvalExpr reads the WeWork auth0 cache + account keys from localStorage and
// returns them as a JSON string.
const jsEvalExpr = `(function(){try{` +
	`var k=Object.keys(localStorage).find(function(x){return /@@auth0spajs@@/.test(x)&&/openid/.test(x)});` +
	`if(!k)return JSON.stringify({error:'no WeWork session in this tab'});` +
	`var c=JSON.parse(localStorage.getItem(k));` +
	`return JSON.stringify({token:(c.body&&c.body.access_token)||'',refresh:(c.body&&c.body.refresh_token)||'',` +
	`uuid:(localStorage.getItem('CurrentAccountUUID')||'').replace(/"/g,''),` +
	`member:(localStorage.getItem('WWMemberType')||'').replace(/"/g,'')});` +
	`}catch(e){return JSON.stringify({error:String(e)})}})()`

// readChromeSessionCDP connects to a Chrome debug port, finds the
// members.wework.com page, and reads the live session.
func readChromeSessionCDP(port int) (weworkSession, error) {
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	targets, err := cdpListTargets(base)
	if err != nil {
		return weworkSession{}, fmt.Errorf("connecting to Chrome debug port %d: %w (start Chrome with --remote-debugging-port=%d)", port, err, port)
	}
	wsURL := pickWeworkTarget(targets)
	if wsURL == "" {
		return weworkSession{}, fmt.Errorf("no members.wework.com tab found on the debug Chrome (open and log in to members.wework.com there)")
	}
	raw, err := cdpEvaluate(wsURL, jsEvalExpr)
	if err != nil {
		return weworkSession{}, err
	}
	sess, err := parseWeworkSession(raw)
	if err != nil {
		return weworkSession{}, err
	}
	if sess.Error != "" {
		return sess, fmt.Errorf("reading session from Chrome: %s", sess.Error)
	}
	if sess.Token == "" {
		return sess, fmt.Errorf("no access token found in the members.wework.com tab (are you logged in?)")
	}
	return sess, nil
}

func cdpListTargets(base string) ([]cdpTarget, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/json", nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var targets []cdpTarget
	if err := json.Unmarshal(body, &targets); err != nil {
		return nil, fmt.Errorf("parsing target list: %w", err)
	}
	return targets, nil
}

// pickWeworkTarget returns the WebSocket debugger URL of a members.wework.com
// page target, or "".
func pickWeworkTarget(targets []cdpTarget) string {
	for _, t := range targets {
		if t.Type == "page" && strings.Contains(t.URL, "members.wework.com") && t.WebSocketDebuggerURL != "" {
			return t.WebSocketDebuggerURL
		}
	}
	return ""
}

func cdpEvaluate(wsURL, expr string) (string, error) {
	dialer := websocket.Dialer{HandshakeTimeout: 8 * time.Second}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		return "", fmt.Errorf("opening CDP websocket: %w", err)
	}
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(8 * time.Second))
	req := map[string]any{
		"id":     1,
		"method": "Runtime.evaluate",
		"params": map[string]any{"expression": expr, "returnByValue": true, "awaitPromise": false},
	}
	if err := conn.WriteJSON(req); err != nil {
		return "", fmt.Errorf("sending CDP eval: %w", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return "", fmt.Errorf("reading CDP response: %w", err)
		}
		val, done, perr := parseCDPEvalMessage(data)
		if perr != nil {
			return "", perr
		}
		if done {
			return val, nil
		}
	}
}

// parseCDPEvalMessage inspects one CDP message; returns (value, true, nil) when
// it is the id:1 Runtime.evaluate result, otherwise (—, false, nil) to keep
// reading. Returns an error if the evaluation reported an exception.
func parseCDPEvalMessage(data []byte) (string, bool, error) {
	var msg struct {
		ID     int `json:"id"`
		Result struct {
			Result struct {
				Value string `json:"value"`
			} `json:"result"`
			ExceptionDetails *struct {
				Text string `json:"text"`
			} `json:"exceptionDetails"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return "", false, nil // ignore non-JSON / event frames
	}
	if msg.ID != 1 {
		return "", false, nil
	}
	if msg.Result.ExceptionDetails != nil {
		return "", true, fmt.Errorf("CDP evaluation error: %s", msg.Result.ExceptionDetails.Text)
	}
	return msg.Result.Result.Value, true, nil
}

func parseWeworkSession(raw string) (weworkSession, error) {
	var s weworkSession
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return weworkSession{}, fmt.Errorf("parsing session JSON: %w", err)
	}
	return s, nil
}
