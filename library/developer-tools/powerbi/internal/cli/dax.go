// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/powerbi/internal/store"

	"github.com/spf13/cobra"
)

// daxRequest matches the executeQueries POST body. One query at a time per
// API contract; multiple-query support is documented but not in v1 scope.
type daxRequest struct {
	Queries            []daxQuery            `json:"queries"`
	SerializerSettings *daxSerializerOptions `json:"serializerSettings,omitempty"`
	ImpersonatedUser   string                `json:"impersonatedUserName,omitempty"`
}

type daxQuery struct {
	Query string `json:"query"`
}

type daxSerializerOptions struct {
	IncludeNulls bool `json:"includeNulls"`
}

type daxResponse struct {
	Results []struct {
		Tables []struct {
			Rows []map[string]any `json:"rows"`
		} `json:"tables"`
		Error *daxAPIError `json:"error"`
	} `json:"results"`
	Error *daxAPIError `json:"error"`
}

type daxAPIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func defaultDAXDBPath() string {
	home, _ := os.UserHomeDir()
	return home + "/.config/powerbi-pp-cli/powerbi.db"
}

func ensureDAXTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS dax_queries (
		name TEXT PRIMARY KEY,
		query TEXT NOT NULL,
		description TEXT,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`)
	return err
}

func newDAXCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dax",
		Short: "Execute DAX queries against a Power BI dataset, or manage a local catalog of saved queries",
		Long: `Run DAX (Data Analysis Expressions) queries against a Power BI semantic model.

The 'dax run' subcommand POSTs to /datasets/{id}/executeQueries (or its
workspace-scoped variant). Power BI imposes a 100,000 row / 1,000,000 value /
15MB hard cap per query, and 120 queries-per-minute-per-user.

Saved queries are kept in a local SQLite catalog (~/.config/powerbi-pp-cli/powerbi.db)
so you can iterate on a query, save it under a name, and re-run it without
re-pasting.`,
	}
	cmd.AddCommand(newDAXRunCmd(flags))
	cmd.AddCommand(newDAXSaveCmd(flags))
	cmd.AddCommand(newDAXListCmd(flags))
	cmd.AddCommand(newDAXShowCmd(flags))
	cmd.AddCommand(newDAXDeleteCmd(flags))
	return cmd
}

func newDAXRunCmd(flags *rootFlags) *cobra.Command {
	var query, file, group, dataset, out, impersonateUser string
	var includeNulls bool
	cmd := &cobra.Command{
		Use:   "run [SAVED_NAME]",
		Short: "Execute a DAX query and emit the result as JSON or CSV",
		Long: `Execute a DAX query. The query body is provided one of three ways:

  1. Inline via --query
  2. From a file via --file
  3. By the name of a saved query (positional arg; created with 'dax save')

Output is JSON by default (executeQueries native shape). Add --csv to flatten
the first result table into columns suitable for spreadsheet or pandas use.`,
		Example: `  # Inline DAX
  powerbi-pp-cli dax run --query 'EVALUATE TOPN(10, Sales)' --group W --dataset D

  # Saved DAX
  powerbi-pp-cli dax save monthly-rev 'EVALUATE SUMMARIZECOLUMNS(Dates[Month], "Revenue", [Total Revenue])'
  powerbi-pp-cli dax run monthly-rev --group W --dataset D --csv

  # From a file, write CSV to disk
  powerbi-pp-cli dax run --file q.dax --group W --dataset D --csv --out out.csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			// Resolve query text from one of: positional arg (saved), --query, --file.
			text := ""
			switch {
			case query != "" && file != "":
				return usageErr(fmt.Errorf("specify --query OR --file, not both"))
			case query != "":
				text = query
			case file != "":
				b, err := os.ReadFile(file)
				if err != nil {
					return usageErr(fmt.Errorf("reading --file %s: %w", file, err))
				}
				text = string(b)
			case len(args) == 1:
				// Saved query lookup.
				dbPath := defaultDAXDBPath()
				st, err := store.OpenWithContext(cmd.Context(), dbPath)
				if err != nil {
					return configErr(fmt.Errorf("opening local store: %w", err))
				}
				defer st.Close()
				if err := ensureDAXTable(cmd.Context(), st.DB()); err != nil {
					return configErr(err)
				}
				row := st.DB().QueryRowContext(cmd.Context(), `SELECT query FROM dax_queries WHERE name = ?`, args[0])
				if err := row.Scan(&text); err != nil {
					if err == sql.ErrNoRows {
						return notFoundErr(fmt.Errorf("no saved DAX query named %q (see 'powerbi-pp-cli dax list')", args[0]))
					}
					return configErr(err)
				}
			default:
				return usageErr(fmt.Errorf("provide --query, --file, or the name of a saved query"))
			}
			text = strings.TrimSpace(text)
			if text == "" {
				return usageErr(fmt.Errorf("DAX query is empty"))
			}
			if dataset == "" {
				return usageErr(fmt.Errorf("--dataset is required"))
			}

			// Build the request and POST.
			path := fmt.Sprintf("/datasets/%s/executeQueries", dataset)
			if group != "" {
				path = fmt.Sprintf("/groups/%s/datasets/%s/executeQueries", group, dataset)
			}
			body := daxRequest{
				Queries:            []daxQuery{{Query: text}},
				SerializerSettings: &daxSerializerOptions{IncludeNulls: includeNulls},
			}
			if impersonateUser != "" {
				body.ImpersonatedUser = impersonateUser
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, status, err := c.Post(path, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if status == 429 {
				return rateLimitErr(fmt.Errorf("HTTP 429 — Power BI throttled (120 queries/min/user limit)"))
			}
			if status < 200 || status >= 300 {
				return apiErr(fmt.Errorf("executeQueries returned HTTP %d: %s", status, string(raw)))
			}

			// Emit output.
			return emitDAXResult(cmd, flags, raw, out)
		},
	}
	cmd.Flags().StringVarP(&query, "query", "q", "", "DAX query text (inline)")
	cmd.Flags().StringVarP(&file, "file", "f", "", "Path to a file containing the DAX query")
	cmd.Flags().StringVarP(&group, "group", "g", "", "Workspace (group) ID. Omit for My workspace.")
	cmd.Flags().StringVarP(&dataset, "dataset", "d", "", "Dataset ID (required)")
	cmd.Flags().StringVarP(&out, "out", "o", "", "Write output to this file instead of stdout")
	cmd.Flags().BoolVar(&includeNulls, "include-nulls", false, "Include null/blank values in the result")
	cmd.Flags().StringVar(&impersonateUser, "impersonate", "", "UPN of a user to impersonate (RLS-enabled datasets only)")
	cmd.Annotations = map[string]string{"mcp:read-only": "true"}
	return cmd
}

