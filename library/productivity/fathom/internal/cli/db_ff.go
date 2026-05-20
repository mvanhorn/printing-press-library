package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/fathom/internal/store"
	"github.com/spf13/cobra"
)

// PATCH(db-export-restore): added db export/restore commands for local store backup without API calls.
func newDBCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db",
		Short: "Local store management — export and restore without touching the API",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newDBExportCmd(flags))
	cmd.AddCommand(newDBRestoreCmd(flags))
	return cmd
}

func newDBExportCmd(flags *rootFlags) *cobra.Command {
	var output string
	var format string
	var resourceType string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export local store to JSONL or SQLite — no API calls",
		Long: `Dump the local SQLite store to a portable file. Run sync first to
populate the store, then export to share the data with another machine
or agent without re-syncing.

JSONL format writes one JSON record per line, one resource type at a time.
SQLite format copies the raw database file.`,
		Example: strings.TrimLeft(`
  fathom-pp-cli db export --format jsonl --output ~/fathom-backup.jsonl
  fathom-pp-cli db export --format jsonl --output ~/fathom-backup.jsonl --resource meetings
  fathom-pp-cli db export --format sqlite --output ~/fathom-backup.db`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "jsonl" && format != "sqlite" {
				return fmt.Errorf("--format is required: must be 'jsonl' or 'sqlite'")
			}
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("fathom-pp-cli")
			}

			if format == "sqlite" {
				if output == "" {
					return fmt.Errorf("--output is required for sqlite format")
				}
				if err := copyFile(dbPath, output); err != nil {
					return fmt.Errorf("copying database: %w", err)
				}
				fi, _ := os.Stat(output)
				size := int64(0)
				if fi != nil {
					size = fi.Size()
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Exported SQLite database to %s (%d bytes)\n", output, size)
				return nil
			}

			// JSONL format
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w\nRun 'fathom-pp-cli sync' first", err)
			}
			defer db.Close()

			var w io.Writer = cmd.OutOrStdout()
			var f *os.File
			if output != "" {
				if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
					return fmt.Errorf("creating output directory: %w", err)
				}
				f, err = os.Create(output)
				if err != nil {
					return fmt.Errorf("creating output file: %w", err)
				}
				defer f.Close()
				w = f
			}

			bw := bufio.NewWriterSize(w, 1<<20) // 1MB write buffer
			defer bw.Flush()

			// Determine resource types to export
			var types []string
			if resourceType != "" {
				types = []string{resourceType}
			} else {
				rows, err := db.DB().QueryContext(cmd.Context(),
					`SELECT DISTINCT resource_type FROM resources ORDER BY resource_type`)
				if err != nil {
					return fmt.Errorf("listing resource types: %w", err)
				}
				defer rows.Close()
				for rows.Next() {
					var rt string
					if err := rows.Scan(&rt); err == nil {
						types = append(types, rt)
					}
				}
			}

			total := 0
			for _, rt := range types {
				rows, err := db.DB().QueryContext(cmd.Context(),
					`SELECT data FROM resources WHERE resource_type = ? ORDER BY synced_at`, rt)
				if err != nil {
					return fmt.Errorf("querying %s: %w", rt, err)
				}

				count := 0
				for rows.Next() {
					var raw []byte
					if err := rows.Scan(&raw); err != nil {
						rows.Close()
						return fmt.Errorf("scanning %s: %w", rt, err)
					}
					// Write as JSONL envelope: {"resource_type":"meetings","data":{...}}
					rtJSON, _ := json.Marshal(rt) // PATCH(db-resource-type-escape): use json.Marshal for proper escaping
					line, err := json.Marshal(map[string]json.RawMessage{
						"resource_type": json.RawMessage(rtJSON),
						"data":          json.RawMessage(raw),
					})
					if err != nil {
						rows.Close()
						return fmt.Errorf("encoding record: %w", err)
					}
					bw.Write(line)
					bw.WriteByte('\n')
					count++
				}
				rows.Close()
				if err := rows.Err(); err != nil {
					return fmt.Errorf("reading %s: %w", rt, err)
				}
				total += count

				if !flags.asJSON {
					dest := "stdout"
					if output != "" {
						dest = output
					}
					fmt.Fprintf(cmd.ErrOrStderr(), "  %s: %d records → %s\n", rt, count, dest)
				}
			}

			if err := bw.Flush(); err != nil {
				return fmt.Errorf("flushing output: %w", err)
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				if output != "" {
					fi, _ := os.Stat(output)
					size := int64(0)
					if fi != nil {
						size = fi.Size()
					}
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"records": total,
						"output":  output,
						"bytes":   size,
					}, flags)
				}
				return nil
			}

			if output != "" {
				fi, _ := os.Stat(output)
				size := int64(0)
				if fi != nil {
					size = fi.Size()
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Exported %d records to %s (%d bytes)\n", total, output, size)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file path (default: stdout)")
	cmd.Flags().StringVar(&format, "format", "", "Output format: jsonl or sqlite (required)")
	cmd.Flags().StringVar(&resourceType, "resource", "", "Export only this resource type (e.g. meetings)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newDBRestoreCmd(flags *rootFlags) *cobra.Command {
	var input string
	var format string
	var dbPath string
	var clear bool

	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore local store from a JSONL or SQLite export — no API calls",
		Long: `Load a previously exported file into the local SQLite store.
--format jsonl: each line must be a JSON object with "resource_type" and "data" fields
               (the format written by 'db export --format jsonl').
--format sqlite: copies the exported .db file directly to the store path.

Pass --clear to wipe the existing store before restoring.`,
		Example: strings.TrimLeft(`
  fathom-pp-cli db restore --format jsonl --input ~/fathom-backup.jsonl
  fathom-pp-cli db restore --format sqlite --input ~/fathom-backup.db
  fathom-pp-cli db restore --format jsonl --input ~/fathom-backup.jsonl --clear`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if input == "" && len(args) > 0 {
				input = args[0]
			}
			if input == "" {
				return cmd.Help()
			}
			if format != "jsonl" && format != "sqlite" {
				return fmt.Errorf("--format is required: must be 'jsonl' or 'sqlite'")
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would restore %s from %s\n", format, input)
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("fathom-pp-cli")
			}

			if format == "sqlite" {
				// PATCH(db-sqlite-restore-validate): guard before touching live DB
				if !isSQLiteFile(input) {
					return fmt.Errorf("input file %q does not appear to be a valid SQLite database", input)
				}
				if clear {
					os.Remove(dbPath)
				}
				if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
					return fmt.Errorf("creating db directory: %w", err)
				}
				if err := copyFile(input, dbPath); err != nil {
					return fmt.Errorf("restoring database: %w", err)
				}
				fi, _ := os.Stat(dbPath)
				size := int64(0)
				if fi != nil {
					size = fi.Size()
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Restored SQLite database from %s (%d bytes)\n", input, size)
				return nil
			}

			// format == "jsonl"
			f, err := os.Open(input)
			if err != nil {
				return fmt.Errorf("opening input: %w", err)
			}
			defer f.Close()

			if clear {
				os.Remove(dbPath)
			}

			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			type envelope struct {
				ResourceType string          `json:"resource_type"`
				Data         json.RawMessage `json:"data"`
			}

			counts := map[string]int{}
			scanner := bufio.NewScanner(f)
			scanner.Buffer(make([]byte, 10*1024*1024), 10*1024*1024) // 10MB per line for large transcripts
			line := 0
			for scanner.Scan() {
				line++
				var env envelope
				if err := json.Unmarshal(scanner.Bytes(), &env); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: skipping line %d: %v\n", line, err)
					continue
				}
				if _, _, err := db.UpsertBatch(env.ResourceType, []json.RawMessage{env.Data}); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: upsert failed on line %d: %v\n", line, err)
					continue
				}
				counts[env.ResourceType]++
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("reading input: %w", err)
			}

			total := 0
			for rt, n := range counts {
				fmt.Fprintf(cmd.ErrOrStderr(), "  %s: %d records\n", rt, n)
				total += n
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Restored %d records into %s\n", total, dbPath)
			return nil
		},
	}

	cmd.Flags().StringVarP(&input, "input", "i", "", "Input file path")
	cmd.Flags().StringVar(&format, "format", "", "Input format: jsonl or sqlite (required)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().BoolVar(&clear, "clear", false, "Wipe existing store before restoring")
	return cmd
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func isSQLiteFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	magic := make([]byte, 16)
	if _, err := f.Read(magic); err != nil {
		return false
	}
	return string(magic) == "SQLite format 3\x00"
}
