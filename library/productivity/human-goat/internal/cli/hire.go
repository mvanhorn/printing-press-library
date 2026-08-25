// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/human-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/human-goat/internal/pricing"
	"github.com/mvanhorn/printing-press-library/library/productivity/human-goat/internal/source/taskrabbit"

	"github.com/spf13/cobra"
)

func newNovelHireCmd(flags *rootFlags) *cobra.Command {
	var flagOn string
	var flagMinRating string
	var flagMaxTotal string
	var flagLat float64
	var flagLng float64
	var flagState string
	var flagAddressJSON string
	var flagEndAddressJSON string
	var flagNote string

	cmd := &cobra.Command{
		Use:     "hire <job-query>",
		Short:   "Say the job and the date; goat searches, ranks by review quality and honest all-in price",
		Example: "human-goat-pp-cli hire \"help moving\" --on 2026-07-11 --lat 37.7749 --lng -122.4194 --state CA --min-rating 4.8 --max-total 200 --dry-run --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !commandHasChangedFlags(cmd) {
				return cmd.Help()
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				return usageErr(fmt.Errorf("missing job-query"))
			}

			primaryAddress, hasPrimaryAddress, err := parseTaskRabbitAddressFlag("--address-json", flagAddressJSON)
			if err != nil {
				return usageErr(err)
			}
			endAddress, hasEndAddress, err := parseTaskRabbitAddressFlag("--end-address-json", flagEndAddressJSON)
			if err != nil {
				return usageErr(err)
			}
			realCommit := !dryRunOK(flags) && !cliutil.IsVerifyEnv() && !cliutil.IsDogfoodEnv()
			if realCommit && !hasPrimaryAddress {
				return usageErr(fmt.Errorf("--address-json is required to book (a geocoded address incl metro_id); use --dry-run to preview without it"))
			}
			if hasPrimaryAddress {
				if err := validateTaskRabbitAddress("--address-json", *primaryAddress); err != nil {
					return usageErr(err)
				}
			}
			if hasEndAddress {
				if err := validateTaskRabbitAddress("--end-address-json", *endAddress); err != nil {
					return usageErr(err)
				}
			}

			lat := flagLat
			lng := flagLng
			if hasPrimaryAddress {
				lat = primaryAddress.Lat
				lng = primaryAddress.Lng
			}
			if !hasPrimaryAddress && (!cmd.Flags().Changed("lat") || !cmd.Flags().Changed("lng")) {
				return usageErr(fmt.Errorf("pass --lat and --lng for your location"))
			}

			date, err := parseOnDate(flagOn)
			if err != nil {
				return usageErr(err)
			}
			minRating, err := parseOptionalFloatFlag("--min-rating", flagMinRating)
			if err != nil {
				return usageErr(err)
			}
			maxTotal, err := parseOptionalFloatFlag("--max-total", flagMaxTotal)
			if err != nil {
				return usageErr(err)
			}
			if minRating < 0 {
				return usageErr(fmt.Errorf("--min-rating must be non-negative"))
			}
			if maxTotal < 0 {
				return usageErr(fmt.Errorf("--max-total must be non-negative"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				c.DryRun = false
			}
			category, err := resolveTaskRabbitCategory(ctx, c, query)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			tr := taskrabbit.New(c)
			taskers, _, recommendationID, err := tr.Recommendations(ctx, taskrabbit.BuildRecommendationsInput(category.CategoryID, category.DefaultTemplateID, lat, lng, []string{date}))
			if err != nil {
				return classifyAPIError(err, flags)
			}

			best, ok := selectHireTasker(taskers, minRating, flagState)
			if !ok {
				return fmt.Errorf("no Tasker matches --min-rating")
			}

			inviteeID, notes := hireInviteeID(best)
			availableDates, err := tr.Schedule(ctx, category.CategoryID, inviteeID, "en-US", lat, lng, category.DefaultTemplateID)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			slotDate, slotLabel, durationSeconds, offsetSeconds, availabilityNote := firstTaskRabbitSlot(availableDates, date)
			if availabilityNote != "" {
				notes = append(notes, availabilityNote)
			}
			// firstTaskRabbitSlot silently falls back to another day when the
			// requested date has no availability; warn in the preview too so the
			// dry-run summary never hides a date substitution before checkout.
			if slotDate != "" && date != "" && slotDate != date {
				notes = append(notes, fmt.Sprintf("requested %s was unavailable; would use %s instead", date, slotDate))
			}

			allInHourly := pricing.AllIn(best.PosterHourlyRateCents, flagState).AllInCents
			// Estimate the booking total from the actual slot duration rather than a
			// flat 2-hour guess, so the spend cap does not reject in-budget bookings.
			// Use float hours so a fractional slot (e.g. 1.5h) is not truncated by
			// integer division, floor at 1 hour, honor the Tasker's 2-hour minimum,
			// and round the estimate UP so the cap never underestimates the charge.
			estHours := float64(durationSeconds) / 3600.0
			if estHours < 1 {
				estHours = 1
			}
			if taskerRequiresTwoHourMinimum(best) && estHours < 2 {
				estHours = 2
			}
			totalCents := int(math.Ceil(float64(allInHourly) * estHours))
			withinCap := maxTotal == 0 || float64(totalCents)/100.0 <= maxTotal
			if maxTotal > 0 && !withinCap {
				fmt.Fprintf(cmd.ErrOrStderr(), "refusing: all-in total %s exceeds cap %s\n", pricing.FormatCents(totalCents), formatDollarCap(maxTotal))
				return fmt.Errorf("spend cap exceeded")
			}

			summary := hireConfirmSummary{
				Tasker:             taskerDisplayName(best),
				AllInHourly:        pricing.FormatCents(allInHourly),
				AllInTotalEstimate: pricing.FormatCents(totalCents),
				Rating:             best.RabbitRating,
				Date:               slotDate,
				SlotLabel:          slotLabel,
				WithinCap:          withinCap,
				Note:               strings.Join(notes, "; "),
			}
			if dryRunOK(flags) || cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() {
				return printHireConfirmSummary(cmd, flags, summary)
			}

			// firstTaskRabbitSlot only sets availabilityNote when no day had any
			// open slot (it then fabricates a 1-hour placeholder). Never commit a
			// real checkout against a slot the Schedule API did not actually offer.
			if availabilityNote != "" {
				return fmt.Errorf("no available slot for %q around %s; not booking (try a different date)", query, date)
			}

			if recommendationID == "" {
				return fmt.Errorf("TaskRabbit recommendations response did not include recommendation_id")
			}
			if best.UserIDInt == 0 {
				return fmt.Errorf("TaskRabbit recommendation for %s did not include numeric user_id", taskerDisplayName(best))
			}

			token, err := csrfToken(ctx, flags)
			if err != nil {
				return classifyAPIError(fmt.Errorf("hire TaskRabbit booking: get CSRF token: %w", err), flags)
			}

			categoryName := taskRabbitCategoryName(category)
			input := taskrabbit.HireInput{
				Source:                  "recommendation",
				JobType:                 "Template",
				FixedRate:               false,
				SecondsBetween:          "0",
				ShownCancellationPolicy: true,
				TaskTemplateID:          category.DefaultTemplateID,
				CategoryID:              category.CategoryID,
				CategoryName:            categoryName,
				Title:                   categoryName,
				MarketingGroupID:        15,
				FunnelID:                taskrabbit.SynthFunnelID(),
				SessionID:               taskrabbit.SynthSessionID(),
				RecommendationID:        recommendationID,
				InviteeID:               best.UserIDInt,
				RabbitID:                best.UserIDInt,
				PosterHourlyRateCents:   best.PosterHourlyRateCents,
				JobDraftGUID:            "",
				FormReferrer:            "",
				JobSize:                 "small",
				Description:             hireDescription(flagNote, query),
				Schedule: taskrabbit.DateSpec{
					Date:            slotDate,
					DurationSeconds: durationSeconds,
					OffsetSeconds:   offsetSeconds,
				},
				Address: *primaryAddress,
			}
			if hasEndAddress {
				input.SecondaryLocation = endAddress
			}

			body, err := tr.Hire(ctx, input, token)
			if err != nil {
				return classifyAPIError(fmt.Errorf("hire TaskRabbit booking: %w", err), flags)
			}

			// Prefer a confirmed charge from the hire response; fall back to the
			// pre-checkout estimate and say so, so all_in_total is never presented
			// as a settled amount when it is only an estimate.
			allInTotal := pricing.FormatCents(totalCents)
			totalConfirmed := false
			if confirmedCents, ok := hireConfirmedTotalCents(body); ok {
				allInTotal = pricing.FormatCents(confirmedCents)
				totalConfirmed = true
			}
			booked := hireBookedResult{
				Booked:         true,
				Tasker:         taskerDisplayName(best),
				AllInTotal:     allInTotal,
				TotalConfirmed: totalConfirmed,
				JobID:          hireJobID(body),
				RequestedDate:  date,
				Date:           slotDate,
				SlotLabel:      slotLabel,
			}
			if !totalConfirmed {
				booked.Note = joinNotes(booked.Note, "all_in_total is a pre-checkout estimate; confirm the final charge on the TaskRabbit invoice")
			}
			// Surface a date change loudly: firstTaskRabbitSlot falls back to
			// another day when the requested date has no availability, and a
			// silent booking on an unrequested date is exactly what an
			// autonomous checkout must never hide.
			if slotDate != "" && date != "" && slotDate != date {
				booked.Note = fmt.Sprintf("requested %s was unavailable; booked %s instead", date, slotDate)
			}
			return printHireBookedResult(cmd, flags, booked)
		},
	}
	cmd.Flags().StringVar(&flagOn, "on", "", "Date to book: YYYY-MM-DD, today, tomorrow, or weekday")
	cmd.Flags().StringVar(&flagMinRating, "min-rating", "", "Minimum Tasker rating")
	cmd.Flags().StringVar(&flagMaxTotal, "max-total", "", "Maximum all-in booking total in dollars (0 for no cap)")
	cmd.Flags().Float64Var(&flagLat, "lat", 0, "Latitude for TaskRabbit recommendations")
	cmd.Flags().Float64Var(&flagLng, "lng", 0, "Longitude for TaskRabbit recommendations")
	cmd.Flags().StringVar(&flagState, "state", "", "State for CA/MA service-fee-only pricing rule")
	cmd.Flags().StringVar(&flagAddressJSON, "address-json", "", "Geocoded TaskRabbit address JSON for checkout")
	cmd.Flags().StringVar(&flagEndAddressJSON, "end-address-json", "", "Optional secondary TaskRabbit address JSON for checkout")
	cmd.Flags().StringVar(&flagNote, "note", "", "Task description sent during checkout")
	return cmd
}

