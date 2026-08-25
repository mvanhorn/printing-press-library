// Copyright 2026 matthew.martin and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/square/internal/store"
	"github.com/spf13/cobra"
)

const maxWebhookBodyBytes = 10 << 20
const maxWebhookFutureSkew = 5 * time.Minute

var exactWebhookSecretKeys = map[string]struct{}{
	"access-token": {}, "api-key": {}, "authorization": {}, "client-secret": {},
	"bearer-token": {}, "cookie": {}, "oauth-token": {}, "password": {}, "private-key": {},
	"refresh-token": {}, "secret": {}, "secret-key": {}, "signature": {}, "signature-key": {},
	"signing-key": {}, "signing-secret": {}, "token": {}, "webhook-signature": {},
	"x-square-hmacsha256-signature": {},
}

func normalizedSecretKey(key string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(key)), "_", "-")
}

func redactWebhookSecrets(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		redacted := make(map[string]any, len(typed))
		for key, child := range typed {
			if _, secret := exactWebhookSecretKeys[normalizedSecretKey(key)]; secret {
				redacted[key] = "<redacted>"
			} else {
				redacted[key] = redactWebhookSecrets(child)
			}
		}
		return redacted
	case []any:
		redacted := make([]any, len(typed))
		for i, child := range typed {
			redacted[i] = redactWebhookSecrets(child)
		}
		return redacted
	default:
		return typed
	}
}

func decodeWebhookBody(raw []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var body map[string]any
	if err := decoder.Decode(&body); err != nil {
		return nil, fmt.Errorf("invalid webhook JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("invalid webhook JSON: trailing content")
	}
	if body == nil {
		return nil, fmt.Errorf("webhook body must be a JSON object")
	}
	return body, nil
}

func webhookOccurredAt(body map[string]any) time.Time {
	for _, path := range [][]string{{"created_at"}, {"occurred_at"}, {"event_time"}} {
		if raw := stringValue(body, path...); raw != "" {
			if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
				return parsed
			}
		}
	}
	return time.Time{}
}

func newNovelWebhookIngestCmd(flags *rootFlags) *cobra.Command {
	var bodyPath string
	var receivedAtValue string
	var receiptID string
	cmd := &cobra.Command{
		Use:         "ingest",
		Short:       "Record one webhook delivery in the local health log.",
		Example:     "  square-pp-cli webhook ingest --body testdata/dogfood-webhook.json",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"mcp:read-only": "false", "pp:happy-args": "--body=testdata/dogfood-webhook.json"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "webhook ingest")
			}
			// #nosec G304 -- --body intentionally names a user-selected local
			// payload; it is opened read-only and validated as a regular file.
			bodyFile, err := os.Open(bodyPath)
			if err != nil {
				return fmt.Errorf("reading --body: %w", err)
			}
			defer bodyFile.Close()
			info, err := bodyFile.Stat()
			if err != nil {
				return fmt.Errorf("stating --body: %w", err)
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("--body must name a regular file")
			}
			if info.Size() > maxWebhookBodyBytes {
				return fmt.Errorf("webhook body exceeds %d-byte limit", maxWebhookBodyBytes)
			}
			raw, err := io.ReadAll(io.LimitReader(bodyFile, maxWebhookBodyBytes+1))
			if err != nil {
				return fmt.Errorf("reading --body: %w", err)
			}
			if len(raw) > maxWebhookBodyBytes {
				return fmt.Errorf("webhook body exceeds %d-byte limit", maxWebhookBodyBytes)
			}
			body, err := decodeWebhookBody(raw)
			if err != nil {
				return err
			}
			eventID := firstString(body, []string{"event_id"}, []string{"event", "id"}, []string{"id"})
			if eventID == "" {
				return fmt.Errorf("webhook body has no event_id")
			}
			eventType := firstString(body, []string{"type"}, []string{"event_type"})
			redacted := redactWebhookSecrets(body)
			persisted, err := json.Marshal(redacted)
			if err != nil {
				return fmt.Errorf("encoding redacted webhook: %w", err)
			}
			ingestNow := time.Now().UTC()
			receivedAt := ingestNow
			receivedAtSource := "ingest_time_default_now"
			if receivedAtValue != "" {
				parsed, err := time.Parse(time.RFC3339Nano, receivedAtValue)
				if err != nil {
					return fmt.Errorf("invalid --received-at %q: expected RFC3339 timestamp", receivedAtValue)
				}
				receivedAt = parsed.UTC()
				if receivedAt.After(ingestNow.Add(maxWebhookFutureSkew)) {
					return fmt.Errorf("invalid --received-at %q: timestamp is more than %s in the future", receivedAtValue, maxWebhookFutureSkew)
				}
				receivedAtSource = "provided"
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("square-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local webhook log: %w", err)
			}
			defer db.Close()
			observationID, err := db.RecordWebhookDelivery(store.WebhookDelivery{
				ReceiptID: receiptID, EventID: eventID, EventType: eventType, OccurredAt: webhookOccurredAt(body),
				ReceivedAt: receivedAt, ReceivedAtSource: receivedAtSource, Payload: persisted,
			})
			if err != nil {
				return fmt.Errorf("recording webhook delivery: %w", err)
			}
			return flags.printJSON(cmd, map[string]any{
				"data_source": "local", "recorded": true, "observation_id": observationID,
				"receipt_id": receiptID, "event_id": eventID, "event_type": eventType, "received_at": receivedAt,
				"received_at_source": receivedAtSource,
				"secrets_redacted":   true,
			})
		},
	}
	cmd.Flags().StringVar(&bodyPath, "body", "", "Path to a captured Square webhook JSON body")
	cmd.Flags().StringVar(&receivedAtValue, "received-at", "", "Actual receipt time in RFC3339 format (defaults to the current ingest time)")
	cmd.Flags().StringVar(&receiptID, "receipt-id", "", "Optional receiver-assigned receipt identifier")
	_ = cmd.MarkFlagRequired("body")
	return cmd
}
