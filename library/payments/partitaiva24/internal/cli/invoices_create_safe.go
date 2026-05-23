// Copyright 2026 giuseppe-bisemi. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH: hand-authored partial-failure-safe wrapper around `invoices create`.
//
// Background. partitaiva24 is a WordPress REST API. Its mutating endpoints
// insert the parent row (`e_invoices`) before validating the cascade insert
// into `e_invoice_meta`. When the second insert fails — observed on a body
// that triggered HTTP 409 `foreign key constraint fails` and again on an
// HTTP 500 from a malformed body — the parent row is left orphaned. The
// client receives a non-2xx response and naturally believes the operation
// failed; the server, however, has a draft invoice in DB. Retrying compounds
// the problem: each retry creates an additional phantom.
//
// `invoices create-safe` defends against this with two checks around the raw
// POST. The intent is unattended-safe invoice creation; for interactive use
// `invoices create` remains available unchanged.
//
//   1. Pre-check. Refuse the create if the requested `number` already exists
//      in any non-cancelled status. With --idempotent the existing invoice
//      is returned as a successful no-op.
//
//   2. Post-error verify. If the underlying POST returns non-2xx, list
//      invoices and detect phantoms — same number, draft status, created
//      after the POST started. With --auto-cleanup the phantom is deleted
//      and the original error is surfaced annotated. Without it the phantom
//      ID is included in the error so the agent can decide.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// invoiceWebURLTemplate is the canonical web UI deep-link for an invoice id.
// Matches the path the partitaiva24 SPA uses for "Fatture emesse → <invoice>".
const invoiceWebURLTemplate = "https://partitaiva24.cloud/app/fatturazione/vendita/%s/"

// webURLForInvoice returns the partitaiva24 web UI URL for a given invoice ID.
func webURLForInvoice(id string) string {
	if id == "" {
		return ""
	}
	return fmt.Sprintf(invoiceWebURLTemplate, id)
}

func newInvoicesCreateSafeCmd(flags *rootFlags) *cobra.Command {
	var stdinFlag bool
	var bodyFile string
	var autoCleanup bool
	var sendPDFTo string
	var savePDFXMLTo string

	cmd := &cobra.Command{
		Use:   "create-safe",
		Short: "Create an invoice with phantom-record protection",
		Long: `Wraps invoices create with two defenses against partitaiva24's non-atomic
WordPress REST backend:

  1. Pre-check. Refuse the create if the requested 'number' already exists in
     any non-cancelled status. With --idempotent the existing invoice is
     returned as a successful no-op.

  2. Post-error verify. If the underlying POST returns non-2xx, list invoices
     and detect phantoms (same number, draft status, recent). With
     --auto-cleanup the phantom is deleted before the error is surfaced;
     otherwise the phantom ID is included in the error so the caller can
     decide.

The successful response is enriched with a 'web_url' field pointing at the
partitaiva24 SPA so the caller can open the draft directly. Optional flags:

  --send-pdf <email>      POST /sendpdf, emails the PDF to the recipient.
                          Works on both draft and charged invoices.
  --save-xml <path>       Resolve /file and download the SDI XML envelope to
                          the given path. Only works on charged/transmitted
                          invoices — the path is empty for drafts.

This is the recommended path for unattended invoice creation. For interactive
use 'invoices create' remains available unchanged.`,
		Example: `  cat fattura.json | partitaiva24-pp-cli invoices create-safe --stdin --auto-cleanup
  partitaiva24-pp-cli invoices create-safe --file fattura.json --auto-cleanup
  cat fattura.json | partitaiva24-pp-cli invoices create-safe --stdin --send-pdf cliente@example.com`,
		Annotations: map[string]string{}, // mutator — not mcp:read-only
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			bodyBytes, err := readSafeCreateBody(cmd, stdinFlag, bodyFile)
			if err != nil {
				return err
			}

			var body map[string]any
			if err := json.Unmarshal(bodyBytes, &body); err != nil {
				return fmt.Errorf("parsing body JSON: %w", err)
			}
			number := stringifyNumber(body["number"])
			if number == "" {
				return fmt.Errorf("body must include a non-empty 'number' field — required for safe-emit pre-check")
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// 1. Pre-check
			existingID, existingStatus, perr := findInvoiceByNumber(c, number)
			if perr != nil {
				return fmt.Errorf("pre-check list failed: %w", perr)
			}
			if existingID != "" && existingStatus != "cancelled" && existingStatus != "annullata" {
				if flags.idempotent {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"action":  "noop",
						"reason":  "idempotent: invoice already exists",
						"id":      existingID,
						"status":  existingStatus,
						"number":  number,
						"web_url": webURLForInvoice(existingID),
					}, flags)
				}
				return fmt.Errorf("invoice number %s already exists (id=%s, status=%s) — pass --idempotent to no-op or change number",
					number, existingID, existingStatus)
			}

			// 2. POST
			startedAt := time.Now()
			resp, _, postErr := c.Post("/user/invoices", body)
			if postErr == nil {
				return emitCreateSuccess(cmd, c, flags, resp, sendPDFTo, savePDFXMLTo)
			}

			// 3. Post-error verify
			phantomID, phantomStatus, _ := findInvoicePhantom(c, number, startedAt)
			if phantomID == "" {
				return classifyAPIError(postErr, flags)
			}

			if !autoCleanup {
				return fmt.Errorf(
					"create failed: %w — server has phantom invoice id=%s (number=%s, status=%s); pass --auto-cleanup to delete it automatically, or run `invoices delete %s`",
					postErr, phantomID, number, phantomStatus, phantomID,
				)
			}

			if _, _, dErr := c.Delete(fmt.Sprintf("/user/invoices/%s", phantomID)); dErr != nil {
				return fmt.Errorf(
					"create failed: %w — phantom %s left behind, auto-cleanup also failed: %v",
					postErr, phantomID, dErr,
				)
			}
			return fmt.Errorf(
				"create failed: %w — phantom %s was auto-cleaned (number %s, status was %s)",
				postErr, phantomID, number, phantomStatus,
			)
		},
	}
	cmd.Flags().BoolVar(&stdinFlag, "stdin", false, "Read body as JSON from stdin")
	cmd.Flags().StringVar(&bodyFile, "file", "", "Read body as JSON from a file")
	cmd.Flags().BoolVar(&autoCleanup, "auto-cleanup", false, "Auto-delete phantom records left by partial-failure responses")
	cmd.Flags().StringVar(&sendPDFTo, "send-pdf", "", "After creating, email the PDF to the given address (POST /sendpdf)")
	cmd.Flags().StringVar(&savePDFXMLTo, "save-xml", "", "After creating, download the SDI XML to the given path (only works once charged/transmitted)")
	return cmd
}

