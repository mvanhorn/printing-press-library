// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/everbee/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/everbee/internal/config"
	"github.com/spf13/cobra"
	"golang.org/x/net/websocket"
)

const (
	defaultCaptureCDPURL     = "http://127.0.0.1:9222"
	defaultCaptureLoginURL   = "https://auth.everbee.io/login"
	onePasswordExtensionID   = "aeblfdkhhhdcdjpifhhbdiojplfjncoa"                                                                     // #nosec G101 -- Public Chrome extension ID, not a credential.
	onePasswordWebStoreURL   = "https://chromewebstore.google.com/detail/1password-password-manager/aeblfdkhhhdcdjpifhhbdiojplfjncoa" // #nosec G101 -- Public Chrome Web Store URL, not a credential.
	captureRequestIDKey      = "requestId"
	captureNetworkHeaderName = "x-access-token"
)

type authCaptureOptions struct {
	cdpURL             string
	chromePath         string
	profileDir         string
	loginURL           string
	wait               time.Duration
	launch             bool
	requireOnePassword bool
}

type cdpTarget struct {
	ID                   string `json:"id"`
	URL                  string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

type cdpMessage struct {
	ID     int             `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Error  *cdpError       `json:"error,omitempty"`
}

type cdpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type capturedToken struct {
	Token  string
	Source string
}

func newAuthCaptureCmd(flags *rootFlags) *cobra.Command {
	opts := authCaptureOptions{
		cdpURL:             defaultCaptureCDPURL,
		loginURL:           defaultCaptureLoginURL,
		wait:               5 * time.Minute,
		launch:             true,
		requireOnePassword: true,
	}
	cmd := &cobra.Command{
		Use:   "capture",
		Short: "Capture an EverBee token from a Chrome login session",
		Long: `Launch or attach to Chrome over the DevTools protocol, let you complete
Google SSO manually, then save the EverBee x-access-token captured from network
traffic. The default launch uses a persistent Chrome profile so extensions such
as 1Password stay installed and available between captures.`,
		Example: "  everbee-pp-cli auth capture\n  everbee-pp-cli auth capture --profile-dir ~/.everbee-capture-chrome\n  everbee-pp-cli auth capture --launch=false --cdp-url http://127.0.0.1:9222",
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.noInput {
				return usageErr(fmt.Errorf("auth capture requires interactive browser login; remove --no-input"))
			}
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"would_capture": true,
					"cdp_url":       opts.cdpURL,
					"profile_dir":   opts.profileDir,
					"login_url":     opts.loginURL,
				}, flags)
			}
			if opts.profileDir == "" {
				dir, err := defaultChromeCaptureProfileDir()
				if err != nil {
					return configErr(err)
				}
				opts.profileDir = dir
			}
			result, err := runAuthCapture(cmd, flags, opts)
			if err != nil {
				return authErr(err)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Token saved to %s\n", result["config_path"])
			fmt.Fprintf(w, "  Source: %s\n", result["source"])
			fmt.Fprintf(w, "  Expires: %s\n", result["expires_at"])
			fmt.Fprintf(w, "  Token hash: %s\n", result["token_hash"])
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.cdpURL, "cdp-url", opts.cdpURL, "Chrome DevTools base URL")
	cmd.Flags().StringVar(&opts.chromePath, "chrome-path", "", "Chrome executable path")
	cmd.Flags().StringVar(&opts.profileDir, "profile-dir", "", "Persistent Chrome user data dir for capture")
	cmd.Flags().StringVar(&opts.loginURL, "login-url", opts.loginURL, "URL to open for EverBee login")
	cmd.Flags().DurationVar(&opts.wait, "wait", opts.wait, "How long to wait for the EverBee token")
	cmd.Flags().BoolVar(&opts.launch, "launch", opts.launch, "Launch Chrome when no DevTools endpoint is available")
	cmd.Flags().BoolVar(&opts.requireOnePassword, "require-1password", opts.requireOnePassword, "Open the 1Password extension page when it is not installed in the capture profile")
	return cmd
}

func runAuthCapture(cmd *cobra.Command, flags *rootFlags, opts authCaptureOptions) (map[string]any, error) {
	if opts.wait <= 0 {
		return nil, fmt.Errorf("--wait must be greater than zero")
	}
	onePasswordInstalled := onePasswordExtensionInstalled(opts.profileDir)
	client := &http.Client{Timeout: 5 * time.Second}
	if !cdpAvailable(client, opts.cdpURL) {
		if !opts.launch {
			return nil, fmt.Errorf("Chrome DevTools endpoint not available at %s", opts.cdpURL)
		}
		if err := launchCaptureChrome(opts); err != nil {
			return nil, err
		}
		if err := waitForCDP(client, opts.cdpURL, 15*time.Second); err != nil {
			return nil, err
		}
	}
	if opts.requireOnePassword && !onePasswordInstalled {
		_, _ = openCDPTarget(client, opts.cdpURL, onePasswordWebStoreURL)
		fmt.Fprintln(cmd.ErrOrStderr(), "1Password extension was not found in the capture profile.")
		fmt.Fprintln(cmd.ErrOrStderr(), "Install/unlock 1Password in the Chrome window, then complete EverBee Google SSO in the login tab.")
	}
	target, err := openCDPTarget(client, opts.cdpURL, opts.loginURL)
	if err != nil {
		return nil, err
	}
	fmt.Fprintln(cmd.ErrOrStderr(), "Chrome capture ready. Complete EverBee Google SSO in the opened Chrome tab.")
	fmt.Fprintln(cmd.ErrOrStderr(), "The CLI will save only the EverBee API token, not your Google or 1Password credentials.")
	captured, err := captureTokenFromTarget(target.WebSocketDebuggerURL, opts.cdpURL, opts.wait)
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return nil, err
	}
	cfg.AuthHeaderVal = ""
	if err := cfg.SaveCredential(captured.Token); err != nil {
		return nil, fmt.Errorf("saving token: %w", err)
	}
	tokenHash, expiresAt := summarizeCapturedToken(captured.Token)
	return map[string]any{
		"saved":                 true,
		"source":                captured.Source,
		"config_path":           cfg.Path,
		"profile_dir":           opts.profileDir,
		"onepassword_detected":  onePasswordInstalled,
		"token_hash":            tokenHash,
		"expires_at":            expiresAt,
		"chrome_extensions_dir": filepath.Join(opts.profileDir, "Default", "Extensions"),
	}, nil
}

func defaultChromeCaptureProfileDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home dir: %w", err)
	}
	return filepath.Join(home, ".everbee-capture-chrome"), nil
}

func onePasswordExtensionInstalled(profileDir string) bool {
	if profileDir == "" {
		return false
	}
	candidates := []string{
		filepath.Join(profileDir, "Default", "Extensions", onePasswordExtensionID),
		filepath.Join(profileDir, "Extensions", onePasswordExtensionID),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

func cdpAvailable(client *http.Client, cdpURL string) bool {
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(cdpURL, "/")+"/json/version", nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func waitForCDP(client *http.Client, cdpURL string, wait time.Duration) error {
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		if cdpAvailable(client, cdpURL) {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("Chrome DevTools endpoint did not become available at %s", cdpURL)
}

func launchCaptureChrome(opts authCaptureOptions) error {
	chromePath := opts.chromePath
	if chromePath == "" {
		var err error
		chromePath, err = findChromeExecutable()
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(opts.profileDir, 0o700); err != nil {
		return fmt.Errorf("creating Chrome profile dir: %w", err)
	}
	port := "9222"
	if parsed, err := url.Parse(opts.cdpURL); err == nil && parsed.Port() != "" {
		port = parsed.Port()
	}
	args := []string{
		"--remote-debugging-port=" + port,
		"--remote-allow-origins=http://127.0.0.1:" + port,
		"--user-data-dir=" + opts.profileDir,
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-popup-blocking",
		opts.loginURL,
	}
	command := exec.Command(chromePath, args...) // #nosec G204 -- Operator-selected/local Chrome executable with a fixed argument vector for interactive capture.
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	if err := command.Start(); err != nil {
		return fmt.Errorf("launching Chrome: %w", err)
	}
	return command.Process.Release()
}

func findChromeExecutable() (string, error) {
	candidates := []string{}
	if runtime.GOOS == "darwin" {
		candidates = append(candidates,
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			filepath.Join(os.Getenv("HOME"), "Applications/Google Chrome.app/Contents/MacOS/Google Chrome"),
		)
	}
	if runtime.GOOS == "linux" {
		candidates = append(candidates, "google-chrome", "google-chrome-stable", "chromium", "chromium-browser")
	}
	if runtime.GOOS == "windows" {
		candidates = append(candidates,
			filepath.Join(os.Getenv("ProgramFiles"), "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(os.Getenv("ProgramFiles(x86)"), "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(os.Getenv("LocalAppData"), "Google", "Chrome", "Application", "chrome.exe"),
		)
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if strings.Contains(candidate, string(filepath.Separator)) {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() { // #nosec G703 -- Candidate paths are fixed Chrome install locations or platform app dirs.
				return candidate, nil
			}
			continue
		}
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("Chrome executable not found; pass --chrome-path")
}

func openCDPTarget(client *http.Client, cdpURL, targetURL string) (*cdpTarget, error) {
	endpoint := strings.TrimRight(cdpURL, "/") + "/json/new?" + url.QueryEscape(targetURL)
	req, err := http.NewRequest(http.MethodPut, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("opening Chrome target: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("opening Chrome target returned HTTP %d", resp.StatusCode)
	}
	var target cdpTarget
	if err := json.NewDecoder(resp.Body).Decode(&target); err != nil {
		return nil, fmt.Errorf("decoding Chrome target: %w", err)
	}
	if target.WebSocketDebuggerURL == "" {
		return nil, errors.New("Chrome target did not include a websocket debugger URL")
	}
	return &target, nil
}

func captureTokenFromTarget(webSocketURL, origin string, wait time.Duration) (capturedToken, error) {
	ws, err := websocket.Dial(webSocketURL, "", strings.TrimRight(origin, "/"))
	if err != nil {
		return capturedToken{}, fmt.Errorf("connecting to Chrome target: %w", err)
	}
	defer ws.Close()
	nextID := 1
	for _, command := range []string{"Network.enable", "Page.enable", "Runtime.enable"} {
		if err := sendCDPCommand(ws, nextID, command, map[string]any{}); err != nil {
			return capturedToken{}, err
		}
		nextID++
	}
	deadline := time.Now().Add(wait)
	for {
		if time.Now().After(deadline) {
			return capturedToken{}, fmt.Errorf("timed out waiting for EverBee token")
		}
		_ = ws.SetReadDeadline(time.Now().Add(1 * time.Second))
		var message cdpMessage
		if err := websocket.JSON.Receive(ws, &message); err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				continue
			}
			return capturedToken{}, fmt.Errorf("reading Chrome event: %w", err)
		}
		token := extractEverbeeTokenFromCDPMessage(message)
		if token.Token != "" {
			return token, nil
		}
	}
}

func sendCDPCommand(ws *websocket.Conn, id int, method string, params any) error {
	payload := map[string]any{
		"id":     id,
		"method": method,
		"params": params,
	}
	if err := websocket.JSON.Send(ws, payload); err != nil {
		return fmt.Errorf("sending Chrome command %s: %w", method, err)
	}
	return nil
}

func extractEverbeeTokenFromCDPMessage(message cdpMessage) capturedToken {
	if len(message.Params) == 0 {
		return capturedToken{}
	}
	var params map[string]any
	if err := json.Unmarshal(message.Params, &params); err != nil {
		return capturedToken{}
	}
	if request, ok := params["request"].(map[string]any); ok {
		if token := extractEverbeeTokenFromURL(stringValue(request["url"])); token != "" {
			return capturedToken{Token: token, Source: "chrome-cdp:url"}
		}
		if token := extractEverbeeTokenFromHeaders(request["headers"]); token != "" {
			return capturedToken{Token: token, Source: "chrome-cdp:request-header"}
		}
	}
	if token := extractEverbeeTokenFromHeaders(params["headers"]); token != "" {
		return capturedToken{Token: token, Source: "chrome-cdp:extra-info-header"}
	}
	if token := extractEverbeeTokenFromURL(stringValue(params["documentURL"])); token != "" {
		return capturedToken{Token: token, Source: "chrome-cdp:document-url"}
	}
	return capturedToken{}
}

func extractEverbeeTokenFromURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	if parsed.Host != "app.everbee.io" || parsed.Path != "/ext" {
		return ""
	}
	return normalizeCapturedToken(parsed.Query().Get("token"))
}

func extractEverbeeTokenFromHeaders(headers any) string {
	headerMap, ok := headers.(map[string]any)
	if !ok {
		return ""
	}
	for name, value := range headerMap {
		if strings.EqualFold(name, captureNetworkHeaderName) {
			return normalizeCapturedToken(stringValue(value))
		}
	}
	return ""
}

func normalizeCapturedToken(token string) string {
	token = strings.TrimSpace(token)
	token = strings.TrimPrefix(token, "Bearer ")
	if strings.Count(token, ".") != 2 {
		return ""
	}
	return token
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return ""
	}
}

func summarizeCapturedToken(token string) (string, string) {
	sum := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(sum[:])[:12]
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return tokenHash, ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return tokenHash, ""
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Exp == 0 {
		return tokenHash, ""
	}
	return tokenHash, time.Unix(claims.Exp, 0).UTC().Format(time.RFC3339)
}
