// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/pagliacci/internal/client"
)

// HalfHalfResult is the structured output of `menu half-half`.
type HalfHalfResult struct {
	StoreID     int            `json:"store_id"`
	SizeID      int            `json:"size_id"`
	SizeName    string         `json:"size_name"`
	LeftName    string         `json:"left_name"`
	LeftID      int            `json:"left_menu_item_id"`
	RightName   string         `json:"right_name"`
	RightID     int            `json:"right_menu_item_id"`
	UnitPrice   float64        `json:"unit_price"`
	ProductBody map[string]any `json:"product_body"`
	Note        string         `json:"note,omitempty"`
}

func newMenuHalfHalfCmd(flags *rootFlags) *cobra.Command {
	var leftName string
	var rightName string
	var size string
	var storeIDFlag int
	var validate bool

	cmd := &cobra.Command{
		Use:   "half-half",
		Short: "Build a half-and-half pizza cart entry (left + right + size; emits the API body)",
		Long: `Build a half-and-half pizza by name. Looks up each side in the menu cache,
assembles the API's TwoSides product shape, and emits a ready-to-send cart
entry. Pass --validate to additionally POST to /ProductPrice and confirm the
exact unit price (the body shape required by /ProductPrice for half-and-half
pies is partially undocumented and may need manual adjustment per store).

Common case for households with picky kids: --left pepperoni --right cheese.`,
		Example: `  pagliacci-pp-cli menu half-half --left pepperoni --right cheese --size large
  pagliacci-pp-cli menu half-half --left "original cheese" --right "deluxe" --size medium --json
  pagliacci-pp-cli menu half-half --left pepperoni --right cheese --validate
  pagliacci-pp-cli menu half-half --left pepperoni --right cheese --dry-run`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if leftName == "" || rightName == "" {
				return usageErr(fmt.Errorf("--left and --right are required (e.g. --left pepperoni --right cheese)"))
			}
			storeID := storeIDFlag
			if storeID == 0 {
				storeID = 490 // sensible default; user can override with --store
			}

			// Dry-run: emit a representative shape without contacting the API.
			if flags.dryRun {
				result := HalfHalfResult{
					StoreID:   storeID,
					SizeID:    sizeIDForName(size),
					SizeName:  size,
					LeftName:  leftName,
					LeftID:    0,
					RightName: rightName,
					RightID:   0,
					ProductBody: map[string]any{
						"Cat":  1,
						"Size": sizeIDForName(size),
						"Qty":  1,
						"Side1": map[string]any{
							"MenuItem":  "<resolved from MenuCache by name: " + leftName + ">",
							"Modifiers": []any{},
						},
						"Side2": map[string]any{
							"MenuItem":  "<resolved from MenuCache by name: " + rightName + ">",
							"Modifiers": []any{},
						},
					},
					Note: "dry-run: not calling /MenuCache or /ProductPrice. Run without --dry-run to resolve the actual MenuItem IDs.",
				}
				out, _ := json.Marshal(result)
				return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			menuRaw, err := c.Get(fmt.Sprintf("/MenuCache/%d", storeID), nil)
			if err != nil {
				return classifyAPIError(err)
			}

			leftItem, err := findPizzaItemByName(menuRaw, leftName)
			if err != nil {
				return usageErr(fmt.Errorf("left side: %w", err))
			}
			rightItem, err := findPizzaItemByName(menuRaw, rightName)
			if err != nil {
				return usageErr(fmt.Errorf("right side: %w", err))
			}

			sizeID, sizeName, err := resolveSizeID(menuRaw, size)
			if err != nil {
				return usageErr(err)
			}

			body := map[string]any{
				"Cat":  1, // Pizza
				"Size": sizeID,
				"Qty":  1,
				"Side1": map[string]any{
					"MenuItem":  leftItem.ID,
					"Modifiers": []any{},
				},
				"Side2": map[string]any{
					"MenuItem":  rightItem.ID,
					"Modifiers": []any{},
				},
			}

			result := HalfHalfResult{
				StoreID:     storeID,
				SizeID:      sizeID,
				SizeName:    sizeName,
				LeftName:    leftItem.Name,
				LeftID:      leftItem.ID,
				RightName:   rightItem.Name,
				RightID:     rightItem.ID,
				ProductBody: body,
				Note:        "Cart-entry body assembled from MenuCache. Pass --validate to call /ProductPrice for live unit price (best-effort; body shape may need adjustment per store).",
			}

			if validate {
				priceRaw, _, perr := c.Post("/ProductPrice", body)
				if perr != nil {
					result.Note = fmt.Sprintf("/ProductPrice 400-class on this body: %v. The cart-entry body above is still useful as a starting point for manual adjustment.", perr)
				} else {
					result.UnitPrice = extractUnitPrice(priceRaw)
					result.Note = ""
				}
			}

			out, err := json.Marshal(result)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&leftName, "left", "", "Left-side pizza name (e.g. pepperoni). Match is case-insensitive substring against the menu cache.")
	cmd.Flags().StringVar(&rightName, "right", "", "Right-side pizza name (e.g. cheese)")
	cmd.Flags().StringVar(&size, "size", "large", "Pizza size: small, medium, large, or a numeric size ID from MenuCache")
	cmd.Flags().IntVar(&storeIDFlag, "store", 0, "Store ID to use for the menu lookup (default 490)")
	cmd.Flags().BoolVar(&validate, "validate", false, "Also call /ProductPrice to confirm the live unit price (best-effort)")
	return cmd
}

// sizeIDForName returns a default size ID for common names, used during
// --dry-run when MenuCache isn't fetched.
func sizeIDForName(s string) int {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "small":
		return 1
	case "medium":
		return 2
	case "large", "":
		return 3
	}
	return 3
}