// emitCreateSuccess augments the create response with the web_url, optionally
// triggers a sendpdf and/or downloads the SDI XML, and routes the final
// payload through the standard output flags.
func emitCreateSuccess(cmd *cobra.Command, c clientPostGetter, flags *rootFlags, resp []byte, sendPDFTo, savePDFXMLTo string) error {
	enriched := unwrapInvoicePayload(resp)
	id, _ := enriched["id"].(string)
	if id != "" {
		enriched["web_url"] = webURLForInvoice(id)
	}

	// Optional --send-pdf
	if sendPDFTo != "" && id != "" {
		if _, _, err := c.Post(fmt.Sprintf("/user/invoices/%s/sendpdf", id), map[string]any{"to": sendPDFTo}); err != nil {
			enriched["send_pdf_error"] = err.Error()
		} else {
			enriched["send_pdf_to"] = sendPDFTo
		}
	}

	// Optional --save-xml: only succeeds on charged/transmitted invoices
	if savePDFXMLTo != "" && id != "" {
		body, gerr := c.Get(fmt.Sprintf("/user/invoices/%s/file", id), nil)
		if gerr != nil {
			enriched["save_xml_error"] = gerr.Error()
		} else {
			var url string
			_ = json.Unmarshal(body, &url)
			if !strings.HasSuffix(url, ".xml") && !strings.Contains(url, ".xml") {
				enriched["save_xml_error"] = "no XML available yet (likely status=draft) — transmit the invoice first"
			} else if err := writePathFromURL(url, savePDFXMLTo); err != nil {
				enriched["save_xml_error"] = err.Error()
			} else {
				enriched["xml_saved"] = savePDFXMLTo
			}
		}
	}

	return printJSONFiltered(cmd.OutOrStdout(), enriched, flags)
}

// unwrapInvoicePayload normalises the various wrapping shapes the create
// response can take (`{action,data:{...}}` envelope, `{results:{...}}`,
// or a bare invoice object) into a single map we can mutate.
func unwrapInvoicePayload(resp []byte) map[string]any {
	var out map[string]any
	if err := json.Unmarshal(resp, &out); err != nil {
		return map[string]any{"raw_response": string(resp)}
	}
	if data, ok := out["data"].(map[string]any); ok {
		// Envelope shape — flatten so id is at the top level.
		flat := map[string]any{}
		for k, v := range data {
			flat[k] = v
		}
		flat["_envelope_action"] = out["action"]
		return flat
	}
	if results, ok := out["results"].(map[string]any); ok {
		flat := map[string]any{}
		for k, v := range results {
			flat[k] = v
		}
		return flat
	}
	return out
}

