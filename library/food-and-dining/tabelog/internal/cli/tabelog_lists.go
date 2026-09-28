// pp:data-source local
package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tabelog/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tabelog/internal/domain"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tabelog/internal/notebook"

	"github.com/spf13/cobra"
)

type savedRestaurant struct {
	domain.Restaurant
	Note          string `json:"note"`
	AddedAt       string `json:"added_at"`
	NoteUpdatedAt string `json:"note_updated_at"`
	AgeSeconds    *int64 `json:"age_seconds"`
	Stale         bool   `json:"stale"`
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		candidate := newTabelogListsCmd(flags)
		var parent *cobra.Command
		for _, existing := range root.Commands() {
			if existing.Name() == "lists" {
				parent = existing
				break
			}
		}
		if parent == nil {
			root.AddCommand(candidate)
			return
		}
		parent.Short, parent.Long, parent.Example = candidate.Short, candidate.Long, candidate.Example
		// Lists is an overview group; only its leaves execute workflows.
		parent.Run, parent.RunE = nil, nil
		if parent.Annotations == nil {
			parent.Annotations = map[string]string{}
		}
		parent.Annotations["pp:parent-group"] = "true"
		for _, child := range candidate.Commands() {
			for _, existing := range parent.Commands() {
				if existing.Name() == child.Name() {
					parent.RemoveCommand(existing)
				}
			}
			parent.AddCommand(child)
		}
	})
}

func newTabelogListsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "lists", Short: "Keep notes and compare saved trip candidates", Long: "Save fetched restaurants in named trip lists. Add requires a candidate fetched with find or show first. Show, compare, alternatives and audit use saved facts without network requests; refresh explicitly retrieves current facts.", Example: "  tabelog-pp-cli lists show --agent\n  tabelog-pp-cli lists compare tokyo-bars --agent", Annotations: map[string]string{"pp:parent-group": "true"}}
	cmd.AddCommand(newTabelogListAddCmd(flags), newTabelogListShowCmd(flags), newTabelogListNoteCmd(flags), newTabelogListRemoveCmd(flags), newTabelogListCompareCmd(flags), newTabelogListAlternativesCmd(flags), newTabelogListAuditCmd(flags), newTabelogListRefreshCmd(flags))
	return cmd
}

func listReadAnnotations() map[string]string  { return map[string]string{"mcp:read-only": "true"} }
func listWriteAnnotations() map[string]string { return map[string]string{"mcp:local-write": "true"} }

func listArgCount(flags *rootFlags, args []string, minimum, maximum int) error {
	if maximum >= 0 && len(args) > maximum {
		return fmt.Errorf("expected at most %d positional arguments", maximum)
	}
	if !flags.dryRun && len(args) < minimum {
		return fmt.Errorf("expected at least %d positional arguments", minimum)
	}
	return nil
}

func listName(args []string) string {
	if len(args) > 0 {
		return strings.TrimSpace(args[0])
	}
	return ""
}
func listIDs(args []string) []string {
	if len(args) > 1 {
		return args[1:]
	}
	return nil
}

func validateListArguments(flags *rootFlags, args []string, note *string) error {
	if flags.dataSource == "live" {
		return fmt.Errorf("saved-list commands have no live equivalent; use lists refresh for current source facts")
	}
	if len(args) > 0 {
		if err := notebook.ValidateName(args[0]); err != nil {
			return err
		}
	}
	if len(args) > 1 {
		for _, id := range args[1:] {
			if err := notebook.ValidateID(id); err != nil {
				return err
			}
		}
	}
	if note != nil {
		return notebook.ValidateNote(*note)
	}
	return nil
}

func listMeta(name, operation string, returned, scanned int) domain.Meta {
	return domain.Meta{Source: "local", Returned: returned, Scanned: scanned, Coverage: "saved_list", Criteria: map[string]any{"list": name, "operation": operation}}
}

func listDryRun(cmd *cobra.Command, flags *rootFlags, operation, name string, ids []string, extra map[string]any) error {
	plan := map[string]any{"operation": operation, "list": name, "ids": ids, "planned": true}
	for k, v := range extra {
		plan[k] = v
	}
	meta := listMeta(name, operation, 0, 0)
	meta.Coverage = "dry_run"
	meta.Note = "No storage or network access; cached-ID and membership checks remain planned."
	return tripPrint(cmd, flags, []map[string]any{plan}, meta)
}

