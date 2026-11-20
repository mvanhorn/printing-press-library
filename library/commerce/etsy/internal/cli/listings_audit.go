// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
	"github.com/spf13/cobra"
	"sort"
)

type shopxAuditFinding struct {
	Rule    string `json:"rule"`
	Detail  string `json:"detail"`
	ImageID int64  `json:"image_id,omitempty"`
	Rank    int64  `json:"-"`
}
type shopxAuditRow struct {
	ListingID int64               `json:"listing_id"`
	Title     string              `json:"title"`
	Findings  []shopxAuditFinding `json:"findings"`
}

func shopxAuditListings(db *store.Store, shop string, rules string) (rows []shopxAuditRow, notes []string) {
	rows = make([]shopxAuditRow, 0)
	notes = make([]string, 0)
	if shop == "" {
		return rows, []string{shopxNoteShopUnset}
	}
	listings, dropped := shopxActiveListings(db, shop)
	if dropped > 0 {
		notes = append(notes, shopxForeignNote(dropped))
	}
	images := shopxRows(db, shopxImageTypes...)
	byListing := make(map[int64]map[int64]map[string]any)
	for _, im := range images {
		lid := shopxLinkedListingID(im)
		id, _ := shopxInt(im["listing_image_id"])
		if byListing[lid] == nil {
			byListing[lid] = make(map[int64]map[string]any)
		}
		byListing[lid][id] = im
	}
	operators := shopxRows(db, shopxOperatorTypes...)
	ops := make(map[int64]bool)
	for _, op := range operators {
		id, _ := shopxInt(op["economic_operator_id"])
		ops[id] = true
	}
	unknownImages, unknownOps := false, false
	for _, l := range listings {
		r := shopxAuditRow{ListingID: shopxListingID(l), Title: shopxStr(l["title"]), Findings: make([]shopxAuditFinding, 0)}
		add := func(rule, detail string) {
			r.Findings = append(r.Findings, shopxAuditFinding{Rule: rule, Detail: detail})
		}
		if rules == "default" || rules == "all" {
			if tags, ok := l["tags"]; ok && len(shopxList(tags)) < 13 {
				add("tags_lt_13", fmt.Sprintf("%d of 13 tags used", len(shopxList(tags))))
			}
			if m, ok := l["materials"]; ok && len(shopxList(m)) == 0 {
				add("no_materials", "no materials provided")
			}
			_, known := l["images"]
			if known || len(images) > 0 {
				ids := make(map[int64]bool)
				for id := range byListing[r.ListingID] {
					ids[id] = true
				}
				for _, im := range shopxList(l["images"]) {
					id, ok := shopxInt(im)
					if m := shopxMap(im); m != nil {
						id, ok = shopxInt(m["listing_image_id"])
					}
					if ok {
						ids[id] = true
					}
				}
				if len(ids) < 5 {
					add("photos_lt_5", fmt.Sprintf("%d of 5 photos provided", len(ids)))
				}
				sorted := make([]int64, 0, len(byListing[r.ListingID]))
				for id := range byListing[r.ListingID] {
					sorted = append(sorted, id)
				}
				sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
				for _, id := range sorted {
					im := byListing[r.ListingID][id]
					if shopxStr(im["alt_text"]) == "" {
						rank, _ := shopxInt(im["rank"])
						r.Findings = append(r.Findings, shopxAuditFinding{Rule: "image_missing_alt", Detail: "image missing alt text", ImageID: id, Rank: rank})
					}
				}
			} else {
				unknownImages = true
			}
			if _, ok := l["title"]; ok && len([]rune(r.Title)) < 40 {
				add("title_lt_40", "title has fewer than 40 characters")
			}
		}
		if rules == "gpsr" || rules == "all" {
			mid, _ := shopxInt(l["gpsr_manufacturer_id"])
			rid, _ := shopxInt(l["gpsr_responsible_person_id"])
			mt, rt := shopxStr(l["gpsr_manufacturer_type"]), shopxStr(l["gpsr_responsible_person_type"])
			// etsy-ext: Only seller and economic_operator are valid GPSR field types.
			if mt != "" && mt != "seller" && mt != "economic_operator" {
				add("gpsr_manufacturer_type_invalid", "invalid manufacturer type")
			}
			if rt != "" && rt != "seller" && rt != "economic_operator" {
				add("gpsr_responsible_person_type_invalid", "invalid responsible person type")
			}
			if mt == "" || (mt == "economic_operator" && mid == 0) {
				add("gpsr_manufacturer_missing", "manufacturer id or type missing")
			}
			if rt == "" || (rt == "economic_operator" && rid == 0) {
				add("gpsr_responsible_person_missing", "responsible person id or type missing")
			}
			if len(operators) == 0 && (mt == "economic_operator" || rt == "economic_operator") {
				unknownOps = true
			} else {
				if mt == "economic_operator" && mid != 0 && !ops[mid] {
					add("gpsr_operator_not_synced", "manufacturer operator not synced")
				}
				if rt == "economic_operator" && rid != 0 && !ops[rid] {
					add("gpsr_operator_not_synced", "responsible person operator not synced")
				}
			}
		}
		if len(r.Findings) > 0 {
			rows = append(rows, r)
		}
	}
	if unknownImages {
		notes = append(notes, "listing images not synced; photos_lt_5 and image_missing_alt skipped")
	}
	if unknownOps {
		notes = append(notes, "economic_operators not synced; gpsr_operator_not_synced skipped")
	}
	return
}

