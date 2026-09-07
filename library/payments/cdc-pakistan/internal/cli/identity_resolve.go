// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type resolveView struct {
	Query   string `json:"query"`
	AsOf    string `json:"as_of"`
	Outcome string `json:"outcome"`
	// NameThen is the identity the notice put IN FORCE -- the "to" side of the
	// change -- not the notice's prose title. Assigning the whole title here
	// was a real defect: callers asked for an identity to join on and received
	// "Change of Security Name and Symbol - X to Y", which joins to nothing and
	// therefore cannot do the one job this command exists for (stopping symbol
	// recycling from splicing two issuers into one series).
	NameThen string `json:"name_in_force,omitempty"`
	// NoticeTitle keeps the raw title so nothing is dropped, and so an
	// unparseable notice is still inspectable.
	NoticeTitle string `json:"source_notice_title,omitempty"`
	// Parsed says whether NameThen was actually extracted. Deliberately
	// WITHOUT omitempty: NameThen already has omitempty, so on an unparseable
	// title that key vanishes, and a false-but-omitted flag would leave the
	// caller unable to tell "unparsed" from "key absent for another reason".
	// identity ledger sets the same precedent -- unparsed events are returned
	// with parsed=false, never dropped.
	Parsed     bool   `json:"identity_parsed"`
	Renames    int    `json:"renames_known"`
	NextChange string `json:"next_change_after_as_of,omitempty"`
	PrevChange string `json:"last_change_before_as_of,omitempty"`
	Note       string `json:"note"`
	Caveat     string `json:"caveat"`
}

// Outcomes. NOT_IN_VINTAGE is a first-class answer: returning a guess where the
// record does not reach is how a symbol-keyed join silently splices issuers.
const (
	outcomeResolved     = "RESOLVED"
	outcomeNotInVintage = "NOT_IN_VINTAGE"
	outcomeNoRenames    = "NO_RENAMES_KNOWN"
)

