// pp:data-source live
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/activity-japan/internal/activityjapan"
	"github.com/mvanhorn/printing-press-library/library/travel/activity-japan/internal/cliutil"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		experience, _, _ := root.Find([]string{"experience"})
		if experience != nil && experience.Name() == "experience" {
			for _, c := range []*cobra.Command{ajDetail(flags), ajSessions(flags), ajCheck(flags), ajHandoff(flags), ajPrice(flags), ajDates(flags), ajCompare(flags), ajBrief(flags)} {
				addNovelCommandIfAbsent(experience, c)
			}
		}
		inventory, _, _ := root.Find([]string{"inventory"})
		if inventory != nil && inventory.Name() == "inventory" {
			addNovelCommandIfAbsent(inventory, ajLanguages(flags))
		}
		sourcePlan, _, _ := root.Find([]string{"source-plan"})
		if sourcePlan != nil && sourcePlan.Name() == "source-plan" {
			examples := map[string]string{
				"date-price":  "  activity-japan-pp-cli source-plan date-price --plan-id 62375 --date 2026-10-08 --agent",
				"detail":      "  activity-japan-pp-cli source-plan detail --plan-id 62375 --lang-flag en --url https://en.activityjapan.com/publish/plan/62375 --agent",
				"sessions":    "  activity-japan-pp-cli source-plan sessions --plan-id 62375 --selected-date 2026-10-08 --agent",
				"stock-check": "  activity-japan-pp-cli source-plan stock-check --plan-id 62375 --date 2026-10-08 --c-id 189637 --count 2 --status 1 --type 2 --agent",
			}
			for _, leaf := range sourcePlan.Commands() {
				if example, ok := examples[leaf.Name()]; ok {
					leaf.Example = example
				}
				if leaf.Name() == "stock-check" {
					leaf.Short = "Fetch raw stock response without validating plan eligibility"
					leaf.Long = "Returns the Activity Japan source stock result for an exact plan, course, date and count. This raw mirror does not validate the plan's party or age limits, and its result is not a reservation. Use experience check for a party-aware stock observation."
				}
			}
		}
	})
}
func ajAnnotations(happy string) map[string]string {
	return map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": happy}
}
func ajContext(parent context.Context, flags *rootFlags) (context.Context, context.CancelFunc) {
	ctx, cancel := boundCtx(parent, flags)
	if flags.timeout <= 0 || flags.timeout > 25*time.Second {
		ctx2, cancel2 := context.WithTimeout(ctx, 25*time.Second)
		return ctx2, func() { cancel2(); cancel() }
	}
	return ctx, cancel
}
func ajService(flags *rootFlags) (*activityjapan.Service, error) {
	c, e := flags.newClient()
	if e != nil {
		return nil, e
	}
	c.NoCache = true
	return activityjapan.NewService(c), nil
}
func ajOutput(cmd *cobra.Command, flags *rootFlags, results any, requests int, extra map[string]any) error {
	meta := map[string]any{"source": "live", "provider": "activity-japan", "observed_at": activityjapan.ObservedAt(), "time_zone": "Asia/Tokyo", "currency": "JPY", "upstream_requests": requests, "cache": "disabled", "reservation_confirmed": false}
	for k, v := range extra {
		meta[k] = v
	}
	if flags.selectFields != "" {
		fields := make([]string, 0)
		for _, item := range strings.Split(flags.selectFields, ",") {
			item = strings.TrimSpace(item)
			if item == "results" {
				fields = nil
				break
			}
			if strings.HasPrefix(item, "meta") {
				return usageErr(errors.New("--select addresses result fields; use jq .meta for response metadata"))
			}
			item = strings.TrimPrefix(item, "results.")
			if item != "" {
				fields = append(fields, item)
			}
		}
		if len(fields) > 0 {
			raw, err := json.Marshal(results)
			if err != nil {
				return err
			}
			filtered, err := filterFieldsChecked(raw, strings.Join(fields, ","))
			if err != nil {
				return err
			}
			results = json.RawMessage(filtered)
		}
		original := flags.selectFields
		flags.selectFields = ""
		defer func() { flags.selectFields = original }()
	}
	return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"meta": meta, "results": results}, flags)
}
func ajInput(cmd *cobra.Command, flags *rootFlags, args []string, expected int, action string) error {
	if dryRunOK(flags) {
		return writeDryRun(cmd.OutOrStdout(), flags, action)
	}
	if len(args) != expected {
		return usageErr(fmt.Errorf("%s requires %d plan ID argument(s); run %s --help", cmd.CommandPath(), expected, cmd.CommandPath()))
	}
	return nil
}
func ajID(v string) (string, error) { return activityjapan.URLFromPlanInput(v) }
func ajPositive(n int, label string) error {
	if n < 1 || n > 50 {
		return usageErr(fmt.Errorf("%s must be 1..50", label))
	}
	return nil
}

