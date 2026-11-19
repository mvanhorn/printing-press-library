// Hand-authored (Slice 2): inventory set-qty.
// pp:data-source live

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		if p := s2FindChild(root, "inventory"); p != nil {
			addNovelCommandIfAbsent(p, newS2InventorySetQtyCmd(flags))
			addNovelCommandIfAbsent(p, newS2InventoryLowStockCmd(flags))
		}
	})
}

// s2FindChild returns the direct child command with the given name.
func s2FindChild(parent *cobra.Command, name string) *cobra.Command {
	for _, c := range parent.Commands() {
		if c.Name() == name {
			return c
		}
	}
	return nil
}

// s2InvEdit lets a caller mutate the PUT form of one offering while the
// inventory is normalized. prod/off are the source (GET) objects; out is the
// offering as it will be sent.
type s2InvEdit func(prod, off, out map[string]any)

func s2DecodeObject(raw []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	if m == nil {
		return nil, fmt.Errorf("empty JSON object")
	}
	return m, nil
}

func s2Str(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	}
	return fmt.Sprint(v)
}

func s2Bool(v any) bool { b, _ := v.(bool); return b }

// s2MoneyFloat converts an Etsy Money object {amount,divisor,currency_code}
// (or a bare number) into a decimal float.
func s2MoneyFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	case float64:
		return t, true
	case map[string]any:
		a, aok := s2MoneyFloat(t["amount"])
		d, dok := s2MoneyFloat(t["divisor"])
		if aok && dok && d != 0 {
			return a / d, true
		}
	}
	return 0, false
}

// s2InventoryPutBody turns a getListingInventory response into a full
// updateListingInventory request body: every product and offering is sent
// back (the PUT replaces the whole inventory), deleted products/offerings and
// read-only ids/labels are dropped, and Money prices become decimal numbers as
// the PUT schema requires. edit may mutate individual offerings.
func s2InventoryPutBody(raw []byte, edit s2InvEdit) (map[string]any, error) {
	src, err := s2DecodeObject(raw)
	if err != nil {
		return nil, fmt.Errorf("parsing inventory: %w", err)
	}
	prods, _ := src["products"].([]any)
	outProds := make([]any, 0, len(prods))
	for _, pi := range prods {
		prod, ok := pi.(map[string]any)
		if !ok || s2Bool(prod["is_deleted"]) {
			continue
		}
		outProd := map[string]any{}
		if sku, ok := prod["sku"]; ok {
			outProd["sku"] = sku
		}
		if pvs, ok := prod["property_values"].([]any); ok {
			outPVs := make([]any, 0, len(pvs))
			for _, pv := range pvs {
				m, ok := pv.(map[string]any)
				if !ok {
					continue
				}
				o := map[string]any{}
				for _, k := range []string{"property_id", "value_ids", "scale_id", "values"} {
					if v, ok := m[k]; ok && v != nil {
						o[k] = v
					}
				}
				outPVs = append(outPVs, o)
			}
			outProd["property_values"] = outPVs
		}
		offs, _ := prod["offerings"].([]any)
		outOffs := make([]any, 0, len(offs))
		for _, oi := range offs {
			off, ok := oi.(map[string]any)
			if !ok || s2Bool(off["is_deleted"]) {
				continue
			}
			out := map[string]any{}
			if p, ok := off["price"]; ok {
				if f, ok := s2MoneyFloat(p); ok {
					out["price"] = f
				}
			}
			for _, k := range []string{"quantity", "is_enabled"} {
				if v, ok := off[k]; ok {
					out[k] = v
				}
			}
			if v, ok := off["readiness_state_id"]; ok && v != nil {
				out["readiness_state_id"] = v
			}
			if edit != nil {
				edit(prod, off, out)
			}
			outOffs = append(outOffs, out)
		}
		if len(outOffs) == 0 {
			continue
		}
		outProd["offerings"] = outOffs
		outProds = append(outProds, outProd)
	}
	body := map[string]any{"products": outProds}
	for _, k := range []string{"price_on_property", "quantity_on_property", "sku_on_property", "readiness_state_on_property"} {
		if v, ok := src[k]; ok && v != nil {
			body[k] = v
		}
	}
	return body, nil
}

type s2QtyDiff struct {
	ListingID         int64  `json:"listing_id"`
	ProductID         string `json:"product_id"`
	SKU               string `json:"sku,omitempty"`
	OfferingID        string `json:"offering_id"`
	OldQuantity       *int   `json:"old_quantity"`
	NewQuantity       int    `json:"new_quantity"`
	OfferingsUnchaged int    `json:"offerings_unchanged"`
}

type s2QtyResult struct {
	Mode    string    `json:"mode"`
	Applied bool      `json:"applied"`
	Diff    s2QtyDiff `json:"diff"`
	Status  int       `json:"http_status,omitempty"`
	Next    string    `json:"next,omitempty"`
}

