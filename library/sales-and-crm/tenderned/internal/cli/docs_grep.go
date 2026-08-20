package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newDocsGrepCmd(flags *rootFlags) *cobra.Command {
	var dbPath, cpv, buyer string
	var limit, maxHits int

	cmd := &cobra.Command{
		Use:   "grep [pattern]",
		Short: "Bulk regex grep across attached tender documents (bestek, PvE, criteria)",
		Long: `Iterates notices matching the supplied filters (CPV, buyer), streams each
notice's attached documents, extracts text from PDFs (when possible) and plain
text bodies, and runs the regex against each. Prints publicatieId, documentId
and the matching line.

PDF text extraction is best-effort; scanned-image PDFs return no matches.
Use --limit to cap the number of notices scanned and --max-hits to cap
total matches kept in memory (default 1000) so a broad pattern across a
large corpus doesn't grow unbounded.

Operates on the local SQLite snapshot for notice selection; document content
is fetched live from TenderNed.`,
		Example: `  tenderned-pp-cli docs grep "aansprakelijkheidsclausule" --cpv 45000000-7 --limit 25`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			pat, err := regexp.Compile("(?i)" + args[0])
			if err != nil {
				return fmt.Errorf("pattern: %w", err)
			}
			s, err := tnOpenStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			notices, err := tnLoadNotices(cmd.Context(), s)
			if err != nil {
				return err
			}
			cpvList := tnSplitCSV(cpv)
			needle := strings.ToLower(strings.TrimSpace(buyer))

			scanned := 0
			hits := []map[string]any{}
			truncated := false
			// PATCH: cap total hits to bound memory. A broad pattern across
			// thousands of documents would otherwise grow `hits` unbounded
			// (each PDF can contribute hundreds of matching lines). Default
			// of 1000 is a reasonable triage size; raise via --max-hits.
		scanLoop:
			for _, n := range notices {
				if needle != "" && !strings.Contains(strings.ToLower(n.OpdrachtgeverNaam), needle) {
					continue
				}
				if !tnHasCPV(n, cpvList) {
					continue
				}
				if limit > 0 && scanned >= limit {
					break
				}
				scanned++
				pubID := tnPubIDToInt(n.PublicatieID)
				docs, err := tnFetchDocumentList(cmd.Context(), pubID)
				if err != nil {
					continue
				}
				for _, d := range docs {
					text, err := tnFetchDocumentText(cmd.Context(), pubID, d.DocumentID)
					if err != nil {
						continue
					}
					for _, line := range strings.Split(text, "\n") {
						if pat.MatchString(line) {
							if maxHits > 0 && len(hits) >= maxHits {
								truncated = true
								break scanLoop
							}
							hits = append(hits, map[string]any{
								"publicatieId": pubID,
								"documentId":   d.DocumentID,
								"documentNaam": d.DocumentNaam,
								"line":         strings.TrimSpace(line),
							})
						}
					}
				}
			}
			result := map[string]any{
				"pattern":         args[0],
				"notices_scanned": scanned,
				"hits":            hits,
				"hit_count":       len(hits),
				"truncated":       truncated,
			}
			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			for _, h := range hits {
				fmt.Fprintf(cmd.OutOrStdout(), "%v\t%v\t%s\n", h["publicatieId"], h["documentId"], h["line"])
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d hit(s) across %d notices scanned\n", len(hits), scanned)
			if truncated {
				fmt.Fprintf(cmd.OutOrStdout(), "(truncated at --max-hits=%d; raise the cap to see more)\n", maxHits)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&cpv, "cpv", "", "Comma-separated CPV stripes")
	cmd.Flags().StringVar(&buyer, "buyer", "", "Substring match on buyer name")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max notices to scan")
	cmd.Flags().IntVar(&maxHits, "max-hits", 1000, "Cap on total matching lines kept in memory (0 = no cap)")
	return cmd
}

type tnDocSummary struct {
	DocumentID   string     `json:"documentId"`
	DocumentNaam string     `json:"documentNaam"`
	TypeDocument codeOmschr `json:"typeDocument"`
}

func tnFetchDocumentList(ctx context.Context, pubID int64) ([]tnDocSummary, error) {
	url := fmt.Sprintf("%s/publicaties/%d/documenten", tnBaseURL, pubID)
	// PATCH: check error from http.NewRequestWithContext; discarding it left req=nil → panic on Header.Set
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var body struct {
		Documenten []tnDocSummary `json:"documenten"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	return body.Documenten, nil
}

// tnFetchDocumentText fetches one document and returns whatever text bytes
// it can extract. For plain-text or HTML payloads this is the raw body;
// for PDFs we return an empty string (proper PDF text extraction is best
// added as a follow-up; for now grep operates over text/HTML bodies and
// PDF metadata strings present in the binary).
func tnFetchDocumentText(ctx context.Context, pubID int64, docID string) (string, error) {
	url := fmt.Sprintf("%s/publicaties/%d/documenten/%s/content", tnBaseURL, pubID, docID)
	// PATCH: check error from http.NewRequestWithContext; docID is API-supplied and could yield an invalid URL
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("building request: %w", err)
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return "", err
	}
	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/") || strings.Contains(ct, "application/json") || strings.Contains(ct, "application/xml") {
		return string(body), nil
	}
	// For PDFs/Word, pull only the printable-ASCII runs as a degraded grep
	// surface. Real text-extraction (pdftotext / docx-unzip) is a follow-up.
	return extractPrintableRuns(body), nil
}

func extractPrintableRuns(data []byte) string {
	var sb strings.Builder
	run := make([]byte, 0, 64)
	flush := func() {
		if len(run) >= 4 {
			sb.Write(run)
			sb.WriteByte('\n')
		}
		run = run[:0]
	}
	for _, b := range data {
		if (b >= 0x20 && b < 0x7f) || b == '\t' {
			run = append(run, b)
		} else {
			flush()
		}
	}
	flush()
	return sb.String()
}
