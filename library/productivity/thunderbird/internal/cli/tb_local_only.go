// pp:data-source local

package cli

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var errNoLiveSource = errors.New("no live source: this CLI reads the local Thunderbird profile; run sync (then use --data-source auto or local)")

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		if f := root.PersistentFlags().Lookup("data-source"); f != nil {
			f.Usage = "Data source for read commands: auto or local (the store filled by 'sync'); live is not available, this CLI has no API"
		}
		orig := root.PersistentPreRunE
		root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
			if flags.dataSource == "live" {
				return usageErr(errNoLiveSource)
			}
			if orig != nil {
				if err := orig(cmd, args); err != nil {
					return err
				}
			}
			if flags.dataSource == "live" {
				return usageErr(errNoLiveSource)
			}
			return nil
		}

		tbReplaceCommand(root, "export", newTBExportCmd(flags))
		for _, c := range root.Commands() {
			switch c.Name() {
			case "workflow":
				tbReplaceCommand(c, "archive", newTBWorkflowArchiveCmd(flags))
			case "api":
				c.Hidden = true
				if c.Annotations == nil {
					c.Annotations = map[string]string{}
				}
				c.Annotations["mcp:hidden"] = "true"
			}
		}
	})
}

func tbReplaceCommand(parent *cobra.Command, name string, repl *cobra.Command) {
	for _, c := range parent.Commands() {
		if c.Name() == name {
			parent.RemoveCommand(c)
		}
	}
	parent.AddCommand(repl)
}

func newTBExportCmd(flags *rootFlags) *cobra.Command {
	var format, outputFile string
	var limit int

	cmd := &cobra.Command{
		Use:   "export <resource> [id]",
		Short: "Export stored documents to JSONL or JSON for backup, migration, or analysis",
		Long: strings.Trim(`
Export the documents of one resource type from the local store (filled by
'sync') as JSONL (one JSON object per line, streamed) or as a JSON array.
Pass an id to export a single document.

Resources: `+strings.Join(tbResourceTypes, ", ")+`.
Nothing is fetched over the network and the Thunderbird profile is not read.`, "\n"),
		Example: strings.Trim(`
  # Export all messages as JSONL
  thunderbird-pp-cli export messages --format jsonl --output messages.jsonl

  # Export with limit
  thunderbird-pp-cli export messages --format jsonl --limit 1000

  # One message as JSON
  thunderbird-pp-cli export messages 0123456789ab --format json

  # Pipe contacts to another tool
  thunderbird-pp-cli export contacts | jq '.id'`, "\n"),
		Annotations: map[string]string{"pp:data-source": "local", "pp:typed-exit-codes": "0,2,3", "pp:happy-args": "resource=messages"},
		Args:        cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "export")
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("resource required; valid: %s", strings.Join(tbResourceTypes, ", ")))
			}
			resource := args[0]
			if !slices.Contains(tbResourceTypes, resource) {
				return usageErr(fmt.Errorf("unknown resource %q; valid: %s", resource, strings.Join(tbResourceTypes, ", ")))
			}
			if format != "jsonl" && format != "json" {
				return usageErr(fmt.Errorf("--format must be jsonl or json, got %q", format))
			}
			if limit < 0 {
				return usageErr(fmt.Errorf("--limit must be >= 0 (0 = unlimited)"))
			}

			db, err := tbOpenStore(cmd)
			if err != nil {
				return err
			}
			var rows *sql.Rows
			if db != nil {
				defer db.Close()
				if len(args) == 2 {
					var n int
					if err := db.DB().QueryRow(`SELECT COUNT(*) FROM resources WHERE resource_type = ? AND id = ?`, resource, args[1]).Scan(&n); err != nil {
						return err
					}
					if n == 0 {
						return notFoundErr(fmt.Errorf("%s %q not found in the local store", resource, args[1]))
					}
				} else {
					hintIfUnsynced(cmd, db, resource)
				}
				query := `SELECT data FROM resources WHERE resource_type = ?`
				qargs := []any{resource}
				if len(args) == 2 {
					query += ` AND id = ?`
					qargs = append(qargs, args[1])
				}
				query += ` ORDER BY id`
				if limit > 0 {
					query += ` LIMIT ?`
					qargs = append(qargs, limit)
				}
				if rows, err = db.DB().QueryContext(cmd.Context(), query, qargs...); err != nil {
					return err
				}
				defer rows.Close()
			}

			var out io.Writer = cmd.OutOrStdout()
			var file *os.File
			if outputFile != "" {
				if file, err = os.Create(filepath.Clean(outputFile)); err != nil {
					return fmt.Errorf("creating output file: %w", err)
				}
				defer func() {
					if cerr := file.Close(); err == nil && cerr != nil {
						err = fmt.Errorf("closing export file: %w", cerr)
					}
				}()
				out = file
			}
			w := bufio.NewWriter(out)
			count, err := tbWriteExport(w, rows, format, len(args) == 2)
			if err != nil {
				return err
			}
			if err := w.Flush(); err != nil {
				return fmt.Errorf("writing export: %w", err)
			}
			if outputFile != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "Exported %d records to %s\n", count, outputFile)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "jsonl", "Output format: jsonl or json")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file path (default: stdout)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum records to export (0 = unlimited)")
	return cmd
}

