package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type facebookPageContextTokens struct {
	FBDTSG   string `json:"fb_dtsg"`
	LSD      string `json:"lsd"`
	Jazoest  string `json:"jazoest"`
	Revision string `json:"revision"`
}

type cdpClient struct {
	conn   *websocket.Conn
	nextID int
}

func (c *Client) extractFacebookPageContextTokens(_ string) (facebookPageContextTokens, error) {
	if tokens, err := extractFacebookTokensViaRemoteChrome(c.BaseURL + "/marketplace/"); err == nil && tokens.FBDTSG != "" {
		return tokens, nil
	}
	if tokens, err := extractFacebookTokensViaChromeAppleScript(c.BaseURL + "/marketplace/"); err == nil && tokens.FBDTSG != "" {
		return tokens, nil
	}
	return facebookPageContextTokens{}, fmt.Errorf("fb_dtsg not found in existing Chrome page context")
}

func extractFacebookTokensViaRemoteChrome(targetURL string) (facebookPageContextTokens, error) {
	var lastErr error
	for _, port := range []string{"9222", "9229"} {
		tokens, err := extractFacebookTokensFromCDPTarget(port, targetURL)
		if err == nil && tokens.FBDTSG != "" {
			return tokens, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return facebookPageContextTokens{}, lastErr
	}
	return facebookPageContextTokens{}, fmt.Errorf("remote Chrome DevTools port not available")
}

func extractFacebookTokensFromCDPTarget(port, targetURL string) (facebookPageContextTokens, error) {
	pageWS, err := createCDPTarget(port, targetURL)
	if err != nil {
		return facebookPageContextTokens{}, err
	}
	conn, _, err := websocket.DefaultDialer.Dial(pageWS, nil)
	if err != nil {
		return facebookPageContextTokens{}, fmt.Errorf("connecting to Chrome DevTools: %w", err)
	}
	defer conn.Close()
	cdp := &cdpClient{conn: conn}
	if _, err := cdp.call("Page.enable", nil); err != nil {
		return facebookPageContextTokens{}, err
	}
	if _, err := cdp.call("Runtime.enable", nil); err != nil {
		return facebookPageContextTokens{}, err
	}
	if _, err := cdp.call("Page.navigate", map[string]any{"url": targetURL}); err != nil {
		return facebookPageContextTokens{}, err
	}
	return pollFacebookPageTokens(cdp)
}

func (c *Client) postFacebookGraphQLViaBrowserContext(fields url.Values, headers map[string]string) ([]byte, int, error) {
	var lastErr error
	for _, port := range []string{"9222", "9229"} {
		body, status, err := c.postFacebookGraphQLViaCDP(port, fields, headers)
		if err == nil {
			return body, status, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, 0, lastErr
	}
	return nil, 0, fmt.Errorf("remote Chrome DevTools port not available")
}

func (c *Client) postFacebookGraphQLViaCDP(port string, fields url.Values, headers map[string]string) ([]byte, int, error) {
	pageWS, err := createCDPTarget(port, c.BaseURL+"/marketplace/")
	if err != nil {
		return nil, 0, err
	}
	conn, _, err := websocket.DefaultDialer.Dial(pageWS, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("connecting to Chrome DevTools: %w", err)
	}
	defer conn.Close()
	cdp := &cdpClient{conn: conn}
	if _, err := cdp.call("Page.enable", nil); err != nil {
		return nil, 0, err
	}
	if _, err := cdp.call("Runtime.enable", nil); err != nil {
		return nil, 0, err
	}
	if _, err := cdp.call("Page.navigate", map[string]any{"url": c.BaseURL + "/marketplace/"}); err != nil {
		return nil, 0, err
	}
	if _, err := pollFacebookPageTokens(cdp); err != nil {
		return nil, 0, err
	}

	fieldMap := make(map[string]string, len(fields))
	for key := range fields {
		fieldMap[key] = fields.Get(key)
	}
	headerMap := map[string]string{}
	for key, value := range headers {
		if strings.EqualFold(key, "Cookie") {
			continue
		}
		headerMap[key] = value
	}
	fieldJSON, err := json.Marshal(fieldMap)
	if err != nil {
		return nil, 0, err
	}
	headerJSON, err := json.Marshal(headerMap)
	if err != nil {
		return nil, 0, err
	}
	expression := fmt.Sprintf(`(async () => {
  const fields = %s;
  const headers = %s;
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(fields)) params.set(key, value);
  const response = await fetch("/api/graphql/", {
    method: "POST",
    credentials: "include",
    headers: Object.assign({"Content-Type": "application/x-www-form-urlencoded"}, headers),
    body: params.toString()
  });
  return JSON.stringify({status: response.status, body: await response.text()});
})()`, string(fieldJSON), string(headerJSON))
	result, err := cdp.evaluateString(expression, true)
	if err != nil {
		return nil, 0, err
	}
	var envelope struct {
		Status int    `json:"status"`
		Body   string `json:"body"`
	}
	if err := json.Unmarshal([]byte(result), &envelope); err != nil {
		return nil, 0, err
	}
	return []byte(envelope.Body), envelope.Status, nil
}

func extractFacebookTokensViaChromeAppleScript(targetURL string) (facebookPageContextTokens, error) {
	if runtime.GOOS != "darwin" {
		return facebookPageContextTokens{}, fmt.Errorf("Chrome AppleScript page-context extraction requires macOS")
	}
	script := fmt.Sprintf(`
tell application "Google Chrome"
  if (count of windows) = 0 then make new window
  set URL of active tab of front window to %q
  delay 3
  execute front window's active tab javascript %q
end tell
`, targetURL, "JSON.stringify("+facebookPageTokenExpression()+")")
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		return facebookPageContextTokens{}, err
	}
	var tokens facebookPageContextTokens
	if err := json.Unmarshal(bytes.TrimSpace(out), &tokens); err != nil {
		return facebookPageContextTokens{}, err
	}
	if tokens.FBDTSG == "" {
		return facebookPageContextTokens{}, fmt.Errorf("fb_dtsg not found in Chrome page context")
	}
	return tokens, nil
}

func pollFacebookPageTokens(cdp *cdpClient) (facebookPageContextTokens, error) {
	expression := facebookPageTokenExpression()
	for attempt := 0; attempt < 15; attempt++ {
		tokens, err := cdp.evaluateTokens(expression)
		if err == nil && tokens.FBDTSG != "" {
			return tokens, nil
		}
		time.Sleep(time.Second)
	}
	return facebookPageContextTokens{}, fmt.Errorf("fb_dtsg not found in browser page context")
}

func facebookPageTokenExpression() string {
	return `(() => {
  const html = document.documentElement.innerHTML;
  const getModule = (name) => {
    try {
      return window.require ? window.require(name) : null;
    } catch (_) {
      return null;
    }
  };
  const dtsg = getModule("DTSGInitialData") || getModule("DTSG") || {};
  const lsd = getModule("LSD") || {};
  const siteData = getModule("SiteData") || {};
  const extractModuleToken = (name) => {
    const match = html.match(new RegExp('"' + name + '",\\[\\],\\{"token":"([^"]+)"'));
    return match ? match[1] : "";
  };
  const extractJSONValue = (name) => {
    const match = html.match(new RegExp('"' + name + '"\\s*:\\s*"?([^",}\\]]+)'));
    return match ? match[1] : "";
  };
  const fb_dtsg =
    dtsg.token ||
    document.querySelector('input[name="fb_dtsg"]')?.value ||
    extractModuleToken("DTSGInitialData") ||
    extractModuleToken("DTSG") ||
    extract("fb_dtsg") ||
    "";
  const lsdToken =
    lsd.token ||
    document.querySelector('input[name="lsd"]')?.value ||
    extractModuleToken("LSD") ||
    extract("lsd") ||
    "";
  const jazoest =
    document.querySelector('input[name="jazoest"]')?.value ||
    extractJSONValue("jazoest") ||
    extract("jazoest") ||
    (fb_dtsg ? "2" + Array.from(fb_dtsg).reduce((sum, ch) => sum + ch.charCodeAt(0), 0) : "");
  function extract(name) {
    const resources = [];
    try {
      resources.push(...performance.getEntriesByType("resource").map((entry) => entry.name));
    } catch (_) {}
    const haystack = [location.href, document.documentElement.innerHTML, ...resources].join("\n");
    const queryMatch = haystack.match(new RegExp("[?&]" + name + "=([^&#\\s]+)"));
    if (queryMatch) {
      try {
        return decodeURIComponent(queryMatch[1]);
      } catch (_) {
        return queryMatch[1];
      }
    }
    const jsonMatch = haystack.match(new RegExp('"' + name + '"\\s*:\\s*"([^"]+)"'));
    if (jsonMatch) {
      return jsonMatch[1].replace(/\\u0025/g, "%");
    }
    return "";
  }
  return {
    fb_dtsg,
    lsd: lsdToken,
    jazoest,
    revision: String(siteData.__spin_r || siteData.client_revision || extractJSONValue("__spin_r") || extractJSONValue("client_revision") || "")
  };
})()`
}

func chromeExecutablePath() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		path := "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	case "linux":
		for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser"} {
			if path, err := exec.LookPath(name); err == nil {
				return path, nil
			}
		}
	case "windows":
		for _, candidate := range []string{
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(os.Getenv("PROGRAMFILES"), "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(os.Getenv("PROGRAMFILES(X86)"), "Google", "Chrome", "Application", "chrome.exe"),
		} {
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
	}
	return "", fmt.Errorf("Google Chrome executable not found")
}