// emitDAXResult writes the executeQueries response in the requested shape.
// JSON: passthrough of the API response. CSV: flatten the first table.
func emitDAXResult(cmd *cobra.Command, flags *rootFlags, raw json.RawMessage, outPath string) error {
	var sink io.Writer = cmd.OutOrStdout()
	if outPath != "" {
		f, err := os.Create(outPath)
		if err != nil {
			return configErr(fmt.Errorf("opening --out: %w", err))
		}
		defer f.Close()
		sink = f
	}
	if flags.csv {
		var resp daxResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			return apiErr(fmt.Errorf("parsing executeQueries response for CSV: %w", err))
		}
		if resp.Error != nil {
			return apiErr(fmt.Errorf("DAX error: %s — %s", resp.Error.Code, resp.Error.Message))
		}
		if len(resp.Results) == 0 || len(resp.Results[0].Tables) == 0 {
			return apiErr(fmt.Errorf("DAX response has no result tables"))
		}
		if resp.Results[0].Error != nil {
			return apiErr(fmt.Errorf("DAX query error: %s — %s", resp.Results[0].Error.Code, resp.Results[0].Error.Message))
		}
		rows := resp.Results[0].Tables[0].Rows
		if len(rows) == 0 {
			return nil // empty CSV
		}
		// Stable column order: keys of the first row, sorted.
		cols := make([]string, 0, len(rows[0]))
		for k := range rows[0] {
			cols = append(cols, k)
		}
		sort.Strings(cols)
		cw := csv.NewWriter(sink)
		defer cw.Flush()
		if err := cw.Write(cols); err != nil {
			return apiErr(err)
		}
		for _, r := range rows {
			rec := make([]string, len(cols))
			for i, k := range cols {
				rec[i] = formatCSVCell(r[k])
			}
			if err := cw.Write(rec); err != nil {
				return apiErr(err)
			}
		}
		return nil
	}
	// JSON passthrough.
	if outPath != "" {
		if _, err := sink.Write(raw); err != nil {
			return apiErr(err)
		}
		if _, err := sink.Write([]byte{'\n'}); err != nil {
			return apiErr(err)
		}
		return nil
	}
	return printJSONFiltered(cmd.OutOrStdout(), json.RawMessage(raw), flags)
}

