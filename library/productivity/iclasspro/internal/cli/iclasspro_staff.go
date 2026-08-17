// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call

// Read-only Office Portal commands. This file intentionally exposes only
// enumerated GET endpoints and POST-backed searches. It has no arbitrary
// request escape hatch and no create, update, delete, export, or report-run
// operation.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/iclasspro/internal/client"
)

const icpStaffAPIPath = "/api/v1"

var icpStaffReportCategories = map[string]bool{
	"families": true, "students": true, "classes": true, "camps": true,
	"staff": true, "leads": true, "financial": true, "marketing": true,
	"custom": true,
}

type icpStaffListFlags struct {
	query string
	from  string
	to    string
	page  int
	limit int
}

func icpStaffClient(flags *rootFlags) (*client.Client, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	// Office Portal responses can contain family, student, attendance, and
	// financial data. Keep them out of the generated client's disk cache.
	c.NoCache = true
	c.BaseURL = icpStaffOfficeBase + icpStaffAPIPath
	return c, nil
}

func icpStaffGet(ctx context.Context, c *client.Client, account, path string, params map[string]string) (json.RawMessage, error) {
	session, err := icpStaffSessionFor(account)
	if err != nil {
		return nil, err
	}
	raw, err := c.GetWithHeadersNoCache(ctx, path, params, icpStaffHeaders(account, session.Cookie))
	if err != nil {
		return nil, classifyAPIError(err, nil)
	}
	return raw, nil
}

func icpStaffPostQuery(ctx context.Context, c *client.Client, account, path string, body any) (json.RawMessage, error) {
	session, err := icpStaffSessionFor(account)
	if err != nil {
		return nil, err
	}
	raw, _, err := c.PostQueryWithParamsAndHeaders(ctx, path, nil, body, icpStaffHeaders(account, session.Cookie))
	if err != nil {
		return nil, classifyAPIError(err, nil)
	}
	return raw, nil
}

func icpPrintStaff(cmd *cobra.Command, flags *rootFlags, raw json.RawMessage) error {
	if len(strings.TrimSpace(string(raw))) == 0 {
		raw = json.RawMessage(`{}`)
	}
	return printOutputWithFlagsMeta(cmd.OutOrStdout(), raw, flags, map[string]any{
		"source": "live", "surface": "office-portal", "read_only": true,
	})
}

func icpStaffValidateListFlags(f icpStaffListFlags) error {
	if f.page < 1 {
		return fmt.Errorf("--page must be at least 1")
	}
	if f.limit < 1 || f.limit > 500 {
		return fmt.Errorf("--limit must be between 1 and 500")
	}
	if err := icpStaffValidateDate("--from", f.from); err != nil {
		return err
	}
	return icpStaffValidateDate("--to", f.to)
}

func icpStaffValidateDate(flag, value string) error {
	if value == "" {
		return nil
	}
	if _, err := time.Parse("2006-01-02", value); err != nil {
		return fmt.Errorf("%s must be YYYY-MM-DD", flag)
	}
	return nil
}

func icpStaffSafeSegment(name, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	for _, r := range value {
		if r != '-' && r != '_' && r != ':' && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return "", fmt.Errorf("invalid %s %q", name, value)
		}
	}
	return url.PathEscape(value), nil
}

func icpStaffSearchFilters(f icpStaffListFlags) map[string]any {
	filters := map[string]any{
		"page": f.page, "pageSize": f.limit,
	}
	if f.query != "" {
		filters["searchString"] = f.query
		filters["search"] = f.query
	}
	if f.from != "" {
		filters["startDate"] = f.from
	}
	if f.to != "" {
		filters["endDate"] = f.to
	}
	return filters
}

func icpStaffDefaultFilters(raw json.RawMessage) map[string]any {
	var decoded map[string]any
	if json.Unmarshal(raw, &decoded) != nil {
		return map[string]any{}
	}
	if data, ok := decoded["data"].(map[string]any); ok {
		if filters, ok := data["filters"].(map[string]any); ok {
			return filters
		}
		return data
	}
	if filters, ok := decoded["filters"].(map[string]any); ok {
		return filters
	}
	return decoded
}

func icpStaffMergedFilters(defaults map[string]any, f icpStaffListFlags) map[string]any {
	merged := make(map[string]any, len(defaults)+6)
	for key, value := range defaults {
		merged[key] = value
	}
	for key, value := range icpStaffSearchFilters(f) {
		merged[key] = value
	}
	return merged
}

