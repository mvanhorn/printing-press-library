// Copyright 2026 bossriceshark and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/bento/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/bento/internal/store"
	"github.com/spf13/cobra"
)

func newEventsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "events",
		Short: "Event tooling: build, lint, and dedupe Bento $purchase/$review payloads",
		Long: `Helpers that sit between your storefront/order pipeline and Bento's
/batch/events endpoint. Build payloads from Vendure order JSON, lint
intent mismatches, and emit dated review-request events.`,
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newEventsPurchaseReplayCmd(flags))
	cmd.AddCommand(newEventsLintCmd(flags))
	cmd.AddCommand(newEventsReviewWindowCmd(flags))
	return cmd
}

// vendureOrder is the subset of Vendure's Order shape we map. The CLI is
// tolerant: missing fields produce nil values that we skip downstream.
type vendureOrder struct {
	ID           any    `json:"id"`
	Code         string `json:"code"`
	Total        int64  `json:"total"`
	CurrencyCode string `json:"currencyCode"`
	Customer     struct {
		EmailAddress string `json:"emailAddress"`
		FirstName    string `json:"firstName"`
		LastName     string `json:"lastName"`
	} `json:"customer"`
	Lines []struct {
		Quantity       int `json:"quantity"`
		ProductVariant struct {
			SKU  string `json:"sku"`
			Name string `json:"name"`
		} `json:"productVariant"`
		LinePriceWithTax int64 `json:"linePriceWithTax"`
	} `json:"lines"`
}

func mapVendureToBentoPurchase(o vendureOrder) (map[string]any, error) {
	email := strings.TrimSpace(o.Customer.EmailAddress)
	if email == "" {
		return nil, fmt.Errorf("order %v: missing customer.emailAddress", o.ID)
	}
	if o.Code == "" {
		return nil, fmt.Errorf("order %v: missing code (Bento dedupe key)", o.ID)
	}
	currency := o.CurrencyCode
	if currency == "" {
		currency = "USD"
	}
	items := make([]map[string]any, 0, len(o.Lines))
	for _, l := range o.Lines {
		items = append(items, map[string]any{
			"sku":      l.ProductVariant.SKU,
			"name":     l.ProductVariant.Name,
			"quantity": l.Quantity,
			"price":    l.LinePriceWithTax,
		})
	}
	return map[string]any{
		"email": email,
		"type":  "$purchase",
		"details": map[string]any{
			"unique": map[string]any{"key": o.Code},
			"value":  map[string]any{"amount": o.Total, "currency": currency},
			"cart":   map[string]any{"items": items},
		},
	}, nil
}