func decodeSaved(entries []notebook.Entry, maxAge time.Duration) ([]savedRestaurant, error) {
	items := make([]savedRestaurant, 0, len(entries))
	for _, entry := range entries {
		var r domain.Restaurant
		if err := json.Unmarshal(entry.Snapshot, &r); err != nil {
			return nil, fmt.Errorf("saved restaurant %s is invalid: %w", entry.ID, err)
		}
		if r.ID != entry.ID || r.Name == "" || r.URL == "" {
			return nil, fmt.Errorf("saved restaurant %s has inconsistent identity", entry.ID)
		}
		items = append(items, savedView(entry, r, maxAge))
	}
	return items, nil
}

func savedView(entry notebook.Entry, r domain.Restaurant, maxAge time.Duration) savedRestaurant {
	item := savedRestaurant{Restaurant: r, Note: entry.Note, AddedAt: entry.AddedAt, NoteUpdatedAt: entry.UpdatedAt}
	if !r.FetchedAt.IsZero() {
		age := int64(time.Since(r.FetchedAt).Seconds())
		if age < 0 {
			age = 0
		}
		item.AgeSeconds = &age
		item.Stale = maxAge > 0 && time.Since(r.FetchedAt) > maxAge
	}
	return item
}

func listHints(cmd *cobra.Command, items []savedRestaurant, flags *rootFlags) {
	if flags.quiet {
		return
	}
	if len(items) == 0 {
		fmt.Fprintln(cmd.ErrOrStderr(), "No candidates saved in this list; use find/show, then lists add.")
		return
	}
	stale := 0
	for _, item := range items {
		if item.Stale {
			stale++
		}
	}
	if stale > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "%d saved snapshots exceed the age threshold; use lists audit or lists refresh for selected candidates.\n", stale)
	}
}

func newTabelogListAddCmd(flags *rootFlags) *cobra.Command {
	var note string
	cmd := &cobra.Command{Use: "add <list> <id>", Short: "Save a fetched restaurant and optional personal note", Long: "Save an already fetched restaurant in a named trip list. Re-adding preserves its position and note unless --note is supplied. Use lists refresh to update source facts.", Example: "  tabelog-pp-cli lists add tokyo-bars 13005012 --note 'Ginza bar option' --agent", Annotations: listWriteAnnotations()}
	cmd.Flags().StringVar(&note, "note", "", "Personal note; preserved on repeat add unless this flag is supplied")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := listArgCount(flags, args, 2, 2); err != nil {
			return err
		}
		if err := validateListArguments(flags, args, &note); err != nil {
			return err
		}
		name := listName(args)
		set := cmd.Flags().Changed("note")
		if flags.dryRun {
			return listDryRun(cmd, flags, "add", name, listIDs(args), map[string]any{"note": note, "note_supplied": set})
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		db, nb, err := tripOpen(ctx, flags)
		if err != nil {
			return err
		}
		defer db.Close()
		if err := nb.Add(ctx, name, args[1], note, set); err != nil {
			return err
		}
		entries, err := nb.Entries(ctx, name, listIDs(args))
		if err != nil {
			return err
		}
		items, err := decodeSaved(entries, flags.maxAge)
		if err != nil {
			return err
		}
		return tripPrint(cmd, flags, items, listMeta(name, "add", len(items), len(items)))
	}
	return cmd
}

func newTabelogListShowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "show [list]", Short: "Show saved candidates, or list the notebooks", Example: "  tabelog-pp-cli lists show tokyo-bars --agent", Annotations: listReadAnnotations()}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := listArgCount(flags, args, 0, 1); err != nil {
			return err
		}
		if err := validateListArguments(flags, args, nil); err != nil {
			return err
		}
		name := ""
		if len(args) > 0 {
			name = strings.TrimSpace(args[0])
		}
		if flags.dryRun {
			return listDryRun(cmd, flags, "show", name, nil, nil)
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		db, nb, err := tripOpen(ctx, flags)
		if err != nil {
			return err
		}
		defer db.Close()
		if name == "" {
			lists, err := nb.Lists(ctx)
			if err != nil {
				return err
			}
			meta := listMeta("", "show", len(lists), len(lists))
			meta.Coverage = "saved_notebooks"
			return tripPrint(cmd, flags, lists, meta)
		}
		entries, err := nb.Entries(ctx, name, nil)
		if err != nil {
			return err
		}
		items, err := decodeSaved(entries, flags.maxAge)
		if err != nil {
			return err
		}
		listHints(cmd, items, flags)
		return tripPrint(cmd, flags, items, listMeta(name, "show", len(items), len(items)))
	}
	return cmd
}

