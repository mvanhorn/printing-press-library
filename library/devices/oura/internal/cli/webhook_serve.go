// Copyright 2026 slinsmaier and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/devices/oura/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/oura/internal/store"
)

func newNovelWebhookServeCmd(flags *rootFlags) *cobra.Command {
	var flagPort int
	var flagPath string
	var flagRegister bool
	var flagCallbackURL string
	var flagEventType string
	var flagDataType string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start a local HTTP server that registers with Oura's webhook API and writes incoming events to the local SQLite store",
		Long: `Starts a local HTTP server that receives Oura webhook POST callbacks and
writes each event to the local store as it arrives, instead of polling on
demand like every other Oura tool. Oura requires the callback URL to be
publicly reachable — front this with a tunnel (e.g. ngrok) or a public
endpoint and pass that URL via --callback-url --register to subscribe.`,
		Example: `  oura-pp-cli webhook serve --port 8080
  oura-pp-cli webhook serve --port 8080 --register --callback-url https://example.com/oura/webhook --event-type create --data-type tag`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		// pp:data-source auto
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			if dryRunOK(flags) {
				fmt.Fprintf(out, "would listen on :%d%s and write incoming webhook events to the local store\n", flagPort, flagPath)
				if flagRegister {
					fmt.Fprintf(out, "would register subscription: callback_url=%s event_type=%s data_type=%s\n", flagCallbackURL, flagEventType, flagDataType)
				}
				return nil
			}
			if cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() {
				fmt.Fprintf(out, "would listen on :%d%s (skipped: verify/dogfood environment)\n", flagPort, flagPath)
				return nil
			}
			if flagRegister && (flagCallbackURL == "" || flagEventType == "" || flagDataType == "") {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--register requires --callback-url, --event-type, and --data-type"))
			}

			if dbPath == "" {
				dbPath = defaultDBPath("oura-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if flagRegister {
				if err := registerWebhookSubscription(cmd, flags, flagCallbackURL, flagEventType, flagDataType); err != nil {
					return fmt.Errorf("registering webhook subscription: %w", err)
				}
				fmt.Fprintln(out, "registered webhook subscription:", flagCallbackURL)
			}

			mux := http.NewServeMux()
			mux.HandleFunc(flagPath, webhookEventHandler(db, out))
			addr := fmt.Sprintf(":%d", flagPort)
			fmt.Fprintf(out, "listening on %s%s — press Ctrl+C to stop\n", addr, flagPath)
			server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				return fmt.Errorf("webhook server: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&flagPort, "port", 8080, "Local port to listen on for incoming webhook callbacks")
	cmd.Flags().StringVar(&flagPath, "path", "/oura/webhook", "URL path Oura will POST events to")
	cmd.Flags().BoolVar(&flagRegister, "register", false, "Also register a webhook subscription with Oura pointing at --callback-url")
	cmd.Flags().StringVar(&flagCallbackURL, "callback-url", "", "Publicly reachable callback URL to register with Oura (requires --register)")
	cmd.Flags().StringVar(&flagEventType, "event-type", "", "Event type to subscribe to: create, update, or delete (requires --register)")
	cmd.Flags().StringVar(&flagDataType, "data-type", "", "Oura data type to subscribe to, e.g. tag, sleep, workout (requires --register)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func registerWebhookSubscription(cmd *cobra.Command, flags *rootFlags, callbackURL, eventType, dataType string) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	body := map[string]string{
		"callback_url": callbackURL,
		"event_type":   eventType,
		"data_type":    dataType,
	}
	_, _, err = c.Post(cmd.Context(), "/v2/webhook/subscription", body)
	return err
}

func webhookEventHandler(db *store.Store, out io.Writer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB limit
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}

		id := uuid.NewString()
		if err := db.Upsert("webhook-event", id, body); err != nil {
			fmt.Fprintf(out, "[%s] failed to store webhook event: %v\n", time.Now().Format(time.RFC3339), err)
			http.Error(w, "failed to store event", http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(out, "[%s] stored webhook event %s (%d bytes)\n", time.Now().Format(time.RFC3339), id, len(body))
		w.WriteHeader(http.StatusOK)
	}
}