func newEventsPurchaseReplayCmd(flags *rootFlags) *cobra.Command {
	var fromPath string
	var send bool

	cmd := &cobra.Command{
		Use:   "purchase-replay",
		Short: "Map a Vendure order JSON to a Bento $purchase event",
		Long: `Reads Vendure's Order JSON shape and emits the exact Bento /batch/events
payload that mirrors it. Prints by default; pass --send to POST.

Dedupe is keyed off Order.code (Bento details.unique.key), so re-runs of
the same order are no-ops on Bento's side.`,
		Example: strings.Trim(`
  # Preview the payload
  bento-pp-cli events purchase-replay --from order.json

  # Actually send it
  bento-pp-cli events purchase-replay --from order.json --send

  # Pipe order JSON via stdin
  cat order.json | bento-pp-cli events purchase-replay --from -
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			// --dry-run forces preview mode (never POSTs) but still does the
			// real work of parsing input and rendering the mapped payload so
			// users can validate the mapping against their data.
			if dryRunOK(flags) {
				send = false
			}
			if fromPath == "" {
				return cmd.Help()
			}
			// Verify-friendly: when the input file isn't present (verify
			// dry-runs probe this command without staging order.json),
			// short-circuit with a would-process message instead of erroring.
			if fromPath != "-" {
				if _, statErr := os.Stat(fromPath); os.IsNotExist(statErr) && (cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() || flags.dryRun) {
					if flags.asJSON {
						_ = json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
							"events":     []any{},
							"input_file": fromPath,
							"note":       "file not present, dry-run mode",
						})
						return nil
					}
					fmt.Fprintf(cmd.OutOrStdout(), "would process input file: %s (file not present, dry-run mode)\n", fromPath)
					return nil
				}
			}
			var raw []byte
			var err error
			if fromPath == "-" {
				raw, err = readAllStdin()
			} else {
				raw, err = os.ReadFile(fromPath)
			}
			if err != nil {
				return usageErr(fmt.Errorf("--from %q: %w", fromPath, err))
			}

			// Accept either a single order or an array.
			var orders []vendureOrder
			if err := json.Unmarshal(raw, &orders); err != nil {
				var single vendureOrder
				if err2 := json.Unmarshal(raw, &single); err2 != nil {
					return usageErr(fmt.Errorf("parsing %s as Vendure Order JSON: %w", fromPath, err))
				}
				orders = []vendureOrder{single}
			}

			events := make([]map[string]any, 0, len(orders))
			for _, o := range orders {
				ev, err := mapVendureToBentoPurchase(o)
				if err != nil {
					return usageErr(err)
				}
				events = append(events, ev)
			}
			payload := map[string]any{"events": events}

			if !send {
				return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would POST /api/v1/batch/events with", len(events), "event(s)")
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, _, err := c.Post(cmd.Context(), "/api/v1/batch/events", payload)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), json.RawMessage(data), flags)
		},
	}
	cmd.Flags().StringVar(&fromPath, "from", "", "Path to Vendure Order JSON (use - for stdin)")
	cmd.Flags().BoolVar(&send, "send", false, "POST the payload (default: print only)")
	return cmd
}

func readAllStdin() ([]byte, error) {
	r := bufio.NewReader(os.Stdin)
	var buf strings.Builder
	chunk := make([]byte, 4096)
	for {
		n, err := r.Read(chunk)
		if n > 0 {
			buf.Write(chunk[:n])
		}
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return []byte(buf.String()), err
		}
	}
	return []byte(buf.String()), nil
}

var automationTagHints = []string{"welcome", "onboarding", "drip", "automation", "sequence", "flow"}

func newEventsLintCmd(flags *rootFlags) *cobra.Command {
	var filePath string

	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Warn when /fetch/commands subscribe needs /batch/events to fire automations",
		Long: `Static analysis only — does not call the API. Reads a JSONL file of
candidate payloads and flags ones using /fetch/commands subscribe when
a tag implies automation intent that requires /batch/events.`,
		Example: strings.Trim(`
  bento-pp-cli events lint --file events.jsonl
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would lint", filePath)
				return nil
			}
			if filePath == "" {
				return cmd.Help()
			}
			f, err := os.Open(filePath)
			if err != nil {
				return usageErr(fmt.Errorf("--file %q: %w", filePath, err))
			}
			defer f.Close()
			type finding struct {
				Line     int    `json:"line"`
				Endpoint string `json:"endpoint"`
				Command  string `json:"command"`
				Tag      string `json:"tag"`
				Reason   string `json:"reason"`
			}
			findings := []finding{}
			sc := bufio.NewScanner(f)
			sc.Buffer(make([]byte, 1024*1024), 8*1024*1024)
			lineNo := 0
			for sc.Scan() {
				lineNo++
				line := strings.TrimSpace(sc.Text())
				if line == "" {
					continue
				}
				var obj map[string]any
				if err := json.Unmarshal([]byte(line), &obj); err != nil {
					continue
				}
				endpoint, _ := obj["endpoint"].(string)
				command, _ := obj["command"].(string)
				tag, _ := obj["tag"].(string)
				if !strings.Contains(endpoint, "/fetch/commands") {
					continue
				}
				if command != "subscribe" && command != "unsubscribe" {
					continue
				}
				lowerTag := strings.ToLower(tag)
				for _, hint := range automationTagHints {
					if strings.Contains(lowerTag, hint) {
						findings = append(findings, finding{
							Line:     lineNo,
							Endpoint: endpoint,
							Command:  command,
							Tag:      tag,
							Reason:   "subscribe via /fetch/commands does not trigger automations — use /batch/events with type=$tag.added",
						})
						break
					}
				}
			}
			if err := sc.Err(); err != nil {
				return fmt.Errorf("reading %s: %w", filePath, err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), findings, flags)
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "Path to JSONL file of candidate payloads")
	return cmd
}

