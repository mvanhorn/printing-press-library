package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/zoho-expense/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/zoho-expense/internal/zohotools"
)

func newReceiptCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "receipt",
		Short: "Upload receipts to Zoho Expense with SHA256 hash dedup and autoscan polling",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newReceiptUploadCmd(flags))
	return cmd
}

func newReceiptUploadCmd(flags *rootFlags) *cobra.Command {
	var autoTag bool
	var wait bool
	var force bool
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:   "upload <file>",
		Short: "Upload a single receipt; SHA256-dedups against the local store and polls autoscan to completion",
		Example: strings.Trim(`
  zoho-expense-pp-cli receipt upload ./receipt.jpg
  zoho-expense-pp-cli receipt upload ./receipt.pdf --auto-tag
  zoho-expense-pp-cli receipt upload ./receipt.png --wait=false
  zoho-expense-pp-cli receipt upload ./receipt.jpg --force --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			path := args[0]
			info, err := os.Stat(path)
			if err != nil {
				return usageErr(fmt.Errorf("receipt file: %w", err))
			}
			if info.IsDir() {
				return usageErr(fmt.Errorf("%s is a directory — use 'invoice ingest' for batches", path))
			}

			s, err := openZohoStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()

			result, err := uploadReceiptOnce(cmd.Context(), flags, s.DB(), path, force, wait, autoTag, timeout)
			if err != nil {
				return err
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			if flags.quiet {
				if id, ok := result["expense_id"].(string); ok && id != "" {
					fmt.Fprintln(cmd.OutOrStdout(), id)
				}
				return nil
			}
			status := result["status"]
			if status == "duplicate" {
				fmt.Fprintf(cmd.OutOrStdout(), "duplicate (already uploaded as expense_id=%v); use --force to override\n", result["expense_id"])
				return nil
			}
			if status == "race-skipped" {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: a concurrent reservation is active for this file — another upload of the same content is in progress or a previous run crashed mid-upload; re-run after %s or use --force\n", zohotools.ReservationTTL)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "uploaded expense_id=%v autoscan=%v\n", result["expense_id"], result["autoscan_status"])
			return nil
		},
	}
	cmd.Flags().BoolVar(&autoTag, "auto-tag", false, "After autoscan completes, apply merchant memory mappings via PUT /expenses/{id}")
	cmd.Flags().BoolVar(&wait, "wait", true, "Block until autoscan completes (or timeout); pass --wait=false to fire-and-forget")
	cmd.Flags().BoolVar(&force, "force", false, "Skip SHA256 hash dedup and upload anyway")
	cmd.Flags().DurationVar(&timeout, "timeout", 60*time.Second, "Autoscan poll timeout")
	return cmd
}

// uploadReceiptOnce runs the full single-receipt pipeline so `invoice ingest`
// can share it. Returns a result map describing the outcome.
//
// PATCH(2026-05-23): Switched the hash-gate from LookupHash → upload →
// RecordHash to ReserveHash → upload → ConfirmHash/ReleaseHash. The
// previous pattern had a TOCTOU race under parallel ingestion: N
// goroutines processing files with identical content all see the hash
// missing, all POST /expenses, all get distinct expense_ids, and only
// the last RecordHash wins — the earlier uploads become ghost expenses
// that the next run's hash gate can no longer catch. The reservation
// pattern serializes claim-the-slot across goroutines without
// serializing the upload itself. Named return values let the
// failure-path ReleaseHash defer fire cleanly. Filed per Greptile P1.
func uploadReceiptOnce(ctx context.Context, flags *rootFlags, db *sql.DB, path string, force, wait, autoTag bool, timeout time.Duration) (result map[string]any, retErr error) {
	hash, err := zohotools.HashFile(path)
	if err != nil {
		return nil, fmt.Errorf("hashing %s: %w", path, err)
	}
	reserved := false
	if !force {
		claimed, existingID, err := zohotools.ReserveHash(db, hash, filepath.Base(path))
		if err != nil {
			return nil, fmt.Errorf("reserving hash: %w", err)
		}
		if !claimed {
			if existingID != "" {
				// Real duplicate — earlier upload's expense_id is recorded.
				return map[string]any{
					"status":     "duplicate",
					"expense_id": existingID,
					"hash":       hash,
					"file":       filepath.Base(path),
				}, nil
			}
			// Sentinel still present — a sibling goroutine in this batch is
			// mid-upload of the same content. Skip rather than create a
			// duplicate Zoho expense; the sibling will record the canonical
			// expense_id.
			return map[string]any{
				"status": "race-skipped",
				"hash":   hash,
				"file":   filepath.Base(path),
				"note":   "concurrent upload of identical content in this batch; one sibling will be recorded",
			}, nil
		}
		reserved = true
		defer func() {
			// Release the reservation on any error path so the next run can
			// retry. Successful uploads call ConfirmHash below; this defer
			// is a no-op for confirmed rows because ReleaseHash only
			// deletes rows whose expense_id is still the empty sentinel.
			if retErr != nil && reserved {
				_ = zohotools.ReleaseHash(db, hash)
			}
		}()
	}

	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	fileBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	postResult, err := uploadReceiptFile(ctx, c, fileBytes, filepath.Base(path))
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	expenseID := postResult.expenseID
	if expenseID == "" {
		return nil, apiErr(fmt.Errorf("upload succeeded but Zoho response had no expense_id; raw: %s", string(postResult.raw)))
	}
	if force {
		// --force callers skipped reservation entirely; use the legacy
		// upsert path so a stale row from a prior reservation gets
		// overwritten with the new expense_id.
		if err := zohotools.RecordHash(db, hash, expenseID, filepath.Base(path)); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		}
	} else {
		// Confirm the reservation, replacing the empty sentinel with the
		// real expense_id. After this point the row counts as a recorded
		// upload and ReleaseHash will not touch it.
		if err := zohotools.ConfirmHash(db, hash, expenseID); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		}
	}

	result = map[string]any{
		"status":     "uploaded",
		"expense_id": expenseID,
		"hash":       hash,
		"file":       filepath.Base(path),
	}

	if wait {
		poll, err := zohotools.PollAutoscan(ctx, c, expenseID, timeout)
		if poll != nil {
			result["autoscan_status"] = poll.Status
			result["expense"] = poll.Expense
		}
		if err != nil {
			result["autoscan_error"] = err.Error()
		}
		if autoTag && poll != nil && poll.Expense != nil && err == nil {
			if tagErr := autoTagFromMemoryDB(ctx, c, db, expenseID, poll.Expense); tagErr != nil {
				result["auto_tag_error"] = tagErr.Error()
			} else {
				result["auto_tagged"] = true
			}
		}
	}
	return result, nil
}

// uploadResult bundles the parsed expense_id and the raw response body
// from POST /expenses with a receipt attachment.
type uploadResult struct {
	expenseID string
	raw       json.RawMessage
}

func uploadReceiptFile(ctx context.Context, c *client.Client, fileBytes []byte, filename string) (*uploadResult, error) {
	raw, status, err := c.PostMultipart(ctx, "/expenses", nil, "receipt", filename, fileBytes)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("upload failed: HTTP %d", status)
	}
	var env struct {
		Expense struct {
			ExpenseID string `json:"expense_id"`
		} `json:"expense"`
	}
	if err := json.Unmarshal(raw, &env); err == nil && env.Expense.ExpenseID != "" {
		return &uploadResult{expenseID: env.Expense.ExpenseID, raw: raw}, nil
	}
	var bare map[string]any
	if err := json.Unmarshal(raw, &bare); err == nil {
		if id, ok := bare["expense_id"].(string); ok && id != "" {
			return &uploadResult{expenseID: id, raw: raw}, nil
		}
		if exp, ok := bare["expense"].(map[string]any); ok {
			if id, ok := exp["expense_id"].(string); ok && id != "" {
				return &uploadResult{expenseID: id, raw: raw}, nil
			}
		}
	}
	return &uploadResult{raw: raw}, nil
}

// autoTagFromMemoryDB looks up the merchant in the local memory table and
// PUTs category_id/project_id onto the expense.
func autoTagFromMemoryDB(ctx context.Context, c *client.Client, db *sql.DB, expenseID string, expense map[string]any) error {
	merchant := asStringOpt(expense, "merchant_name")
	if merchant == "" {
		return fmt.Errorf("no merchant_name on expense")
	}
	mapping, err := zohotools.GetMerchant(db, merchant)
	if err != nil {
		return fmt.Errorf("merchant lookup: %w", err)
	}
	if mapping == nil || (mapping.CategoryID == "" && mapping.ProjectID == "") {
		return fmt.Errorf("no merchant memory for %q", merchant)
	}
	body := map[string]any{}
	if mapping.CategoryID != "" {
		body["category_id"] = mapping.CategoryID
	}
	if mapping.ProjectID != "" {
		body["project_id"] = mapping.ProjectID
	}
	if len(body) == 0 {
		return nil
	}
	_, _, err = c.Put(ctx, "/expenses/"+expenseID, body)
	if err != nil {
		return fmt.Errorf("PUT /expenses/%s: %w", expenseID, err)
	}
	return nil
}
