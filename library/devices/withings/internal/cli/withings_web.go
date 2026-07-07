// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Web-extras tier: a thin, honest authenticated passthrough to Withings'
// internal web backend (scalews.withings.com/cgi-bin/...), which the HealthMate
// SPA uses. This surface is UNDOCUMENTED and FRAGILE — it authenticates with a
// short-lived `session_token` cookie captured from a logged-in Chrome session,
// not OAuth. We deliberately do NOT ship typed commands with hardcoded
// action/param contracts here (the exact params per endpoint are undocumented),
// so `web call` is a raw form-POST passthrough: you/your agent supply the
// action and params. The official OAuth API commands are the durable surface;
// reach for `web` only for the web-only endpoints (timeline feed, goals,
// targets, plans) that the official API doesn't expose.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/withings/internal/cliutil"
)

const scalewsBaseURL = "https://scalews.withings.com"

func webSessionPath() string {
	if p := os.Getenv("WITHINGS_WEB_SESSION_FILE"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "withings-pp-cli", "web-session")
}

// loadWebSessionToken resolves the web session token from (in order) the
// WITHINGS_WEB_SESSION_TOKEN env var, then the on-disk session file.
func loadWebSessionToken() string {
	if v := strings.TrimSpace(os.Getenv("WITHINGS_WEB_SESSION_TOKEN")); v != "" {
		return v
	}
	data, err := os.ReadFile(webSessionPath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func saveWebSessionToken(token string) error {
	p := webSessionPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(strings.TrimSpace(token)+"\n"), 0o600)
}

func newWebCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "web",
		Short: "Web-extras tier: authenticated passthrough to Withings' internal web backend (fragile, cookie-based)",
		Long: "Raw authenticated access to scalews.withings.com — the undocumented backend the\n" +
			"HealthMate web app uses. This is the FRAGILE tier: it authenticates with a\n" +
			"short-lived session_token cookie from your logged-in browser (not OAuth), and the\n" +
			"backend is undocumented and actively hardened. Prefer the official-API commands\n" +
			"(measure, activity, sleep, heart, ...) for anything they cover; use `web` only for\n" +
			"web-only endpoints (the timeline feed, goals, targets, plans).\n\n" +
			"Setup: copy your session_token cookie from a logged-in HealthMate tab\n" +
			"(DevTools > Application > Cookies > session_token, or run `document.cookie` in the\n" +
			"console) and import it with `web import-cookie`.",
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newWebImportCookieCmd(flags))
	cmd.AddCommand(newWebStatusCmd(flags))
	cmd.AddCommand(newWebCallCmd(flags))
	return cmd
}

func newWebImportCookieCmd(flags *rootFlags) *cobra.Command {
	var token string
	cmd := &cobra.Command{
		Use:   "import-cookie",
		Short: "Save your HealthMate session_token cookie for the web tier",
		Long: "Save the session_token cookie that authenticates the web backend.\n" +
			"Provide it via --token, the WITHINGS_WEB_SESSION_TOKEN env var, or piped stdin.\n" +
			"The token is stored at " + webSessionPath() + " with 0600 permissions.\n\n" +
			"Get it from a logged-in HealthMate tab: DevTools > Application > Cookies >\n" +
			"session_token (or `document.cookie` in the console). It is short-lived and must\n" +
			"be re-imported when it expires.",
		Example:     "  withings-pp-cli web import-cookie --token <session_token>\n  echo <session_token> | withings-pp-cli web import-cookie",
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() || dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would save the web session token")
				return nil
			}
			if token == "" {
				token = strings.TrimSpace(os.Getenv("WITHINGS_WEB_SESSION_TOKEN"))
			}
			// Accept piped stdin (non-interactive) when no flag/env provided.
			if token == "" {
				if fi, err := os.Stdin.Stat(); err == nil && (fi.Mode()&os.ModeCharDevice) == 0 {
					b, _ := io.ReadAll(os.Stdin)
					token = strings.TrimSpace(string(b))
				}
			}
			if token == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("no token provided: pass --token, set WITHINGS_WEB_SESSION_TOKEN, or pipe it on stdin"))
			}
			if err := saveWebSessionToken(token); err != nil {
				return configErr(fmt.Errorf("saving web session: %w", err))
			}
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{"saved": true, "path": webSessionPath()})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Web session token saved to %s\n", webSessionPath())
			return nil
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "The session_token cookie value (or WITHINGS_WEB_SESSION_TOKEN / stdin)")
	return cmd
}

func newWebStatusCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "status",
		Short:       "Show whether a web session token is configured",
		Example:     "  withings-pp-cli web status",
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			tok := loadWebSessionToken()
			configured := tok != ""
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{"configured": configured, "path": webSessionPath()})
			}
			if !configured {
				fmt.Fprintln(cmd.OutOrStdout(), "No web session token configured. Run: withings-pp-cli web import-cookie")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Web session token configured (%s).\n", webSessionPath())
			return nil
		},
	}
}

func newWebCallCmd(flags *rootFlags) *cobra.Command {
	var params []string
	var action string
	cmd := &cobra.Command{
		Use:   "call <path>",
		Short: "Raw authenticated form-POST to a scalews endpoint (you supply action + params)",
		Long: "Make a raw authenticated POST to a scalews.withings.com path using your imported\n" +
			"session_token cookie, and print the JSON the backend returns. You supply the\n" +
			"action and parameters — nothing is hardcoded.\n\n" +
			"Discovered web-only paths include: /cgi-bin/v2/timeline, /cgi-bin/v2/aggregate,\n" +
			"/cgi-bin/v2/target, /cgi-bin/v2/plan, /cgi-bin/v2/feature, /cgi-bin/v2/subcategory.\n" +
			"The exact action/params per endpoint are undocumented — inspect a logged-in\n" +
			"HealthMate session's network calls to learn them.",
		Example: "  withings-pp-cli web call /cgi-bin/v2/measure --action getmeas --param meastypes=1\n" +
			"  withings-pp-cli web call /cgi-bin/v2/timeline --param offset=0",
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) != 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("exactly one <path> argument is required (e.g. /cgi-bin/v2/timeline)"))
			}
			path := args[0]
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}
			form := url.Values{}
			if action != "" {
				form.Set("action", action)
			}
			for _, kv := range params {
				k, v, ok := strings.Cut(kv, "=")
				if !ok || k == "" {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("invalid --param %q: expected key=value", kv))
				}
				form.Set(k, v)
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "POST %s%s\n", scalewsBaseURL, path)
				for k := range form {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s=%s\n", k, form.Get(k))
				}
				fmt.Fprintln(cmd.OutOrStdout(), "(dry run - no request sent)")
				return nil
			}
			token := loadWebSessionToken()
			if token == "" {
				return authErr(fmt.Errorf("no web session token; run: withings-pp-cli web import-cookie"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			data, err := scalewsCall(ctx, token, path, form)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&action, "action", "", "The scalews action (e.g. getmeas, getsummary)")
	cmd.Flags().StringArrayVar(&params, "param", nil, "A request parameter, key=value (repeatable)")
	return cmd
}

// scalewsCall form-POSTs to scalews with the session cookie and returns the
// response body. It unwraps the {status, body} envelope on status 0 and returns
// an error on non-zero status. Kept self-contained (not on *client.Client)
// because the web tier targets a different host + cookie auth. The deadline is
// carried by ctx (the caller wraps with boundCtx for the root --timeout).
func scalewsCall(ctx context.Context, token, path string, form url.Values) (json.RawMessage, error) {
	hc := &http.Client{}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, scalewsBaseURL+path, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "withings-pp-cli/0.1.0")
	req.Header.Set("Cookie", "session_token="+token)

	resp, err := hc.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("scalews request failed: %w", err)
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("scalews POST %s returned HTTP %d", path, resp.StatusCode)
	}
	var env struct {
		Status int             `json:"status"`
		Body   json.RawMessage `json:"body"`
		Error  string          `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		// Not an envelope — return raw.
		return json.RawMessage(body), nil
	}
	if env.Status != 0 {
		msg := env.Error
		if msg == "" {
			msg = "request failed"
		}
		return nil, fmt.Errorf("scalews %s: status %d: %s (the session_token may be expired — re-import with `web import-cookie`)", path, env.Status, msg)
	}
	if len(env.Body) == 0 {
		return json.RawMessage(`{}`), nil
	}
	return env.Body, nil
}