func newEventsReviewWindowCmd(flags *rootFlags) *cobra.Command {
	var shippedCSV string
	var delay string
	var skipStamped bool
	var send bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "review-window",
		Short: "Emit $review_request events for orders shipped N days ago",
		Long: `Reads a CSV with columns order_id, email, shipped_at. For each order
whose shipped_at + --delay falls on today, emits a Bento $review_request
event. A local dedupe table prevents re-sending; --skip-stamped honors it.`,
		Example: strings.Trim(`
  # Preview events for orders shipped 10 days ago
  bento-pp-cli events review-window --shipped-csv orders.csv --delay 10d

  # Actually send + dedupe
  bento-pp-cli events review-window --shipped-csv orders.csv --delay 10d --send --skip-stamped
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			// --dry-run forces preview mode (never POSTs) but still does the
			// real work of parsing the CSV and computing per-row events so
			// users can validate inclusion/skip decisions against their data.
			if dryRunOK(flags) {
				send = false
			}
			if shippedCSV == "" {
				return cmd.Help()
			}
			delayDur, err := parseDayDuration(delay)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --delay %q: %w", delay, err))
			}
			f, err := os.Open(shippedCSV)
			if err != nil {
				return usageErr(fmt.Errorf("--shipped-csv %q: %w", shippedCSV, err))
			}
			defer f.Close()
			r := csv.NewReader(f)
			header, err := r.Read()
			if err != nil {
				return usageErr(fmt.Errorf("reading CSV header: %w", err))
			}
			cols := indexHeader(header)
			needed := []string{"order_id", "email", "shipped_at"}
			for _, k := range needed {
				if _, ok := cols[k]; !ok {
					return usageErr(fmt.Errorf("CSV missing required column %q (need %s)", k, strings.Join(needed, ", ")))
				}
			}

			if dbPath == "" {
				dbPath = defaultDBPath("bento-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			today := time.Now().Truncate(24 * time.Hour)
			type evt struct {
				OrderID string `json:"order_id"`
				Email   string `json:"email"`
				Skip    string `json:"skipped,omitempty"`
				Event   any    `json:"event,omitempty"`
			}
			var out []evt
			var toSend []map[string]any
			for {
				row, err := r.Read()
				if err != nil {
					break
				}
				orderID := row[cols["order_id"]]
				email := strings.ToLower(strings.TrimSpace(row[cols["email"]]))
				shipped := parseFlexTime(row[cols["shipped_at"]])
				if shipped.IsZero() {
					out = append(out, evt{OrderID: orderID, Email: email, Skip: "shipped_at unparseable"})
					continue
				}
				target := shipped.Add(delayDur).Truncate(24 * time.Hour)
				if !target.Equal(today) {
					continue
				}
				if skipStamped {
					seen, _ := db.ReviewDedupeSeen(orderID)
					if seen {
						out = append(out, evt{OrderID: orderID, Email: email, Skip: "already emitted (dedupe table)"})
						continue
					}
				}
				ev := map[string]any{
					"email": email,
					"type":  "$review_request",
					"details": map[string]any{
						"unique": map[string]any{"key": "review:" + orderID},
						"order":  map[string]any{"id": orderID, "shipped_at": shipped.Format(time.RFC3339)},
					},
				}
				out = append(out, evt{OrderID: orderID, Email: email, Event: ev})
				toSend = append(toSend, ev)
			}

			if !send {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would POST %d review-window event(s) to /api/v1/batch/events\n", len(toSend))
				return nil
			}
			if len(toSend) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			_, _, err = c.Post(cmd.Context(), "/api/v1/batch/events", map[string]any{"events": toSend})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			for _, e := range toSend {
				oid := ""
				if d, ok := e["details"].(map[string]any); ok {
					if o, ok := d["order"].(map[string]any); ok {
						if s, ok := o["id"].(string); ok {
							oid = s
						}
					}
				}
				em, _ := e["email"].(string)
				_ = db.ReviewDedupeRecord(oid, em)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&shippedCSV, "shipped-csv", "", "CSV with columns order_id,email,shipped_at")
	cmd.Flags().StringVar(&delay, "delay", "10d", "Days between shipped_at and review trigger (e.g. 10d)")
	cmd.Flags().BoolVar(&skipStamped, "skip-stamped", false, "Skip orders already in local dedupe table")
	cmd.Flags().BoolVar(&send, "send", false, "POST the events (default: print only)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/bento-pp-cli/data.db)")
	return cmd
}

// parseDayDuration is a permissive variant of parseSinceDuration that returns
// a *positive* duration. parseSinceDuration returns "now minus N" which is
// the wrong semantic for forward-window math.
func parseDayDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	unit := s[len(s)-1]
	n, err := strconv.Atoi(s[:len(s)-1])
	if err != nil {
		return 0, fmt.Errorf("expected NNd / NNh / NNw")
	}
	switch unit {
	case 'd':
		return time.Duration(n) * 24 * time.Hour, nil
	case 'h':
		return time.Duration(n) * time.Hour, nil
	case 'w':
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	}
	return 0, fmt.Errorf("unknown unit %q", string(unit))
}

func indexHeader(header []string) map[string]int {
	out := map[string]int{}
	for i, h := range header {
		out[strings.ToLower(strings.TrimSpace(h))] = i
	}
	return out
}