func waitForDevToolsPort(userDataDir string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	path := filepath.Join(userDataDir, "DevToolsActivePort")
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			if len(lines) > 0 && lines[0] != "" {
				return lines[0], nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return "", fmt.Errorf("timed out waiting for Chrome DevTools port")
}

func createCDPTarget(port, targetURL string) (string, error) {
	endpoint := "http://127.0.0.1:" + port + "/json/new?" + url.QueryEscape(targetURL)
	req, err := http.NewRequest(http.MethodPut, endpoint, nil)
	if err != nil {
		return "", err
	}
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return "", fmt.Errorf("creating Chrome DevTools target: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("creating Chrome DevTools target returned HTTP %d: %s", resp.StatusCode, truncateBody(body))
	}
	var target struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.Unmarshal(body, &target); err != nil {
		return "", fmt.Errorf("parsing Chrome DevTools target: %w", err)
	}
	if target.WebSocketDebuggerURL == "" {
		return "", fmt.Errorf("Chrome DevTools target did not include websocket URL")
	}
	return target.WebSocketDebuggerURL, nil
}

func (c *cdpClient) call(method string, params map[string]any) (json.RawMessage, error) {
	c.nextID++
	msg := map[string]any{"id": c.nextID, "method": method}
	if params != nil {
		msg["params"] = params
	}
	if err := c.conn.WriteJSON(msg); err != nil {
		return nil, err
	}
	for {
		var resp struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := c.conn.ReadJSON(&resp); err != nil {
			return nil, err
		}
		if resp.ID != c.nextID {
			continue
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("%s: CDP error %d: %s", method, resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	}
}

func (c *cdpClient) evaluateTokens(expression string) (facebookPageContextTokens, error) {
	result, err := c.call("Runtime.evaluate", map[string]any{
		"expression":    expression,
		"returnByValue": true,
	})
	if err != nil {
		return facebookPageContextTokens{}, err
	}
	var envelope struct {
		Result struct {
			Value facebookPageContextTokens `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(result, &envelope); err != nil {
		return facebookPageContextTokens{}, err
	}
	return envelope.Result.Value, nil
}

func (c *cdpClient) evaluateString(expression string, awaitPromise bool) (string, error) {
	result, err := c.call("Runtime.evaluate", map[string]any{
		"expression":    expression,
		"returnByValue": true,
		"awaitPromise":  awaitPromise,
	})
	if err != nil {
		return "", err
	}
	var envelope struct {
		Result struct {
			Value string `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(result, &envelope); err != nil {
		return "", err
	}
	return envelope.Result.Value, nil
}

type parsedCookie struct {
	Name  string
	Value string
}

func parseCookieHeader(header string) []parsedCookie {
	var cookies []parsedCookie
	for _, part := range strings.Split(header, ";") {
		name, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok || name == "" {
			continue
		}
		cookies = append(cookies, parsedCookie{Name: name, Value: value})
	}
	return cookies
}
