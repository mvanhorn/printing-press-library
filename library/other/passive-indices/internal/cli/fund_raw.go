// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call — calls the hand-written sibling client (internal/niftyindices or internal/indiapassivefunds) via a package-local newXClient() helper, not the generated internal/client package.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelFundRawCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "raw <schemeId>",
		Short:       "See a fund's raw API response with cryptic field codes resolved to human-readable names.",
		Example:     "  passive-indices-pp-cli fund raw 1150 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch raw fund detail")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("schemeId is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c := newIndiaPassiveFundsClient(flags)
			fd, err := c.FundDetail(ctx, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var generic any
			if err := json.Unmarshal(fd.Raw, &generic); err != nil {
				return fmt.Errorf("parsing raw fund detail: %w", err)
			}
			decoded := decodeFieldCodesDeep(generic)
			return flags.printJSON(cmd, decoded)
		},
	}
	return cmd
}

// decodeFieldCodesDeep recursively walks a raw indiapassivefunds response
// and, at every nested {columns, data} section it finds, replaces field
// codes (f_29, f_36, ...) in each data row with the section's own
// displayName. indiapassivefunds nests these sections arbitrarily deep and
// inconsistently (see funddescription's mid-array section1/section2), so a
// flat single-pass decoder misses most of the payload.
func decodeFieldCodesDeep(v any) any {
	switch t := v.(type) {
	case map[string]any:
		if cols, ok := t["columns"].([]any); ok {
			if rows, ok := t["data"].([]any); ok {
				fieldToName := make(map[string]string, len(cols))
				for _, colAny := range cols {
					col, ok := colAny.(map[string]any)
					if !ok {
						continue
					}
					field, _ := col["field"].(string)
					name, _ := col["displayName"].(string)
					if name == "" {
						name, _ = col["displayname"].(string)
					}
					if field != "" && name != "" {
						fieldToName[field] = name
					}
				}
				decodedRows := make([]any, 0, len(rows))
				for _, rowAny := range rows {
					row, ok := rowAny.(map[string]any)
					if !ok {
						decodedRows = append(decodedRows, decodeFieldCodesDeep(rowAny))
						continue
					}
					decodedRow := make(map[string]any, len(row))
					for k, val := range row {
						key := k
						if name, ok := fieldToName[k]; ok {
							key = name
						}
						decodedRow[key] = decodeFieldCodesDeep(val)
					}
					decodedRows = append(decodedRows, decodedRow)
				}
				out := make(map[string]any, len(t))
				for k, val := range t {
					if k == "data" {
						out[k] = decodedRows
						continue
					}
					out[k] = decodeFieldCodesDeep(val)
				}
				return out
			}
		}
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = decodeFieldCodesDeep(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = decodeFieldCodesDeep(val)
		}
		return out
	default:
		return v
	}
}
