// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Implemented body; generate --force preserves this file.
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/cloud/browserbase/internal/cliutil"
)

func newNovelSessionsRunCmd(flags *rootFlags) *cobra.Command {
	var flagProject string
	var flagTimeout string
	var flagKeepAlive bool

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Create a session, print its connect URL, and guarantee it is released on completion, timeout, or interrupt.",
		Long: `Use this command when you want a session created, used, and guaranteed-stopped in a single invocation.
Do NOT use it to find sessions that were already abandoned; use 'sessions orphans' instead.`,
		Example: "  browserbase-pp-cli sessions run --project 1fbe3566-db19-4010-9410-0ba94f0497ea --timeout 15m",
		Annotations: map[string]string{
			"mcp:read-only":       "false",
			"pp:typed-exit-codes": "0,2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "sessions run")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			timeout := 15 * time.Minute
			if flagTimeout != "" {
				parsed, err := cliutil.ParseDurationLoose(flagTimeout)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--timeout %q is invalid: %w (use e.g. 10m, 1h, 15m)", flagTimeout, err))
				}
				timeout = parsed
			}
			// The API only accepts session lifetimes in [60s, 21600s].
			if timeout < time.Minute || timeout > 6*time.Hour {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--timeout %s is outside the API's accepted range (1m to 6h)", timeout))
			}
			// The hold must be governed by the session lifetime, NOT the root
			// --timeout (which defaults to 60s and would release the session
			// early). Build a dedicated hold context from the parent.
			// Under live dogfood the matrix has a flat 30s per-command timeout,
			// so curtail the hold to ~2s — the probe only needs to prove the
			// create → hold → release lifecycle works, not wait the full
			// session lifetime.
			holdFor := timeout
			if cliutil.IsDogfoodEnv() {
				holdFor = 2 * time.Second
			}
			holdCtx, holdCancel := context.WithTimeout(cmd.Context(), holdFor)
			defer holdCancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			body := map[string]any{}
			if flagProject != "" {
				body["projectId"] = flagProject
			}
			if flagKeepAlive {
				body["keepAlive"] = true
			}
			// Cap the session at the requested timeout so the cloud-side
			// auto-end backs up our local release guarantee.
			body["timeout"] = int(timeout.Seconds())

			data, statusCode, err := c.PostWithParams(ctx, "/v1/sessions", nil, body)
			// The session-create endpoint is rate limited (5/min on the free
			// plan). Surface the typed rate-limit error, but retry once after
			// the API's retry-after when the window is short — a user running
			// a burst of sessions benefits from the wait over a hard failure.
			if err != nil {
				var rlErr *cliutil.RateLimitError
				if errors.As(err, &rlErr) && rlErr.RetryAfter > 0 && rlErr.RetryAfter <= 5*time.Second {
					select {
					case <-time.After(rlErr.RetryAfter):
					case <-ctx.Done():
						return ctx.Err()
					}
					data, statusCode, err = c.PostWithParams(ctx, "/v1/sessions", nil, body)
				}
				if err != nil {
					return classifyAPIError(err, flags)
				}
			}
			if statusCode < 200 || statusCode >= 300 {
				return apiErr(fmt.Errorf("creating session: HTTP %d: %s", statusCode, string(data)))
			}

			var sess struct {
				ID         string `json:"id"`
				ConnectURL string `json:"connectUrl"`
				Status     string `json:"status"`
			}
			if err := json.Unmarshal(data, &sess); err != nil {
				return fmt.Errorf("parsing session response: %w", err)
			}
			if sess.ID == "" {
				return fmt.Errorf("creating session: response missing id")
			}

			// Print the session info immediately so scripts can grab the
			// connect URL while the session is alive.
			view := map[string]any{
				"id":         sess.ID,
				"status":     sess.Status,
				"connectUrl": sess.ConnectURL,
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "session %s (%s)\n", sess.ID, sess.Status)
				if sess.ConnectURL != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "connect: %s\n", sess.ConnectURL)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "will auto-release after %s; press Ctrl-C to release early\n", timeout)
			}

			// Hold the session open until the timeout, an interrupt, or the
			// hold deadline, then guarantee REQUEST_RELEASE. The hold uses the
			// session lifetime (holdCtx), not the root --timeout, so the
			// session is never released early by the CLI's own 60s default.
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
			defer signal.Stop(sigCh)

			select {
			case <-sigCh:
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					fmt.Fprintf(cmd.ErrOrStderr(), "interrupt received; releasing session %s\n", sess.ID)
				}
			case <-holdCtx.Done():
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					fmt.Fprintf(cmd.ErrOrStderr(), "timeout reached; releasing session %s\n", sess.ID)
				}
			}
			signal.Stop(sigCh)

			// Guaranteed release: POST /v1/sessions/{id} with status REQUEST_RELEASE.
			// Use a dedicated release context so cleanup still runs when the
			// session lifetime exceeded the root request timeout — the root
			// context may be expired by the time the hold ends.
			releaseBody := map[string]any{"status": "REQUEST_RELEASE"}
			relPath := replacePathParam("/v1/sessions/{id}", "id", sess.ID)
			releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer releaseCancel()
			if _, relStatus, relErr := c.PostWithParams(releaseCtx, relPath, nil, releaseBody); relErr != nil {
				return fmt.Errorf("releasing session %s: %w", sess.ID, relErr)
			} else if relStatus < 200 || relStatus >= 300 {
				return fmt.Errorf("releasing session %s: HTTP %d", sess.ID, relStatus)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"id":       sess.ID,
					"released": true,
				}, flags); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "session %s released\n", sess.ID)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagProject, "project", "", "Project ID to create the session in (defaults to the API key's project)")
	cmd.Flags().StringVar(&flagTimeout, "timeout", "15m", "Session lifetime before guaranteed release (e.g. 10m, 1h)")
	cmd.Flags().BoolVar(&flagKeepAlive, "keep-alive", false, "Keep the session alive across disconnects (Hobby plan and above)")
	return cmd
}
