// Copyright 2026 riccardovandra and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel (transcendence) command: campaigns variants.

package cli

import (
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"
)

type variantRow struct {
	StepID          string  `json:"step_id"`
	Order           int64   `json:"order"`
	IsVariant       bool    `json:"is_variant"`
	VariantFromStep string  `json:"variant_from_step"`
	Subject         string  `json:"subject"`
	Sent            int64   `json:"sent"`
	Replies         int64   `json:"replies"`
	Interested      int64   `json:"interested"`
	ReplyRate       float64 `json:"reply_rate"`
	InterestedRate  float64 `json:"interested_rate"`
}

func newNovelCampaignsVariantsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "variants <campaign-id>",
		Short: "Per A/B sequence-step variant, the reply rate and interested rate computed from local data.",
		Long: `Lists a campaign's sequence-step variants from the local store and computes the
reply rate and interested rate per variant from synced sent and reply counts.
Sync first with 'email-bison-pp-cli sync'.`,
		Example:     "  email-bison-pp-cli campaigns variants 6 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 1 {
				return cmd.Help()
			}
			campaignID := args[0]

			db, err := openNovelStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			const q = `
SELECT
  CAST(COALESCE(json_extract(data,'$.id'), id) AS TEXT)                 AS step_id,
  COALESCE(json_extract(data,'$.order'), 0)                             AS step_order,
  COALESCE(json_extract(data,'$.variant'), 0)                           AS is_variant,
  CAST(COALESCE(json_extract(data,'$.variant_from_step'), '') AS TEXT)  AS variant_from,
  COALESCE(json_extract(data,'$.email_subject'), '')                    AS subject,
  COALESCE(json_extract(data,'$.sent_count'), json_extract(data,'$.sent'), 0)         AS sent,
  COALESCE(json_extract(data,'$.reply_count'), json_extract(data,'$.replies'), 0)     AS replies,
  COALESCE(json_extract(data,'$.interested_count'), json_extract(data,'$.interested'), 0) AS interested
FROM sequence_steps
WHERE CAST(campaigns_id AS TEXT) = ?
ORDER BY step_order;`
			rows, err := db.DB().QueryContext(cmd.Context(), q, campaignID)
			if err != nil {
				return fmt.Errorf("querying campaign variants: %w", err)
			}
			defer rows.Close()

			results := []variantRow{}
			for rows.Next() {
				var r variantRow
				var variantFrom, subject sql.NullString
				var isVariant int64
				if err := rows.Scan(&r.StepID, &r.Order, &isVariant, &variantFrom, &subject, &r.Sent, &r.Replies, &r.Interested); err != nil {
					return fmt.Errorf("scanning variant row: %w", err)
				}
				r.IsVariant = isVariant != 0
				r.VariantFromStep = variantFrom.String
				r.Subject = subject.String
				if r.Sent > 0 {
					r.ReplyRate = float64(r.Replies) / float64(r.Sent)
					r.InterestedRate = float64(r.Interested) / float64(r.Sent)
				}
				results = append(results, r)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating variant rows: %w", err)
			}

			out := cmd.OutOrStdout()
			if flags.asJSON {
				return printJSONFiltered(out, results, flags)
			}
			if len(results) == 0 {
				fmt.Fprintf(out, "No sequence steps for campaign %s. Run 'email-bison-pp-cli sync' first if the store is empty.\n", campaignID)
				return nil
			}
			fmt.Fprintln(out, "STEP\tORDER\tVARIANT\tSUBJECT\tSENT\tREPLIES\tREPLY RATE\tINT RATE")
			for _, r := range results {
				fmt.Fprintf(out, "%s\t%d\t%t\t%s\t%d\t%d\t%.2f\t%.2f\n",
					r.StepID, r.Order, r.IsVariant, r.Subject, r.Sent, r.Replies, r.ReplyRate, r.InterestedRate)
			}
			return nil
		},
	}
	return cmd
}