// writePathFromURL fetches the given URL anonymously (the URL is signed so
// no auth headers are needed) and writes the body to dst.
func writePathFromURL(url, dst string) error {
	resp, err := pdfClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, body, 0o644)
}

// pdfClient is a short-timeout HTTP client used only for following the
// signed URL returned by /file. We don't reuse the configured Client here
// because that one wraps every request in the auth/cookie/nonce stack and
// the signed URL is anonymous (auth is in the path).
var pdfClient = &http.Client{Timeout: 30 * time.Second}

// clientPostGetter narrows the client interface to the methods used by
// emitCreateSuccess so the helper stays unit-testable.
type clientPostGetter interface {
	Post(path string, body any) (json.RawMessage, int, error)
	Get(path string, params map[string]string) (json.RawMessage, error)
}

// readSafeCreateBody resolves the body bytes from --file, --stdin, or by
// failing back to help. We require an explicit source rather than auto-
// detecting stdin, to keep the verify-friendly RunE pattern clean (the
// agent dogfood matrix probes commands with --dry-run and --help only).
func readSafeCreateBody(cmd *cobra.Command, stdinFlag bool, file string) ([]byte, error) {
	switch {
	case file != "":
		b, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", file, err)
		}
		return b, nil
	case stdinFlag:
		b, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return nil, fmt.Errorf("reading stdin: %w", err)
		}
		if len(b) == 0 {
			return nil, errors.New("stdin was empty")
		}
		return b, nil
	default:
		return nil, cmd.Help()
	}
}

func stringifyNumber(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case float64:
		// JSON numbers come in as float64; print without trailing .0 when integer
		if x == float64(int64(x)) {
			return fmt.Sprintf("%d", int64(x))
		}
		return fmt.Sprintf("%v", x)
	default:
		return fmt.Sprintf("%v", x)
	}
}

// findInvoiceByNumber returns the id+status of the first invoice whose
// number matches. Returns ("", "", nil) when not found.
func findInvoiceByNumber(c clientGetter, number string) (string, string, error) {
	body, err := c.Get("/user/invoices", nil)
	if err != nil {
		return "", "", err
	}
	id, status, _ := scanInvoiceList(body, func(inv map[string]any) bool {
		return stringifyNumber(inv["number"]) == number
	})
	return id, status, nil
}

// findInvoicePhantom returns the id+status of an invoice that looks like
// a partial-failure leftover: same number, draft status, created within
// ~60s of the POST start time (or no created field).
func findInvoicePhantom(c clientGetter, number string, since time.Time) (string, string, error) {
	body, err := c.Get("/user/invoices", nil)
	if err != nil {
		return "", "", err
	}
	id, status, _ := scanInvoiceList(body, func(inv map[string]any) bool {
		if stringifyNumber(inv["number"]) != number {
			return false
		}
		st, _ := inv["status"].(string)
		if st != "draft" {
			return false
		}
		// Created field may or may not be populated immediately after a
		// partial-failure insert. When it is, treat anything in the last
		// ~120s window as a candidate phantom; when absent, accept it.
		created, _ := inv["created"].(string)
		if created == "" {
			return true
		}
		t, perr := time.Parse(time.RFC3339, created)
		if perr != nil {
			t, perr = time.Parse("2006-01-02 15:04:05", created)
		}
		if perr != nil {
			return true
		}
		return t.After(since.Add(-120 * time.Second))
	})
	return id, status, nil
}

func scanInvoiceList(body []byte, match func(map[string]any) bool) (string, string, error) {
	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", "", err
	}
	var arr []any
	if r, ok := resp["results"].(map[string]any); ok {
		if d, ok := r["data"].([]any); ok {
			arr = d
		}
	}
	if arr == nil {
		if d, ok := resp["data"].([]any); ok {
			arr = d
		}
	}
	if arr == nil {
		if d, ok := resp["results"].([]any); ok {
			arr = d
		}
	}
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if match(m) {
			return stringifyNumber(m["id"]), fmt.Sprintf("%v", m["status"]), nil
		}
	}
	return "", "", nil
}

// clientGetter narrows the interface our helpers need from *client.Client so
// the same code can be unit-tested without spinning up the full HTTP client.
type clientGetter interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}