func shopxAuditCounts(rows []shopxAuditRow) map[string]int {
	counts := make(map[string]int)
	for _, r := range rows {
		for _, f := range r.Findings {
			counts[f.Rule]++
		}
	}
	return counts
}

type shopxAuditPlanItem struct {
	ListingID int64  `json:"listing_id"`
	Field     string `json:"field"`
	Op        string `json:"op"`
	Value     string `json:"value"`
	ImageID   int64  `json:"image_id,omitempty"`
	Reason    string `json:"reason"`
}

// etsy-ext: force is explicit; the default preserves existing outputs.
func shopxAuditWritePlan(path string, rows []shopxAuditRow, force ...bool) (int, error) {
	items := make([]shopxAuditPlanItem, 0)
	for _, r := range rows {
		for _, f := range r.Findings {
			item := shopxAuditPlanItem{ListingID: r.ListingID, Reason: f.Detail}
			switch f.Rule {
			case "tags_lt_13":
				item.Field = "tags"
				item.Op = "add"
			case "no_materials":
				item.Field = "materials"
				item.Op = "add"
			case "title_lt_40":
				item.Field = "title"
				item.Op = "set"
			case "image_missing_alt":
				item.Field = "alt_text"
				item.Op = "set"
				item.ImageID = f.ImageID
				title := []rune(r.Title)
				if len(title) > 120 {
					title = title[:120]
				}
				rank := f.Rank
				if rank <= 0 {
					rank = 1
				}
				item.Value = fmt.Sprintf("%s - photo %d", string(title), rank)
				if len(title) == 0 {
					item.Value = fmt.Sprintf("Photo %d", rank)
				}
			default:
				continue
			}
			items = append(items, item)
		}
	}
	plan := struct {
		Version     int                  `json:"version"`
		GeneratedBy string               `json:"generated_by"`
		Items       []shopxAuditPlanItem `json:"items"`
	}{1, "listings audit", items}
	// etsy-ext: refuse overwrite and symlink traversal by default.
	file, err := etsyOpenOutput(path, len(force) > 0 && force[0])
	if err != nil {
		return 0, err
	}
	if err = file.Chmod(0600); err == nil {
		err = json.NewEncoder(file).Encode(plan)
	}
	closeErr := file.Close()
	if err != nil {
		return 0, err
	}
	return len(items), closeErr
}

func newNovelListingsAuditCmd(flags *rootFlags) *cobra.Command {
	var rules, planOut, dbPath string
	var force bool // etsy-ext: explicit output overwrite consent.
	cmd := &cobra.Command{
		Use:     "audit",
		Short:   "Find every listing with missing tags, materials, alt text or photos",
		Example: "  etsy-pp-cli listings audit --rules gpsr --agent",
		Long:    "Use this command to find hygiene and GPSR gaps across listings. Do NOT use it for expiry/renewal timing; use 'listings expiring' instead.",
		// etsy-ext: declare filesystem sinks for the MCP walker.
		Annotations: map[string]string{"mcp:write-flags": "plan-out", "mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "listings audit")
			}
			if rules != "default" && rules != "gpsr" && rules != "all" {
				return usageErr(fmt.Errorf("invalid --rules %q: use default, gpsr or all", rules))
			}
			db, err := shopxOpenDB(cmd, dbPath)
			if err != nil {
				return err
			}
			if db != nil {
				defer db.Close()
			}
			familyNotes := shopxCommandNotes(db, "listings")
			rows, notes := shopxAuditListings(db, shopxShopID(cmd), rules)
			notes = append(familyNotes, notes...)
			if planOut != "" {
				n, err := shopxAuditWritePlan(planOut, rows, force)
				if err != nil {
					return fmt.Errorf("write audit plan: %w", err)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "plan written: %s (%d items)\n", planOut, n)
			}
			return shopxEmit(cmd, flags, rows, notes)
		},
	}
	cmd.Flags().StringVar(&rules, "rules", "default", "Audit rules: default, gpsr or all")
	// etsy-ext: output overwrite is opt-in.
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite --plan-out if it already exists")
	cmd.Flags().StringVar(&planOut, "plan-out", "", "Write a bulk-edit plan to FILE")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}