func icpBindStaffListFlags(cmd *cobra.Command, f *icpStaffListFlags, dates bool) {
	cmd.Flags().StringVar(&f.query, "q", "", "Search text")
	if dates {
		cmd.Flags().StringVar(&f.from, "from", "", "Start date (YYYY-MM-DD)")
		cmd.Flags().StringVar(&f.to, "to", "", "End date (YYYY-MM-DD)")
	}
	cmd.Flags().IntVar(&f.page, "page", 1, "Result page (1-based)")
	cmd.Flags().IntVar(&f.limit, "limit", 100, "Rows per page (1-500)")
}

func newIclassproAdminCmd(flags *rootFlags) *cobra.Command {
	admin := &cobra.Command{
		Use:   "admin",
		Short: "Read authenticated Office Portal data without mutation or export",
		Long: "Read-only access to explicitly supported Office Portal resources. " +
			"Requires 'auth staff-login'. No arbitrary request, write, report generation, or export command is provided.",
		RunE: parentNoSubcommandRunE(flags),
	}
	admin.AddCommand(
		newIclassproAdminCapabilitiesCmd(flags),
		newIclassproAdminDashboardCmd(flags),
		newIclassproAdminFamiliesCmd(flags),
		newIclassproAdminStudentsCmd(flags),
		newIclassproAdminClassSearchCmd(flags),
		newIclassproAdminEnrollmentsCmd(flags),
		newIclassproAdminAttendanceCmd(flags),
		newIclassproAdminTransactionsCmd(flags),
		newIclassproAdminReportsCmd(flags),
	)
	return admin
}

func newIclassproAdminDashboardCmd(flags *rootFlags) *cobra.Command {
	var locationID string
	cmd := &cobra.Command{
		Use:         "dashboard <account>",
		Short:       "Read the saved dashboard and available widget catalog",
		Example:     "  iclasspro-pp-cli admin dashboard examplegym --location-id 1 --json",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:requires-tier": "staff", "pp:endpoint": "staff.dashboard"},
		RunE: func(cmd *cobra.Command, args []string) error {
			account, err := icpStaffAccount(args[0])
			if err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "admin dashboard")
			}
			c, err := icpStaffClient(flags)
			if err != nil {
				return err
			}
			params := map[string]string{}
			if locationID != "" {
				params["locationId"] = locationID
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			dashboard, err := icpStaffGet(ctx, c, account, "/dashboard/get", params)
			if err != nil {
				return err
			}
			widgets, err := icpStaffGet(ctx, c, account, "/dashboard/get-widgets", params)
			if err != nil {
				return err
			}
			var dashboardValue, widgetsValue any
			_ = json.Unmarshal(dashboard, &dashboardValue)
			_ = json.Unmarshal(widgets, &widgetsValue)
			result := map[string]any{"dashboard": dashboardValue, "available_widgets": widgetsValue}
			savedWidgets := icpStaffDashboardWidgets(dashboardValue)
			if len(savedWidgets) > 0 {
				locationFilter := make([]string, 0, 1)
				if locationID != "" {
					locationFilter = append(locationFilter, locationID)
				}
				widgetData, requestErr := icpStaffPostQuery(ctx, c, account, "/dashboard/get-data", map[string]any{
					"widgets": savedWidgets,
					"filters": map[string]any{"locationId": locationFilter},
				})
				if requestErr != nil {
					return requestErr
				}
				var widgetDataValue any
				_ = json.Unmarshal(widgetData, &widgetDataValue)
				result["widget_data"] = widgetDataValue
			}
			raw, err := json.Marshal(result)
			if err != nil {
				return err
			}
			return icpPrintStaff(cmd, flags, raw)
		},
	}
	cmd.Flags().StringVar(&locationID, "location-id", "", "Optional Office Portal location ID")
	return cmd
}

func icpStaffDashboardWidgets(value any) []map[string]any {
	var found []map[string]any
	var walk func(any)
	walk = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			for key, child := range typed {
				if strings.EqualFold(key, "widgets") {
					if list, ok := child.([]any); ok {
						for _, item := range list {
							if widget, ok := item.(map[string]any); ok {
								found = append(found, widget)
							}
						}
					}
				}
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(value)
	return found
}

func newIclassproAdminFamiliesCmd(flags *rootFlags) *cobra.Command {
	var f icpStaffListFlags
	cmd := &cobra.Command{
		Use: "families <account>", Short: "Search families", Args: cobra.ExactArgs(1),
		Example:     "  iclasspro-pp-cli admin families examplegym --q smith --limit 25 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:requires-tier": "staff", "pp:endpoint": "staff.families.search"},
		RunE: icpStaffListRunE(flags, &f, "/get-family-filter-data", "/get-family-list/", func(f icpStaffListFlags, defaults map[string]any) any {
			return map[string]any{"filters": icpStaffMergedFilters(defaults, f), "sortable": map[string]any{}, "page": f.page, "pageSize": f.limit}
		}),
	}
	icpBindStaffListFlags(cmd, &f, false)
	return cmd
}

