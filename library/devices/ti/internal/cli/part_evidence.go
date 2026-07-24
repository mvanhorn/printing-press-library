// Copyright 2026 Arun Gopalakrishnan and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/ti/internal/ticompliance"
)

// evidenceRung reports one tier of the evidence ladder with its own status,
// so callers see exactly which evidence was available.
type evidenceRung struct {
	Status string `json:"status"` // ok | failed | skipped
	Detail string `json:"detail,omitempty"`
	Data   any    `json:"data,omitempty"`
}

type evidenceView struct {
	PartNumber string       `json:"part_number"`
	Anonymous  evidenceRung `json:"anonymous_compliance"`
	Statements evidenceRung `json:"coc_statements"`
	Ratings    evidenceRung `json:"ratings"`
	FMD        evidenceRung `json:"fmd_class_d"`
	FetchedAt  time.Time    `json:"fetched_at"`
}

func newNovelPartEvidenceCmd(flags *rootFlags) *cobra.Command {
	var flagCookies string
	var flagOutDir string

	cmd := &cobra.Command{
		Use:   "evidence <opn>",
		Short: "The full evidence ladder in one call: anonymous verdicts + statement PDFs always; ratings + FMD Class-D XML when cookies work.",
		Long: strings.TrimSpace(`
Runs every evidence tier and degrades honestly. The anonymous rungs (product-
page verdicts, signed statement PDFs) always run. The login-gated rungs
(Environmental Ratings, IPC-1752A Class-D FMD XML) run when a cookie snapshot
is provided and report their own status — a stale-cookie failure on a gated
rung does not fail the anonymous rungs. Exits non-zero only when the anonymous
compliance rung itself fails.`),
		Example: strings.Trim(`
  ti-pp-cli part evidence TUSB320RWBR --out-dir ./evidence --json
  ti-pp-cli part evidence TUSB320RWBR --cookies ./ti-cookies.json --out-dir ./evidence --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would run the full evidence ladder (anonymous + gated rungs)")
				return nil
			}
			if len(args) < 1 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an orderable part number is required"))
			}
			opn := strings.ToUpper(strings.TrimSpace(args[0]))
			if err := ticompliance.ValidateOPN(opn); err != nil {
				return usageErr(err)
			}
			outDir := flagOutDir
			if outDir == "" {
				outDir = "."
			}
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			view := evidenceView{PartNumber: opn, FetchedAt: time.Now().UTC()}

			// Rung 1: anonymous verdicts (the only fatal rung).
			anon, anonErr := ticompliance.ResolveCompliance(ctx, opn)
			if anonErr != nil {
				view.Anonymous = evidenceRung{Status: "failed", Detail: anonErr.Error()}
			} else {
				view.Anonymous = evidenceRung{Status: "ok", Data: anon}
			}

			// Rung 2: blanket statement PDFs.
			if coc, err := ticompliance.DownloadStatements(ctx, outDir, nil); err != nil {
				view.Statements = evidenceRung{Status: "failed", Detail: err.Error(), Data: coc}
			} else {
				view.Statements = evidenceRung{Status: "ok", Data: coc}
			}

			// Rungs 3+4: gated, only with a cookie snapshot.
			cookiesPath := resolveCookiesPath(flagCookies)
			if cookiesPath == "" {
				view.Ratings = evidenceRung{Status: "skipped", Detail: "no cookie snapshot provided"}
				view.FMD = evidenceRung{Status: "skipped", Detail: "no cookie snapshot provided"}
			} else {
				runGatedRungs(ctx, cookiesPath, opn, outDir, &view)
			}

			if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
				return err
			}
			if anonErr != nil {
				return fmt.Errorf("anonymous compliance rung failed: %w", anonErr)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagCookies, "cookies", "", "path to the deposited myTI cookie snapshot JSON (fallback: TI_COOKIES_FILE); omit to skip gated rungs")
	cmd.Flags().StringVar(&flagOutDir, "out-dir", "", "directory for statement PDFs and FMD XML (default: current directory)")
	return cmd
}

func runGatedRungs(ctx context.Context, cookiesPath, opn, outDir string, view *evidenceView) {
	cookies, err := ticompliance.LoadCookies(cookiesPath)
	if err != nil {
		view.Ratings = evidenceRung{Status: "failed", Detail: err.Error()}
		view.FMD = evidenceRung{Status: "skipped", Detail: "cookie load failed"}
		return
	}
	b, err := ticompliance.NewBrowser(false)
	if err != nil {
		view.Ratings = evidenceRung{Status: "failed", Detail: err.Error()}
		view.FMD = evidenceRung{Status: "skipped", Detail: "browser unavailable"}
		return
	}
	defer b.Close(ctx)
	if err := b.InjectCookies(ctx, cookies); err != nil {
		view.Ratings = evidenceRung{Status: "failed", Detail: err.Error()}
		view.FMD = evidenceRung{Status: "skipped", Detail: "cookie injection failed"}
		return
	}
	if ratings, err := b.ScrapeRatings(ctx, opn); err != nil {
		detail := err.Error()
		if errors.Is(err, ticompliance.ErrAuthRedirect) {
			detail = "login required or cookies stale — trigger a fresh HIL login"
		}
		view.Ratings = evidenceRung{Status: "failed", Detail: detail}
	} else {
		view.Ratings = evidenceRung{Status: "ok", Data: ratings}
	}
	xmlPath := filepath.Join(outDir, "TI_"+opn+"_FMD_1752A_ClassD.xml")
	if fmd, err := b.AcquireFMD(ctx, opn, xmlPath); err != nil {
		detail := err.Error()
		if errors.Is(err, ticompliance.ErrAuthRedirect) {
			detail = "login required or cookies stale — trigger a fresh HIL login"
		}
		view.FMD = evidenceRung{Status: "failed", Detail: detail}
	} else {
		view.FMD = evidenceRung{Status: "ok", Data: fmd}
	}
}
