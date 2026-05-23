package cli

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/zoho-expense/internal/zohotools"
)

func newInvoiceCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "invoice",
		Short: "Batch-ingest receipts and invoices",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newInvoiceIngestCmd(flags))
	return cmd
}

func newInvoiceIngestCmd(flags *rootFlags) *cobra.Command {
	var concurrency int
	var autoTag bool
	var wait bool
	var force bool
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:   "ingest <dir-or-file>",
		Short: "Batch upload a folder of invoices to Zoho Expense; SHA256-dedups, polls autoscan in parallel, optionally auto-tags",
		Example: strings.Trim(`
  zoho-expense-pp-cli invoice ingest ./inbox
  zoho-expense-pp-cli invoice ingest ./inbox --auto-tag --concurrency 5
  zoho-expense-pp-cli invoice ingest ./inbox --dry-run --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			root := args[0]
			info, err := os.Stat(root)
			if err != nil {
				// Short-circuit cleanly on dry-run for non-existent paths so verify probes don't hard-fail.
				if dryRunOK(flags) {
					if flags.asJSON {
						return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
							"ingested": []any{}, "skipped": []any{}, "failed": []any{},
							"note": "dry-run: input path does not exist",
						}, flags)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "dry-run: would scan %s\n", root)
					return nil
				}
				return usageErr(fmt.Errorf("input path: %w", err))
			}

			files := []string{}
			if info.IsDir() {
				err = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					if info.IsDir() {
						return nil
					}
					ext := strings.ToLower(filepath.Ext(p))
					switch ext {
					case ".jpg", ".jpeg", ".png", ".pdf", ".webp", ".heic":
						files = append(files, p)
					}
					return nil
				})
				if err != nil {
					return fmt.Errorf("walking %s: %w", root, err)
				}
			} else {
				files = []string{root}
			}

			if len(files) == 0 {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"ingested": []any{}, "skipped": []any{}, "failed": []any{},
					}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "no files to ingest")
				return nil
			}

			if dryRunOK(flags) {
				// Just compute hashes + dedup state.
				s, err := openZohoStore(cmd.Context())
				if err != nil {
					return err
				}
				defer s.Close()
				summary := map[string]any{"would_upload": 0, "would_skip": 0, "files": []map[string]any{}}
				filesList := []map[string]any{}
				wouldUpload := 0
				wouldSkip := 0
				for _, f := range files {
					row := map[string]any{"file": filepath.Base(f)}
					// best-effort hash; failures are reported but don't abort
					hash, herr := hashAndDedup(s.DB(), f)
					if herr != nil {
						row["error"] = herr.Error()
					} else if hash.duplicate {
						row["status"] = "duplicate"
						row["expense_id"] = hash.existingID
						wouldSkip++
					} else {
						row["status"] = "would_upload"
						wouldUpload++
					}
					filesList = append(filesList, row)
				}
				summary["would_upload"] = wouldUpload
				summary["would_skip"] = wouldSkip
				summary["files"] = filesList
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), summary, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "dry-run: would_upload=%d, would_skip=%d\n", wouldUpload, wouldSkip)
				return nil
			}

			s, err := openZohoStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()

			if concurrency < 1 {
				concurrency = 3
			}
			sem := make(chan struct{}, concurrency)
			var mu sync.Mutex
			var wg sync.WaitGroup
			ingested := []map[string]any{}
			skipped := []map[string]any{}
			failed := []map[string]any{}
			tagged := 0

			for _, f := range files {
				f := f
				wg.Add(1)
				sem <- struct{}{}
				go func() {
					defer wg.Done()
					defer func() { <-sem }()
					result, err := uploadReceiptOnce(cmd.Context(), flags, s.DB(), f, force, wait, autoTag, timeout)
					mu.Lock()
					defer mu.Unlock()
					if err != nil {
						failed = append(failed, map[string]any{
							"file":  filepath.Base(f),
							"error": err.Error(),
						})
						return
					}
					// Any non-uploaded outcome belongs in `skipped`; only the
					// `uploaded` shape (which carries an expense_id) belongs
					// in `ingested`. Previously the check was status=="duplicate"
					// only, which let the race-skipped sentinel (from a parallel
					// reservation loss) leak into ingested without an
					// expense_id — inflating the "uploaded" count for files
					// that were not in fact uploaded. PATCH per Greptile P1.
					if result["status"] != "uploaded" {
						skipped = append(skipped, result)
						return
					}
					ingested = append(ingested, result)
					if v, ok := result["auto_tagged"].(bool); ok && v {
						tagged++
					}
				}()
			}
			wg.Wait()

			out := map[string]any{
				"ingested": ingested,
				"skipped":  skipped,
				"failed":   failed,
				"summary": map[string]any{
					"uploaded":    len(ingested),
					"skipped":     len(skipped),
					"failed":      len(failed),
					"auto_tagged": tagged,
				},
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "uploaded %d, skipped (dup) %d, failed %d, tagged auto %d\n",
				len(ingested), len(skipped), len(failed), tagged)
			if len(failed) > 0 {
				for _, f := range failed {
					fmt.Fprintf(cmd.ErrOrStderr(), "  failed %v: %v\n", f["file"], f["error"])
				}
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&concurrency, "concurrency", 3, "Parallel uploads")
	cmd.Flags().BoolVar(&autoTag, "auto-tag", false, "Apply merchant memory after autoscan")
	cmd.Flags().BoolVar(&wait, "wait", true, "Wait for autoscan per file (default true)")
	cmd.Flags().BoolVar(&force, "force", false, "Bypass SHA256 dedup")
	cmd.Flags().DurationVar(&timeout, "ingest-timeout", 60*time.Second, "Per-file autoscan poll timeout")
	return cmd
}

type dedupResult struct {
	hash       string
	duplicate  bool
	existingID string
}

// hashAndDedup computes the SHA256 of the file at path and consults
// receipt_hashes for an existing expense_id. Used by the --dry-run path
// of `invoice ingest` to preview which files would upload vs skip.
func hashAndDedup(db *sql.DB, path string) (dedupResult, error) {
	hash, err := zohotools.HashFile(path)
	if err != nil {
		return dedupResult{}, err
	}
	id, found, err := zohotools.LookupHash(db, hash)
	if err != nil {
		return dedupResult{hash: hash}, err
	}
	return dedupResult{hash: hash, duplicate: found, existingID: id}, nil
}