func ajPlanParty(plan activityjapan.Plan, participants int) error {
	if plan.PartyMin != nil && participants < *plan.PartyMin {
		return usageErr(fmt.Errorf("plan requires at least %d participants", *plan.PartyMin))
	}
	if plan.PartyMax != nil && participants > *plan.PartyMax {
		return usageErr(fmt.Errorf("plan allows at most %d participants", *plan.PartyMax))
	}
	return nil
}

func ajCompareRequestBounds(date string, adults, age, maxJPY, maxMinutes int, instant bool) error {
	if adults < 0 || adults > 50 || age < -1 || age > 120 || maxJPY < 0 || maxMinutes < 0 {
		return usageErr(errors.New("invalid adults, age, max-jpy or max-minutes bound"))
	}
	if instant && date == "" {
		return usageErr(errors.New("--instant-only requires --date to evaluate availability"))
	}
	if maxJPY > 0 && (date == "" || adults == 0) {
		return usageErr(errors.New("--max-jpy requires --date and --adults to evaluate a party budget"))
	}
	return nil
}

func ajPartialItems(err error, count int) bool {
	var partial *activityjapan.PartialItemsError
	return count > 0 && errors.As(err, &partial)
}

func ajIndexedURL(ctx context.Context, id, lang string, flags *rootFlags) (*string, int, error) {
	dir, err := cliutil.CacheDir()
	if err != nil {
		return nil, 0, err
	}
	presence, requests, err := activityjapan.CheckLanguages(ctx, id, dir, flags.noCache, false)
	if err != nil {
		return nil, requests, err
	}
	var url *string
	checked := presence.EnglishChecked
	if lang == "ja" {
		url, checked = presence.JapaneseURL, presence.JapaneseChecked
	} else {
		url = presence.EnglishURL
	}
	if !checked {
		return nil, requests, fmt.Errorf("%s plan sitemap could not be checked; run inventory languages %s --refresh", lang, id)
	}
	if url == nil {
		return nil, requests, fmt.Errorf("plan %s is absent from the %s plan sitemap; try the other --lang", id, lang)
	}
	return url, requests, nil
}

func ajPage[T any](items []T, page, limit int) ([]T, map[string]any, error) {
	if page < 1 || page > 100 || limit < 1 || limit > 50 {
		return nil, nil, usageErr(errors.New("--page must be 1..100 and --limit must be 1..50"))
	}
	start := (page - 1) * limit
	if start > len(items) {
		start = len(items)
	}
	end := start + limit
	if end > len(items) {
		end = len(items)
	}
	return items[start:end], map[string]any{"page": page, "page_size": limit, "total": len(items), "has_more": end < len(items)}, nil
}

func ajSessionSummary(sessions []activityjapan.Session) map[string]any {
	counts := map[string]int{"instant_confirmable": 0, "reservation_request": 0, "closed": 0, "not_accepted": 0, "sold_out_for_party": 0, "unknown": 0}
	for _, session := range sessions {
		counts[session.Availability]++
	}
	var first, last any
	if len(sessions) > 0 {
		first = sessions[0].StartLocal
		last = sessions[len(sessions)-1].StartLocal
	}
	return map[string]any{"total": len(sessions), "states": counts, "first_start_local": first, "last_start_local": last}
}

