// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/cdc-pakistan/internal/store"
)

// identityEvent is one symbol-or-name change.
type identityEvent struct {
	SymbolFrom string `json:"symbol_from,omitempty"`
	SymbolTo   string `json:"symbol_to,omitempty"`
	NameFrom   string `json:"name_from,omitempty"`
	NameTo     string `json:"name_to,omitempty"`
	AsOfHint   string `json:"as_of_hint"`
	UploadYM   string `json:"upload_year_month,omitempty"`
	SourceURL  string `json:"source_url"`
	Title      string `json:"title"`
	Parsed     bool   `json:"parsed"`
}

type identityView struct {
	Subject string          `json:"subject,omitempty"`
	Events  []identityEvent `json:"events"`
	Total   int             `json:"total_identity_documents"`
	Parsed  int             `json:"parsed_into_from_to"`
	Note    string          `json:"note,omitempty"`
	Caveat  string          `json:"caveat"`
}

// changeRe pulls the from/to pair out of a change-of-name-and-symbol title.
// CDC writes these as prose, e.g.
//
//	"Change of Security Name and Symbol – ICI Pakistan Limited to Lucky Core Industries Limited"
//
// so the arrow is the word "to" and the separator is an en dash. Titles that do
// not fit are kept with Parsed=false rather than discarded -- a dropped rename
// is exactly how symbol recycling becomes invisible.
var changeRe = regexp.MustCompile(`(?i)change of (?:security )?(?:name and symbol|symbol and name|symbol|name)\s*[–\-—:]\s*(.+?)\s+to\s+(.+?)\s*$`)

func newNovelIdentityLedgerCmd(flags *rootFlags) *cobra.Command {
	var subject string
	var limit int
	var dbPath string
	var unparsedOnly bool

	cmd := &cobra.Command{
		Use:   "ledger",
		Short: "Reconstruct a security's symbol, name and ISIN history so you never splice two different issuers into one return series.",
		Long: strings.Trim(`
List CDC's dated identity-change events.

Symbol recycling is the failure this exists to prevent: PSX reuses ticker
symbols, so a price series keyed on symbol alone can silently splice two
different issuers into one return history and corrupt every factor computed
from it.

CDC announces renames in its circulars and notices, which is the only dated
public record of them. This command extracts the from/to pair where the title
follows CDC's usual prose shape and KEEPS the event flagged as unparsed where it
does not, because a dropped rename is worse than an ugly one.

HONEST LIMIT: CDC does not attach a machine-readable identifier to these
notices, so the from/to values are the company names or symbols as written in
the title. They are not guaranteed to match a PSX symbol exactly.

Requires a document index; run 'coverage map --probe' first.
`, "\n"),
		Example:     "  cdc-pakistan-pp-cli identity ledger --symbol LOTCHEM --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--limit=5", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "identity ledger")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("cdc-pakistan-pp-cli")
			}
			view := &identityView{Events: []identityEvent{}, Caveat: identityCaveat}
			db, ready, oerr := openCDCMirror(ctx, dbPath, "cdc_documents")
			if oerr != nil {
				return oerr
			}
			if !ready {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: cdc-pakistan-pp-cli coverage map --probe --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				return nil
			}
			db, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return configErr(fmt.Errorf("opening store: %w", err))
			}
			defer db.Close()

			q := `SELECT title, url, COALESCE(meta_day_month,''),
			             COALESCE(upload_year,'')||'-'||COALESCE(upload_month,'')
			        FROM cdc_documents WHERE event_kind = 'identity'`
			var argv []any
			if s := strings.TrimSpace(subject); s != "" {
				q += ` AND title LIKE ?`
				argv = append(argv, "%"+s+"%")
				view.Subject = s
			}
			q += ` ORDER BY COALESCE(upload_year,''), COALESCE(upload_month,''), url`
			if limit > 0 {
				q += fmt.Sprintf(" LIMIT %d", limit)
			}
			rows, err := db.DB().QueryContext(ctx, q, argv...)
			if err != nil {
				return apiErr(fmt.Errorf("querying identity events: %w", err))
			}
			for rows.Next() {
				var e identityEvent
				if err := rows.Scan(&e.Title, &e.SourceURL, &e.AsOfHint, &e.UploadYM); err != nil {
					_ = rows.Close()
					return apiErr(err)
				}
				e.UploadYM = strings.Trim(e.UploadYM, "-")
				if m := changeRe.FindStringSubmatch(e.Title); m != nil {
					e.NameFrom = strings.TrimSpace(m[1])
					e.NameTo = strings.TrimSpace(m[2])
					e.Parsed = true
					view.Parsed++
				}
				view.Total++
				if unparsedOnly && e.Parsed {
					continue
				}
				view.Events = append(view.Events, e)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return apiErr(err)
			}
			if err := rows.Close(); err != nil {
				return apiErr(err)
			}

			if view.Total == 0 {
				view.Note = "no identity-change documents in the local store. Either coverage map has not run, or no rename notice title matched."
			} else {
				view.Note = fmt.Sprintf("%d of %d identity documents parsed into an explicit from/to pair; the remainder are returned with parsed=false rather than dropped",
					view.Parsed, view.Total)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(view.Events) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			w := cmd.OutOrStdout()
			for _, e := range view.Events {
				if e.Parsed {
					fmt.Fprintf(w, "%-12s %s\n             -> %s\n", e.AsOfHint, e.NameFrom, e.NameTo)
				} else {
					fmt.Fprintf(w, "%-12s [unparsed] %s\n", e.AsOfHint, truncStr(e.Title, 68))
				}
			}
			fmt.Fprintf(w, "\n%s\ncaveat: %s\n", view.Note, view.Caveat)
			return nil
		},
	}
	cmd.Flags().StringVar(&subject, "symbol", "", "match rename notices whose title contains this symbol or company name")
	cmd.Flags().IntVar(&limit, "limit", 200, "maximum events to return")
	cmd.Flags().BoolVar(&unparsedOnly, "unparsed-only", false, "return only events whose from/to pair could not be extracted")
	cmd.Flags().StringVar(&dbPath, "db", "", "database path")
	return cmd
}

const identityCaveat = "from/to values are company names or symbols as written in CDC's own notice title; they are not guaranteed to match a PSX symbol exactly. Unparsed events are returned with parsed=false, never dropped."
