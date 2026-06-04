// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// batchOp is one Mailchimp batch operation as documented at
// https://mailchimp.com/developer/marketing/api/batch-operations/start-batch-operation/.
type batchOp struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	Body        string `json:"body"`
	OperationID string `json:"operation_id"`
}

// batchResultEntry is one row in the tar.gz of JSONL Mailchimp returns when
// a batch finishes. The body is itself JSON; the outer struct gives us the
// status code and operation_id so we can map back to input rows.
type batchResultEntry struct {
	StatusCode  int    `json:"status_code"`
	OperationID string `json:"operation_id"`
	Response    string `json:"response"`
}

// bulkRow is the per-input-row outcome we surface to the user.
type bulkRow struct {
	OperationID string `json:"operation_id"`
	Email       string `json:"email"`
	Status      string `json:"status"`
	HTTPCode    int    `json:"http_code,omitempty"`
	Error       string `json:"error,omitempty"`
}

type bulkResult struct {
	BatchID      string    `json:"batch_id"`
	Started      time.Time `json:"started"`
	Finished     time.Time `json:"finished"`
	TotalOps     int       `json:"total_operations"`
	Errored      int       `json:"errored_operations"`
	Finished_Ops int       `json:"finished_operations"`
	Status       string    `json:"status"`
	Rows         []bulkRow `json:"rows,omitempty"`
}

func newBulkSubscribeCmd(flags *rootFlags) *cobra.Command {
	var listID string
	var csvPath string
	var tagStr string
	var statusStr string
	var watch bool
	var pollSeconds int

	cmd := &cobra.Command{
		Use:   "bulk-subscribe",
		Short: "Bulk subscribe from a CSV via Mailchimp's /batches endpoint. Decodes the tar.gz of JSONL results within the 10-minute response URL window.",
		Long: `Read a CSV of subscribers, fan out through POST /batches (the official
escape hatch from Mailchimp's 10-concurrent-connection cap), and decode the
tar.gz of JSONL results within the 10-minute response URL window.

CSV format:
  email,FNAME,LNAME             (any merge-field columns may follow email)
  alice@example.com,Alice,Smith
  bob@example.com,Bob,Jones

The first column must be 'email'. Additional columns become merge fields
(automatically uppercased: 'fname' -> FNAME). Tags from --tags apply to all rows.

Without --watch the command returns immediately after submitting the batch and
prints the batch_id; poll status with 'mailchimp-pp-cli batches get <id>'.
With --watch the command polls every <poll-seconds>, downloads the result
archive when the batch finishes, and prints per-row outcomes.`,
		Example: `  mailchimp-pp-cli bulk-subscribe --csv contacts.csv --list b7661f2918 --tags newsletter
  mailchimp-pp-cli bulk-subscribe --csv contacts.csv --list b7661f2918 --watch
  mailchimp-pp-cli bulk-subscribe --csv contacts.csv --list b7661f2918 --status pending --watch`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if listID == "" {
				return fmt.Errorf("--list is required (audience/list id)")
			}
			if csvPath == "" {
				return fmt.Errorf("--csv is required (path to CSV file with email column first)")
			}

			// Dry-run with a missing CSV is allowed — useful for example/help
			// invocations that document the shape without a real file on disk.
			// The dry-run output then describes what the command *would* do
			// given a CSV at that path; it doesn't actually read the file.
			if dryRunOK(flags) {
				if _, statErr := os.Stat(csvPath); statErr != nil {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"would_post_batch": map[string]any{
							"POST":       "/batches",
							"csv_path":   csvPath,
							"csv_status": "not found (dry-run still proceeds with the documented shape)",
							"list_id":    listID,
							"tags":       tagStr,
							"status":     statusStr,
						},
					}, flags)
				}
			}

			f, err := os.Open(csvPath)
			if err != nil {
				return fmt.Errorf("opening CSV: %w", err)
			}
			defer f.Close()
			reader := csv.NewReader(f)
			header, err := reader.Read()
			if err != nil {
				return fmt.Errorf("reading CSV header: %w", err)
			}
			if len(header) == 0 || strings.ToLower(strings.TrimSpace(header[0])) != "email" {
				return fmt.Errorf("CSV must have 'email' as the first column; got %q", header)
			}

			var tags []map[string]string
			for _, t := range strings.Split(tagStr, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					tags = append(tags, map[string]string{"name": t, "status": "active"})
				}
			}

			// Build operations array
			var ops []batchOp
			rowMeta := map[string]string{} // operation_id -> email (for result mapping)
			lineNum := 1
			for {
				row, err := reader.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					return fmt.Errorf("reading CSV line %d: %w", lineNum+1, err)
				}
				lineNum++
				if len(row) == 0 {
					continue
				}
				email := strings.TrimSpace(row[0])
				if email == "" || !strings.Contains(email, "@") {
					continue
				}
				hash := subscriberHash(email)
				// Mailchimp's PUT /lists/{id}/members/{hash} does NOT honor a
				// "tags" field in the request body — tags must go through the
				// dedicated POST /lists/{id}/members/{hash}/tags endpoint.
				// Same reason we drop "status" here as in the single subscribe
				// command: including it would forcibly change opted-out members'
				// state on re-import. Only status_if_new applies.
				body := map[string]any{
					"email_address": email,
					"status_if_new": statusStr,
				}
				mf := map[string]string{}
				for i := 1; i < len(row) && i < len(header); i++ {
					key := strings.ToUpper(strings.TrimSpace(header[i]))
					val := strings.TrimSpace(row[i])
					if key != "" && val != "" {
						mf[key] = val
					}
				}
				if len(mf) > 0 {
					body["merge_fields"] = mf
				}
				bodyJSON, _ := json.Marshal(body)
				opID := fmt.Sprintf("bulk-%d", lineNum)
				ops = append(ops, batchOp{
					Method:      "PUT",
					Path:        fmt.Sprintf("/lists/%s/members/%s", listID, hash),
					Body:        string(bodyJSON),
					OperationID: opID,
				})
				rowMeta[opID] = email

				// If --tags was passed, append a second batch operation per row
				// that POSTs the tag set to the member-tags endpoint. This is
				// the only correct path: silently dropping --tags would let
				// users believe their tagging happened when the API ignored it.
				if len(tags) > 0 {
					tagsBody, _ := json.Marshal(map[string]any{"tags": tags})
					tagsOpID := fmt.Sprintf("bulk-%d-tags", lineNum)
					ops = append(ops, batchOp{
						Method:      "POST",
						Path:        fmt.Sprintf("/lists/%s/members/%s/tags", listID, hash),
						Body:        string(tagsBody),
						OperationID: tagsOpID,
					})
					rowMeta[tagsOpID] = email + " (tags)"
				}
			}
			if len(ops) == 0 {
				return fmt.Errorf("no valid rows in %s", csvPath)
			}

			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"would_post_batch": map[string]any{
						"POST":        "/batches",
						"operations":  len(ops),
						"first_email": rowMeta[ops[0].OperationID],
					},
				}, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			batchBody := map[string]any{"operations": ops}
			submit, _, err := c.Post("/batches", batchBody)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var submitResp map[string]any
			_ = json.Unmarshal(submit, &submitResp)
			batchID, _ := submitResp["id"].(string)
			if batchID == "" {
				return fmt.Errorf("batch submitted but no id in response: %s", string(submit))
			}

			result := bulkResult{
				BatchID:  batchID,
				Started:  time.Now(),
				TotalOps: len(ops),
				Status:   "pending",
			}

			if !watch {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			// Watch mode: poll until finished, then decode result archive.
			if pollSeconds <= 0 {
				pollSeconds = 5
			}
			for {
				time.Sleep(time.Duration(pollSeconds) * time.Second)
				data, err := c.GetNoCache(fmt.Sprintf("/batches/%s", batchID), nil)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var st map[string]any
				_ = json.Unmarshal(data, &st)
				if v, ok := st["status"].(string); ok {
					result.Status = v
				}
				if v, ok := st["finished_operations"].(float64); ok {
					result.Finished_Ops = int(v)
				}
				if v, ok := st["errored_operations"].(float64); ok {
					result.Errored = int(v)
				}
				if result.Status == "finished" {
					if v, ok := st["response_body_url"].(string); ok {
						// Download + decode the tar.gz of JSONL. URL is valid 10 minutes.
						rows, derr := decodeBatchArchive(v, rowMeta)
						if derr != nil {
							return fmt.Errorf("decoding batch results: %w", derr)
						}
						result.Rows = rows
					}
					result.Finished = time.Now()
					break
				}
				if result.Status == "failed" {
					return fmt.Errorf("batch %s failed: %v", batchID, st)
				}
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&listID, "list", "", "Audience (list) ID")
	cmd.Flags().StringVar(&csvPath, "csv", "", "Path to CSV file (first column must be 'email')")
	cmd.Flags().StringVar(&tagStr, "tags", "", "Comma-separated tags to apply to every row")
	cmd.Flags().StringVar(&statusStr, "status", "subscribed", "Initial status: subscribed, pending (double opt-in), cleaned, unsubscribed")
	cmd.Flags().BoolVar(&watch, "watch", false, "Poll until the batch finishes, then download and decode results")
	cmd.Flags().IntVar(&pollSeconds, "poll-seconds", 5, "Seconds between status polls when --watch is set")
	return cmd
}

