package client

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
)

const DefaultMarketplaceUploadTargetID = "1663689853903557"

type MarketplacePhotoUpload struct {
	PhotoID string          `json:"photo_id"`
	Width   int             `json:"width,omitempty"`
	Height  int             `json:"height,omitempty"`
	Raw     json.RawMessage `json:"raw,omitempty"`
}

func (c *Client) UploadMarketplacePhoto(photoPath, targetID string, uploadID int) (MarketplacePhotoUpload, int, error) {
	if photoPath == "" {
		return MarketplacePhotoUpload{}, 0, fmt.Errorf("photo path is required")
	}
	if uploadID <= 0 {
		uploadID = 1024
	}
	if targetID == "" {
		targetID = DefaultMarketplaceUploadTargetID
	}

	authHeader, err := c.authHeader()
	if err != nil {
		return MarketplacePhotoUpload{}, 0, err
	}
	userID := firstRegexGroup(authHeader, `(?:^|;\s*)c_user=([^;]+)`)
	if userID == "" {
		return MarketplacePhotoUpload{}, 0, fmt.Errorf("facebook user id not available from saved session")
	}

	if upload, status, err := c.uploadMarketplacePhotoViaBrowserContext(photoPath, targetID, userID, uploadID); err == nil {
		c.invalidateCache()
		return upload, status, nil
	}

	body, contentType, err := marketplacePhotoUploadBody(photoPath, targetID, userID, uploadID)
	if err != nil {
		return MarketplacePhotoUpload{}, 0, err
	}

	uploadURL, headers, err := c.marketplacePhotoUploadRequestContext(authHeader, userID)
	if err != nil {
		return MarketplacePhotoUpload{}, 0, err
	}
	req, err := http.NewRequest(http.MethodPost, uploadURL, body)
	if err != nil {
		return MarketplacePhotoUpload{}, 0, err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Cookie", authHeader)
	req.Header.Set("Accept", "*/*")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	if c.Config != nil {
		for key, value := range c.Config.Headers {
			req.Header.Set(key, value)
		}
	}

	c.limiter.Wait()
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return MarketplacePhotoUpload{}, 0, fmt.Errorf("uploading marketplace photo: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return MarketplacePhotoUpload{}, resp.StatusCode, fmt.Errorf("reading photo upload response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return MarketplacePhotoUpload{}, resp.StatusCode, &APIError{Method: http.MethodPost, Path: "/ajax/react_composer/attachments/photo/upload", StatusCode: resp.StatusCode, Body: truncateBody(respBody)}
	}
	upload, err := parseMarketplacePhotoUploadResponse(respBody)
	if err != nil {
		return MarketplacePhotoUpload{}, resp.StatusCode, err
	}
	c.invalidateCache()
	return upload, resp.StatusCode, nil
}

func (c *Client) uploadMarketplacePhotoViaBrowserContext(photoPath, targetID, userID string, uploadID int) (MarketplacePhotoUpload, int, error) {
	data, err := os.ReadFile(photoPath)
	if err != nil {
		return MarketplacePhotoUpload{}, 0, err
	}
	expression, err := marketplacePhotoUploadExpression(c.BaseURL, targetID, userID, uploadID, filepath.Base(photoPath), photoContentType(photoPath), data)
	if err != nil {
		return MarketplacePhotoUpload{}, 0, err
	}

	var lastErr error
	for _, port := range []string{"9222", "9229"} {
		body, status, err := c.evaluateMarketplacePhotoUploadViaCDP(port, expression)
		if err == nil {
			upload, parseErr := parseMarketplacePhotoUploadResponse(body)
			return upload, status, parseErr
		}
		lastErr = err
	}
	if body, status, err := c.evaluateMarketplacePhotoUploadViaChromeAppleScript(expression); err == nil {
		upload, parseErr := parseMarketplacePhotoUploadResponse(body)
		return upload, status, parseErr
	} else {
		lastErr = err
	}
	if lastErr != nil {
		return MarketplacePhotoUpload{}, 0, lastErr
	}
	return MarketplacePhotoUpload{}, 0, fmt.Errorf("browser page-context upload unavailable")
}

func (c *Client) evaluateMarketplacePhotoUploadViaCDP(port, expression string) ([]byte, int, error) {
	pageWS, err := createCDPTarget(port, c.BaseURL+"/marketplace/create/item")
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
	if _, err := cdp.call("Page.navigate", map[string]any{"url": c.BaseURL + "/marketplace/create/item"}); err != nil {
		return nil, 0, err
	}
	if _, err := pollFacebookPageTokens(cdp); err != nil {
		return nil, 0, err
	}
	result, err := cdp.evaluateString(expression, true)
	if err != nil {
		return nil, 0, err
	}
	return parseBrowserUploadEnvelope(result)
}

func (c *Client) evaluateMarketplacePhotoUploadViaChromeAppleScript(expression string) ([]byte, int, error) {
	if runtime.GOOS != "darwin" {
		return nil, 0, fmt.Errorf("Chrome AppleScript upload requires macOS")
	}
	pollExpression := `window.__facebookMarketplacePhotoUploadResult || ""`
	runExpression := fmt.Sprintf(`(() => {
  window.__facebookMarketplacePhotoUploadResult = "";
  %s.then(
    value => { window.__facebookMarketplacePhotoUploadResult = value; },
    error => {
      window.__facebookMarketplacePhotoUploadResult = JSON.stringify({
        status: 0,
        body: JSON.stringify({
          errorSummary: "Browser upload exception",
          errorDescription: String(error && error.message || error)
        })
      });
    }
  );
  return "started";
})()`, expression)
	script := fmt.Sprintf(`
tell application "Google Chrome"
  if (count of windows) = 0 then make new window
  set w to front window
  set t to make new tab at end of tabs of w with properties {URL:%q}
  set active tab index of w to (count of tabs of w)
  delay 6
  execute t javascript %q
  set resultText to ""
  repeat with i from 1 to 60
    delay 1
    set resultText to execute t javascript %q
    if resultText is not "" then exit repeat
  end repeat
  close t
  return resultText
end tell
`, c.BaseURL+"/marketplace/create/item", runExpression, pollExpression)
	cmd := exec.Command("osascript")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.Output()
	if err != nil {
		return nil, 0, err
	}
	result := strings.TrimSpace(string(out))
	if result == "" {
		return nil, 0, fmt.Errorf("timed out waiting for Chrome AppleScript upload")
	}
	return parseBrowserUploadEnvelope(result)
}

func parseBrowserUploadEnvelope(value string) ([]byte, int, error) {
	var envelope struct {
		Status int    `json:"status"`
		Body   string `json:"body"`
	}
	if err := json.Unmarshal([]byte(value), &envelope); err != nil {
		return nil, 0, err
	}
	return []byte(envelope.Body), envelope.Status, nil
}

func marketplacePhotoUploadExpression(baseURL, targetID, userID string, uploadID int, filename, contentType string, data []byte) (string, error) {
	payload := map[string]any{
		"baseURL":     baseURL,
		"targetID":    targetID,
		"userID":      userID,
		"uploadID":    strconv.Itoa(uploadID),
		"filename":    filename,
		"contentType": contentType,
		"base64":      base64.StdEncoding.EncodeToString(data),
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`(async () => {
  const cfg = %s;
  const html = document.documentElement.innerHTML;
  const resourceURLs = (() => {
    try { return performance.getEntriesByType("resource").map((entry) => entry.name); } catch (_) { return []; }
  })();
  const getModule = (name) => {
    try { return window.require ? window.require(name) : null; } catch (_) { return null; }
  };
  const siteData = getModule("SiteData") || {};
  const dtsgData = getModule("DTSGInitialData") || getModule("DTSG") || {};
  const lsdData = getModule("LSD") || {};
  const extractModuleToken = (name) => {
    const match = html.match(new RegExp('"' + name + '",\\[\\],\\{"token":"([^"]+)"'));
    return match ? match[1] : "";
  };
  const extractJSONValue = (name) => {
    const match = html.match(new RegExp('"' + name + '"\\s*:\\s*"?([^",}\\]]+)'));
    return match ? match[1] : "";
  };
  const extractResourceParam = (name) => {
    for (let i = resourceURLs.length - 1; i >= 0; i--) {
      try {
        const value = new URL(resourceURLs[i], location.href).searchParams.get(name);
        if (value) return value;
      } catch (_) {}
    }
    return "";
  };
  const fbDtsg = dtsgData.token || document.querySelector('input[name="fb_dtsg"]')?.value || extractModuleToken("DTSGInitialData") || extractModuleToken("DTSG") || "";
  const lsd = lsdData.token || document.querySelector('input[name="lsd"]')?.value || extractModuleToken("LSD") || "";
  const jazoest = document.querySelector('input[name="jazoest"]')?.value || extractJSONValue("jazoest") || (fbDtsg ? "2" + Array.from(fbDtsg).reduce((sum, ch) => sum + ch.charCodeAt(0), 0) : "");
  const rev = String(siteData.__spin_r || siteData.client_revision || extractJSONValue("__spin_r") || extractJSONValue("client_revision") || "1");
  const browserUserID = (document.cookie.match(/(?:^|;\s*)c_user=([^;]+)/) || [])[1] || cfg.userID;
  const params = new URLSearchParams({
    av: browserUserID,
    __user: browserUserID,
    __a: "1",
    __req: "1",
    dpr: "2",
    __ccg: "EXCELLENT",
    __rev: rev,
    __comet_req: "15",
    __spin_r: rev,
    __spin_b: "trunk",
    __crn: "comet.fbweb.CometMarketplaceComposerRoute",
    qpl_active_flow_ids: "138820675"
  });
  for (const [key, value] of Object.entries({
    __aaid: extractResourceParam("__aaid") || extractJSONValue("__aaid"),
    __hs: siteData.haste_session || extractJSONValue("haste_session"),
    __hsi: siteData.hsi || extractJSONValue("hsi"),
    __s: extractResourceParam("__s"),
    __dyn: extractResourceParam("__dyn"),
    __csr: extractResourceParam("__csr"),
    __hsdp: extractResourceParam("__hsdp"),
    __hblp: extractResourceParam("__hblp"),
    __sjsp: extractResourceParam("__sjsp")
  })) {
    if (value) params.set(key, String(value));
  }
  const binary = atob(cfg.base64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
  const form = new FormData();
  if (fbDtsg) form.append("fb_dtsg", fbDtsg);
  form.append("qn", "comet_marketplace_composer");
  form.append("target_id", cfg.targetID);
  form.append("source", "8");
  form.append("profile_id", browserUserID);
  form.append("waterfallxapp", "comet");
  form.append("farr", new File([bytes], cfg.filename, {type: cfg.contentType}));
  form.append("upload_id", cfg.uploadID);
  const response = await fetch("https://upload.facebook.com/ajax/react_composer/attachments/photo/upload?" + params.toString(), {
    method: "POST",
    credentials: "include",
    body: form
  });
  return JSON.stringify({status: response.status, body: await response.text()});
})()`, string(payloadJSON)), nil
}

func marketplacePhotoUploadBody(photoPath, targetID, userID string, uploadID int) (*bytes.Buffer, string, error) {
	file, err := os.Open(photoPath)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields := map[string]string{
		"qn":            "comet_marketplace_composer",
		"target_id":     targetID,
		"source":        "8",
		"profile_id":    userID,
		"waterfallxapp": "comet",
		"upload_id":     strconv.Itoa(uploadID),
	}
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			return nil, "", err
		}
	}
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{
		"name":     "farr",
		"filename": filepath.Base(photoPath),
	}))
	partHeader.Set("Content-Type", photoContentType(photoPath))
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		return nil, "", err
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, "", err
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return &body, writer.FormDataContentType(), nil
}