func newTabelogListNoteCmd(flags *rootFlags) *cobra.Command {
	var note string
	cmd := &cobra.Command{Use: "note <list> <id>", Short: "Set a personal note on a saved candidate", Example: "  tabelog-pp-cli lists note tokyo-bars 13005012 --note 'Try after dinner' --agent", Annotations: listWriteAnnotations()}
	cmd.Flags().StringVar(&note, "note", "", "Personal note; an explicitly empty note clears it")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := listArgCount(flags, args, 2, 2); err != nil {
			return err
		}
		if !flags.dryRun && !cmd.Flags().Changed("note") {
			return fmt.Errorf("--note is required")
		}
		if err := validateListArguments(flags, args, &note); err != nil {
			return err
		}
		name := listName(args)
		if flags.dryRun {
			return listDryRun(cmd, flags, "note", name, listIDs(args), map[string]any{"note": note})
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		db, nb, err := tripOpen(ctx, flags)
		if err != nil {
			return err
		}
		defer db.Close()
		if err := nb.Note(ctx, name, args[1], note); err != nil {
			return err
		}
		entries, err := nb.Entries(ctx, name, listIDs(args))
		if err != nil {
			return err
		}
		items, err := decodeSaved(entries, flags.maxAge)
		if err != nil {
			return err
		}
		return tripPrint(cmd, flags, items, listMeta(name, "note", len(items), len(items)))
	}
	return cmd
}

func newTabelogListRemoveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "remove <list> <id>", Short: "Remove membership while retaining fetched source facts", Example: "  tabelog-pp-cli lists remove tokyo-bars 13005012 --agent", Annotations: listWriteAnnotations()}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := listArgCount(flags, args, 2, 2); err != nil {
			return err
		}
		if err := validateListArguments(flags, args, nil); err != nil {
			return err
		}
		name := listName(args)
		if flags.dryRun {
			return listDryRun(cmd, flags, "remove", name, listIDs(args), nil)
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		db, nb, err := tripOpen(ctx, flags)
		if err != nil {
			return err
		}
		defer db.Close()
		if err := nb.Remove(ctx, name, args[1]); err != nil {
			return err
		}
		return tripPrint(cmd, flags, []map[string]any{{"id": args[1], "removed": true}}, listMeta(name, "remove", 1, 1))
	}
	return cmd
}

func newTabelogListCompareCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "compare <list> [id...]", Short: "Compare saved facts and notes without fetching", Long: "Compare known source facts, personal notes and snapshot ages in list order. Missing details remain not fetched; source-unknown facts remain unknown. Use lists alternatives to find a backup for one candidate.", Example: "  tabelog-pp-cli lists compare tokyo-bars 13005012 --agent", Annotations: listReadAnnotations()}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := listArgCount(flags, args, 1, -1); err != nil {
			return err
		}
		if err := validateListArguments(flags, args, nil); err != nil {
			return err
		}
		name := listName(args)
		if flags.dryRun {
			return listDryRun(cmd, flags, "compare", name, listIDs(args), nil)
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		db, nb, err := tripOpen(ctx, flags)
		if err != nil {
			return err
		}
		defer db.Close()
		entries, err := nb.Entries(ctx, name, listIDs(args))
		if err != nil {
			return err
		}
		items, err := decodeSaved(entries, flags.maxAge)
		if err != nil {
			return err
		}
		listHints(cmd, items, flags)
		return tripPrint(cmd, flags, items, listMeta(name, "compare", len(items), len(items)))
	}
	return cmd
}

