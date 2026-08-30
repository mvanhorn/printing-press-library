// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written `trash report`: what the cleanup engine trashed, grouped by
// applied plan (ledger), with the days remaining before Gmail's ~30-day
// auto-purge makes undo impossible — plus an honest count of TRASH-labeled
// mail no ledger accounts for (trashed outside this tool). Pure local read.

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/store"
)

// gmailTrashRetentionDays is Gmail's approximate trash retention. Gmail
// controls the actual purge; ~30 days is its documented behavior, so every
// number derived from it is labeled approximate.
const gmailTrashRetentionDays = 30

const trashRetentionNote = "~Gmail-controlled, approximately 30 days"

// trashReportLedgerRow is one applied plan's trash summary.
type trashReportLedgerRow struct {
	LedgerID      string `json:"ledger_id"`
	PlanSha       string `json:"plan_sha,omitempty"`
	Action        string `json:"action"`
	CreatedAt     string `json:"created_at"`
	Trashed       int    `json:"trashed"`
	Undone        int    `json:"undone"`
	Conflicts     int    `json:"conflicts"`
	AgeDays       int    `json:"age_days"`
	DaysRemaining int    `json:"days_remaining"`
	UndoHint      string `json:"undo_hint"`
}

// trashReportOutput is the trash report JSON envelope.
type trashReportOutput struct {
	Account              string                 `json:"account"`
	RetentionNote        string                 `json:"retention_note"`
	Ledgers              []trashReportLedgerRow `json:"ledgers"`
	OutsideLedgerTrashed int                    `json:"outside_ledger_trashed"`
	OutsideLedgerNote    string                 `json:"outside_ledger_note,omitempty"`
}

// trashLedgerReportRow derives the age/remaining math for one ledger row.
func trashLedgerReportRow(r store.TrashLedgerRow, now time.Time) trashReportLedgerRow {
	ageDays := 0
	if ts, err := time.Parse(time.RFC3339, r.CreatedAt); err == nil {
		ageDays = int(now.Sub(ts).Hours() / 24)
		if ageDays < 0 {
			ageDays = 0
		}
	}
	return trashReportLedgerRow{
		LedgerID:      r.LedgerID,
		PlanSha:       r.PlanSha,
		Action:        r.Action,
		CreatedAt:     r.CreatedAt,
		Trashed:       r.Trashed,
		Undone:        r.Undone,
		Conflicts:     r.Conflict,
		AgeDays:       ageDays,
		DaysRemaining: gmailTrashRetentionDays - ageDays,
		UndoHint:      undoHint(r.LedgerID),
	}
}

func newNovelTrashReportCmd(flags *rootFlags) *cobra.Command {
	var closingSoon bool

	cmd := &cobra.Command{
		Use:   "report",
		Short: "What you trashed, grouped by applied plan, with days remaining before Gmail's ~30-day purge makes undo impossible",
		Long: `Read the cleanup engine's delta ledger and report every applied plan that
trashed messages: counts (including how many entries were since undone or
conflicted), the ledger's age, and days_remaining before Gmail's auto-purge
— which is Gmail-controlled and approximately 30 days, so treat the number
as an estimate, not a deadline you can bank on.

--closing-soon keeps only ledgers with 7 or fewer days remaining — the
last-call list for 'undo --ledger <id>'.

Also surfaces how many currently-TRASH-labeled messages in the local store
belong to NO ledger (trashed outside this tool — another client, webmail);
those cannot be undone from here.

Reads only the local store and ledger — run 'sync' first so trash labels
are current.`,
		Example: `  # Everything the engine trashed, newest last
  gmail-pp-cli trash report --account personal

  # Last call: undo windows closing within 7 days
  gmail-pp-cli trash report --account personal --closing-soon --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			account, err := resolveGauthAccount(flags)
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("gmail-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			ledgers, err := db.TrashLedgers(account)
			if err != nil {
				return fmt.Errorf("reading trash ledgers: %w", err)
			}
			outside, err := db.TrashOutsideLedgerCount(account)
			if err != nil {
				return fmt.Errorf("counting outside-ledger trash: %w", err)
			}

			now := time.Now()
			out := trashReportOutput{
				Account:              account,
				RetentionNote:        trashRetentionNote,
				Ledgers:              []trashReportLedgerRow{},
				OutsideLedgerTrashed: outside,
			}
			if outside > 0 {
				out.OutsideLedgerNote = fmt.Sprintf("%d TRASH-labeled message(s) in the local store belong to no ledger (trashed outside this tool) — not undoable from here", outside)
			}
			for _, r := range ledgers {
				row := trashLedgerReportRow(r, now)
				if closingSoon && row.DaysRemaining > 7 {
					continue
				}
				out.Ledgers = append(out.Ledgers, row)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().BoolVar(&closingSoon, "closing-soon", false, "Only ledgers with 7 or fewer days of undo window remaining")
	return cmd
}
