package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-pricebook/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-pricebook/internal/config"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-pricebook/internal/pricebook"
)

// changeFileRow is one reviewed SKU change read from a bulk-plan input file.
// cost and price are *float64 so a row can carry only the field it changes.
type changeFileRow struct {
	Kind  string   `json:"kind"`
	ID    int64    `json:"id"`
	Cost  *float64 `json:"cost"`
	Price *float64 `json:"price"`
}

func newBulkPlanCmd(flags *rootFlags) *cobra.Command {
	var apply bool

	cmd := &cobra.Command{
		Use:   "bulk-plan <file>",
		Short: "Turn a reviewed cost/price change file into one pricebook bulk-update (preview unless --apply)",
		Long: "Reads a reviewed list of SKU changes — a CSV or JSON file with kind,\n" +
			"id, and an optional cost and/or price per row — and assembles a single\n" +
			"pricebook bulk-update payload. Routing a batch through one call instead\n" +
			"of N individual updates matters under ServiceTitan's ~7k/hr rate limit.\n\n" +
			"Without --apply this prints the assembled payload. With --apply it\n" +
			"sends it.\n\n" +
			"CSV header columns: kind, id, cost, price (cost and price cells may be\n" +
			"blank). JSON: an array of {\"kind\",\"id\",\"cost\",\"price\"} objects.",
		Example: strings.Trim(`
  servicetitan-pricebook-pp-cli bulk-plan ./reviewed-changes.csv
  servicetitan-pricebook-pp-cli bulk-plan ./reviewed-changes.json --apply
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			rows, err := parseChangeFile(args[0])
			if err != nil {
				return err
			}
			changes := make([]pricebook.BulkChange, 0, len(rows))
			for _, r := range rows {
				kind := pricebook.SKUKind(strings.ToLower(strings.TrimSpace(r.Kind)))
				if kind != pricebook.KindMaterial && kind != pricebook.KindEquipment {
					return fmt.Errorf("row id %d: kind must be \"material\" or \"equipment\", got %q", r.ID, r.Kind)
				}
				if r.ID == 0 {
					return fmt.Errorf("change file has a row with no id")
				}
				changes = append(changes, pricebook.BulkChange{Kind: kind, ID: r.ID, Cost: r.Cost, Price: r.Price})
			}
			payload := pricebook.BulkPlan(changes)

			if !apply {
				out, err := payload.MarshalIndent()
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}

			if len(payload.Materials) == 0 && len(payload.Equipment) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no applicable changes in the file (every row was missing both cost and price)")
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would apply %d material + %d equipment changes to the pricebook\n",
					len(payload.Materials), len(payload.Equipment))
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			tenant := cfg.TenantID
			if tenant == "" {
				return fmt.Errorf("ST_TENANT_ID is not set — cannot apply changes")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			_, status, err := c.Patch("/tenant/"+tenant+"/pricebook", payload)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "applied %d material + %d equipment changes (HTTP %d)\n",
				len(payload.Materials), len(payload.Equipment), status)
			return nil
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "Send the assembled bulk-update (default: print the payload only)")
	return cmd
}

// parseChangeFile reads a bulk-plan input file as JSON (when the extension is
// .json) or CSV otherwise.
func parseChangeFile(path string) ([]changeFileRow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading change file: %w", err)
	}
	if strings.EqualFold(filepath.Ext(path), ".json") {
		var rows []changeFileRow
		if err := json.Unmarshal(data, &rows); err != nil {
			return nil, fmt.Errorf("parsing change JSON: %w", err)
		}
		return rows, nil
	}
	return parseChangeCSV(data)
}

func parseChangeCSV(data []byte) ([]changeFileRow, error) {
	r := csv.NewReader(strings.NewReader(string(data)))
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parsing change CSV: %w", err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("change CSV has no data rows (need a header row plus at least one change)")
	}
	kindIdx, idIdx, costIdx, priceIdx := -1, -1, -1, -1
	for i, h := range records[0] {
		switch strings.ToLower(strings.TrimSpace(h)) {
		case "kind", "sku_kind", "type":
			kindIdx = i
		case "id", "sku_id":
			idIdx = i
		case "cost":
			costIdx = i
		case "price":
			priceIdx = i
		}
	}
	if kindIdx < 0 || idIdx < 0 {
		return nil, fmt.Errorf("change CSV header must include a kind column and an id column; got %v", records[0])
	}
	var rows []changeFileRow
	for _, rec := range records[1:] {
		if kindIdx >= len(rec) || idIdx >= len(rec) {
			continue
		}
		idStr := strings.TrimSpace(rec[idIdx])
		if idStr == "" {
			continue
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("change CSV: invalid id %q: %w", rec[idIdx], err)
		}
		row := changeFileRow{Kind: strings.TrimSpace(rec[kindIdx]), ID: id}
		if costIdx >= 0 && costIdx < len(rec) {
			if v, ok := parseOptFloat(rec[costIdx]); ok {
				row.Cost = &v
			}
		}
		if priceIdx >= 0 && priceIdx < len(rec) {
			if v, ok := parseOptFloat(rec[priceIdx]); ok {
				row.Price = &v
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// parseOptFloat parses a possibly-blank numeric cell, stripping $ and commas.
func parseOptFloat(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "$")
	s = strings.ReplaceAll(s, ",", "")
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