func savedFieldState(r domain.Restaurant, field string) string {
	// Older snapshots may have marked the source's dash placeholder as known.
	if field == "categories" && len(normalizedCategories(r.Categories)) == 0 {
		return "source_unknown"
	}
	if state, ok := r.Evidence[field]; ok {
		if state == "known" || state == "source_unknown" || state == "detail_not_fetched" {
			return state
		}
		return "source_unknown"
	}
	known := false
	switch field {
	case "id":
		known = r.ID != ""
	case "name":
		known = r.Name != ""
	case "url":
		known = r.URL != ""
	case "rating":
		known = r.Rating != nil
	case "review_count":
		known = r.ReviewCount != nil
	case "categories":
		known = len(r.Categories) > 0
	case "area":
		known = r.Area.Verified && r.Area.Prefecture != "" && r.Area.Area1 != "" && r.Area.Area2 != ""
	case "nearest_station":
		known = r.NearestStation != ""
	case "nearest_station_distance_m":
		known = r.NearestStationDistanceM != nil
	case "lunch_budget":
		known = r.LunchBudget.MinJPY != nil || r.LunchBudget.MaxJPY != nil
	case "dinner_budget":
		known = r.DinnerBudget.MinJPY != nil || r.DinnerBudget.MaxJPY != nil
	case "review_lunch_budget":
		known = r.ReviewLunchBudget != nil && (r.ReviewLunchBudget.MinJPY != nil || r.ReviewLunchBudget.MaxJPY != nil)
	case "review_dinner_budget":
		known = r.ReviewDinnerBudget != nil && (r.ReviewDinnerBudget.MinJPY != nil || r.ReviewDinnerBudget.MaxJPY != nil)
	case "hours":
		known = r.Hours != nil && strings.TrimSpace(*r.Hours) != ""
	case "payment":
		known = r.Payment != nil && strings.TrimSpace(*r.Payment) != ""
	case "reservation":
		known = r.Reservation != nil && strings.TrimSpace(*r.Reservation) != ""
	case "address":
		known = r.Address != nil && strings.TrimSpace(*r.Address) != ""
	case "transportation":
		known = r.Transportation != nil && strings.TrimSpace(*r.Transportation) != ""
	case "closures":
		known = r.Closures != nil && strings.TrimSpace(*r.Closures) != ""
	case "service_charge":
		known = r.ServiceCharge != nil && strings.TrimSpace(*r.ServiceCharge) != ""
	case "status", "source_status":
		known = r.Status != nil && strings.TrimSpace(*r.Status) != ""
	case "source_warnings":
		known = len(r.SourceWarnings) > 0
	case "facilities":
		known = len(r.Facilities) > 0
	case "awards":
		known = len(r.Awards) > 0
	}
	if known {
		return "known"
	}
	if r.Surface != "detail" {
		switch field {
		case "hours", "payment", "reservation", "address", "transportation", "service_charge", "review_lunch_budget", "review_dinner_budget":
			return "detail_not_fetched"
		}
	}
	return r.FieldState(field)
}

type listMatchReason struct {
	Field string `json:"field"`
	Value any    `json:"value"`
}
type listAlternative struct {
	savedRestaurant
	Reasons []listMatchReason `json:"reasons"`
}
type listExcluded struct {
	ID      string            `json:"id"`
	Status  string            `json:"status"`
	Reasons []listMatchReason `json:"reasons"`
}

func parseListFields(raw string, allowed map[string]bool) ([]string, error) {
	fields := make([]string, 0)
	seen := map[string]bool{}
	for _, field := range strings.Split(raw, ",") {
		field = strings.TrimSpace(field)
		if field == "" || !allowed[field] {
			return nil, fmt.Errorf("unsupported field %q", field)
		}
		if !seen[field] {
			seen[field] = true
			fields = append(fields, field)
		}
	}
	return fields, nil
}

func normalizedCategories(categories []string) map[string]string {
	values := map[string]string{}
	for _, category := range categories {
		label := strings.Join(strings.Fields(category), " ")
		if label != "" && label != "-" {
			values[strings.ToLower(label)] = label
		}
	}
	return values
}

func sharedCategories(a, b []string) []string {
	left, right := normalizedCategories(a), normalizedCategories(b)
	shared := make([]string, 0)
	for key, label := range left {
		if _, ok := right[key]; ok {
			shared = append(shared, label)
		}
	}
	sort.Strings(shared)
	return shared
}

func verifiedSavedArea(r domain.Restaurant) bool {
	return r.Area.Verified && r.Area.Prefecture != "" && r.Area.Area1 != "" && r.Area.Area2 != "" && savedFieldState(r, "area") == "known"
}