func formatCSVCell(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		// JSON numbers come back as float64; trim trailing zeros for integers.
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%g", t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func newDAXSaveCmd(flags *rootFlags) *cobra.Command {
	var desc string
	cmd := &cobra.Command{
		Use:     "save <name> <query>",
		Short:   "Save a DAX query to the local catalog under a name",
		Example: `  powerbi-pp-cli dax save monthly-rev 'EVALUATE SUMMARIZECOLUMNS(Dates[Month], "Revenue", [Total Revenue])'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("usage: dax save <name> <query>"))
			}
			// Join all remaining args as the query body. Some shells
			// (notably PowerShell on Windows) don't escape embedded
			// double quotes when forwarding a string to a native exe,
			// so a single logical query may arrive as multiple argv
			// entries. Re-joining with spaces reconstructs the original
			// well enough for DAX, which is whitespace-insensitive.
			name, q := args[0], strings.TrimSpace(strings.Join(args[1:], " "))
			if q == "" {
				return usageErr(fmt.Errorf("query is empty"))
			}
			st, err := store.OpenWithContext(cmd.Context(), defaultDAXDBPath())
			if err != nil {
				return configErr(err)
			}
			defer st.Close()
			if err := ensureDAXTable(cmd.Context(), st.DB()); err != nil {
				return configErr(err)
			}
			_, err = st.DB().ExecContext(cmd.Context(), `INSERT INTO dax_queries(name, query, description, updated_at) VALUES (?,?,?,datetime('now')) ON CONFLICT(name) DO UPDATE SET query=excluded.query, description=excluded.description, updated_at=excluded.updated_at`, name, q, desc)
			if err != nil {
				return configErr(err)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"saved": true, "name": name}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Saved %q.\n", name)
			return nil
		},
	}
	cmd.Flags().StringVar(&desc, "description", "", "Optional description for the saved query")
	return cmd
}

func newDAXListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List saved DAX queries",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			st, err := store.OpenWithContext(cmd.Context(), defaultDAXDBPath())
			if err != nil {
				return configErr(err)
			}
			defer st.Close()
			if err := ensureDAXTable(cmd.Context(), st.DB()); err != nil {
				return configErr(err)
			}
			rows, err := st.DB().QueryContext(cmd.Context(), `SELECT name, COALESCE(description,''), updated_at FROM dax_queries ORDER BY name`)
			if err != nil {
				return configErr(err)
			}
			defer rows.Close()
			type entry struct {
				Name        string `json:"name"`
				Description string `json:"description,omitempty"`
				UpdatedAt   string `json:"updated_at"`
			}
			var out []entry
			for rows.Next() {
				var e entry
				if err := rows.Scan(&e.Name, &e.Description, &e.UpdatedAt); err != nil {
					return configErr(err)
				}
				out = append(out, e)
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no saved DAX queries — try 'powerbi-pp-cli dax save <name> <query>')")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "NAME\tDESCRIPTION\tUPDATED")
			for _, e := range out {
				d := e.Description
				if d == "" {
					d = "-"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\n", e.Name, truncate(d, 60), e.UpdatedAt)
			}
			return tw.Flush()
		},
	}
}

func newDAXShowCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "show <name>",
		Short:       "Print the body of a saved DAX query",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			st, err := store.OpenWithContext(cmd.Context(), defaultDAXDBPath())
			if err != nil {
				return configErr(err)
			}
			defer st.Close()
			if err := ensureDAXTable(cmd.Context(), st.DB()); err != nil {
				return configErr(err)
			}
			var q, desc, updated string
			err = st.DB().QueryRowContext(cmd.Context(), `SELECT query, COALESCE(description,''), updated_at FROM dax_queries WHERE name=?`, args[0]).Scan(&q, &desc, &updated)
			if err != nil {
				if err == sql.ErrNoRows {
					return notFoundErr(fmt.Errorf("no saved DAX query named %q", args[0]))
				}
				return configErr(err)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"name": args[0], "query": q, "description": desc, "updated_at": updated}, flags)
			}
			fmt.Fprintln(cmd.OutOrStdout(), q)
			return nil
		},
	}
}

func newDAXDeleteCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "delete NAME",
		Short: "Delete a saved DAX query",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			st, err := store.OpenWithContext(cmd.Context(), defaultDAXDBPath())
			if err != nil {
				return configErr(err)
			}
			defer st.Close()
			if err := ensureDAXTable(cmd.Context(), st.DB()); err != nil {
				return configErr(err)
			}
			res, err := st.DB().ExecContext(cmd.Context(), `DELETE FROM dax_queries WHERE name=?`, args[0])
			if err != nil {
				return configErr(err)
			}
			n, _ := res.RowsAffected()
			if n == 0 {
				return notFoundErr(fmt.Errorf("no saved DAX query named %q", args[0]))
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"deleted": true, "name": args[0]}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted %q.\n", args[0])
			return nil
		},
	}
}

// daxBackgroundPlaceholder keeps the time import used when this file is
// compiled standalone in test scenarios that don't reach the polling code.
var _ = time.Now
