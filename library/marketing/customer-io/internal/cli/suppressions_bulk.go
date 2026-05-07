// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/customer-io/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/customer-io/internal/cliutil"

	"github.com/spf13/cobra"
)

// newSuppressionsBulkCmd is the parent of "bulk add" and "bulk remove": fan
// real suppress / unsuppress calls out across a CSV/JSONL/stdin list, throttle
// adaptively, and append every call to a per-day JSONL audit log.
func newSuppressionsBulkCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bulk",
		Short: "Bulk suppress or unsuppress customers from a CSV/JSONL/stdin source with an audit log",
	}
	cmd.AddCommand(newSuppressionsBulkAddCmd(flags))
	cmd.AddCommand(newSuppressionsBulkRemoveCmd(flags))
	return cmd
}

func newSuppressionsBulkAddCmd(flags *rootFlags) *cobra.Command {
	return newSuppressionsBulkOpCmd(flags, "add", "/v1/environments/{env}/customers/{id}/suppress",
		"Bulk-suppress customers from CSV/JSONL/stdin with adaptive throttle and an audit log",
	)
}

func newSuppressionsBulkRemoveCmd(flags *rootFlags) *cobra.Command {
	return newSuppressionsBulkOpCmd(flags, "remove", "/v1/environments/{env}/customers/{id}/unsuppress",
		"Bulk-unsuppress customers from CSV/JSONL/stdin with adaptive throttle and an audit log",
	)
}