type hireConfirmSummary struct {
	Tasker             string  `json:"tasker"`
	AllInHourly        string  `json:"all_in_hourly"`
	AllInTotalEstimate string  `json:"all_in_total_estimate"`
	Rating             float64 `json:"rating"`
	Date               string  `json:"date"`
	SlotLabel          string  `json:"slot_label"`
	WithinCap          bool    `json:"within_cap"`
	Note               string  `json:"note,omitempty"`
}

type hireBookedResult struct {
	Booked         bool   `json:"booked"`
	Tasker         string `json:"tasker"`
	AllInTotal     string `json:"all_in_total"`
	TotalConfirmed bool   `json:"total_confirmed"`
	JobID          string `json:"job_id,omitempty"`
	RequestedDate  string `json:"requested_date,omitempty"`
	Date           string `json:"date,omitempty"`
	SlotLabel      string `json:"slot_label,omitempty"`
	Note           string `json:"note,omitempty"`
}

func parseOptionalFloatFlag(name, value string) (float64, error) {
	clean := strings.TrimSpace(value)
	clean = strings.TrimPrefix(clean, "$")
	if clean == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a number", name)
	}
	return parsed, nil
}

func parseTaskRabbitAddressFlag(name, value string) (*taskrabbit.Address, bool, error) {
	clean := strings.TrimSpace(value)
	if clean == "" {
		return nil, false, nil
	}
	var address taskrabbit.Address
	if err := json.Unmarshal([]byte(clean), &address); err != nil {
		return nil, false, fmt.Errorf("%s must be a TaskRabbit address JSON object: %w", name, err)
	}
	return &address, true, nil
}