type pizzaItem struct {
	ID   int
	Name string
}

// findPizzaItemByName scans MenuCache (which returns categories whose Members
// reference menu items elsewhere in the payload). Pagliacci's MenuSlices
// endpoint is the cleaner index for slice/whole-pizza names with IDs.
// We use /MenuSlices via the captured shape: ID + Name fields.
func findPizzaItemByName(menuRaw json.RawMessage, name string) (pizzaItem, error) {
	target := strings.ToLower(strings.TrimSpace(name))
	if target == "" {
		return pizzaItem{}, fmt.Errorf("empty name")
	}

	// MenuCache is an array of category records. The category with Name=Pizza
	// (or CategoryCode=PIZZA) holds the list of pizza items via its Members
	// field; some payloads inline items in a Products array. Walk both shapes.
	var cats []map[string]any
	if err := json.Unmarshal(menuRaw, &cats); err != nil {
		return pizzaItem{}, fmt.Errorf("parsing MenuCache: %w", err)
	}

	type candidate struct {
		id       int
		name     string
		matched  bool
		exactHit bool
	}
	best := candidate{}
	pickIfBetter := func(id int, n string) {
		nl := strings.ToLower(n)
		if nl == target {
			best = candidate{id: id, name: n, matched: true, exactHit: true}
			return
		}
		if best.exactHit {
			return
		}
		if strings.Contains(nl, target) && !best.matched {
			best = candidate{id: id, name: n, matched: true}
		}
	}

	for _, c := range cats {
		// Walk inline Products on the category.
		if prods, ok := c["Products"].([]any); ok {
			for _, p := range prods {
				pm, isMap := p.(map[string]any)
				if !isMap {
					continue
				}
				id := extractInt(pm, "ID", "id")
				n, _ := pm["Name"].(string)
				if id != 0 && n != "" {
					pickIfBetter(id, n)
				}
			}
		}
		// Walk inline Items on the category.
		if items, ok := c["Items"].([]any); ok {
			for _, p := range items {
				pm, isMap := p.(map[string]any)
				if !isMap {
					continue
				}
				id := extractInt(pm, "ID", "id")
				n, _ := pm["Name"].(string)
				if id != 0 && n != "" {
					pickIfBetter(id, n)
				}
			}
		}
	}

	if best.matched {
		return pizzaItem{ID: best.id, Name: best.name}, nil
	}
	return pizzaItem{}, fmt.Errorf("no pizza item matched %q in MenuCache", name)
}

