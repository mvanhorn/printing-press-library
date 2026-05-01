// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

func newSlicesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "slices",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Slice availability across stores (today's perishable rotation)",
	}
	cmd.AddCommand(newSlicesTodayCmd(flags))
	return cmd
}

// SliceRow is one row in the joined "slices today" output: a slice name
// available right now at a specific store, with the live price.
type SliceRow struct {
	StoreID   int     `json:"store_id"`
	StoreName string  `json:"store_name"`
	StoreCity string  `json:"store_city,omitempty"`
	StoreAddr string  `json:"store_address,omitempty"`
	SliceID   int     `json:"slice_id"`
	SliceName string  `json:"slice_name"`
	Price     float64 `json:"price,omitempty"`
}

// joinSlicesAcrossStores walks the per-store Slices array on /Store and joins
// each entry with the system-wide /MenuSlices price. storeFilter, if non-zero,
// keeps only that store. limit caps the number of returned rows (0 = no cap).
func joinSlicesAcrossStores(menuSlices, stores json.RawMessage, storeFilter int, limit int) ([]SliceRow, error) {
	// Build menu price lookup keyed by slice ID. Match against either the
	// MenuID field on the slice (which links back to the slice product) or
	// the ID itself — the per-store Slices array uses MenuID-style values.
	type menuSlice struct {
		ID     int     `json:"ID"`
		MenuID int     `json:"MenuID"`
		Name   string  `json:"Name"`
		Price  float64 `json:"Price"`
	}
	var ms []menuSlice
	if len(menuSlices) > 0 {
		_ = json.Unmarshal(menuSlices, &ms)
	}
	priceByID := map[int]float64{}
	nameByID := map[int]string{}
	for _, s := range ms {
		// The per-store Slices array references the MenuID, so map both
		// the slice's own ID and the linked MenuID to the price.
		if s.MenuID != 0 {
			priceByID[s.MenuID] = s.Price
			nameByID[s.MenuID] = s.Name
		}
		if s.ID != 0 {
			priceByID[s.ID] = s.Price
			if _, ok := nameByID[s.ID]; !ok {
				nameByID[s.ID] = s.Name
			}
		}
	}

	type sliceEntry struct {
		ID   int    `json:"ID"`
		Name string `json:"Name"`
	}
	type storeShape struct {
		ID      int          `json:"ID"`
		Name    string       `json:"Name"`
		Address string       `json:"Address"`
		City    string       `json:"City"`
		Slices  []sliceEntry `json:"Slices"`
	}
	var ss []storeShape
	if err := json.Unmarshal(stores, &ss); err != nil {
		return nil, fmt.Errorf("parsing stores: %w", err)
	}

	var rows []SliceRow
	for _, st := range ss {
		if storeFilter != 0 && st.ID != storeFilter {
			continue
		}
		for _, sl := range st.Slices {
			row := SliceRow{
				StoreID:   st.ID,
				StoreName: st.Name,
				StoreCity: st.City,
				StoreAddr: st.Address,
				SliceID:   sl.ID,
				SliceName: sl.Name,
				Price:     priceByID[sl.ID],
			}
			if row.SliceName == "" {
				row.SliceName = nameByID[sl.ID]
			}
			rows = append(rows, row)
			if limit > 0 && len(rows) >= limit {
				return rows, nil
			}
		}
	}
	return rows, nil
}

func newSlicesTodayCmd(flags *rootFlags) *cobra.Command {
	var storeFilter string
	var limit int

	cmd := &cobra.Command{
		Use:   "today",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Show slices available right now at every Pagliacci store, joined with store name and address",
		Example: `  pagliacci-pizza-pp-cli slices today
  pagliacci-pizza-pp-cli slices today --store 492
  pagliacci-pizza-pp-cli slices today --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// /MenuSlices is auth-free; carries the live price list.
			menu, err := c.Get("/MenuSlices", nil)
			if err != nil {
				return classifyAPIError(err)
			}

			// Stores: prefer local cache, fall back to live /Store. Either
			// path produces an array of store objects with per-store
			// Slices populated for today.
			var stores json.RawMessage
			if db, dberr := openStoreForRead("pagliacci-pizza-pp-cli"); dberr == nil && db != nil {
				if items, lerr := db.List("store", 0); lerr == nil && len(items) > 0 {
					if marshaled, merr := json.Marshal(items); merr == nil {
						stores = marshaled
					}
				}
				db.Close()
			}
			if len(stores) == 0 {
				stores, err = c.Get("/Store", nil)
				if err != nil {
					return classifyAPIError(err)
				}
			}

			storeID := 0
			if storeFilter != "" {
				n, perr := strconv.Atoi(storeFilter)
				if perr != nil {
					return usageErr(fmt.Errorf("--store must be a numeric store ID, got %q", storeFilter))
				}
				storeID = n
			}

			rows, err := joinSlicesAcrossStores(menu, stores, storeID, limit)
			if err != nil {
				return apiErr(err)
			}

			if len(rows) == 0 {
				if isTerminal(cmd.OutOrStdout()) && !flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), "No slices available right now.")
					return nil
				}
				out, _ := json.Marshal([]SliceRow{})
				return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
			}

			out, err := json.Marshal(rows)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&storeFilter, "store", "", "Filter to a single store ID (e.g. 492)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows to return (0 = no limit)")
	return cmd
}