// decodeBatchArchive downloads the response_body_url archive, untars + ungzips
// it, parses the JSONL stream of result rows, and maps each operation_id back
// to the original email so the user gets actionable per-row outcomes.
//
// The download uses a bounded-timeout HTTP client rather than http.DefaultClient
// because Mailchimp's response_body_url is valid only 10 minutes after batch
// completion, and a stalled S3 transfer would otherwise hang --watch mode
// indefinitely while the URL silently expires. The 5-minute timeout is
// well under the 10-minute window and well above the time a healthy
// download takes (a 50k-row batch archive is typically ~5-20 MB).
func decodeBatchArchive(url string, rowMeta map[string]string) ([]bulkRow, error) {
	dlClient := &http.Client{Timeout: 5 * time.Minute}
	resp, err := dlClient.Get(url) // #nosec G107 — URL comes from Mailchimp API response
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("batch archive download returned %d (URL may have expired the 10-minute window)", resp.StatusCode)
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	var rows []bulkRow
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		// Each tar entry is a JSONL file. One result per line.
		scanner := bufio.NewScanner(tr)
		scanner.Buffer(make([]byte, 1024*1024), 16*1024*1024)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			var entry batchResultEntry
			if err := json.Unmarshal(line, &entry); err != nil {
				continue
			}
			row := bulkRow{
				OperationID: entry.OperationID,
				Email:       rowMeta[entry.OperationID],
				HTTPCode:    entry.StatusCode,
			}
			if entry.StatusCode >= 200 && entry.StatusCode < 300 {
				row.Status = "subscribed"
			} else {
				row.Status = "error"
				// Try to extract the error detail from the inner JSON response.
				var inner map[string]any
				if json.Unmarshal([]byte(entry.Response), &inner) == nil {
					if d, ok := inner["detail"].(string); ok {
						row.Error = d
					} else if t, ok := inner["title"].(string); ok {
						row.Error = t
					}
				}
				if row.Error == "" {
					row.Error = fmt.Sprintf("HTTP %d", entry.StatusCode)
				}
			}
			rows = append(rows, row)
		}
	}
	return rows, nil
}