func newIclassproAdminStudentsCmd(flags *rootFlags) *cobra.Command {
	var f icpStaffListFlags
	cmd := &cobra.Command{
		Use: "students <account>", Short: "Search students", Args: cobra.ExactArgs(1),
		Example:     "  iclasspro-pp-cli admin students examplegym --q jordan --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:requires-tier": "staff", "pp:endpoint": "staff.students.search"},
		RunE: icpStaffListRunE(flags, &f, "/get-student-filter-data", "/students-list", func(f icpStaffListFlags, defaults map[string]any) any {
			return map[string]any{"filters": icpStaffMergedFilters(defaults, f), "page": f.page, "pageSize": f.limit}
		}),
	}
	icpBindStaffListFlags(cmd, &f, false)
	return cmd
}

func newIclassproAdminClassSearchCmd(flags *rootFlags) *cobra.Command {
	var f icpStaffListFlags
	var status, date string
	cmd := &cobra.Command{
		Use: "class-search <account>", Short: "Search Office Portal classes", Args: cobra.ExactArgs(1),
		Example:     "  iclasspro-pp-cli admin class-search examplegym --status active --date 2026-08-12 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:requires-tier": "staff", "pp:endpoint": "staff.classes.search"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := icpStaffValidateDate("--date", date); err != nil {
				return usageErr(err)
			}
			return icpStaffListRunE(flags, &f, "/get-class-filter-data", "/class-data", func(f icpStaffListFlags, defaults map[string]any) any {
				filters := icpStaffMergedFilters(defaults, f)
				if status != "" {
					filters["status"] = status
				}
				if date != "" {
					filters["startDate"], filters["endDate"] = date, date
				}
				return map[string]any{"filters": filters, "page": f.page, "pageSize": f.limit}
			})(cmd, args)
		},
	}
	icpBindStaffListFlags(cmd, &f, false)
	cmd.Flags().StringVar(&status, "status", "", "Class status filter")
	cmd.Flags().StringVar(&date, "date", "", "Class date (YYYY-MM-DD)")
	return cmd
}

func newIclassproAdminEnrollmentsCmd(flags *rootFlags) *cobra.Command {
	var f icpStaffListFlags
	cmd := &cobra.Command{
		Use: "enrollments <account>", Short: "Search enrollments", Args: cobra.ExactArgs(1),
		Example:     "  iclasspro-pp-cli admin enrollments examplegym --from 2026-08-01 --to 2026-08-31 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:requires-tier": "staff", "pp:endpoint": "staff.enrollments.search"},
		RunE: icpStaffListRunE(flags, &f, "/get-enrollment-filter-data", "/enrollments/filtered", func(f icpStaffListFlags, defaults map[string]any) any {
			return map[string]any{"filters": icpStaffMergedFilters(defaults, f)}
		}),
	}
	icpBindStaffListFlags(cmd, &f, true)
	return cmd
}

func newIclassproAdminTransactionsCmd(flags *rootFlags) *cobra.Command {
	var f icpStaffListFlags
	cmd := &cobra.Command{
		Use: "transactions <account>", Short: "Search gateway transaction history", Args: cobra.ExactArgs(1),
		Example:     "  iclasspro-pp-cli admin transactions examplegym --from 2026-08-01 --to 2026-08-31 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:requires-tier": "staff", "pp:endpoint": "staff.transactions.search"},
		RunE: icpStaffListRunE(flags, &f, "/get-gateway-transaction-filter-data", "/transaction-history/search", func(f icpStaffListFlags, defaults map[string]any) any {
			return map[string]any{"filters": icpStaffMergedFilters(defaults, f)}
		}),
	}
	icpBindStaffListFlags(cmd, &f, true)
	return cmd
}

func icpStaffListRunE(flags *rootFlags, f *icpStaffListFlags, filterPath, path string, body func(icpStaffListFlags, map[string]any) any) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		account, err := icpStaffAccount(args[0])
		if err != nil {
			return usageErr(err)
		}
		if err := icpStaffValidateListFlags(*f); err != nil {
			return usageErr(err)
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "admin "+cmd.Name())
		}
		c, err := icpStaffClient(flags)
		if err != nil {
			return err
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		defaults := map[string]any{}
		if filterPath != "" {
			filterRaw, requestErr := icpStaffGet(ctx, c, account, filterPath, nil)
			if requestErr != nil {
				return requestErr
			}
			defaults = icpStaffDefaultFilters(filterRaw)
		}
		raw, err := icpStaffPostQuery(ctx, c, account, path, body(*f, defaults))
		if err != nil {
			return err
		}
		return icpPrintStaff(cmd, flags, raw)
	}
}

func newIclassproAdminAttendanceCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "attendance <account> <class-id> <date> [timeslot-id]",
		Short:       "Read a class roster and attendance state for one date",
		Example:     "  iclasspro-pp-cli admin attendance examplegym 12345 2026-08-12 --json\n  iclasspro-pp-cli admin attendance examplegym 12345 2026-08-12 67890 --json",
		Args:        cobra.RangeArgs(3, 4),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:requires-tier": "staff", "pp:endpoint": "staff.attendance.read"},
		RunE: func(cmd *cobra.Command, args []string) error {
			account, err := icpStaffAccount(args[0])
			if err != nil {
				return usageErr(err)
			}
			classID, err := icpStaffSafeSegment("class-id", args[1])
			if err != nil {
				return usageErr(err)
			}
			if err := icpStaffValidateDate("date", args[2]); err != nil {
				return usageErr(err)
			}
			timeslotID := ""
			if len(args) == 4 {
				timeslotID, err = icpStaffSafeSegment("timeslot-id", args[3])
				if err != nil {
					return usageErr(err)
				}
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "admin attendance")
			}
			c, err := icpStaffClient(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if timeslotID == "" {
				timeslotID, err = icpStaffResolveTimeslotID(ctx, c, account, classID, args[2])
				if err != nil {
					return err
				}
			}
			path := "/roster/classes/" + classID + "/" + args[2] + "/" + timeslotID
			raw, err := icpStaffGet(ctx, c, account, path, nil)
			if err != nil {
				return err
			}
			return icpPrintStaff(cmd, flags, raw)
		},
	}
}

type icpStaffScheduleItem struct {
	Date       string          `json:"date"`
	TimeslotID json.RawMessage `json:"tsId"`
}

func icpStaffResolveTimeslotID(ctx context.Context, c *client.Client, account, classID, date string) (string, error) {
	raw, err := icpStaffGet(ctx, c, account, "/schedule/"+classID+"/class/"+date+"/"+date, nil)
	if err != nil {
		return "", fmt.Errorf("discover attendance timeslot: %w", err)
	}

	var envelope struct {
		Data []icpStaffScheduleItem `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return "", fmt.Errorf("discover attendance timeslot: decode schedule response: %w", err)
	}

	seen := map[string]struct{}{}
	for _, item := range envelope.Data {
		if item.Date != date {
			continue
		}
		id, ok := icpStaffScheduleTimeslotID(item.TimeslotID)
		if ok {
			seen[id] = struct{}{}
		}
	}

	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	switch len(ids) {
	case 0:
		return "", fmt.Errorf("no attendance timeslot found for class %s on %s", classID, date)
	case 1:
		return ids[0], nil
	default:
		return "", fmt.Errorf("multiple attendance timeslots found for class %s on %s (%s); rerun with one timeslot-id as the fourth argument", classID, date, strings.Join(ids, ", "))
	}
}

func icpStaffScheduleTimeslotID(raw json.RawMessage) (string, bool) {
	value := strings.TrimSpace(string(raw))
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		var decoded string
		if json.Unmarshal(raw, &decoded) != nil {
			return "", false
		}
		value = decoded
	}
	value, err := icpStaffSafeSegment("timeslot-id", value)
	return value, err == nil
}

func newIclassproAdminReportsCmd(flags *rootFlags) *cobra.Command {
	var category string
	cmd := &cobra.Command{
		Use:         "reports <account>",
		Short:       "List report definitions; never generate or export a report",
		Example:     "  iclasspro-pp-cli admin reports examplegym --category financial --json",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:requires-tier": "staff", "pp:endpoint": "staff.reports.list"},
		RunE: func(cmd *cobra.Command, args []string) error {
			account, err := icpStaffAccount(args[0])
			if err != nil {
				return usageErr(err)
			}
			category = strings.ToLower(strings.TrimSpace(category))
			if !icpStaffReportCategories[category] {
				return usageErr(fmt.Errorf("invalid --category %q; choose families, students, classes, camps, staff, leads, financial, marketing, or custom", category))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "admin reports")
			}
			c, err := icpStaffClient(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			raw, err := icpStaffGet(ctx, c, account, "/reports/"+category, nil)
			if err != nil {
				return err
			}
			return icpPrintStaff(cmd, flags, raw)
		},
	}
	cmd.Flags().StringVar(&category, "category", "families", "Report category")
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newIclassproAdminCmd(flags))
	})
}