func newNovelIdentityResolveCmd(flags *rootFlags) *cobra.Command {
	var symbol, asOf, dbPath string

	cmd := &cobra.Command{
		Use:   "resolve",
		Short: "Resolve a symbol to the identity in force on a given date, or say NOT_IN_VINTAGE rather than guess.",
		Long: strings.Trim(`
Resolve (symbol, date) to the security identity in force.

This is the guard against symbol recycling. If CDC's record does not reach back
to the requested date, the answer is NOT_IN_VINTAGE -- never a guess. A guess
here is precisely how two different issuers get spliced into one return series.

HONEST LIMIT ON THE DATE. CDC's listing gives a rename notice a day and month
but NO YEAR, and the only full date available is the /assets/uploads/YYYY/MM/
UPLOAD date, which can trail the notice's own effective date. So resolution is
to upload-month precision and the output says so. A same-month query near a
rename boundary is not decidable from this source.

Requires a document index; run 'coverage map --probe' first.
`, "\n"),
		Example:     "  cdc-pakistan-pp-cli identity resolve --symbol LOTCHEM --as-of 2019-06-30 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--symbol=LOTCHEM;--as-of=2019-06-30", "pp:typed-exit-codes": "0,2,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "identity resolve")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if strings.TrimSpace(symbol) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--symbol is required"))
			}
			if asOf == "" {
				asOf = time.Now().Format("2006-01-02")
			}
			if _, err := time.Parse("2006-01-02", asOf); err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--as-of must be YYYY-MM-DD, got %q", asOf))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("cdc-pakistan-pp-cli")
			}
			view := &resolveView{Query: symbol, AsOf: asOf, Caveat: resolveCaveat}
			db, ready, oerr := openCDCMirror(ctx, dbPath, "cdc_documents")
			if oerr != nil {
				return oerr
			}
			if !ready {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: cdc-pakistan-pp-cli coverage map --probe --db %s\n", dbPath, dbPath)
				view.Outcome, view.Note = outcomeNotInVintage, "no local document index; nothing can be resolved"
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				return nil
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(ctx,
				`SELECT title, COALESCE(upload_year,''), COALESCE(upload_month,'')
				   FROM cdc_documents
				  WHERE event_kind = 'identity' AND title LIKE ?
				  ORDER BY COALESCE(upload_year,''), COALESCE(upload_month,'')`,
				"%"+symbol+"%")
			if err != nil {
				return apiErr(err)
			}
			type change struct{ ym, title string }
			var changes []change
			for rows.Next() {
				var t, y, m string
				if err := rows.Scan(&t, &y, &m); err != nil {
					_ = rows.Close()
					return apiErr(err)
				}
				if y == "" || m == "" {
					// A legacy path carries no date at all; it cannot position a
					// rename on a timeline, so it is counted but not used.
					changes = append(changes, change{ym: "", title: t})
					continue
				}
				changes = append(changes, change{ym: y + "-" + m, title: t})
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return apiErr(err)
			}
			if err := rows.Close(); err != nil {
				return apiErr(err)
			}

			view.Renames = len(changes)
			target := asOf[:7] // YYYY-MM, matching the source's real precision
			switch {
			case len(changes) == 0:
				view.Outcome = outcomeNoRenames
				view.Note = fmt.Sprintf("no rename notice in the local index mentions %q. That means CDC's notice TITLES never named it -- it is not proof the symbol was never reused.", symbol)
			default:
				var prev, next string
				for _, c := range changes {
					if c.ym == "" {
						continue
					}
					if c.ym <= target {
						prev = c.ym
					} else if next == "" {
						next = c.ym
					}
				}
				view.PrevChange, view.NextChange = prev, next
				if prev == "" && next == "" {
					view.Outcome = outcomeNotInVintage
					view.Note = "rename notices exist for this symbol but none carry a usable date, so the identity in force cannot be positioned on a timeline"
				} else if prev == "" {
					view.Outcome = outcomeNotInVintage
					view.Note = fmt.Sprintf("the earliest dated rename for this symbol is %s, which is AFTER the requested as-of month %s; CDC's record does not reach back that far", next, target)
				} else {
					view.Outcome = outcomeResolved
					for _, c := range changes {
						if c.ym != prev {
							continue
						}
						// Keep the provenance regardless of whether the title
						// parses, then extract the identity the notice put in
						// force. changeRe is the same expression identity
						// ledger uses, so both surfaces agree on what a
						// rename notice means.
						view.NoticeTitle = c.title
						if m := changeRe.FindStringSubmatch(c.title); m != nil {
							view.NameThen = strings.TrimSpace(m[2])
							view.Parsed = true
						}
					}
					view.Note = fmt.Sprintf("identity in force resolved to UPLOAD-MONTH precision (%s). The next known change is %s.", prev, orNone(next))
					if !view.Parsed {
						view.Note += " The notice title did not match the change-of-name/symbol shape, so no identity could be extracted from it: source_notice_title carries the raw title and identity_parsed is false. Do NOT join on this result."
					}
				}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s as of %s -> %s\n", view.Query, view.AsOf, view.Outcome)
			if view.Parsed {
				fmt.Fprintf(w, "  identity in force: %s\n", truncStr(view.NameThen, 72))
			} else if view.NoticeTitle != "" {
				fmt.Fprintf(w, "  identity in force: UNPARSED (do not join)\n")
			}
			if view.NoticeTitle != "" {
				fmt.Fprintf(w, "  source notice:     %s\n", truncStr(view.NoticeTitle, 72))
			}
			fmt.Fprintf(w, "  renames known: %d  last before: %s  next after: %s\n",
				view.Renames, orNone(view.PrevChange), orNone(view.NextChange))
			fmt.Fprintf(w, "\n%s\ncaveat: %s\n", view.Note, view.Caveat)
			return nil
		},
	}
	cmd.Flags().StringVar(&symbol, "symbol", "", "symbol or company name to resolve (required)")
	cmd.Flags().StringVar(&asOf, "as-of", "", "resolve the identity in force on this date, YYYY-MM-DD (default: today)")
	cmd.Flags().StringVar(&dbPath, "db", "", "database path")
	return cmd
}

const resolveCaveat = "resolution is to UPLOAD-MONTH precision because CDC's rename notices carry a day and month but no year; NOT_IN_VINTAGE is returned rather than a guess when the record does not reach the requested date."

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}
