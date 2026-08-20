// Copyright 2026 Stephan Stoeber and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/bird/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/bird/internal/store"
	"github.com/spf13/cobra"
)

type sendBatchRow struct {
	BatchID        string `json:"batchId"`
	Recipient      string `json:"recipient"`
	Body           string `json:"body"`
	IdempotencyKey string `json:"idempotencyKey"`
	MessageID      string `json:"messageId,omitempty"`
	Status         string `json:"status"`
	Error          string `json:"error,omitempty"`
}

type sendBatchSummary struct {
	BatchID   string         `json:"batchId"`
	ChannelID string         `json:"channelId"`
	Total     int            `json:"total"`
	Sent      int            `json:"sent"`
	Failed    int            `json:"failed"`
	DryRun    bool           `json:"dryRun"`
	Rows      []sendBatchRow `json:"rows"`
	CreatedAt string         `json:"createdAt"`
}

func newSmsSendBatchCmd(flags *rootFlags) *cobra.Command {
	var (
		csvPath      string
		bodyTemplate string
		channelID    string
		dbPath       string
		apply        bool
	)
	cmd := &cobra.Command{
		Use:   "send-batch",
		Short: "Send a batch of SMS messages from a CSV; persist the batch in the local store.",
		Long: `Reads recipients from a CSV (header row required; each row's columns are
available to the --body-template as {{column}}), assigns a deterministic
idempotency key per row, and sends each row via the Channels API. The batch
is persisted in the local store under resource_type "sms_batch" so
'sms reconcile' can re-fetch interactions later.

By default this runs in dry mode and prints the plan. Pass --apply to
actually send. Verify mode (PRINTING_PRESS_VERIFY=1) always short-circuits
to dry behavior.`,
		Example: `  bird-pp-cli sms send-batch --csv recipients.csv --body-template "Hi {{name}}, code {{code}}" --json
  bird-pp-cli sms send-batch --csv recipients.csv --body-template "Reminder: {{event}}" --apply --json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if csvPath == "" {
				return fmt.Errorf("--csv is required (path to recipients CSV)")
			}
			if bodyTemplate == "" {
				return fmt.Errorf("--body-template is required (e.g. \"Hi {{name}}, code {{code}}\")")
			}
			if channelID == "" {
				channelID = defaultChannelID()
			}
			if channelID == "" {
				return fmt.Errorf("--channel-id is required (or set BIRD_CHANNEL_ID)")
			}
			rows, err := readBatchCSV(csvPath, bodyTemplate)
			if err != nil {
				return err
			}
			batchID := newBatchID()
			summary := sendBatchSummary{
				BatchID:   batchID,
				ChannelID: channelID,
				Total:     len(rows),
				DryRun:    !apply || cliutil.IsVerifyEnv(),
				CreatedAt: time.Now().UTC().Format(time.RFC3339),
				Rows:      make([]sendBatchRow, 0, len(rows)),
			}
			if dbPath == "" {
				dbPath = defaultDBPath("bird-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			for _, r := range rows {
				row := sendBatchRow{
					BatchID:        batchID,
					Recipient:      r.recipient,
					Body:           r.body,
					IdempotencyKey: rowIdempotencyKey(batchID, r.recipient, r.body),
					Status:         "planned",
				}
				if !summary.DryRun {
					if err := sendOneRow(flags, channelID, &row); err != nil {
						row.Status = "failed"
						row.Error = err.Error()
						summary.Failed++
					} else {
						summary.Sent++
					}
				}
				summary.Rows = append(summary.Rows, row)
			}
			if !summary.DryRun {
				if err := persistBatch(db, summary); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to persist batch %s: %v\n", batchID, err)
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), summary, flags)
		},
	}
	cmd.Flags().StringVar(&csvPath, "csv", "", "Path to recipients CSV (header row required)")
	cmd.Flags().StringVar(&bodyTemplate, "body-template", "", "Body template using {{column}} placeholders")
	cmd.Flags().StringVar(&channelID, "channel-id", "", "SMS channel ID (or set BIRD_CHANNEL_ID)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually send (default is dry-run)")
	return cmd
}

type batchPlanRow struct {
	recipient string
	body      string
}

// readBatchCSV reads the CSV at path and applies bodyTemplate to each row.
// The first column is treated as the recipient phone number unless a column
// named "to" or "phone" is present, in which case that column wins.
func readBatchCSV(path, bodyTemplate string) ([]batchPlanRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.TrimLeadingSpace = true
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("reading CSV header: %w", err)
	}
	recipientIdx := 0
	for i, col := range header {
		l := strings.ToLower(strings.TrimSpace(col))
		if l == "to" || l == "phone" || l == "phonenumber" || l == "msisdn" {
			recipientIdx = i
			break
		}
	}
	rows := make([]batchPlanRow, 0, 32)
	for {
		rec, err := r.Read()
		if err != nil {
			// PATCH: distinguish normal EOF from real read/parse errors.
			// Surfaced by Greptile P1 in PR #417 review. Bird-specific
			// compound command — not in any generator template.
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("reading CSV row %d: %w", len(rows)+1, err)
		}
		if len(rec) == 0 {
			continue
		}
		row := batchPlanRow{recipient: strings.TrimSpace(rec[recipientIdx])}
		row.body = applyTemplate(bodyTemplate, header, rec)
		rows = append(rows, row)
	}
	return rows, nil
}

func applyTemplate(tmpl string, header, rec []string) string {
	out := tmpl
	for i, col := range header {
		if i >= len(rec) {
			continue
		}
		ph := "{{" + strings.TrimSpace(col) + "}}"
		out = strings.ReplaceAll(out, ph, rec[i])
	}
	return out
}

func newBatchID() string {
	// PATCH: suffix 4 random hex bytes so concurrent send-batch invocations
	// within the same second produce distinct batch IDs. Surfaced by
	// Greptile P2 in PR #417 review. Bird-specific compound command —
	// not in any generator template.
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		buf = []byte{0, 0, 0, 0}
	}
	return "batch_" + time.Now().UTC().Format("20060102_150405") + "_" + hex.EncodeToString(buf)
}

func rowIdempotencyKey(batchID, recipient, body string) string {
	h := sha256.New()
	h.Write([]byte(batchID))
	h.Write([]byte("|"))
	h.Write([]byte(recipient))
	h.Write([]byte("|"))
	h.Write([]byte(body))
	return "ik_" + hex.EncodeToString(h.Sum(nil))[:16]
}

func sendOneRow(flags *rootFlags, channelID string, row *sendBatchRow) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	payload := buildFlatSendPayload(flatSmsSendArgs{
		to:             row.Recipient,
		body:           row.Body,
		channelID:      channelID,
		idempotencyKey: row.IdempotencyKey,
	})
	path := fmt.Sprintf("/channels/%s/messages", channelID)
	data, _, err := c.Post(path, payload)
	if err != nil {
		return err
	}
	var resp map[string]any
	if json.Unmarshal(data, &resp) == nil {
		if id, ok := resp["id"].(string); ok {
			row.MessageID = id
		}
		if s, ok := resp["status"].(string); ok {
			row.Status = s
		}
		if row.Status == "" {
			row.Status = "accepted"
		}
	} else {
		row.Status = "accepted"
	}
	return nil
}

func persistBatch(db *store.Store, summary sendBatchSummary) error {
	body, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	return db.Upsert("sms_batch", summary.BatchID, body)
}