// tbWriteExport streams rows (nil = no store) as JSONL, a JSON array, or a
// single JSON object when single is set. Write errors are sticky in
// bufio.Writer and surface from the caller's Flush.
func tbWriteExport(w *bufio.Writer, rows *sql.Rows, format string, single bool) (int, error) {
	count := 0
	if format == "json" && !single {
		_, _ = w.WriteString("[")
	}
	for rows != nil && rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return count, err
		}
		if format == "jsonl" {
			var buf bytes.Buffer
			if err := json.Compact(&buf, raw); err != nil {
				return count, err
			}
			buf.WriteByte('\n')
			_, _ = w.Write(buf.Bytes())
		} else {
			prefix, sep := "", "\n  "
			if single {
				sep = ""
			} else {
				prefix = "  "
				if count > 0 {
					sep = ",\n  "
				}
			}
			var buf bytes.Buffer
			if err := json.Indent(&buf, raw, prefix, "  "); err != nil {
				return count, err
			}
			_, _ = w.WriteString(sep)
			_, _ = w.Write(buf.Bytes())
			if single {
				_, _ = w.WriteString("\n")
			}
		}
		count++
	}
	if rows != nil {
		if err := rows.Err(); err != nil {
			return count, err
		}
	}
	if format == "json" && !single {
		if count > 0 {
			_, _ = w.WriteString("\n")
		}
		_, _ = w.WriteString("]\n")
	}
	return count, nil
}

func newTBWorkflowArchiveCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var full bool
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "archive",
		Short: "Sync all resources to local store for offline access and search",
		Long: strings.Trim(`
Archive indexes every resource of the local Thunderbird profile (accounts,
identities, folders, messages, attachments, contacts, filters, events) into the
offline store. It is 'sync' with all resources plus a wall-clock timeout; the
profile is only read. Runs are incremental unless --full is given.`, "\n"),
		Example: strings.Trim(`
  # Archive all resources
  thunderbird-pp-cli workflow archive

  # Full re-archive (re-parse every folder)
  thunderbird-pp-cli workflow archive --full --json

  # Archive without a wall-clock timeout
  thunderbird-pp-cli workflow archive --timeout 0`, "\n"),
		Annotations: map[string]string{"mcp:local-write": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "workflow archive")
			}
			if timeout < 0 {
				return usageErr(fmt.Errorf("--timeout must be >= 0 (0 = no timeout)"))
			}
			if timeout > 0 {
				ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
				defer cancel()
				cmd.SetContext(ctx)
			}
			return runTBSyncCommand(cmd, flags, "", full, dbPath)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.Flags().BoolVar(&full, "full", false, "Ignore saved mbox checkpoints and re-parse every folder")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Minute, "Maximum time to spend archiving (0 = no timeout)")
	return cmd
}
