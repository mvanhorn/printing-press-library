// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written label mutations — the ONLY two in this binary (grill
// R1-C1/R2-C2): `labels create` (idempotent-by-name: an existing
// case-insensitive name match returns that label instead of creating a
// near-duplicate) and `labels rename` (ledgered with its inverse so undo
// can verify-and-reverse it). Label DELETE does not exist anywhere in this
// binary — not as a command, and not as a transport shape.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/store"
)

// gmailLabel is the subset of a label resource these commands consume.
type gmailLabel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

func newLabelsCreateSafeCmd(flags *rootFlags) *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a label, idempotently by name: a case-insensitive existing match returns that label instead of minting a duplicate",
		Long: `Create a user label — safely re-runnable.

The live labels list is checked first; if any label already carries this
name (case-insensitively), that label's id is returned with existing=true
and nothing is created. Otherwise the label is created and returned with
created=true. This makes scripted setup idempotent and prevents the
"Newsletters" / "newsletters" near-duplicate mess.

Typed exits: 0 created or already-existing / 2 usage / 4 identity refusal /
5 auth or API failure.`,
		Example: `  gmail-pp-cli labels create --name Newsletters --account personal
  gmail-pp-cli labels create --name "Receipts/2026" --agent`,
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2,4,5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			account, err := resolveGauthAccount(flags)
			if err != nil {
				return err
			}
			name = strings.TrimSpace(name)
			if name == "" {
				return usageErr(fmt.Errorf("--name must not be empty"))
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true
			if err := verifyLiveIdentity(ctx, c, flags, account); err != nil {
				return classifyEngineAPIError(err)
			}

			data, err := c.GetWithHeadersNoCache(ctx, "/gmail/v1/users/me/labels", nil, nil)
			if err != nil {
				return classifyEngineAPIError(err)
			}
			var page struct {
				Labels []gmailLabel `json:"labels"`
			}
			if err := json.Unmarshal(data, &page); err != nil {
				return fmt.Errorf("parsing labels list: %w", err)
			}
			for _, l := range page.Labels {
				if strings.EqualFold(l.Name, name) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"id": l.ID, "name": l.Name, "existing": true, "created": false,
						"note": "a label with this name already exists (matched case-insensitively); returning it instead of creating a duplicate",
					}, flags)
				}
			}

			var created gmailLabel
			if err := engineCall(ctx, func(cctx context.Context) error {
				resp, _, perr := c.Post(cctx, "/gmail/v1/users/me/labels", map[string]any{"name": name})
				if perr != nil {
					return perr
				}
				return json.Unmarshal(resp, &created)
			}); err != nil {
				return classifyEngineAPIError(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"id": created.ID, "name": created.Name, "existing": false, "created": true,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Label name to create (or return, when it already exists)")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func newLabelsRenameCmd(flags *rootFlags) *cobra.Command {
	var labelID, to string

	cmd := &cobra.Command{
		Use:   "rename",
		Short: "Rename a label by id, ledgering the inverse so 'undo --ledger <id>' can verify-and-reverse it",
		Long: `Rename one label. The old name is ledgered as the rename's inverse:
'undo --ledger <id>' verifies the label is still named the new name and
renames it back — if someone renamed it again since, undo reports a
conflict instead of forcing.

Typed exits: 0 renamed (or no-op when already named) / 2 usage / 4 identity
refusal / 5 auth or API failure.`,
		Example: `  gmail-pp-cli labels rename --id Label_1234 --to "Receipts (archive)"
  # undo hint is printed with a ledger id:
  gmail-pp-cli undo --ledger <ledger_id>`,
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2,4,5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			account, err := resolveGauthAccount(flags)
			if err != nil {
				return err
			}
			labelID = strings.TrimSpace(labelID)
			to = strings.TrimSpace(to)
			if labelID == "" || to == "" {
				return usageErr(fmt.Errorf("--id and --to are both required"))
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true
			if err := verifyLiveIdentity(ctx, c, flags, account); err != nil {
				return classifyEngineAPIError(err)
			}

			var current gmailLabel
			if err := engineCall(ctx, func(cctx context.Context) error {
				data, gerr := c.GetWithHeadersNoCache(cctx, "/gmail/v1/users/me/labels/"+url.PathEscape(labelID), nil, nil)
				if gerr != nil {
					return gerr
				}
				return json.Unmarshal(data, &current)
			}); err != nil {
				return classifyEngineAPIError(fmt.Errorf("reading label %s: %w", labelID, err))
			}
			if current.Name == to {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"id": labelID, "from": current.Name, "to": to, "renamed": false,
					"note": "label already carries this exact name — nothing to do, nothing ledgered",
				}, flags)
			}

			if err := engineCall(ctx, func(cctx context.Context) error {
				_, _, perr := c.Patch(cctx, "/gmail/v1/users/me/labels/"+url.PathEscape(labelID), map[string]any{"name": to})
				return perr
			}); err != nil {
				return classifyEngineAPIError(err)
			}

			// Ledger the inverse (old name) so undo can verify-and-reverse.
			db, err := store.OpenWithContext(ctx, defaultDBPath("gmail-pp-cli"))
			if err != nil {
				return fmt.Errorf("renamed, but opening the local database to ledger the inverse failed: %w", err)
			}
			defer db.Close()
			ledgerID, err := mintHex(8)
			if err != nil {
				return err
			}
			if err := db.CreateMailLedger(store.MailLedger{
				LedgerID: ledgerID, Account: account, Action: "label_rename",
			}); err != nil {
				return fmt.Errorf("renamed, but recording the undo ledger failed: %w", err)
			}
			if _, err := db.InsertMailLedgerEntries([]store.MailLedgerEntry{{
				LedgerID: ledgerID, ID: labelID, Kind: "label_rename",
				OldName: current.Name, NewName: to,
			}}); err != nil {
				return fmt.Errorf("renamed, but recording the undo ledger entry failed: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"id": labelID, "from": current.Name, "to": to, "renamed": true,
				"ledger_id": ledgerID, "undo_hint": undoHint(ledgerID),
			}, flags)
		},
	}
	cmd.Flags().StringVar(&labelID, "id", "", "Label id to rename (see 'labels list')")
	cmd.Flags().StringVar(&to, "to", "", "New label name")
	return cmd
}