func validateTaskRabbitAddress(name string, address taskrabbit.Address) error {
	missing := make([]string, 0)
	if strings.TrimSpace(address.Address1) == "" {
		missing = append(missing, "address1")
	}
	if strings.TrimSpace(address.Country) == "" {
		missing = append(missing, "country")
	}
	if strings.TrimSpace(address.FormattedAddress) == "" {
		missing = append(missing, "formatted_address")
	}
	if address.Lat == 0 {
		missing = append(missing, "lat")
	}
	if address.Lng == 0 {
		missing = append(missing, "lng")
	}
	if strings.TrimSpace(address.Locality) == "" {
		missing = append(missing, "locality")
	}
	if address.MetroID == 0 {
		missing = append(missing, "metro_id")
	}
	if strings.TrimSpace(address.MetroName) == "" {
		missing = append(missing, "metro_name")
	}
	if strings.TrimSpace(address.PostalCode) == "" {
		missing = append(missing, "postal_code")
	}
	if strings.TrimSpace(address.Region) == "" {
		missing = append(missing, "region")
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s missing required fields: %s", name, strings.Join(missing, ", "))
	}
	return nil
}

func selectHireTasker(taskers []taskrabbit.Tasker, minRating float64, state string) (taskrabbit.Tasker, bool) {
	candidates := make([]taskrabbit.Tasker, 0, len(taskers))
	for _, tasker := range taskers {
		if minRating > 0 && tasker.RabbitRating < minRating {
			continue
		}
		candidates = append(candidates, tasker)
	}
	if len(candidates) == 0 {
		return taskrabbit.Tasker{}, false
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left := pricing.AllIn(candidates[i].PosterHourlyRateCents, state).AllInCents
		right := pricing.AllIn(candidates[j].PosterHourlyRateCents, state).AllInCents
		if left != right {
			return left < right
		}
		return candidates[i].RabbitRating > candidates[j].RabbitRating
	})
	return candidates[0], true
}