func newS2InventorySetQtyCmd(flags *rootFlags) *cobra.Command {
	var product string
	var qty, offering int
	cmd := &cobra.Command{
		Use:   "set-qty [listing_id]",
		Short: "Change one offering's quantity via a read-modify-write of the full inventory (preview by default).",
		Long: strings.Trim(`
Etsy only lets you replace a listing's whole inventory. set-qty reads the live
inventory, changes the quantity of ONE offering and shows the old->new diff.
Nothing is written until you pass --yes; then the full inventory is PUT back
with every other offering exactly as read (Money prices become decimals, which
is what the PUT requires). --product takes a product id or a SKU; use
--offering when a product has more than one offering. Requires the
listings_w OAuth scope.
`, "\n"),
		Example: strings.Trim(`
  etsy-pp-cli inventory set-qty 1000000001 --product SKU-LINEN-APRON-NAT --qty 12
  etsy-pp-cli inventory set-qty 1000000001 --product 4000000001 --qty 0 --yes
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if flags.dryRun && len(args) == 0 {
				return writeDryRun(cmd.OutOrStdout(), flags, "inventory set-qty")
			}
			if len(args) != 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("listing_id is required\nUsage: %s <listing_id> --product <id|sku> --qty N", cmd.CommandPath()))
			}
			listingID, err := strconv.ParseInt(strings.TrimSpace(args[0]), 10, 64)
			if err != nil || listingID <= 0 {
				return usageErr(fmt.Errorf("listing_id %q must be a positive integer", args[0]))
			}
			if strings.TrimSpace(product) == "" {
				return usageErr(fmt.Errorf("--product is required (a product id or SKU)"))
			}
			if !cmd.Flags().Changed("qty") || qty < 0 {
				return usageErr(fmt.Errorf("--qty is required and must be 0 or more"))
			}
			apply := flags.yes && !flags.dryRun
			if apply && cliutil.IsAnyHarness() {
				return writeHarnessRefusal(cmd.OutOrStdout(), flags, "inventory set-qty --yes")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// read-before-write / snapshot / refresh reads must see live data, never the 5-minute response cache
			c.NoCache = true
			if flags.dryRun {
				c.DryRun = false
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			path := "/listings/" + strconv.FormatInt(listingID, 10) + "/inventory"
			raw, err := c.Get(ctx, path, nil)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			diff := s2QtyDiff{ListingID: listingID, NewQuantity: qty}
			wantID := strings.TrimSpace(product)
			matched := 0
			var offIDs []string
			body, err := s2InventoryPutBody(raw, func(prod, off, out map[string]any) {
				pid := s2Str(prod["product_id"])
				sku, _ := prod["sku"].(string)
				if pid != wantID && sku != wantID {
					diff.OfferingsUnchaged++
					return
				}
				oid := s2Str(off["offering_id"])
				offIDs = append(offIDs, oid)
				if offering > 0 && oid != strconv.Itoa(offering) {
					diff.OfferingsUnchaged++
					return
				}
				matched++
				diff.ProductID, diff.SKU, diff.OfferingID = pid, sku, oid
				if v := off["quantity"]; v != nil {
					if _, err := strconv.ParseInt(s2Str(v), 10, 64); err == nil {
						q := int(s2Int(v))
						diff.OldQuantity = &q
					}
				}
				out["quantity"] = qty
			})
			if err != nil {
				return apiErr(err)
			}
			if matched == 0 {
				return notFoundErr(fmt.Errorf("no live product/offering matches --product %q on listing %d", wantID, listingID))
			}
			if matched > 1 && offering == 0 {
				return usageErr(fmt.Errorf("product %q has %d offerings (%s); pass --offering <id> to pick one", wantID, matched, strings.Join(offIDs, ", ")))
			}
			res := s2QtyResult{Mode: "preview", Diff: diff}
			if !apply {
				res.Next = "re-run with --yes to PUT the full inventory with this one change"
				return s2PrintJSON(cmd, flags, res)
			}
			_, status, err := c.PutWithParams(ctx, path, nil, body)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			res.Mode, res.Applied, res.Status = "apply", true, status
			return s2PrintJSON(cmd, flags, res)
		},
	}
	cmd.Flags().StringVar(&product, "product", "", "Product id or SKU whose offering to change")
	cmd.Flags().IntVar(&qty, "qty", 0, "New quantity for the offering (0 or more)")
	cmd.Flags().IntVar(&offering, "offering", 0, "Offering id, needed only when the product has several offerings")
	return cmd
}

// s2PrintJSON prints v honoring --select/--compact/--agent.
func s2PrintJSON(cmd *cobra.Command, flags *rootFlags, v any) error {
	return printJSONFiltered(cmd.OutOrStdout(), v, flags)
}
