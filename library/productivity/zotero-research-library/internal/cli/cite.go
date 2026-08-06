// Copyright 2026 Eric Rash and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: Better BibTeX citekey bridge.
// pp:data-source local

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// bbtEndpoint is Better BibTeX's local JSON-RPC endpoint, served by the
// Zotero desktop app when the BBT plugin is installed.
const bbtEndpoint = "http://127.0.0.1:23119/better-bibtex/json-rpc"

func newNovelCiteCmd(flags *rootFlags) *cobra.Command {
	var flagFormat string

	cmd := &cobra.Command{
		Use:   "cite <citekey-or-item-key>",
		Short: "Resolve a Better BibTeX citekey or item key to its item, with citekey-faithful BibTeX export",
		Long: `Bridges citation identifiers to library items. Better BibTeX citekeys (the
identifiers manuscripts actually use) export through the desktop app's BBT
JSON-RPC endpoint so the entry keeps the exact citekey; when the desktop is
closed, the command falls back to the local cache — first matching pinned
extra-field citekeys, then plain Zotero item keys (the identifiers 'ground'
and 'items recent' emit). The special identifier 'top' cites the most
recently added item — "cite the paper I just added".`,
		Example: `  zotero-research-library-pp-cli cite smithHamstring2024 --format bibtex
  zotero-research-library-pp-cli cite KV6U2ABE --format json --agent
  zotero-research-library-pp-cli cite top`,
		Args: cobra.ExactArgs(1),
		// The 'top' fixture resolves against any non-empty cache, so probes
		// pass without the Zotero desktop app (Better BibTeX) being open.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<citekey>=top;--format=json"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			citekey := strings.TrimSpace(args[0])
			if citekey == "" {
				return fmt.Errorf("citekey must not be empty")
			}
			switch flagFormat {
			case "", "bibtex", "json":
			default:
				return fmt.Errorf("unsupported --format %q (use bibtex or json)", flagFormat)
			}
			ctx := cmd.Context()

			if flagFormat == "" || flagFormat == "bibtex" {
				bib, err := bbtExport(ctx, citekey)
				if err == nil {
					if flags.asJSON {
						return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
							"citekey": citekey,
							"format":  "bibtex",
							"source":  "better-bibtex-rpc",
							"bibtex":  bib,
						}, flags)
					}
					fmt.Fprintln(cmd.OutOrStdout(), bib)
					return nil
				}
				// Desktop closed or BBT absent: fall through to the cache.
				fmt.Fprintf(cmd.ErrOrStderr(), "note: Better BibTeX JSON-RPC unavailable (%v); falling back to the local cache\n", err)
			}

			db, err := requireLocalCache(ctx, "zotero-research-library-pp-cli")
			if err != nil {
				return err
			}
			defer db.Close()

			// BBT pins citekeys in the item's extra field as
			// "Citation Key: <key>" when so configured.
			rows, err := db.DB().QueryContext(ctx,
				`SELECT "data" FROM "items" WHERE "data" LIKE ? LIMIT 5`,
				"%Citation Key: "+citekey+"%")
			if err != nil {
				return fmt.Errorf("searching cache for citekey: %w", err)
			}
			defer rows.Close()
			for rows.Next() {
				var raw []byte
				if err := rows.Scan(&raw); err != nil {
					return err
				}
				d, derr := decodeItemData(raw)
				if derr != nil || !hasCitationKey(d.Extra, citekey) {
					continue
				}
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"citekey": citekey,
						"format":  "json",
						"source":  "local-cache",
						"note":    "BibTeX export needs the Zotero desktop app open (Better BibTeX JSON-RPC)",
						"item":    json.RawMessage(raw),
					}, flags)
				}
				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "%s — %s", d.Title, creatorSummary(d.Creators))
				if d.Date != "" {
					fmt.Fprintf(out, " (%s)", d.Date)
				}
				fmt.Fprintf(out, "\nkey=%s", d.Key)
				if d.DOI != "" {
					fmt.Fprintf(out, "  doi=%s", d.DOI)
				}
				fmt.Fprintln(out)
				fmt.Fprintln(out, "For citekey-faithful BibTeX, open the Zotero desktop app and re-run.")
				return nil
			}
			if err := rows.Err(); err != nil {
				return err
			}

			// Last resort: a plain Zotero item key (the identifiers 'ground'
			// and 'items recent' hand to agents), or the special 'top' —
			// the most recently added item in the cache.
			itemSQL := `SELECT "data" FROM "items" WHERE "id" = ?`
			itemArg := citekey
			if citekey == "top" {
				itemSQL = `SELECT "data" FROM "items"
					WHERE json_extract("data", '$.data.itemType') NOT IN ('attachment', 'note')
					ORDER BY json_extract("data", '$.data.dateAdded') DESC LIMIT 1`
				itemArg = ""
			}
			var raw []byte
			var scanErr error
			if itemArg == "" {
				scanErr = db.DB().QueryRowContext(ctx, itemSQL).Scan(&raw)
			} else {
				scanErr = db.DB().QueryRowContext(ctx, itemSQL, itemArg).Scan(&raw)
			}
			if scanErr == nil {
				d, derr := decodeItemData(raw)
				if derr == nil && d.Key != "" {
					if flags.asJSON {
						return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
							"citekey": citekey,
							"format":  "json",
							"source":  "local-cache-item-key",
							"note":    "resolved as a Zotero item key; citekey-faithful BibTeX needs the Zotero desktop app open (Better BibTeX JSON-RPC)",
							"item":    json.RawMessage(raw),
						}, flags)
					}
					out := cmd.OutOrStdout()
					fmt.Fprintf(out, "%s — %s", d.Title, creatorSummary(d.Creators))
					if d.Date != "" {
						fmt.Fprintf(out, " (%s)", d.Date)
					}
					fmt.Fprintf(out, "\nkey=%s", d.Key)
					if d.DOI != "" {
						fmt.Fprintf(out, "  doi=%s", d.DOI)
					}
					fmt.Fprintln(out)
					fmt.Fprintln(out, "For citekey-faithful BibTeX, open the Zotero desktop app and re-run.")
					return nil
				}
			}
			return fmt.Errorf("%q not found as a Better BibTeX citekey, pinned extra-field citekey, or item key (open the Zotero desktop app for BBT citekeys, or run 'sync' if the cache is stale)", citekey)
		},
	}
	cmd.Flags().StringVar(&flagFormat, "format", "bibtex", "Output format: bibtex (via Better BibTeX RPC) or json (local cache)")
	return cmd
}