func hireInviteeID(tasker taskrabbit.Tasker) (int, []string) {
	if tasker.UserIDInt > 0 {
		return tasker.UserIDInt, make([]string, 0)
	}
	return taskerInviteeID(tasker.ID)
}

func taskerInviteeID(id string) (int, []string) {
	clean := strings.TrimSpace(id)
	if parsed, err := strconv.Atoi(clean); err == nil {
		return parsed, make([]string, 0)
	}
	// Recommendation ids look like "profile_28810417"; the invitee id is the
	// trailing numeric run.
	digits := clean
	if i := strings.LastIndexByte(clean, '_'); i >= 0 {
		digits = clean[i+1:]
	}
	if parsed, err := strconv.Atoi(digits); err == nil {
		return parsed, make([]string, 0)
	}
	return 0, []string{fmt.Sprintf("tasker id %q has no numeric invitee id; using invitee_id=0", clean)}
}

// taskerRequiresTwoHourMinimum reports whether the recommendation's
// two_hour_minimum_required_display field indicates a 2-hour booking minimum.
func taskerRequiresTwoHourMinimum(t taskrabbit.Tasker) bool {
	switch v := t.TwoHourMinimum.(type) {
	case bool:
		return v
	case string:
		s := strings.TrimSpace(strings.ToLower(v))
		return s != "" && s != "false" && s != "0" && s != "no"
	case float64:
		return v != 0
	case map[string]any:
		return len(v) > 0
	default:
		return false
	}
}

func firstTaskRabbitSlot(availableDates []taskrabbit.AvailableDate, requestedDate string) (string, string, int, int, string) {
	for _, day := range availableDates {
		if day.Date == requestedDate && len(day.Slots) > 0 {
			return day.Date, day.Slots[0].SelectLabel, slotDurationSeconds(day.Slots[0]), day.Slots[0].OffsetSeconds, ""
		}
	}
	for _, day := range availableDates {
		if len(day.Slots) > 0 {
			return day.Date, day.Slots[0].SelectLabel, slotDurationSeconds(day.Slots[0]), day.Slots[0].OffsetSeconds, ""
		}
	}
	return requestedDate, "", 3600, 0, fmt.Sprintf("no availability on %s", requestedDate)
}

func slotDurationSeconds(slot taskrabbit.Slot) int {
	if slot.DurationSeconds > 0 {
		return slot.DurationSeconds
	}
	return 3600
}

func taskRabbitCategoryName(category taskrabbitCategoryMatch) string {
	name := strings.TrimSpace(category.CategoryName)
	if name != "" {
		return name
	}
	return strings.TrimSpace(category.Title)
}

func hireDescription(note, query string) string {
	description := strings.TrimSpace(note)
	if description != "" {
		return description
	}
	return fmt.Sprintf("Need help with %s.", strings.TrimSpace(query))
}