// resolveSizeID maps a size name (small/medium/large) or numeric ID to the
// MenuCache Sizes table. Returns the size ID and a friendly name.
func resolveSizeID(menuRaw json.RawMessage, size string) (int, string, error) {
	if id, err := strconv.Atoi(size); err == nil {
		return id, fmt.Sprintf("size %d", id), nil
	}
	target := strings.ToLower(strings.TrimSpace(size))
	if target == "" {
		target = "large"
	}

	var cats []map[string]any
	if err := json.Unmarshal(menuRaw, &cats); err != nil {
		return 0, "", fmt.Errorf("parsing MenuCache: %w", err)
	}

	// Aggregate sizes from the Pizza category. Pagliacci ships sizes as
	// inch labels (11", 14", 17"); we map "small"→smallest, "large"→largest.
	type sz struct {
		id   int
		name string
	}
	var sizes []sz
	for _, c := range cats {
		code, _ := c["CategoryCode"].(string)
		if !strings.EqualFold(code, "PIZZA") {
			continue
		}
		ss, ok := c["Sizes"].([]any)
		if !ok {
			continue
		}
		for _, s := range ss {
			sm, isMap := s.(map[string]any)
			if !isMap {
				continue
			}
			id := extractInt(sm, "ID", "id")
			n, _ := sm["Name"].(string)
			if id != 0 {
				sizes = append(sizes, sz{id: id, name: n})
			}
		}
	}

	if len(sizes) == 0 {
		// Fall back to the conventional Pagliacci size IDs (1=11", 2=14", 3=17")
		switch target {
		case "small":
			return 1, "small (11\")", nil
		case "medium":
			return 2, "medium (14\")", nil
		case "large":
			return 3, "large (17\")", nil
		default:
			return 3, "large (17\")", nil
		}
	}

	switch target {
	case "small":
		return sizes[0].id, sizes[0].name, nil
	case "large":
		return sizes[len(sizes)-1].id, sizes[len(sizes)-1].name, nil
	case "medium":
		if len(sizes) >= 2 {
			return sizes[len(sizes)/2].id, sizes[len(sizes)/2].name, nil
		}
		return sizes[0].id, sizes[0].name, nil
	default:
		// Try to match the target against the size name (e.g. "17", "17\"").
		for _, s := range sizes {
			if strings.Contains(strings.ToLower(s.name), target) {
				return s.id, s.name, nil
			}
		}
		return sizes[len(sizes)-1].id, sizes[len(sizes)-1].name, nil
	}
}

func extractUnitPrice(raw json.RawMessage) float64 {
	var o map[string]any
	if json.Unmarshal(raw, &o) != nil {
		return 0
	}
	for _, k := range []string{"Price", "Total", "UnitPrice", "Subtotal"} {
		if v, ok := o[k]; ok {
			switch x := v.(type) {
			case float64:
				return x
			case string:
				if n, err := strconv.ParseFloat(x, 64); err == nil {
					return n
				}
			}
		}
	}
	return 0
}

// pickFirstStoreID returns the ID of the first Pagliacci store (used as a
// default when the user doesn't specify --store).
func pickFirstStoreID(c *client.Client) (int, error) {
	stores, err := c.Get("/Store", nil)
	if err != nil {
		return 0, classifyAPIError(err)
	}
	var arr []map[string]any
	if json.Unmarshal(stores, &arr) != nil || len(arr) == 0 {
		return 0, apiErr(fmt.Errorf("no stores returned by /Store"))
	}
	id := extractInt(arr[0], "ID", "id")
	if id == 0 {
		return 0, apiErr(fmt.Errorf("first store has no ID"))
	}
	return id, nil
}