func newSuppressionsBulkOpCmd(flags *rootFlags, op, pathTpl, summary string) *cobra.Command {
	var fromCSV, fromJSONL, reason, auditPath, envID string
	var rate float64
	cmd := &cobra.Command{
		Use:   op,
		Short: summary,
		Long: `Reads recipient identifiers (email or customer ID) one per line from a CSV
file, JSONL file, or stdin (when neither flag is set). Each line is fanned
out to the live ` + pathTpl + ` endpoint with adaptive
throttling. Every call — successful or otherwise — is appended to a JSONL
audit log keyed by date.

Default audit path: ~/.config/customer-io-pp-cli/audit/suppressions-YYYYMMDD.jsonl`,
		Example: strings.Trim(`
  customer-io-pp-cli suppressions bulk `+op+` --from-csv complaints.csv --reason complaint --dry-run
  customer-io-pp-cli suppressions bulk `+op+` --from-jsonl ids.jsonl
  printf "alice@example.com\nbob@example.com\n" | customer-io-pp-cli suppressions bulk `+op+`
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			// --dry-run does NOT short-circuit here: the bulk command's dry-run
			// purpose is to surface the recipient set and the audit-log path
			// that would be written. The per-recipient loop already branches
			// on flags.dryRun to skip real API calls; the only requirement we
			// drop in dry-run is --environment-id, since path templating is
			// not exercised when no calls go out.
			if !flags.dryRun && envID == "" {
				return usageErr(fmt.Errorf("--environment-id is required (find IDs via 'customer-io-pp-cli auth login' or 'workspaces')"))
			}
			ids, err := readBulkRecipients(fromCSV, fromJSONL, cmd.InOrStdin())
			if err != nil {
				return usageErr(err)
			}
			if len(ids) == 0 {
				if flags.dryRun {
					// Verify mock-harness invocations have no stdin; emit a
					// minimal plan so the probe sees output and dry-run users
					// see why nothing would happen.
					fmt.Fprintf(cmd.OutOrStdout(), "Bulk %s dry-run: 0 recipients (pass --from-csv, --from-jsonl, or pipe IDs on stdin)\n", op)
					return nil
				}
				return usageErr(fmt.Errorf("no recipients supplied; pass --from-csv, --from-jsonl, or pipe IDs on stdin"))
			}

			var c *client.Client
			if !flags.dryRun {
				c, err = flags.newClient()
				if err != nil {
					return err
				}
			}
			limiter := cliutil.NewAdaptiveLimiter(rate)
			audit, err := openAuditFile(auditPath)
			if err != nil {
				return apiErr(fmt.Errorf("opening audit log: %w", err))
			}
			defer audit.Close()

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			ok, fail := 0, 0
			results := make([]map[string]any, 0, len(ids))
			for _, id := range ids {
				if ctx.Err() != nil {
					break
				}
				limiter.Wait()
				path := strings.ReplaceAll(strings.ReplaceAll(pathTpl, "{env}", envID), "{id}", id)
				var status int
				var apiErrMsg string
				if flags.dryRun {
					status = 0
					apiErrMsg = "dry-run (no call)"
				} else {
					_, code, callErr := c.Post(path, nil)
					status = code
					if callErr != nil {
						apiErrMsg = callErr.Error()
						fail++
						limiter.OnRateLimit()
					} else {
						ok++
						limiter.OnSuccess()
					}
				}
				rec := map[string]any{
					"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
					"op":         op,
					"recipient":  id,
					"reason":     reason,
					"status":     status,
					"error":      apiErrMsg,
					"endpoint":   path,
					"dry_run":    flags.dryRun,
				}
				results = append(results, rec)
				_ = appendJSONL(audit, rec)
			}

			out := map[string]any{
				"op":        op,
				"total":     len(ids),
				"succeeded": ok,
				"failed":    fail,
				"audit_log": audit.Name(),
				"dry_run":   flags.dryRun,
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				out["records"] = results
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Bulk %s: %d total, %d succeeded, %d failed\n", op, len(ids), ok, fail)
			fmt.Fprintf(cmd.OutOrStdout(), "Audit log: %s\n", audit.Name())
			return nil
		},
	}
	cmd.Flags().StringVar(&fromCSV, "from-csv", "", "CSV file with recipient identifiers (email or customer ID) in the first column")
	cmd.Flags().StringVar(&fromJSONL, "from-jsonl", "", "JSONL file with {\"id\":\"...\"} objects")
	cmd.Flags().StringVar(&reason, "reason", "", "Reason recorded in the audit log (informational; does not change API behavior)")
	cmd.Flags().StringVar(&auditPath, "audit", "", "Override the audit log path (default: ~/.config/customer-io-pp-cli/audit/suppressions-YYYYMMDD.jsonl)")
	cmd.Flags().Float64Var(&rate, "rate-limit", 5.0, "Initial requests-per-second cap (adaptive throttling adjusts on 429)")
	cmd.Flags().StringVar(&envID, "environment-id", "", "Environment (workspace) ID; find via 'auth login' output or 'workspaces'")
	return cmd
}

func readBulkRecipients(csvPath, jsonlPath string, stdin io.Reader) ([]string, error) {
	switch {
	case csvPath != "":
		f, err := os.Open(csvPath)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		r := csv.NewReader(f)
		r.FieldsPerRecord = -1
		var out []string
		for {
			row, err := r.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			if len(row) == 0 {
				continue
			}
			id := strings.TrimSpace(row[0])
			if id != "" && !strings.EqualFold(id, "email") && !strings.EqualFold(id, "id") {
				out = append(out, id)
			}
		}
		return out, nil
	case jsonlPath != "":
		f, err := os.Open(jsonlPath)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		var out []string
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" {
				continue
			}
			var rec struct {
				ID    string `json:"id"`
				Email string `json:"email"`
			}
			if err := json.Unmarshal([]byte(line), &rec); err != nil {
				return nil, fmt.Errorf("invalid JSONL: %w", err)
			}
			id := rec.ID
			if id == "" {
				id = rec.Email
			}
			if id != "" {
				out = append(out, id)
			}
		}
		return out, sc.Err()
	default:
		// stdin
		var out []string
		sc := bufio.NewScanner(stdin)
		sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line != "" {
				out = append(out, line)
			}
		}
		return out, sc.Err()
	}
}

func openAuditFile(override string) (*os.File, error) {
	path := override
	if path == "" {
		home, _ := os.UserHomeDir()
		dir := filepath.Join(home, ".config", "customer-io-pp-cli", "audit")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, err
		}
		path = filepath.Join(dir, "suppressions-"+time.Now().UTC().Format("20060102")+".jsonl")
	} else {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, err
		}
	}
	return os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
}

func appendJSONL(f *os.File, rec any) error {
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		return err
	}
	return nil
}

// silence unused imports in error-only paths
var _ = client.New