func (c *Client) marketplacePhotoUploadRequestContext(authHeader, userID string) (string, map[string]string, error) {
	tokens := facebookPageContextTokens{}
	if shell, err := c.fetchMarketplaceShell(authHeader); err == nil {
		tokens.FBDTSG = firstRegexGroup(shell,
			`"DTSGInitialData",\[\],\{"token":"([^"]+)"`,
			`"DTSG[^"]*",\[\],\{[^}]*"token":"([^"]+)"`,
			`"fb_dtsg":"([^"]+)"`,
			`name="fb_dtsg" value="([^"]+)"`,
		)
		tokens.LSD = firstRegexGroup(shell,
			`"LSD",\[\],\{"token":"([^"]+)"`,
			`name="lsd" value="([^"]+)"`,
		)
		tokens.Jazoest = firstRegexGroup(shell,
			`name="jazoest" value="([^"]+)"`,
			`"jazoest",\s*"([^"]+)"`,
		)
		tokens.Revision = firstRegexGroup(shell, `"__spin_r":(\d+)`, `"client_revision":(\d+)`)
	}
	if tokens.FBDTSG == "" {
		if pageTokens, err := c.extractFacebookPageContextTokens(authHeader); err == nil {
			tokens = pageTokens
		}
	}
	if tokens.Jazoest == "" && tokens.FBDTSG != "" {
		tokens.Jazoest = deriveJazoest(tokens.FBDTSG)
	}
	rev := tokens.Revision
	if rev == "" {
		rev = "1"
	}
	values := url.Values{}
	values.Set("av", userID)
	values.Set("__user", userID)
	values.Set("__a", "1")
	values.Set("__req", "1")
	values.Set("dpr", "2")
	values.Set("__ccg", "EXCELLENT")
	values.Set("__rev", rev)
	values.Set("__comet_req", "15")
	values.Set("__spin_r", rev)
	values.Set("__spin_b", "trunk")
	values.Set("__crn", "comet.fbweb.CometMarketplaceComposerRoute")
	values.Set("qpl_active_flow_ids", "138820675")
	headers := map[string]string{
		"Origin":  c.BaseURL,
		"Referer": c.BaseURL + "/marketplace/create/item",
	}
	if tokens.LSD != "" {
		headers["X-FB-LSD"] = tokens.LSD
	}
	if tokens.FBDTSG != "" {
		headers["X-FB-DTSG"] = tokens.FBDTSG
	}
	return "https://upload.facebook.com/ajax/react_composer/attachments/photo/upload?" + values.Encode(), headers, nil
}