// hasCitationKey confirms the extra field pins exactly this citekey — the SQL
// LIKE above is only a prefilter and would also match citekey prefixes.
func hasCitationKey(extra, citekey string) bool {
	for _, line := range strings.Split(extra, "\n") {
		line = strings.TrimSpace(line)
		if v, ok := strings.CutPrefix(line, "Citation Key:"); ok && strings.TrimSpace(v) == citekey {
			return true
		}
	}
	return false
}

// bbtExport asks Better BibTeX to export one citekey as BibTeX.
func bbtExport(ctx context.Context, citekey string) (string, error) {
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  "item.export",
		"params":  []any{[]string{citekey}, "betterbibtex"},
		"id":      1,
	})
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, bbtEndpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var rpc struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpc); err != nil {
		return "", fmt.Errorf("decoding Better BibTeX response: %w", err)
	}
	if rpc.Error != nil {
		return "", fmt.Errorf("better bibtex: %s", rpc.Error.Message)
	}
	// item.export returns either a plain string or [status, mime, body].
	var asString string
	if json.Unmarshal(rpc.Result, &asString) == nil && asString != "" {
		return strings.TrimSpace(asString), nil
	}
	var asTuple []any
	if json.Unmarshal(rpc.Result, &asTuple) == nil && len(asTuple) >= 3 {
		if s, ok := asTuple[2].(string); ok && s != "" {
			return strings.TrimSpace(s), nil
		}
	}
	return "", fmt.Errorf("citekey %q not found by Better BibTeX", citekey)
}
