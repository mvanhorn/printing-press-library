// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source computed

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/milanote/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/milanote/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/milanote/internal/milanote"
)

func newNovelChromeLoginCmd(flags *rootFlags) *cobra.Command {
	var port int

	cmd := &cobra.Command{
		Use:   "chrome-login",
		Short: "Pull the Milanote login from a running Chrome in one step.",
		Long: `Reads only the Milanote session cookie from a Chrome you already started with
--remote-debugging-port, and saves it to the CLI's config (owner-only file).
No other cookies are read or stored. Log in to Milanote in that Chrome first.`,
		Example:     "  milanote-pp-cli chrome-login --port 9222",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "computed", "mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "chrome-login")
			}
			if port < 1 || port > 65535 {
				return usageErr(fmt.Errorf("--port must be between 1 and 65535"))
			}
			value, err := sessionCookieFromChrome(port)
			if err != nil {
				return authErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cookie := milanote.SessionCookie + "=" + value
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			opts, err := mnOptions(flags, cfg)
			if err != nil {
				return err
			}
			mc, err := milanote.New("", cookie, opts)
			if err != nil {
				return mnErr(err)
			}
			me, err := mc.Me(ctx)
			if err != nil {
				return authErr(fmt.Errorf("Chrome's Milanote session was rejected, so nothing was saved: %w", err))
			}
			cfg.CredentialDomain = "app.milanote.com"
			if err := cfg.SaveTokens("", "", cookie, "", time.Time{}); err != nil {
				return configErr(fmt.Errorf("saving login: %w", err))
			}
			if err := client.WriteCookieJarFromMap("app.milanote.com", map[string]string{milanote.SessionCookie: value}); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: saving cookie jar: %v (login still saved)\n", err)
			}
			return emit(cmd, flags, map[string]any{"logged_in": true, "name": me.Name.DisplayName, "root_board": me.RootBoardID}, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "%s logged in as %s\n", green("✓"), me.Name.DisplayName)
				return err
			})
		},
	}
	cmd.Flags().IntVar(&port, "port", 9222, "Chrome's remote debugging port")
	cmd.Annotations["pp:verified-by-proof"] = "milanote-lifecycle.md"
	return cmd
}

// sessionCookieFromChrome asks Chrome's DevTools endpoint for its cookies and
// returns only Milanote's session token.
func sessionCookieFromChrome(port int) (string, error) {
	hc := &http.Client{Timeout: 3 * time.Second}
	resp, err := hc.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", port))
	if err != nil {
		return "", fmt.Errorf("no Chrome listening on port %d; start it with --remote-debugging-port=%d", port, port)
	}
	defer resp.Body.Close()
	var version struct {
		WebSocket string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&version); err != nil || version.WebSocket == "" {
		return "", fmt.Errorf("port %d did not answer like Chrome's DevTools", port)
	}
	conn, _, err := (&websocket.Dialer{HandshakeTimeout: 3 * time.Second}).Dial(version.WebSocket, nil)
	if err != nil {
		return "", fmt.Errorf("connecting to Chrome: %w", err)
	}
	defer conn.Close()
	deadline := time.Now().Add(8 * time.Second)
	_ = conn.SetWriteDeadline(deadline)
	_ = conn.SetReadDeadline(deadline)
	if err := conn.WriteJSON(map[string]any{"id": 1, "method": "Storage.getCookies"}); err != nil {
		return "", fmt.Errorf("asking Chrome for cookies: %w", err)
	}
	for {
		var msg struct {
			ID     int `json:"id"`
			Result struct {
				Cookies []struct {
					Name   string `json:"name"`
					Value  string `json:"value"`
					Domain string `json:"domain"`
				} `json:"cookies"`
			} `json:"result"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := conn.ReadJSON(&msg); err != nil {
			return "", fmt.Errorf("reading Chrome's answer: %w", err)
		}
		if msg.ID != 1 {
			continue
		}
		if msg.Error != nil {
			return "", errors.New("Chrome refused: " + msg.Error.Message)
		}
		for _, ck := range msg.Result.Cookies {
			if ck.Name == milanote.SessionCookie && strings.HasSuffix(strings.TrimPrefix(ck.Domain, "."), "milanote.com") {
				return ck.Value, nil
			}
		}
		return "", errors.New("this Chrome is not logged in to Milanote; log in at app.milanote.com and try again")
	}
}