func parseMarketplacePhotoUploadResponse(body []byte) (MarketplacePhotoUpload, error) {
	clean := stripFacebookJSONGuard(bytes.TrimSpace(body))
	var envelope struct {
		Payload struct {
			PhotoID string `json:"photoID"`
			Width   int    `json:"width"`
			Height  int    `json:"height"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(clean, &envelope); err != nil {
		return MarketplacePhotoUpload{}, fmt.Errorf("parsing photo upload response: %w", err)
	}
	if envelope.Payload.PhotoID == "" {
		return MarketplacePhotoUpload{}, fmt.Errorf("photo upload response did not include photoID: %s", summarizeMarketplacePhotoUploadResponse(clean))
	}
	return MarketplacePhotoUpload{
		PhotoID: envelope.Payload.PhotoID,
		Width:   envelope.Payload.Width,
		Height:  envelope.Payload.Height,
		Raw:     json.RawMessage(clean),
	}, nil
}

func summarizeMarketplacePhotoUploadResponse(body []byte) string {
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return truncateBody(body)
	}
	redactUploadResponse(value)
	if compact := compactUploadError(value); compact != nil {
		data, err := json.Marshal(compact)
		if err == nil {
			return string(data)
		}
	}
	data, err := json.Marshal(value)
	if err != nil {
		return truncateBody(body)
	}
	return truncateBody(data)
}

func compactUploadError(value any) map[string]any {
	root, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	compact := map[string]any{}
	for _, key := range []string{"error", "errorSummary", "errorDescription"} {
		if field, ok := root[key]; ok {
			compact[key] = field
		}
	}
	if payload, ok := root["payload"].(map[string]any); ok {
		if dialog, ok := payload["__dialog"].(map[string]any); ok {
			for _, key := range []string{"body"} {
				if field, ok := dialog[key]; ok {
					compact["dialog_"+key] = field
				}
			}
		}
	}
	if len(compact) == 0 {
		return nil
	}
	return compact
}

func redactUploadResponse(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			lower := strings.ToLower(key)
			if strings.Contains(lower, "src") || strings.Contains(lower, "url") || strings.Contains(lower, "uri") {
				typed[key] = "[redacted]"
				continue
			}
			redactUploadResponse(child)
		}
	case []any:
		for _, child := range typed {
			redactUploadResponse(child)
		}
	}
}

func stripFacebookJSONGuard(body []byte) []byte {
	return bytes.TrimSpace(bytes.TrimPrefix(body, []byte("for (;;);")))
}

func photoContentType(path string) string {
	if contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path))); contentType != "" {
		return contentType
	}
	return "application/octet-stream"
}