func newTabelogListAlternativesCmd(flags *rootFlags) *cobra.Command {
	var anchorID, match, meal string
	var budgetMax int
	cmd := &cobra.Command{Use: "alternatives <list>", Short: "Find factual alternatives within the saved set", Long: "Match saved candidates to an anchor's verified source area and exact normalized category labels. Optional meal budget checks the cached source bracket upper bound. Unknown matching fields are unevaluable; results preserve list order. Use lists compare for general comparison.", Example: "  tabelog-pp-cli lists alternatives tokyo-bars --for 13005012 --match area,category --meal dinner --budget-max 5000 --agent", Annotations: listReadAnnotations()}
	cmd.Flags().StringVar(&anchorID, "for", "", "Saved anchor restaurant ID")
	cmd.Flags().StringVar(&match, "match", "", "Comma-separated anchor fields: area,category")
	cmd.Flags().StringVar(&meal, "meal", "", "Meal for cached budget comparison: lunch or dinner")
	cmd.Flags().IntVar(&budgetMax, "budget-max", 0, "Maximum known cached meal-bracket upper bound in JPY")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := listArgCount(flags, args, 1, 1); err != nil {
			return err
		}
		if err := validateListArguments(flags, args, nil); err != nil {
			return err
		}
		if anchorID == "" {
			if !flags.dryRun || cmd.Flags().Changed("for") {
				return fmt.Errorf("--for requires a saved anchor ID")
			}
		} else if err := notebook.ValidateID(anchorID); err != nil {
			return err
		}
		fields := make([]string, 0)
		if match == "" {
			if !flags.dryRun || cmd.Flags().Changed("match") {
				return fmt.Errorf("--match requires area, category, or both")
			}
		} else {
			var err error
			fields, err = parseListFields(match, map[string]bool{"area": true, "category": true})
			if err != nil {
				return err
			}
		}
		budgetSet := cmd.Flags().Changed("budget-max")
		if meal != "" && meal != "lunch" && meal != "dinner" {
			return fmt.Errorf("meal must be lunch or dinner")
		}
		if budgetSet && (budgetMax <= 0 || meal == "") {
			return fmt.Errorf("cached budget comparison requires a positive --budget-max and explicit --meal lunch or dinner")
		}
		name := listName(args)
		criteria := map[string]any{"for": anchorID, "match": fields, "meal": meal, "budget_max": budgetMax, "budget_scope": "cached_bracket"}
		if flags.dryRun {
			var ids []string
			if anchorID != "" {
				ids = []string{anchorID}
			}
			return listDryRun(cmd, flags, "alternatives", name, ids, criteria)
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		db, nb, err := tripOpen(ctx, flags)
		if err != nil {
			return err
		}
		defer db.Close()
		entries, err := nb.Entries(ctx, name, nil)
		if err != nil {
			return err
		}
		items, err := decodeSaved(entries, flags.maxAge)
		if err != nil {
			return err
		}
		var anchor *savedRestaurant
		for i := range items {
			if items[i].ID == anchorID {
				anchor = &items[i]
				break
			}
		}
		if anchor == nil {
			return fmt.Errorf("anchor %s: %w", anchorID, notebook.ErrMemberNotFound)
		}
		matches := make([]listAlternative, 0)
		excluded := make([]listExcluded, 0)
		unmatched, unknown := 0, 0
		for _, candidate := range items {
			if candidate.ID == anchorID {
				continue
			}
			reasons := make([]listMatchReason, 0)
			failures := make([]listMatchReason, 0)
			unevaluable := false
			for _, field := range fields {
				if field == "area" {
					if !verifiedSavedArea(anchor.Restaurant) || !verifiedSavedArea(candidate.Restaurant) {
						unevaluable = true
						failures = append(failures, listMatchReason{"area", "missing or unverified source area"})
						continue
					}
					if anchor.Area.Prefecture != candidate.Area.Prefecture || anchor.Area.Area1 != candidate.Area.Area1 || anchor.Area.Area2 != candidate.Area.Area2 {
						failures = append(failures, listMatchReason{"area", "different source area"})
					} else {
						reasons = append(reasons, listMatchReason{"area", candidate.Area})
					}
				} else {
					if savedFieldState(anchor.Restaurant, "categories") != "known" || savedFieldState(candidate.Restaurant, "categories") != "known" || len(normalizedCategories(anchor.Categories)) == 0 || len(normalizedCategories(candidate.Categories)) == 0 {
						unevaluable = true
						failures = append(failures, listMatchReason{"category", "source category unknown"})
						continue
					}
					shared := sharedCategories(anchor.Categories, candidate.Categories)
					if len(shared) == 0 {
						failures = append(failures, listMatchReason{"category", "no shared source category"})
					} else {
						reasons = append(reasons, listMatchReason{"category", shared})
					}
				}
			}
			if budgetSet {
				budget, field := candidate.DinnerBudget, "dinner_budget"
				if meal == "lunch" {
					budget, field = candidate.LunchBudget, "lunch_budget"
				}
				if savedFieldState(candidate.Restaurant, field) != "known" || budget.MaxJPY == nil {
					unevaluable = true
					failures = append(failures, listMatchReason{field, "cached upper bound unknown"})
				} else if *budget.MaxJPY > budgetMax {
					failures = append(failures, listMatchReason{field, "cached upper bound exceeds requested JPY ceiling"})
				} else {
					reasons = append(reasons, listMatchReason{field, map[string]any{"upper_bound_jpy": *budget.MaxJPY, "ceiling_jpy": budgetMax}})
				}
			}
			if len(failures) == 0 {
				matches = append(matches, listAlternative{candidate, reasons})
			} else {
				status := "unmatched"
				if unevaluable {
					status = "unevaluable"
					unknown++
				} else {
					unmatched++
				}
				excluded = append(excluded, listExcluded{candidate.ID, status, failures})
			}
		}
		meta := listMeta(name, "alternatives", len(matches), len(items)-1)
		for k, v := range criteria {
			meta.Criteria[k] = v
		}
		meta.Criteria["matched"] = len(matches)
		meta.Criteria["unmatched"] = unmatched
		meta.Criteria["unevaluable"] = unknown
		meta.Criteria["excluded"] = excluded
		listHints(cmd, items, flags)
		return tripPrint(cmd, flags, matches, meta)
	}
	return cmd
}

