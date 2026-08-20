// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/productivity/bonusly/internal/store"
	"github.com/spf13/cobra"
)

func newNovelRecognitionSearchMineCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "search-mine <query>",
		Short:       "Search your own given and received recognition offline, without scrolling the public company feed.",
		Example:     "  bonusly-pp-cli recognition search-mine 'migration project' --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				q := "(no query given)"
				if len(args) >= 1 {
					q = fmt.Sprintf("%q", args[0])
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would search my recognition history for %s\n", q)
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("query argument is required"))
			}
			query := args[0]

			// check missing mirror
			isMissing, dbPath, err := checkMissingMirrorGuard(cmd, flags)
			if err != nil {
				return err
			}
			if isMissing {
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// pp:hand-edit bonusly-endpoint-fix — /users/me wraps the user
			// object in a {"success":...,"result":{...}} envelope; a raw
			// c.Get()+Unmarshal straight into types.User left every field
			// (including Id) silently empty.
			me, err := resolveMyUser(cmd.Context(), c, flags, cmd.ErrOrStderr())
			if err != nil {
				// pp:hand-edit bonusly-dogfood-exit-code — see the matching
				// comment in recognition_gap.go.
				return classifyAPIError(err, flags)
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT id, giver_id, receiver_ids, amount, reason, hashtags, created_at 
				FROM recognition 
				WHERE reason LIKE '%' || ? || '%' COLLATE NOCASE`, query)
			if err != nil {
				return err
			}
			defer rows.Close()

			type recRow struct {
				ID          string
				GiverID     sql.NullString
				ReceiverIDs sql.NullString
				Amount      sql.NullInt64
				Reason      sql.NullString
				Hashtags    sql.NullString
				CreatedAt   sql.NullString
			}

			var recs []recRow
			for rows.Next() {
				var r recRow
				if err := rows.Scan(&r.ID, &r.GiverID, &r.ReceiverIDs, &r.Amount, &r.Reason, &r.Hashtags, &r.CreatedAt); err != nil {
					return err
				}
				recs = append(recs, r)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			var matched []map[string]any
			for _, r := range recs {
				isMine := false
				if r.GiverID.Valid && r.GiverID.String == me.Id {
					isMine = true
				} else if r.ReceiverIDs.Valid && r.ReceiverIDs.String != "" {
					receivers := parseArrayString(r.ReceiverIDs.String)
					for _, recID := range receivers {
						if recID == me.Id {
							isMine = true
							break
						}
					}
				}

				if isMine {
					matchedItem := map[string]any{
						"id":           r.ID,
						"giver_id":     r.GiverID.String,
						"receiver_ids": parseArrayString(r.ReceiverIDs.String),
						"amount":       r.Amount.Int64,
						"reason":       r.Reason.String,
						"hashtags":     parseArrayString(r.Hashtags.String),
						"created_at":   r.CreatedAt.String,
					}
					matched = append(matched, matchedItem)
				}
			}

			matchedJSON, err := json.Marshal(matched)
			if err != nil {
				return err
			}

			return printOutputWithFlagsMeta(cmd.OutOrStdout(), json.RawMessage(matchedJSON), flags, map[string]any{"source": "local"})
		},
	}
	return cmd
}