func ajQuoteSummary(quotes []activityjapan.Quote) []map[string]any {
	out := make([]map[string]any, 0, len(quotes))
	for _, q := range quotes {
		out = append(out, map[string]any{"option_id": q.OptionID, "unit_price_jpy": q.PriceJPY, "basis": q.Basis, "age_class": q.AgeClass, "age_band_text": q.AgeBandText, "min_units": q.MinUnits, "max_units": q.MaxUnits, "derived_adult_subtotal_jpy": q.DerivedSubtotalJPY, "subtotal_note": q.SubtotalNote})
	}
	return out
}
func ajDetail(flags *rootFlags) *cobra.Command {
	var lang string
	cmd := &cobra.Command{Use: "detail <plan-id-or-URL>", Short: "Inspect source plan facts, Japanese name, conditions, options, and explicit unknowns", Example: "  activity-japan-pp-cli experience detail 62375 --lang en --agent", Annotations: ajAnnotations("plan=62375;--lang=en"), RunE: func(cmd *cobra.Command, args []string) error {
		if e := ajInput(cmd, flags, args, 1, "experience detail"); e != nil || flags.dryRun {
			return e
		}
		id, e := ajID(args[0])
		if e != nil {
			return usageErr(e)
		}
		if e := activityjapan.ValidLang(lang); e != nil {
			return usageErr(e)
		}
		ctx, cancel := ajContext(cmd.Context(), flags)
		defer cancel()
		s, e := ajService(flags)
		if e != nil {
			return e
		}
		plan, e := s.Detail(ctx, id, lang)
		if e != nil {
			return e
		}
		return ajOutput(cmd, flags, plan, s.Requests, map[string]any{"coverage": "one exact plan; Japanese name requested independently"})
	}}
	cmd.Flags().StringVar(&lang, "lang", "en", "Website language for localized source fields: en or ja; does not prove guide language")
	return cmd
}
func ajSessions(flags *rootFlags) *cobra.Command {
	var date string
	var page, limit int
	cmd := &cobra.Command{Use: "sessions <plan-id>", Short: "Observe exact dated course IDs, local starts, and source booking states", Example: "  activity-japan-pp-cli experience sessions 62375 --date 2026-10-08 --agent", Annotations: ajAnnotations("plan=62375;--date=2026-10-08"), RunE: func(cmd *cobra.Command, args []string) error {
		if e := ajInput(cmd, flags, args, 1, "experience sessions"); e != nil || flags.dryRun {
			return e
		}
		id, e := ajID(args[0])
		if e != nil {
			return usageErr(e)
		}
		if e := activityjapan.ValidateDate(date); e != nil {
			return usageErr(e)
		}
		ctx, cancel := ajContext(cmd.Context(), flags)
		defer cancel()
		s, e := ajService(flags)
		if e != nil {
			return e
		}
		sessions, e := s.Sessions(ctx, id, date)
		if e != nil && !ajPartialItems(e, len(sessions)) {
			return e
		}
		shown, pagination, pageErr := ajPage(sessions, page, limit)
		if pageErr != nil {
			return pageErr
		}
		var errorsOut []string
		if e != nil {
			errorsOut = []string{e.Error()}
		}
		return ajOutput(cmd, flags, map[string]any{"plan_id": id, "date": date, "sessions": shown, "pagination": pagination, "truncated": pagination["has_more"], "availability_scope": "date-specific observation; not a reservation", "partial": e != nil, "errors": errorsOut}, s.Requests, nil)
	}}
	cmd.Flags().StringVar(&date, "date", "", "Activity date YYYY-MM-DD in Asia/Tokyo (required)")
	cmd.Flags().IntVar(&page, "page", 1, "Result page, 1..100")
	cmd.Flags().IntVar(&limit, "limit", 10, "Sessions per page, 1..50")
	return cmd
}
func ajCheck(flags *rootFlags) *cobra.Command {
	var date, session string
	var adults int
	cmd := &cobra.Command{Use: "check <plan-id>", Short: "Recheck a dated source session for a bounded party without submitting a booking", Example: "  activity-japan-pp-cli experience check 62375 --date 2026-10-08 --session 189637 --adults 2 --agent", Annotations: ajAnnotations("plan=62375;--date=2026-10-08;--session=189637;--adults=2"), RunE: func(cmd *cobra.Command, args []string) error {
		if e := ajInput(cmd, flags, args, 1, "experience check"); e != nil || flags.dryRun {
			return e
		}
		id, e := ajID(args[0])
		if e != nil {
			return usageErr(e)
		}
		if e := activityjapan.ValidateDate(date); e != nil {
			return usageErr(e)
		}
		if e := ajPositive(adults, "--adults"); e != nil {
			return e
		}
		if session == "" {
			return usageErr(errors.New("--session is required; run experience sessions first for an exact session ID"))
		}
		ctx, cancel := ajContext(cmd.Context(), flags)
		defer cancel()
		s, e := ajService(flags)
		if e != nil {
			return e
		}
		check, e := s.Check(ctx, id, date, session, adults)
		if e != nil {
			return e
		}
		return ajOutput(cmd, flags, check, s.Requests, map[string]any{"availability_scope": "observed stock response; not a reservation"})
	}}
	cmd.Flags().StringVar(&date, "date", "", "Activity date YYYY-MM-DD in Asia/Tokyo (required)")
	cmd.Flags().StringVar(&session, "session", "", "Exact source session ID from experience sessions (required)")
	cmd.Flags().IntVar(&adults, "adults", 0, "Basic-fee participant quantity, 1..50 (required)")
	return cmd
}
func ajHandoff(flags *rootFlags) *cobra.Command {
	var lang string
	var refresh bool
	cmd := &cobra.Command{Use: "handoff <plan-id>", Short: "Return an indexed canonical Activity Japan booking URL; no booking action", Example: "  activity-japan-pp-cli experience handoff 62375 --lang en --agent", Annotations: ajAnnotations("plan=62375;--lang=en"), RunE: func(cmd *cobra.Command, args []string) error {
		if e := ajInput(cmd, flags, args, 1, "experience handoff"); e != nil || flags.dryRun {
			return e
		}
		id, e := ajID(args[0])
		if e != nil {
			return usageErr(e)
		}
		if e := activityjapan.ValidLang(lang); e != nil {
			return usageErr(e)
		}
		dir, e := cliutil.CacheDir()
		if e != nil {
			return e
		}
		ctx, cancel := ajContext(cmd.Context(), flags)
		defer cancel()
		p, req, e := activityjapan.CheckLanguages(ctx, id, dir, flags.noCache, refresh)
		if e != nil {
			return e
		}
		url := p.EnglishURL
		if lang == "ja" {
			url = p.JapaneseURL
		}
		if url == nil {
			checked := p.EnglishChecked
			if lang == "ja" {
				checked = p.JapaneseChecked
			}
			if !checked {
				return fmt.Errorf("%s sitemap could not be checked for plan %s; run inventory languages %s --refresh", lang, id, id)
			}
			return fmt.Errorf("plan %s is absent from the %s plan sitemap; try inventory languages %s and the other --lang", id, lang, id)
		}
		return ajOutput(cmd, flags, map[string]any{"plan_id": id, "site_language": lang, "canonical_url": url, "booking_action": "open URL in browser; no reservation submitted", "indexed_in_sitemap": true}, req, map[string]any{"cache": p.Cache, "coverage": "public sitemap index only"})
	}}
	cmd.Flags().StringVar(&lang, "lang", "en", "Canonical website language: en or ja")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Refetch both public plan sitemaps instead of using their 24-hour cache")
	return cmd
}
func ajPrice(flags *rootFlags) *cobra.Command {
	var date, lang, optionID string
	var adults, page, limit int
	cmd := &cobra.Command{Use: "price <plan-id>", Short: "Separate option headline amounts from selected-date unit quotes and safe subtotals", Example: "  activity-japan-pp-cli experience price 62375 --date 2026-10-08 --adults 2 --agent", Annotations: ajAnnotations("plan=62375;--date=2026-10-08;--adults=2"), RunE: func(cmd *cobra.Command, args []string) error {
		if e := ajInput(cmd, flags, args, 1, "experience price"); e != nil || flags.dryRun {
			return e
		}
		id, e := ajID(args[0])
		if e != nil {
			return usageErr(e)
		}
		if e := activityjapan.ValidLang(lang); e != nil {
			return usageErr(e)
		}
		if e := activityjapan.ValidateDate(date); e != nil {
			return usageErr(e)
		}
		if adults < 0 || adults > 50 {
			return usageErr(errors.New("--adults must be 0..50; 0 means no party subtotal"))
		}
		ctx, cancel := ajContext(cmd.Context(), flags)
		defer cancel()
		s, e := ajService(flags)
		if e != nil {
			return e
		}
		plan, e := s.Detail(ctx, id, lang)
		if e != nil {
			return e
		}
		if adults > 0 {
			if e := ajPlanParty(plan, adults); e != nil {
				return e
			}
		}
		quotes, e := s.Price(ctx, id, date, adults, plan.Options)
		if e != nil && !ajPartialItems(e, len(quotes)) {
			return e
		}
		partialErrors := append([]string{}, plan.Errors...)
		if e != nil {
			partialErrors = append(partialErrors, e.Error())
		}
		if optionID != "" {
			filtered := make([]activityjapan.Quote, 0, 1)
			for _, q := range quotes {
				if q.OptionID == optionID {
					filtered = append(filtered, q)
				}
			}
			if len(filtered) == 0 {
				return usageErr(fmt.Errorf("option %s was not returned for plan %s on %s; omit --option to list IDs", optionID, id, date))
			}
			quotes = filtered
		}
		shown, pagination, e := ajPage(quotes, page, limit)
		if e != nil {
			return e
		}
		options := make([]activityjapan.Option, 0, len(shown))
		for _, q := range shown {
			for _, o := range plan.Options {
				if o.OptionID == q.OptionID {
					options = append(options, o)
					break
				}
			}
		}
		return ajOutput(cmd, flags, map[string]any{"plan_id": id, "date": date, "participants": adults, "headline_base_price_jpy": plan.HeadlineBasePriceJPY, "headline_discount_jpy": plan.HeadlineDiscountJPY, "derived_headline_from_jpy": plan.DerivedHeadlineFromJPY, "undated_options": options, "selected_date_quotes": shown, "pagination": pagination, "truncated": pagination["has_more"], "mandatory_fees_jpy": nil, "optional_extras_jpy": nil, "tax_inclusion": "unknown from JSON; source amounts are not multiplied by taxIn", "price_note": "Selected-date unit quotes are source integers; derived adult subtotals exclude unpriced fees and extras", "partial": len(partialErrors) > 0, "errors": partialErrors}, s.Requests, nil)
	}}
	cmd.Flags().StringVar(&date, "date", "", "Activity date YYYY-MM-DD in Asia/Tokyo (required)")
	cmd.Flags().IntVar(&adults, "adults", 0, "Participant count, 1..50; omit for unit prices only")
	cmd.Flags().StringVar(&lang, "lang", "en", "Website language for headline option names: en or ja")
	cmd.Flags().StringVar(&optionID, "option", "", "Exact source option ID; use when comparing a selected option")
	cmd.Flags().IntVar(&page, "page", 1, "Option quote page, 1..100")
	cmd.Flags().IntVar(&limit, "limit", 10, "Quotes per page, 1..50")
	return cmd
}
func ajDates(flags *rootFlags) *cobra.Command {
	var dates string
	cmd := &cobra.Command{Use: "dates <plan-id>", Short: "Compare up to seven explicit dates using dated sessions and option quotes", Example: "  activity-japan-pp-cli experience dates 62375 --dates 2026-10-07,2026-10-08 --agent", Annotations: ajAnnotations("plan=62375;--dates=2026-10-07,2026-10-08"), RunE: func(cmd *cobra.Command, args []string) error {
		if e := ajInput(cmd, flags, args, 1, "experience dates"); e != nil || flags.dryRun {
			return e
		}
		id, e := ajID(args[0])
		if e != nil {
			return usageErr(e)
		}
		parts := strings.Split(dates, ",")
		if dates == "" || len(parts) > 7 {
			return usageErr(errors.New("--dates requires 1..7 comma-separated Asia/Tokyo dates"))
		}
		seen := map[string]bool{}
		for _, d := range parts {
			if e := activityjapan.ValidateDate(d); e != nil {
				return usageErr(e)
			}
			if seen[d] {
				return usageErr(fmt.Errorf("duplicate date %s", d))
			}
			seen[d] = true
		}
		ctx, cancel := ajContext(cmd.Context(), flags)
		defer cancel()
		s, e := ajService(flags)
		if e != nil {
			return e
		}
		out := []map[string]any{}
		errs := []map[string]string{}
		usable := 0
		for _, d := range parts {
			row := map[string]any{"date": d, "session_summary": nil, "option_prices": nil}
			sessions, se := s.Sessions(ctx, id, d)
			if se != nil {
				errs = append(errs, map[string]string{"date": d, "operation": "sessions", "error": se.Error()})
			}
			if se == nil || ajPartialItems(se, len(sessions)) {
				row["session_summary"] = ajSessionSummary(sessions)
				usable++
			}
			quotes, qe := s.Price(ctx, id, d, 0, nil)
			if qe != nil {
				errs = append(errs, map[string]string{"date": d, "operation": "price", "error": qe.Error()})
			}
			if qe == nil || ajPartialItems(qe, len(quotes)) {
				shown, _, _ := ajPage(ajQuoteSummary(quotes), 1, 10)
				row["option_prices"] = shown
				row["option_prices_total"] = len(quotes)
				row["option_prices_truncated"] = len(quotes) > len(shown)
				usable++
			}
			out = append(out, row)
		}
		if usable == 0 {
			return fmt.Errorf("all dated observations failed: %v", errs)
		}
		return ajOutput(cmd, flags, map[string]any{"plan_id": id, "dates": out, "errors": errs, "partial": len(errs) > 0, "truncated": false}, s.Requests, map[string]any{"coverage": "explicit dates only; no operating-period inference"})
	}}
	cmd.Flags().StringVar(&dates, "dates", "", "One to seven comma-separated YYYY-MM-DD dates (required)")
	return cmd
}
func ajCompare(flags *rootFlags) *cobra.Command {
	var date, lang, guideLang string
	var adults, age, maxJPY, maxMinutes int
	var instant bool
	cmd := &cobra.Command{Use: "compare <plan-id> <plan-id> [plan-id...]", Short: "Compare at most five plans against explicit traveler constraints, preserving unknowns", Example: "  activity-japan-pp-cli experience compare 62375 2044 --date 2026-10-08 --adults 2 --age 30 --max-jpy 7000 --agent", Annotations: ajAnnotations("first=62375;second=2044;--date=2026-10-08;--adults=2"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "experience compare")
		}
		if len(args) == 0 && !hasChangedLocalFlags(cmd) && !flags.asJSON {
			return cmd.Help()
		}
		if len(args) < 2 || len(args) > 5 {
			return usageErr(errors.New("compare requires 2..5 plan IDs"))
		}
		if e := activityjapan.ValidLang(lang); e != nil {
			return usageErr(e)
		}
		if date != "" {
			if e := activityjapan.ValidateDate(date); e != nil {
				return usageErr(e)
			}
		}
		if e := ajCompareRequestBounds(date, adults, age, maxJPY, maxMinutes, instant); e != nil {
			return e
		}
		ctx, cancel := ajContext(cmd.Context(), flags)
		defer cancel()
		s, e := ajService(flags)
		if e != nil {
			return e
		}
		rows := []map[string]any{}
		errs := []map[string]string{}
		inventoryRequests := 0
		seen := map[string]bool{}
		for _, arg := range args {
			id, e := ajID(arg)
			if e != nil {
				return usageErr(e)
			}
			if seen[id] {
				return usageErr(fmt.Errorf("duplicate plan %s", id))
			}
			seen[id] = true
			plan, e := s.Detail(ctx, id, lang)
			if e != nil {
				errs = append(errs, map[string]string{"plan_id": id, "error": e.Error()})
				continue
			}
			for _, problem := range plan.Errors {
				errs = append(errs, map[string]string{"plan_id": id, "operation": "original Japanese detail", "error": problem})
			}
			verdict := map[string]string{}
			if adults > 0 {
				min := plan.PartyMin
				switch {
				case min != nil && adults < *min:
					verdict["party"] = "mismatch"
				case plan.PartyMax != nil && adults > *plan.PartyMax:
					verdict["party"] = "mismatch"
				case min != nil && plan.PartyMax != nil:
					verdict["party"] = "match"
				default:
					verdict["party"] = "unknown"
				}
			}
			if age >= 0 {
				switch {
				case plan.AgeMinYears != nil && age < *plan.AgeMinYears:
					verdict["age"] = "mismatch"
				case plan.AgeMaxYears != nil && age > *plan.AgeMaxYears:
					verdict["age"] = "mismatch"
				case plan.AgeMinYears != nil && plan.AgeMaxYears != nil:
					verdict["age"] = "match"
				default:
					verdict["age"] = "unknown"
				}
			}
			if guideLang != "" {
				verdict["spoken_language"] = "unknown"
			}
			if maxMinutes > 0 {
				v := "unknown"
				if plan.DerivedTotalMinutes != nil {
					if *plan.DerivedTotalMinutes <= maxMinutes {
						v = "match"
					} else {
						v = "mismatch"
					}
				}
				verdict["total_duration"] = v
			}
			var sessions []activityjapan.Session
			var quotes []activityjapan.Quote
			if date != "" {
				sessions, e = s.Sessions(ctx, id, date)
				if e != nil {
					errs = append(errs, map[string]string{"plan_id": id, "operation": "sessions", "error": e.Error()})
				}
				if instant {
					v := "unknown"
					if e == nil && len(sessions) > 0 {
						v = "mismatch"
					}
					for _, ss := range sessions {
						if ss.Availability == "instant_confirmable" {
							v = "match"
							break
						}
						if ss.Availability == "unknown" && v != "match" {
							v = "unknown"
						}
					}
					verdict["instant_on_date"] = v
				}
			}
			if date != "" && maxJPY > 0 {
				verdict["budget_jpy"] = "unknown"
				quotes, e = s.Price(ctx, id, date, adults, plan.Options)
				if e != nil {
					errs = append(errs, map[string]string{"plan_id": id, "operation": "price", "error": e.Error()})
				}
				if e == nil {
					v := "unknown"
					allKnownAbove := true
					adultOptions := 0
					for _, q := range quotes {
						if q.AgeClass == "child" || q.AgeClass == "infant" {
							continue
						}
						adultOptions++
						if q.DerivedSubtotalJPY == nil || *q.DerivedSubtotalJPY <= maxJPY {
							allKnownAbove = false
						}
					}
					if adultOptions == 0 {
						allKnownAbove = false
					}
					if allKnownAbove {
						v = "mismatch"
					}
					verdict["budget_jpy"] = v
				}
			} else if maxJPY > 0 {
				verdict["budget_jpy"] = "unknown"
			}
			var sessionSummary any
			if date != "" && sessions != nil {
				sessionSummary = ajSessionSummary(sessions)
			}
			var lowestSubtotal *int
			for _, q := range quotes {
				if q.DerivedSubtotalJPY != nil && (lowestSubtotal == nil || *q.DerivedSubtotalJPY < *lowestSubtotal) {
					lowestSubtotal = q.DerivedSubtotalJPY
				}
			}
			canonicalURL, req, urlErr := ajIndexedURL(ctx, id, lang, flags)
			inventoryRequests += req
			if urlErr != nil {
				errs = append(errs, map[string]string{"plan_id": id, "operation": "canonical handoff URL", "error": urlErr.Error()})
			}
			rows = append(rows, map[string]any{"plan_id": id, "name": plan.NameLocalized, "name_original_ja": plan.NameOriginalJA, "source_url": plan.SourceURL, "canonical_url": canonicalURL, "canonical_url_verified": canonicalURL != nil, "verdicts": verdict, "age_min_years": plan.AgeMinYears, "age_max_years": plan.AgeMaxYears, "party_min": plan.PartyMin, "party_max": plan.PartyMax, "derived_total_minutes": plan.DerivedTotalMinutes, "session_summary": sessionSummary, "lowest_derived_subtotal_jpy": lowestSubtotal})
		}
		if len(rows) == 0 {
			return fmt.Errorf("all plan details failed: %v", errs)
		}
		sort.SliceStable(rows, func(i, j int) bool { return rows[i]["plan_id"].(string) < rows[j]["plan_id"].(string) })
		return ajOutput(cmd, flags, map[string]any{"constraints": map[string]any{"date": date, "adults": adults, "age": age, "max_jpy": maxJPY, "max_minutes": maxMinutes, "spoken_language": guideLang, "instant_only": instant}, "plans": rows, "errors": errs, "partial": len(errs) > 0, "truncated": false}, s.Requests+inventoryRequests, map[string]any{"coverage": "explicit shortlist only", "cache": "plan/price/session disabled; sitemap 24-hour bounded cache"})
	}}
	cmd.Flags().StringVar(&date, "date", "", "Date for session and quote comparison")
	cmd.Flags().StringVar(&lang, "lang", "en", "Website language for names: en or ja")
	cmd.Flags().IntVar(&adults, "adults", 0, "Adult participant count; 0 leaves party unknown")
	cmd.Flags().IntVar(&age, "age", -1, "Traveler age in years; -1 leaves age unknown")
	cmd.Flags().IntVar(&maxJPY, "max-jpy", 0, "Maximum derived party subtotal in JPY; requires date and adults")
	cmd.Flags().IntVar(&maxMinutes, "max-minutes", 0, "Maximum total experience minutes; only exact whole-field durations can match")
	cmd.Flags().StringVar(&guideLang, "spoken-language", "", "Required guide language; unknown unless the source explicitly names it")
	cmd.Flags().BoolVar(&instant, "instant-only", false, "Require an instant-confirmable observed session on --date")
	return cmd
}
func ajBrief(flags *rootFlags) *cobra.Command {
	var date, lang string
	var adults int
	cmd := &cobra.Command{Use: "brief <plan-id>", Short: "Get a compact plan, dated quote, session and booking handoff packet", Example: "  activity-japan-pp-cli experience brief 62375 --date 2026-10-08 --adults 2 --agent", Annotations: ajAnnotations("plan=62375;--date=2026-10-08;--adults=2"), RunE: func(cmd *cobra.Command, args []string) error {
		if e := ajInput(cmd, flags, args, 1, "experience brief"); e != nil || flags.dryRun {
			return e
		}
		id, e := ajID(args[0])
		if e != nil {
			return usageErr(e)
		}
		if e := activityjapan.ValidLang(lang); e != nil {
			return usageErr(e)
		}
		if e := activityjapan.ValidateDate(date); e != nil {
			return usageErr(e)
		}
		if e := ajPositive(adults, "--adults"); e != nil {
			return e
		}
		ctx, cancel := ajContext(cmd.Context(), flags)
		defer cancel()
		s, e := ajService(flags)
		if e != nil {
			return e
		}
		plan, e := s.Detail(ctx, id, lang)
		if e != nil {
			return e
		}
		if e := ajPlanParty(plan, adults); e != nil {
			return e
		}
		quotes, pe := s.Price(ctx, id, date, adults, plan.Options)
		sessions, se := s.Sessions(ctx, id, date)
		errs := append([]string{}, plan.Errors...)
		if pe != nil {
			errs = append(errs, "price: "+pe.Error())
		}
		if se != nil {
			errs = append(errs, "sessions: "+se.Error())
		}
		if pe != nil && se != nil && !ajPartialItems(pe, len(quotes)) && !ajPartialItems(se, len(sessions)) {
			return fmt.Errorf("dated price and sessions both failed: %v", errs)
		}
		canonicalURL, inventoryRequests, urlErr := ajIndexedURL(ctx, id, lang, flags)
		if urlErr != nil {
			errs = append(errs, "canonical handoff URL: "+urlErr.Error())
		}
		unresolved := append([]string{}, plan.Missing...)
		unresolved = append(unresolved, "selected_option", "mandatory_fees", "instructor_language")
		if canonicalURL == nil {
			unresolved = append(unresolved, "canonical_handoff_url")
		}
		if pe != nil && !ajPartialItems(pe, len(quotes)) {
			unresolved = append(unresolved, "selected_date_price")
		}
		if se != nil && !ajPartialItems(se, len(sessions)) {
			unresolved = append(unresolved, "date_specific_availability")
		}
		var sessionSummary any
		if se == nil || ajPartialItems(se, len(sessions)) {
			sessionSummary = ajSessionSummary(sessions)
		}
		shownSessions := []map[string]any{}
		for i, ss := range sessions {
			if i >= 3 {
				break
			}
			shownSessions = append(shownSessions, map[string]any{"session_id": ss.SessionID, "start_local": ss.StartLocal, "availability": ss.Availability})
		}
		shownQuotes, _, _ := ajPage(ajQuoteSummary(quotes), 1, 10)
		return ajOutput(cmd, flags, map[string]any{"plan_id": id, "name": plan.NameLocalized, "name_original_ja": plan.NameOriginalJA, "operator_id": plan.OperatorID, "date": date, "participants": adults, "venue_address": plan.VenueAddress, "meeting_point": plan.MeetingPoint, "pickup_location": plan.PickupLocation, "duration_total_text": plan.DurationTotalText, "derived_total_minutes": plan.DerivedTotalMinutes, "age_min_years": plan.AgeMinYears, "age_max_years": plan.AgeMaxYears, "party_min": plan.PartyMin, "party_max": plan.PartyMax, "option_prices": shownQuotes, "option_prices_total": len(quotes), "option_prices_truncated": len(quotes) > len(shownQuotes), "session_summary": sessionSummary, "first_sessions": shownSessions, "sessions_truncated": len(sessions) > len(shownSessions), "inclusion_text": plan.InclusionText, "restriction_text": plan.RestrictionText, "additional_attention_text": plan.AdditionalAttentionText, "cancellation_text": plan.CancellationText, "source_url": plan.SourceURL, "canonical_url": canonicalURL, "canonical_url_verified": canonicalURL != nil, "unresolved": unresolved, "errors": errs, "partial": len(errs) > 0, "reservation_confirmed": false}, s.Requests+inventoryRequests, map[string]any{"cache": "plan/price/session disabled; sitemap 24-hour bounded cache"})
	}}
	cmd.Flags().StringVar(&date, "date", "", "Activity date YYYY-MM-DD in Asia/Tokyo (required)")
	cmd.Flags().IntVar(&adults, "adults", 0, "Participant count, 1..50 (required)")
	cmd.Flags().StringVar(&lang, "lang", "en", "Website language: en or ja")
	return cmd
}
func ajLanguages(flags *rootFlags) *cobra.Command {
	var refresh bool
	cmd := &cobra.Command{Use: "languages <plan-id>", Short: "Check English and Japanese plan sitemap membership; bookability remains unchecked", Example: "  activity-japan-pp-cli inventory languages 62375 --agent", Annotations: ajAnnotations("plan=62375"), RunE: func(cmd *cobra.Command, args []string) error {
		if e := ajInput(cmd, flags, args, 1, "inventory languages"); e != nil || flags.dryRun {
			return e
		}
		id, e := ajID(args[0])
		if e != nil {
			return usageErr(e)
		}
		dir, e := cliutil.CacheDir()
		if e != nil {
			return e
		}
		ctx, cancel := ajContext(cmd.Context(), flags)
		defer cancel()
		presence, req, e := activityjapan.CheckLanguages(ctx, id, dir, flags.noCache, refresh)
		if e != nil {
			return e
		}
		return ajOutput(cmd, flags, presence, req, map[string]any{"cache": presence.Cache, "coverage": "two public plan sitemaps; index listing only"})
	}}
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Bypass the 24-hour sitemap cache and refetch both language indexes")
	return cmd
}

var _ = strconv.Itoa
