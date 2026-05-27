// Copyright 2026 rushyant-m. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/mvanhorn/printing-press-library/library/other/bse-filings/internal/bseutil"
	"github.com/mvanhorn/printing-press-library/library/other/bse-filings/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/bse-filings/internal/store"
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"
	"github.com/spf13/cobra"
)

// strongConcallKeywords name a transcript-bearing filing unambiguously and are
// matched first. weakConcallKeywords (analyst/investor meet) often attach a
// short intimation letter rather than a transcript, so they are only used as a
// fallback when no strong match exists for the scrip.
var strongConcallKeywords = []string{"transcript", "concall", "con call", "earnings call", "conference call"}
var weakConcallKeywords = []string{"analyst", "investor meet", "investor presentation", "earnings"}

func newConcallCmd(flags *rootFlags) *cobra.Command {
	var quarter, mentions string

	cmd := &cobra.Command{
		Use:   "concall [scrip]",
		Short: "Fetch, parse, and store a holding's latest concall transcript, optionally printing only matching paragraphs.",
		Long: strings.Trim(`
Find the most recent earnings-call / analyst-meet transcript filing for a
scrip, download the attached PDF, extract and split it into paragraphs, and
store them for full-text search (concall-grep) and thesis-drift.

With --mentions, only the paragraphs containing the phrase are printed; without
it, the first ~10 paragraphs are shown. Scanned image-only PDFs that yield no
text are reported as "needs OCR" and skipped (exit 0).`, "\n"),
		Example: strings.Trim(`
  bse-filings-pp-cli concall 500325
  bse-filings-pp-cli concall 500325 --mentions "capex"`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			scrip := strings.TrimSpace(args[0])
			if err := requireNumericScrip(scrip); err != nil {
				return err
			}

			s, err := openBSEStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()

			// Find the transcript filing from synced announcements.
			filing, ferr := findTranscriptFiling(s, scrip, quarter)
			if ferr != nil {
				return ferr
			}
			if filing.attachment == "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "no concall/transcript filing with an attachment found for scrip %s — run 'sync --scrip %s' first\n", scrip, scrip)
				return flags.printJSON(cmd, map[string]any{"scrip_code": scrip, "status": "no_filing"})
			}

			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.ErrOrStderr(), "verify: would fetch and parse %s for scrip %s\n", filing.attachment, scrip)
				return flags.printJSON(cmd, map[string]any{"scrip_code": scrip, "status": "verify_skip"})
			}

			pdfBytes, err := fetchFilingPDF(filing.attachment, flags.timeout)
			if err != nil {
				return apiErr(fmt.Errorf("downloading concall PDF: %w", err))
			}

			text, err := extractPDFText(pdfBytes)
			if err != nil || strings.TrimSpace(text) == "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "transcript for scrip %s yielded no extractable text (likely a scanned image) — needs OCR, skipped\n", scrip)
				return flags.printJSON(cmd, map[string]any{"scrip_code": scrip, "filing_id": filing.newsid, "status": "needs_ocr"})
			}

			paragraphs := bseutil.SplitParagraphs(text)
			if len(paragraphs) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "transcript for scrip %s produced no paragraphs — needs OCR, skipped\n", scrip)
				return flags.printJSON(cmd, map[string]any{"scrip_code": scrip, "filing_id": filing.newsid, "status": "needs_ocr"})
			}

			qLabel := quarter
			if qLabel == "" && !filing.filedAt.IsZero() {
				qLabel = bseutil.QuarterFromDate(filing.filedAt)
			}

			n, err := s.ReplaceConcallChunks(filing.newsid, scrip, qLabel, paragraphs, filing.filedAt)
			if err != nil {
				return err
			}

			// Select paragraphs to print.
			shown := paragraphs
			if mentions != "" {
				var matched []string
				for _, p := range paragraphs {
					if strings.Contains(strings.ToLower(p), strings.ToLower(mentions)) {
						matched = append(matched, p)
					}
				}
				shown = matched
			} else if len(shown) > 10 {
				shown = shown[:10]
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "stored %d paragraphs for scrip %s (%s, filing %s)\n", n, scrip, qLabel, filing.newsid)
			return flags.printJSON(cmd, map[string]any{
				"scrip_code":       scrip,
				"filing_id":        filing.newsid,
				"quarter":          qLabel,
				"paragraphs_total": len(paragraphs),
				"stored":           n,
				"paragraphs":       shown,
			})
		},
	}
	cmd.Flags().StringVar(&quarter, "quarter", "", "Quarter label to tag chunks with (e.g. Q1 FY27); derived from filing date when omitted.")
	cmd.Flags().StringVar(&mentions, "mentions", "", "Print only paragraphs containing this phrase.")
	return cmd
}

type transcriptFiling struct {
	newsid     string
	attachment string
	filedAt    time.Time
}

// findTranscriptFiling picks the most recent announcement for a scrip that
// looks like a transcript/concall (by subject keyword) and has an attachment;
// failing that, the most recent announcement with any attachment. When a
// quarter is supplied, only rows whose quarter_id matches are considered.
func findTranscriptFiling(s *store.Store, scrip, quarter string) (transcriptFiling, error) {
	q := `SELECT newsid, COALESCE(newssub,''), COALESCE(headline,''), COALESCE(attachmentname,''), COALESCE(news_dt,''), COALESCE(quarter_id,'')
	      FROM announcements WHERE scrip_cd = ? AND attachmentname IS NOT NULL AND attachmentname != ''
	      ORDER BY news_dt DESC`
	rows, err := s.Query(q, scrip)
	if err != nil {
		return transcriptFiling{}, err
	}
	defer rows.Close()

	// Rows arrive newest-first; record the first match in each tier so the
	// strongest available signal wins regardless of recency ordering.
	var strong, weak, any transcriptFiling
	var haveStrong, haveWeak, haveAny bool
	for rows.Next() {
		var newsid, sub, headline, attach, newsDt, qid string
		if err := rows.Scan(&newsid, &sub, &headline, &attach, &newsDt, &qid); err != nil {
			continue
		}
		if quarter != "" && !strings.EqualFold(qid, quarter) {
			continue
		}
		filedAt, _ := bseutil.ParseBSEDate(newsDt)
		tf := transcriptFiling{newsid: newsid, attachment: attach, filedAt: filedAt}
		if !haveAny {
			any, haveAny = tf, true
		}
		subj := strings.ToLower(sub + " " + headline)
		if !haveStrong && containsAny(subj, strongConcallKeywords) {
			strong, haveStrong = tf, true
		}
		if !haveWeak && containsAny(subj, weakConcallKeywords) {
			weak, haveWeak = tf, true
		}
	}
	if err := rows.Err(); err != nil {
		return transcriptFiling{}, err
	}
	switch {
	case haveStrong:
		return strong, nil
	case haveWeak:
		return weak, nil
	default:
		return any, nil
	}
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// extractPDFText pulls plain text out of a PDF byte slice using the pure-Go
// ledongthuc/pdf reader. Returns "" with no error when the PDF parses but
// carries no text layer (a scanned image), which the caller treats as
// "needs OCR".
func extractPDFText(data []byte) (string, error) {
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	b, err := r.GetPlainText()
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	buf := make([]byte, 32*1024)
	for {
		n, rerr := b.Read(buf)
		if n > 0 {
			sb.Write(buf[:n])
		}
		if rerr != nil {
			break
		}
	}
	return sb.String(), nil
}