// hireConfirmedTotalCents extracts a settled charge from the hire response when
// TaskRabbit returns one, so the booked result can report a confirmed total
// rather than the pre-checkout estimate.
func hireConfirmedTotalCents(raw json.RawMessage) (int, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return 0, false
	}
	for _, key := range []string{"all_in_total_cents", "total_charged_cents", "charge_total_cents", "invoice_total_cents", "total_cents", "amount_cents"} {
		if cents, ok := findIntFieldByKey(decoded, key); ok && cents > 0 {
			return cents, true
		}
	}
	return 0, false
}

// findIntFieldByKey recursively finds the first integer value for key.
func findIntFieldByKey(value any, key string) (int, bool) {
	switch v := value.(type) {
	case map[string]any:
		if item, ok := v[key]; ok {
			switch n := item.(type) {
			case float64:
				return int(n), true
			case json.Number:
				if i, err := n.Int64(); err == nil {
					return int(i), true
				}
			}
		}
		for _, item := range v {
			if i, ok := findIntFieldByKey(item, key); ok {
				return i, true
			}
		}
	case []any:
		for _, item := range v {
			if i, ok := findIntFieldByKey(item, key); ok {
				return i, true
			}
		}
	}
	return 0, false
}

func hireJobID(raw json.RawMessage) string {
	keys := []string{"job_id", "jobId", "booking_id", "bookingId", "task_id", "taskId", "id"}
	return findJSONID(raw, keys)
}

func findJSONID(raw json.RawMessage, keys []string) string {
	if len(raw) == 0 {
		return ""
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) == nil && obj != nil {
		for _, key := range keys {
			if value, ok := obj[key]; ok {
				if id := jsonIDScalar(value); id != "" {
					return id
				}
			}
		}
		for _, value := range obj {
			if id := findJSONID(value, keys); id != "" {
				return id
			}
		}
		return ""
	}
	var arr []json.RawMessage
	if json.Unmarshal(raw, &arr) == nil {
		for _, value := range arr {
			if id := findJSONID(value, keys); id != "" {
				return id
			}
		}
	}
	return ""
}

func jsonIDScalar(raw json.RawMessage) string {
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return v.String()
	default:
		return ""
	}
}

func formatDollarCap(value float64) string {
	return fmt.Sprintf("$%.2f", value)
}

func printHireConfirmSummary(cmd *cobra.Command, flags *rootFlags, summary hireConfirmSummary) error {
	if flags.asJSON || flags.agent {
		return printJSONFiltered(cmd.OutOrStdout(), summary, flags)
	}
	rows := [][]string{
		{"Tasker", summary.Tasker},
		{"All-in hourly", summary.AllInHourly},
		{"All-in total estimate", summary.AllInTotalEstimate},
		{"Rating", fmt.Sprintf("%.2f", summary.Rating)},
		{"Date", summary.Date},
		{"Slot", summary.SlotLabel},
		{"Within cap", fmt.Sprintf("%t", summary.WithinCap)},
	}
	if summary.Note != "" {
		rows = append(rows, []string{"Note", summary.Note})
	}
	return flags.printTable(cmd, []string{"FIELD", "VALUE"}, rows)
}

func printHireBookedResult(cmd *cobra.Command, flags *rootFlags, result hireBookedResult) error {
	if flags.asJSON || flags.agent {
		return printJSONFiltered(cmd.OutOrStdout(), result, flags)
	}
	rows := [][]string{
		{"Booked", fmt.Sprintf("%t", result.Booked)},
		{"Tasker", result.Tasker},
		{"All-in total", result.AllInTotal},
	}
	if result.AllInTotal != "" {
		confirmed := "estimate"
		if result.TotalConfirmed {
			confirmed = "confirmed"
		}
		rows = append(rows, []string{"Total basis", confirmed})
	}
	if result.Date != "" {
		rows = append(rows, []string{"Date", result.Date})
	}
	if result.SlotLabel != "" {
		rows = append(rows, []string{"Slot", result.SlotLabel})
	}
	if result.JobID != "" {
		rows = append(rows, []string{"Job ID", result.JobID})
	}
	if result.Note != "" {
		rows = append(rows, []string{"Note", result.Note})
	}
	return flags.printTable(cmd, []string{"FIELD", "VALUE"}, rows)
}
