// Hand-written novel command.
//
//	// pp:client-call
//
// transferable-sources reads /api/offers (the real Pointhound endpoint) for a
// given search session and surfaces the per-source transferOptions table —
// which earn programs feed a given redeem program with what ratio and how
// quickly. The transferOptions data is unique to Pointhound's offers payload
// and isn't exposed as a standalone API surface.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newTransferableSourcesCmd(flags *rootFlags) *cobra.Command {
	var searchID string
	var redeemFilter string
	var earnFilter string

	cmd := &cobra.Command{
		Use:   "transferable-sources [redeem-program]",
		Short: "List earn programs that transfer into a redeem program (with ratio + transfer time)",
		Long: strings.TrimSpace(`
Surface Pointhound's per-source transferOptions table. Given a redeem program
(United MileagePlus, Delta SkyMiles, etc.) — referenced either by its name
substring or its prp_* ID — list every transferable earn program (Chase UR,
Amex MR, Bilt, etc.) that feeds it, along with the transfer ratio
(e.g. 1.0 for Chase UR → United, 0.333 for Marriott Bonvoy → United) and
transfer time (instant vs up_to_72 hours).

Reads the live /api/offers response for an existing search session so the data
is always current.
`),
		Example: strings.Trim(`
  pointhound-pp-cli transferable-sources united --search-id ofs_xxx --json
  pointhound-pp-cli transferable-sources --search-id ofs_xxx --earn-program chase-ultimate-rewards
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && searchID == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if searchID == "" {
				return usageErr(fmt.Errorf("required flag --search-id not set"))
			}
			if len(args) > 0 {
				redeemFilter = args[0]
			}
			rows, err := fetchTransferOptions(cmd.Context(), flags, searchID, redeemFilter, earnFilter)
			if err != nil {
				return err
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No transferable sources found for the given filters.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "REDEEM_PROGRAM\tEARN_PROGRAM\tRATIO\tTOTAL_RATIO\tTIME\tBONUS")
			for _, r := range rows {
				bonus := "-"
				if r.BonusTransferRatio != "" {
					bonus = r.BonusTransferRatio
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", r.RedeemProgramName, r.EarnProgramName, r.TransferRatio, r.TotalTransferRatio, r.TransferTime, bonus)
			}
			_ = tw.Flush()
			return nil
		},
	}

	cmd.Flags().StringVar(&searchID, "search-id", "", "Pointhound search session id (ofs_*). Required.")
	cmd.Flags().StringVar(&earnFilter, "earn-program", "", "Filter to one earn program (name substring or pep_* id).")
	return cmd
}

// TransferRow is the flattened view of one transfer option.
type TransferRow struct {
	RedeemProgramID    string `json:"redeemProgramId"`
	RedeemProgramName  string `json:"redeemProgramName"`
	EarnProgramID      string `json:"earnProgramId"`
	EarnProgramName    string `json:"earnProgramName"`
	TransferRatio      string `json:"transferRatio"`
	TotalTransferRatio string `json:"totalTransferRatio"`
	TransferTime       string `json:"transferTime"`
	BonusTransferRatio string `json:"bonusTransferRatio,omitempty"`
	BonusDateEnd       string `json:"bonusDateEnd,omitempty"`
}

// fetchTransferOptions calls /api/offers and walks the embedded
// source.redeemProgram.transferOptions arrays, flattening to TransferRow.
func fetchTransferOptions(ctx context.Context, flags *rootFlags, searchID, redeemFilter, earnFilter string) ([]TransferRow, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	params := map[string]string{
		"searchId":   searchID,
		"take":       "20",
		"offset":     "0",
		"sortOrder":  "asc",
		"sortBy":     "points",
		"cabins":     "economy",
		"passengers": "1",
	}
	data, _, err := resolveRead(ctx, c, flags, "offers", false, "/api/offers", params, nil)
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("re-encoding offers response: %w", err)
	}
	var envelope struct {
		Data []struct {
			Source struct {
				RedeemProgram struct {
					ID              string `json:"id"`
					Name            string `json:"name"`
					TransferOptions []struct {
						ID                 string  `json:"id"`
						EarnProgramID      string  `json:"earnProgramId"`
						RedeemProgramID    string  `json:"redeemProgramId"`
						TransferRatio      string  `json:"transferRatio"`
						TotalTransferRatio string  `json:"totalTransferRatio"`
						TransferTime       string  `json:"transferTime"`
						BonusTransferRatio *string `json:"bonusTransferRatio"`
						BonusDateEnd       *string `json:"bonusDateEnd"`
						EarnProgram        struct {
							ID         string `json:"id"`
							Name       string `json:"name"`
							Identifier string `json:"identifier"`
						} `json:"earnProgram"`
					} `json:"transferOptions"`
				} `json:"redeemProgram"`
			} `json:"source"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decoding offers envelope: %w", err)
	}
	seen := map[string]bool{}
	var rows []TransferRow
	for _, offer := range envelope.Data {
		rp := offer.Source.RedeemProgram
		if redeemFilter != "" {
			if !strings.Contains(strings.ToLower(rp.Name), strings.ToLower(redeemFilter)) && rp.ID != redeemFilter {
				continue
			}
		}
		for _, to := range rp.TransferOptions {
			key := rp.ID + "|" + to.EarnProgramID
			if seen[key] {
				continue
			}
			seen[key] = true
			if earnFilter != "" {
				if !strings.Contains(strings.ToLower(to.EarnProgram.Name), strings.ToLower(earnFilter)) &&
					to.EarnProgramID != earnFilter &&
					to.EarnProgram.Identifier != earnFilter {
					continue
				}
			}
			row := TransferRow{
				RedeemProgramID:    rp.ID,
				RedeemProgramName:  rp.Name,
				EarnProgramID:      to.EarnProgramID,
				EarnProgramName:    to.EarnProgram.Name,
				TransferRatio:      to.TransferRatio,
				TotalTransferRatio: to.TotalTransferRatio,
				TransferTime:       to.TransferTime,
			}
			if to.BonusTransferRatio != nil {
				row.BonusTransferRatio = *to.BonusTransferRatio
			}
			if to.BonusDateEnd != nil {
				row.BonusDateEnd = *to.BonusDateEnd
			}
			rows = append(rows, row)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].RedeemProgramName != rows[j].RedeemProgramName {
			return rows[i].RedeemProgramName < rows[j].RedeemProgramName
		}
		return rows[i].EarnProgramName < rows[j].EarnProgramName
	})
	return rows, nil
}
