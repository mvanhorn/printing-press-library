// Copyright 2026 wayne-lai. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"judgementtw-pp-cli/internal/judicial"
	"judgementtw-pp-cli/internal/source/fjud"
)

// newJudicialJudgmentsCmd builds a 'judgments' parent that owns the real
// scraping-backed get/list subcommands. It replaces the generator-emitted
// version registered in root.go.
func newJudicialJudgmentsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "judgments",
		Short: "Fetch and list Taiwan court judgments by JID",
		Long: `Fetch a single judgment by JID, list previously-synced judgments from the
local store, or write the PDF attachment to disk.`,
	}
	cmd.AddCommand(newJudgmentsGetReal(flags))
	cmd.AddCommand(newJudgmentsListReal(flags))
	return cmd
}

func newJudgmentsGetReal(flags *rootFlags) *cobra.Command {
	var withPDF bool
	var pdfDir string
	cmd := &cobra.Command{
		Use:   "get [jid]",
		Short: "Fetch a single judgment by JID with full text and metadata",
		Long: `Fetch a single judgment by JID from judgment.judicial.gov.tw, parse the body
and metadata, and store it in the local SQLite cache for offline analysis.

Pass --with-pdf to also download the PDF attachment when present. The PDF is
saved to ~/.local/share/judgementtw-pp-cli/pdfs/<jid>.pdf by default; override
with --pdf-dir.`,
		Example: `  # JSON-only output of a Supreme Court ruling
  judgementtw-pp-cli judgments get TPSM,115,台抗,703,20260430,1 --json

  # Just the body text, narrowed via --select
  judgementtw-pp-cli judgments get TPSM,115,台抗,703,20260430,1 --json --select jid,jdate,jtitle,jfullcontent

  # Also download the PDF attachment
  judgementtw-pp-cli judgments get TPSM,115,台抗,703,20260430,1 --with-pdf`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			jid := args[0]

			db, err := openJudicialDB(ctx)
			if err != nil {
				return err
			}
			defer db.Close()

			c := fjudClient(flags)
			j, err := c.GetJudgment(ctx, jid, withPDF)
			if err != nil {
				if errors.Is(err, fjud.ErrNotFound) {
					// Privacy purge — delete any local cache and audit-log it.
					_, _ = db.ExecContext(ctx, `DELETE FROM judgments WHERE id = ?`, jid)
					_ = judicial.LogEvent(ctx, db, "purged", jid, "判決查無資料 — privacy removal honoured")
					fmt.Fprintf(cmd.ErrOrStderr(), "judgment %s not found (查無資料); local cache cleared.\n", jid)
					return notFoundErr(fmt.Errorf("judgment %s not found", jid))
				}
				return err
			}
			if err := upsertJudgmentRow(ctx, db, j); err != nil {
				return fmt.Errorf("storing judgment: %w", err)
			}
			_ = judicial.LogEvent(ctx, db, "synced", jid, "judgments get")

			if withPDF && len(j.PDFBytes) > 0 {
				dir := pdfDir
				if dir == "" {
					home, _ := os.UserHomeDir()
					dir = filepath.Join(home, ".local", "share", "judgementtw-pp-cli", "pdfs")
				}
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return fmt.Errorf("creating PDF dir: %w", err)
				}
				path := filepath.Join(dir, jid+".pdf")
				if err := os.WriteFile(path, j.PDFBytes, 0o644); err != nil {
					return fmt.Errorf("writing PDF: %w", err)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "wrote PDF to %s (%d bytes)\n", path, len(j.PDFBytes))
			}
			// Don't ship the raw bytes in JSON (binary blob); keep the URL.
			j.PDFBytes = nil
			return emitJSON(cmd.OutOrStdout(), j, flags)
		},
	}
	cmd.Flags().BoolVar(&withPDF, "with-pdf", false, "Also download the PDF attachment when present")
	cmd.Flags().StringVar(&pdfDir, "pdf-dir", "", "Directory for downloaded PDFs (default ~/.local/share/judgementtw-pp-cli/pdfs)")
	return cmd
}

func newJudgmentsListReal(flags *rootFlags) *cobra.Command {
	var court, caseType string
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List previously-synced judgments from the local store",
		Long:  `Read the local SQLite store and return judgments matching court / case-type filters. Use 'judgments get' or 'find' to populate the store first.`,
		Example: `  # All synced Supreme Court rulings
  judgementtw-pp-cli judgments list --court TPS --json --limit 50

  # All synced criminal judgments
  judgementtw-pp-cli judgments list --type criminal --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openJudicialDB(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			ids, err := listJudgmentIDs(ctx, db, court, resolveSingleCaseType(caseType), limit)
			if err != nil {
				return err
			}
			out := make([]map[string]any, 0, len(ids))
			for _, id := range ids {
				row := db.QueryRowContext(ctx, `SELECT data FROM judgments WHERE id = ?`, id)
				var blob string
				if err := row.Scan(&blob); err != nil {
					continue
				}
				var j map[string]any
				if err := json.Unmarshal([]byte(blob), &j); err != nil {
					continue
				}
				out = append(out, j)
			}
			return emitJSON(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&court, "court", "", "Filter by court code (single)")
	cmd.Flags().StringVar(&caseType, "type", "", "Filter by case type (criminal|civil|administrative|disciplinary|constitutional or M|V|A|P|C)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max rows to return")
	return cmd
}

func resolveSingleCaseType(s string) string {
	if s == "" {
		return ""
	}
	parts := resolveCaseTypes(s)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}