type listAuditFinding struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}
type listAuditItem struct {
	savedRestaurant
	Findings []listAuditFinding `json:"findings"`
}

func newTabelogListAuditCmd(flags *rootFlags) *cobra.Command {
	var require, ageText string
	cmd := &cobra.Command{Use: "audit <list>", Short: "Inspect missing and old evidence without fetching", Long: "Inspect saved evidence states and snapshot age. Findings distinguish detail_not_fetched, source_unknown and older_than_threshold. Source-unknown facts may stay unknown after another fetch. Use lists refresh to retrieve current facts.", Example: "  tabelog-pp-cli lists audit tokyo-bars --require hours,payment,reservation,dinner_budget --max-age 24h --agent", Annotations: listReadAnnotations()}
	cmd.Flags().StringVar(&require, "require", "hours,payment,reservation,dinner_budget", "Required evidence fields, comma-separated")
	cmd.Flags().StringVar(&ageText, "max-age", "24h", "Snapshot age threshold; 0 disables age checks; supports h, d and w")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := listArgCount(flags, args, 1, 1); err != nil {
			return err
		}
		if err := validateListArguments(flags, args, nil); err != nil {
			return err
		}
		allowed := map[string]bool{"hours": true, "payment": true, "reservation": true, "dinner_budget": true, "lunch_budget": true, "address": true, "transportation": true, "closures": true, "service_charge": true, "rating": true, "review_count": true, "categories": true}
		fields, err := parseListFields(require, allowed)
		if err != nil {
			return err
		}
		maxAge, err := cliutil.ParseDurationLoose(ageText)
		if err != nil || maxAge < 0 {
			return fmt.Errorf("max-age must be a nonnegative duration: %q", ageText)
		}
		name := listName(args)
		criteria := map[string]any{"require": fields, "max_age": ageText}
		if flags.dryRun {
			return listDryRun(cmd, flags, "audit", name, nil, criteria)
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		db, nb, err := tripOpen(ctx, flags)
		if err != nil {
			return err
		}
		defer db.Close()
		entries, err := nb.Entries(ctx, name, nil)
		if err != nil {
			return err
		}
		saved, err := decodeSaved(entries, maxAge)
		if err != nil {
			return err
		}
		items := make([]listAuditItem, 0, len(saved))
		findings, needsReview := 0, 0
		for _, item := range saved {
			problems := make([]listAuditFinding, 0)
			for _, field := range fields {
				if state := savedFieldState(item.Restaurant, field); state != "known" {
					problems = append(problems, listAuditFinding{field, state})
				}
			}
			if maxAge > 0 {
				if item.FetchedAt.IsZero() {
					problems = append(problems, listAuditFinding{"fetched_at", "source_unknown"})
				} else if item.Stale {
					problems = append(problems, listAuditFinding{"snapshot", "older_than_threshold"})
				}
			}
			findings += len(problems)
			if len(problems) > 0 {
				needsReview++
			}
			items = append(items, listAuditItem{item, problems})
		}
		meta := listMeta(name, "audit", len(items), len(items))
		for k, v := range criteria {
			meta.Criteria[k] = v
		}
		meta.Criteria["findings"] = findings
		meta.Criteria["needs_review"] = needsReview
		hintFlags := *flags
		hintFlags.maxAge = maxAge
		listHints(cmd, saved, &hintFlags)
		return tripPrint(cmd, flags, items, meta)
	}
	return cmd
}
