package cli

// pp:data-source auto

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	nethtml "golang.org/x/net/html"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/fooda/internal/client"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/fooda/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/fooda/internal/store"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(rootCmd *cobra.Command, flags *rootFlags) {
		// Helper to replace an existing command of the same name
		replaceCommand := func(newCmd *cobra.Command) {
			for _, existing := range rootCmd.Commands() {
				if existing.Name() == newCmd.Name() {
					rootCmd.RemoveCommand(existing)
					break
				}
			}
			rootCmd.AddCommand(newCmd)
		}

		// 1. events
		eventsCmd := &cobra.Command{
			Use:   "events",
			Short: "Live list of events with restaurants",
			Example: strings.Trim(`
  # List all events for the next 7 days
  fooda-pp-cli events

  # List events for a specific date
  fooda-pp-cli events --date 2026-10-12

  # List only popup events from a specific date range
  fooda-pp-cli events --from 2026-10-12 --to 2026-10-18 --type popup
`, "\n"),
			Annotations: map[string]string{"pp:data-source": "live", "mcp:read-only": "true"},
			RunE: func(cmd *cobra.Command, args []string) error {
				if flags.dataSource == "local" {
					return fmt.Errorf("events is live-only; --data-source local is not supported")
				}

				if dryRunOK(flags) {
					return writeDryRun(cmd.OutOrStdout(), flags, "events")
				}

				ctx, cancel := boundCtx(cmd.Context(), flags)
				defer cancel()

				c, err := flags.newClient()
				if err != nil {
					return err
				}

				acct, bldg, err := getAccountAndBuilding(ctx, c, flags)
				if err != nil {
					return err
				}

				dateFlag, _ := cmd.Flags().GetString("date")
				fromFlag, _ := cmd.Flags().GetString("from")
				toFlag, _ := cmd.Flags().GetString("to")
				typeFlag, _ := cmd.Flags().GetString("type")

				startStr := time.Now().Format("2006-01-02")
				endStr := time.Now().AddDate(0, 0, 7).Format("2006-01-02")

				if dateFlag != "" {
					startStr = dateFlag
					endStr = dateFlag
				} else if fromFlag != "" || toFlag != "" {
					if fromFlag != "" {
						startStr = fromFlag
					}
					if toFlag != "" {
						endStr = toFlag
					}
				}

				productTypes := []string{"POPUP", "CAFE", "DELIVERY", "CATERING"}
				if typeFlag != "" {
					productTypes = []string{strings.ToUpper(typeFlag)}
				}

				variables := map[string]any{
					"input": map[string]any{
						"statuses":      []string{"ACTIVE", "PROPOSED"},
						"productTypes":  productTypes,
						"accountId":     acct,
						"buildingId":    bldg,
						"endTimeAfter":  startStr,
						"endTimeBefore": endStr,
					},
				}

				raw, err := c.Query(cmd.Context(), client.SearchPublicEventsQuery, variables)
				if err != nil {
					return err
				}
				if len(raw) == 0 {
					return nil
				}

				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), raw, flags)
				}

				events, err := parseEventsFromRaw(raw)
				if err != nil {
					return err
				}

				w := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintln(w, "Event ID\tDate\tStart\tEnd\tType\tStatus\tRestaurants")
				for _, ev := range events {
					startTimeParsed, _ := time.Parse(time.RFC3339, ev.StartTime)
					endTimeParsed, _ := time.Parse(time.RFC3339, ev.EndTime)
					dateStr := startTimeParsed.Format("2006-01-02")
					startStr := startTimeParsed.Format("15:04")
					endStr := endTimeParsed.Format("15:04")

					var restNames []string
					for _, r := range ev.Restaurants {
						restNames = append(restNames, r.Name)
					}
					rests := strings.Join(restNames, ", ")
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", ev.ID, dateStr, startStr, endStr, ev.Typename, ev.Status, rests)
				}
				_ = w.Flush()
				return nil
			},
		}
		eventsCmd.Flags().String("date", "", "Date to filter events (YYYY-MM-DD)")
		eventsCmd.Flags().String("from", "", "Start date range (YYYY-MM-DD)")
		eventsCmd.Flags().String("to", "", "End date range (YYYY-MM-DD)")
		eventsCmd.Flags().String("type", "", "Filter by type (popup, cafe, delivery, catering)")
		replaceCommand(eventsCmd)

		// 2. restaurants
		restsCmd := &cobra.Command{
			Use:   "restaurants",
			Short: "Offline or live list of restaurants based on events",
			Example: strings.Trim(`
  # List all available restaurants
  fooda-pp-cli restaurants

  # Filter restaurants by cuisine type
  fooda-pp-cli restaurants --cuisine Italian

  # Show restaurants from locally cached store only
  fooda-pp-cli restaurants --local
`, "\n"),
			Annotations: map[string]string{"pp:data-source": "auto", "mcp:read-only": "true"},
			RunE: func(cmd *cobra.Command, args []string) error {
				if dryRunOK(flags) {
					return writeDryRun(cmd.OutOrStdout(), flags, "restaurants")
				}

				ctx, cancel := boundCtx(cmd.Context(), flags)
				defer cancel()

				c, err := flags.newClient()
				if err != nil {
					return err
				}

				cuisineFilter, _ := cmd.Flags().GetString("cuisine")
				localOnly, _ := cmd.Flags().GetBool("local")

				var events []parsedEvent
				if localOnly || flags.dataSource == "local" {
					db, err := store.OpenReadOnlyContext(ctx, defaultDBPath("fooda-pp-cli"))
					if err != nil || db == nil {
						return notFoundErr(fmt.Errorf("local database not found. Run 'sync' first"))
					}
					defer db.Close()

					rawEvents, err := db.List("event", 1000)
					if err != nil {
						return err
					}
					for _, re := range rawEvents {
						var ev parsedEvent
						if err := json.Unmarshal(re, &ev); err != nil {
							return fmt.Errorf("failed to parse stored event: %w", err)
						}
						events = append(events, ev)
					}
				} else {
					acct, bldg, err := getAccountAndBuilding(ctx, c, flags)
					if err != nil {
						if flags.dataSource != "live" && isNetworkError(err) {
							fmt.Fprintf(cmd.ErrOrStderr(), "warning: live discovery failed (%v); falling back to local database\n", err)
							db, dbErr := store.OpenReadOnlyContext(ctx, defaultDBPath("fooda-pp-cli"))
							if dbErr == nil && db != nil {
								defer db.Close()
								rawEvents, _ := db.List("event", 1000)
								for _, re := range rawEvents {
									var ev parsedEvent
									if json.Unmarshal(re, &ev) == nil {
										events = append(events, ev)
									}
								}
							} else {
								return fmt.Errorf("live discovery failed: %w (no local database fallback available)", err)
							}
						} else {
							return err
						}
					} else {
						variables := map[string]any{
							"input": map[string]any{
								"statuses":      []string{"ACTIVE", "PROPOSED"},
								"productTypes":  []string{"POPUP", "CAFE", "DELIVERY", "CATERING"},
								"accountId":     acct,
								"buildingId":    bldg,
								"endTimeAfter":  time.Now().Format("2006-01-02"),
								"endTimeBefore": time.Now().AddDate(0, 0, 7).Format("2006-01-02"),
							},
						}
						raw, err := c.Query(ctx, client.SearchPublicEventsQuery, variables)
						if err != nil {
							if flags.dataSource != "live" && isNetworkError(err) {
								fmt.Fprintf(cmd.ErrOrStderr(), "warning: live query failed (%v); falling back to local database\n", err)
								db, dbErr := store.OpenReadOnlyContext(ctx, defaultDBPath("fooda-pp-cli"))
								if dbErr == nil && db != nil {
									defer db.Close()
									rawEvents, _ := db.List("event", 1000)
									for _, re := range rawEvents {
										var ev parsedEvent
										if json.Unmarshal(re, &ev) == nil {
											events = append(events, ev)
										}
									}
								} else {
									return fmt.Errorf("live query failed: %w (no local database fallback available)", err)
								}
							} else {
								return err
							}
						} else {
							events, err = parseEventsFromRaw(raw)
							if err != nil {
								return err
							}
						}
					}
				}

				type restInfo struct {
					ID          string   `json:"id,omitempty"`
					VendorID    string   `json:"vendorId,omitempty"`
					Name        string   `json:"name,omitempty"`
					Cuisines    []string `json:"cuisines,omitempty"`
					MealPeriods []string `json:"mealPeriods,omitempty"`
				}

				uniqueRests := map[string]restInfo{}
				for _, ev := range events {
					for _, r := range ev.Restaurants {
						id := r.VendorID
						if id == "" {
							id = r.ID
						}
						info := uniqueRests[id]
						info.ID = r.ID
						info.VendorID = r.VendorID
						info.Name = strings.TrimSpace(r.Name)
						if len(r.Cuisines) > 0 {
							info.Cuisines = r.Cuisines
						}
						periodSeen := false
						for _, p := range info.MealPeriods {
							if p == r.MealPeriod {
								periodSeen = true
								break
							}
						}
						if !periodSeen && r.MealPeriod != "" {
							info.MealPeriods = append(info.MealPeriods, r.MealPeriod)
						}
						uniqueRests[id] = info
					}
				}

				var results []restInfo
				for _, r := range uniqueRests {
					if cuisineFilter != "" {
						match := false
						for _, c := range r.Cuisines {
							if strings.Contains(strings.ToLower(c), strings.ToLower(cuisineFilter)) {
								match = true
								break
							}
						}
						if !match {
							continue
						}
					}
					results = append(results, r)
				}

				sort.Slice(results, func(i, j int) bool {
					return results[i].Name < results[j].Name
				})

				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), results, flags)
				}

				w := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintln(w, "Vendor ID\tName\tCuisines\tMeal Periods")
				for _, r := range results {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.ID, r.Name, strings.Join(r.Cuisines, ", "), strings.Join(r.MealPeriods, ", "))
				}
				_ = w.Flush()
				return nil
			},
		}
		restsCmd.Flags().String("cuisine", "", "Filter by cuisine keyword")
		restsCmd.Flags().Bool("local", false, "Read offline derived list from local DB")
		replaceCommand(restsCmd)

		// 3. menu
		menuCmd := &cobra.Command{
			Use:   "menu <event-id>",
			Short: "Parse and list menu items for an event",
			Example: strings.Trim(`
  fooda-pp-cli menu 12345
`, "\n"),
			Args:        cobra.ExactArgs(1),
			Annotations: map[string]string{"pp:data-source": "live", "mcp:read-only": "true"},
			RunE: func(cmd *cobra.Command, args []string) error {
				if flags.dataSource == "local" {
					return fmt.Errorf("menu is live-only; --data-source local is not supported")
				}

				if dryRunOK(flags) {
					return writeDryRun(cmd.OutOrStdout(), flags, "menu")
				}

				ctx, cancel := boundCtx(cmd.Context(), flags)
				defer cancel()

				c, err := flags.newClient()
				if err != nil {
					return err
				}

				eventID := args[0]
				vendorFilter, _ := cmd.Flags().GetString("vendor")

				acct, bldg, err := getAccountAndBuilding(ctx, c, flags)
				if err != nil {
					return err
				}

				var urlPath string
				if strings.HasPrefix(eventID, "http://") || strings.HasPrefix(eventID, "https://") || strings.Contains(eventID, "/accounts/") {
					urlPath = urlToPath(eventID)
				} else if strings.HasPrefix(strings.ToUpper(eventID), "S") && isNumeric(eventID[1:]) {
					urlPath = fmt.Sprintf("/accounts/%s/select_events/%s/items", acct, eventID)
				} else if isNumeric(eventID) {
					mapped := false
					variables := map[string]any{
						"input": map[string]any{
							"statuses":      []string{"ACTIVE", "PROPOSED"},
							"productTypes":  []string{"POPUP", "CAFE", "DELIVERY", "CATERING"},
							"accountId":     acct,
							"buildingId":    bldg,
							"endTimeAfter":  time.Now().AddDate(0, 0, -1).Format("2006-01-02"),
							"endTimeBefore": time.Now().AddDate(0, 0, 7).Format("2006-01-02"),
						},
					}
					rawEv, err := c.Query(ctx, client.SearchPublicEventsQuery, variables)
					if err == nil {
						events, err := parseEventsFromRaw(rawEv)
						if err == nil {
							for _, ev := range events {
								if ev.ID == eventID && ev.URL != "" {
									urlPath = urlToPath(ev.URL)
									mapped = true
									break
								}
							}
						}
					}
					if !mapped {
						urlPath = fmt.Sprintf("/accounts/%s/select_events/S%s/items", acct, eventID)
					}
				} else {
					urlPath = fmt.Sprintf("/accounts/%s/select_events/%s/items", acct, eventID)
				}

				if vendorFilter != "" {
					if strings.Contains(urlPath, "?") {
						urlPath += "&filterable[vendor_id]=" + vendorFilter
					} else {
						urlPath += "?filterable[vendor_id]=" + vendorFilter
					}
				}

				raw, err := c.GetHTML(ctx, urlPath)
				if err != nil {
					return err
				}
				if len(raw) == 0 {
					return nil
				}

				items, err := client.ParseMenuItems(string(raw))
				if err != nil {
					return err
				}

				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), items, flags)
				}

				w := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintln(w, "Category\tName\tPrice\tDietary\tDescription")
				for _, it := range items {
					dietary := strings.Join(it.DietaryRestrictions, ", ")
					desc := it.Description
					if len(desc) > 60 {
						desc = desc[:57] + "..."
					}
					fmt.Fprintf(w, "%s\t%s\t$%.2f\t%s\t%s\n", it.Category, it.Name, it.Price, dietary, desc)
				}
				_ = w.Flush()
				return nil
			},
		}
		menuCmd.Flags().String("vendor", "", "Filter items by vendor ID")
		replaceCommand(menuCmd)

		// 4. orders
		ordersCmd := &cobra.Command{
			Use:   "orders",
			Short: "List past orders parsed from HTML, or synced DB",
			Example: strings.Trim(`
  fooda-pp-cli orders --since 90d
`, "\n"),
			Annotations: map[string]string{"pp:data-source": "auto", "mcp:read-only": "true"},
			RunE: func(cmd *cobra.Command, args []string) error {
				if dryRunOK(flags) {
					return writeDryRun(cmd.OutOrStdout(), flags, "orders")
				}

				ctx, cancel := boundCtx(cmd.Context(), flags)
				defer cancel()

				c, err := flags.newClient()
				if err != nil {
					return err
				}

				sinceStr, _ := cmd.Flags().GetString("since")
				limitFlag, _ := cmd.Flags().GetInt("limit")

				var orders []client.PastOrdersProps
				isLocal := flags.dataSource == "local"

				if isLocal {
					db, err := store.OpenReadOnlyContext(ctx, defaultDBPath("fooda-pp-cli"))
					if err != nil || db == nil {
						return notFoundErr(fmt.Errorf("local database not found; run 'sync' first to populate local orders"))
					}
					defer db.Close()
					orders, err = loadAndDeduplicateOrders(db)
					if err != nil {
						return err
					}
					if len(orders) == 0 {
						return notFoundErr(fmt.Errorf("no synced orders found; run 'sync' first to populate local orders"))
					}
				} else {
					raw, err := c.GetHTML(ctx, "/settings/orders")
					if err != nil {
						return err
					}
					if len(raw) == 0 {
						return nil
					}
					props, err := client.ParsePastOrders(string(raw))
					if err != nil {
						return err
					}
					orders = []client.PastOrdersProps{*props}
				}

				type flatOrder struct {
					ID       int     `json:"id"`
					Vendor   string  `json:"vendor"`
					Total    float64 `json:"total"`
					Delivery string  `json:"delivery"`
					Items    string  `json:"items"`
					UUID     string  `json:"uuid"`
					TimeUnix int64   `json:"time_unix"`
					Status   string  `json:"status"`
				}

				var flList []flatOrder
				cutoff := time.Time{}
				if sinceStr != "" {
					var err error
					cutoff, err = parseSince(sinceStr)
					if err != nil {
						return err
					}
				}

				for _, op := range orders {
					for _, pres := range op.Presenter {
						o := pres.Order
						orderTime := time.Unix(o.OrderFulfilledTimeUnix, 0)
						if !cutoff.IsZero() && orderTime.Before(cutoff) {
							continue
						}

						flList = append(flList, flatOrder{
							ID:       o.ID,
							Vendor:   strings.Join(o.VendorNames, ", "),
							Total:    float64(o.PaymentCents) / 100.0,
							Delivery: o.Delivery,
							Items:    strings.Join(o.ItemNames, "; "),
							UUID:     o.RequestID,
							TimeUnix: o.OrderFulfilledTimeUnix,
							Status:   o.Status,
						})
					}
				}

				sort.Slice(flList, func(i, j int) bool {
					return flList[i].TimeUnix > flList[j].TimeUnix
				})

				if limitFlag > 0 && len(flList) > limitFlag {
					flList = flList[:limitFlag]
				}

				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), flList, flags)
				}

				w := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintln(w, "Order ID\tDate/Delivery\tVendor\tItems\tTotal\tStatus")
				for _, fo := range flList {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t$%.2f\t%s\n", fo.UUID, fo.Delivery, fo.Vendor, fo.Items, fo.Total, fo.Status)
				}
				_ = w.Flush()
				return nil
			},
		}
		ordersCmd.Flags().String("since", "90d", "Filter orders since (e.g. 90d, 120d, 6mo)")
		ordersCmd.Flags().Int("limit", 100, "Limit the number of returned orders")

		// 4b. orders get
		ordersGetCmd := &cobra.Command{
			Use:   "get <uuid>",
			Short: "View details for a specific past order",
			Example: strings.Trim(`
  fooda-pp-cli orders get 11111111-2222-3333-4444-555555555555
`, "\n"),
			Args:        cobra.ExactArgs(1),
			Annotations: map[string]string{"pp:data-source": "auto", "mcp:read-only": "true"},
			RunE: func(cmd *cobra.Command, args []string) error {
				if dryRunOK(flags) {
					return writeDryRun(cmd.OutOrStdout(), flags, "orders get")
				}

				ctx, cancel := boundCtx(cmd.Context(), flags)
				defer cancel()

				c, err := flags.newClient()
				if err != nil {
					return err
				}

				uuid := args[0]
				var details *client.OrderDetailProps
				isLocal := flags.dataSource == "local"

				if isLocal {
					db, err := store.OpenReadOnlyContext(ctx, defaultDBPath("fooda-pp-cli"))
					if err != nil || db == nil {
						return notFoundErr(fmt.Errorf("local database not found; run 'sync' first to populate local orders"))
					}
					defer db.Close()
					raw, err := db.Get("order_detail", uuid)
					if err != nil || len(raw) == 0 {
						return notFoundErr(fmt.Errorf("order detail %s not found in local database; run 'sync' first", uuid))
					}
					var parsed client.OrderDetailProps
					if err := json.Unmarshal(raw, &parsed); err != nil {
						return fmt.Errorf("failed to parse stored order detail: %w", err)
					}
					details = &parsed
				} else {
					raw, err := c.GetHTML(ctx, "/settings/select_order/"+uuid)
					if err != nil {
						return err
					}
					if len(raw) == 0 {
						return nil
					}
					details, err = client.ParseOrderDetail(string(raw))
					if err != nil {
						return err
					}
				}

				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), details.OrderData, flags)
				}

				od := details.OrderData
				fmt.Fprintf(cmd.OutOrStdout(), "Order UUID:   %s\n", od.OrderUuid)
				fmt.Fprintf(cmd.OutOrStdout(), "Vendor:       %s\n", od.FormattedVendorNames)
				fmt.Fprintf(cmd.OutOrStdout(), "Delivery:     %s\n", od.ActivityDate)
				fmt.Fprintf(cmd.OutOrStdout(), "Location:     %s (%s)\n", od.LocationDetails, od.LocationSpot)
				fmt.Fprintln(cmd.OutOrStdout(), "\nLine Items:")
				for _, oGroup := range od.OrderItems {
					for _, it := range oGroup.OrderItems {
						fmt.Fprintf(cmd.OutOrStdout(), "  - %dx %s (%s)\n", it.Quantity, it.Name, it.Amount)
						for _, opt := range it.ItemOptions {
							fmt.Fprintf(cmd.OutOrStdout(), "      + %s (%s)\n", opt.Name, opt.Amount)
						}
					}
				}
				fmt.Fprintln(cmd.OutOrStdout(), "")
				fmt.Fprintf(cmd.OutOrStdout(), "Subtotal:     %s\n", od.FormattedSubtotalAmount)
				fmt.Fprintf(cmd.OutOrStdout(), "Tax:          %s\n", od.FormattedTaxAmount)
				fmt.Fprintf(cmd.OutOrStdout(), "Subsidy:      %s\n", od.FormattedSubsidyAmount)
				fmt.Fprintf(cmd.OutOrStdout(), "Total Paid:   %s\n", od.FormattedTotalAmount)
				return nil
			},
		}
		ordersCmd.AddCommand(ordersGetCmd)
		replaceCommand(ordersCmd)

		// 5. subsidy
		subsidyCmd := &cobra.Command{
			Use:   "subsidy",
			Short: "List your active subsidies and remaining daily balances",
			Example: strings.Trim(`
  fooda-pp-cli subsidy
`, "\n"),
			Annotations: map[string]string{"pp:data-source": "live", "mcp:read-only": "true"},
			RunE: func(cmd *cobra.Command, args []string) error {
				if flags.dataSource == "local" {
					return fmt.Errorf("subsidy is live-only; --data-source local is not supported")
				}

				if dryRunOK(flags) {
					return writeDryRun(cmd.OutOrStdout(), flags, "subsidy")
				}

				c, err := flags.newClient()
				if err != nil {
					return err
				}

				raw, err := c.Query(cmd.Context(), client.SubsidiesListQuery, nil)
				if err != nil {
					return err
				}
				if len(raw) == 0 {
					return nil
				}

				type parsedSubsidy struct {
					AccountName string `json:"accountName"`
					Name        string `json:"name"`
					Code        string `json:"code"`
					Coverage    string `json:"coverage"`
					State       string `json:"state"`
					ValidAt     string `json:"validAt"`
					Remaining   string `json:"remaining"`
				}

				var envelope struct {
					GetUserSubsidies struct {
						Subsidies []struct {
							AccountName string `json:"accountName"`
							QrInfo      string `json:"qrInfo"`
							Code        string `json:"code"`
							Coverage    string `json:"coverage"`
							Name        string `json:"name"`
							State       string `json:"state"`
							ValidAt     string `json:"validAt"`
						} `json:"subsidies"`
					} `json:"getUserSubsidies"`
				}

				if err := json.Unmarshal(raw, &envelope); err != nil {
					return err
				}

				var subs []parsedSubsidy
				for _, s := range envelope.GetUserSubsidies.Subsidies {
					remainingStr := "N/A"
					if s.QrInfo != "" {
						var qr struct {
							Subsidy struct {
								AmountRemainingCents int `json:"amount_remaining_cents"`
							} `json:"subsidy"`
						}
						if json.Unmarshal([]byte(s.QrInfo), &qr) == nil {
							remainingStr = fmt.Sprintf("$%.2f", float64(qr.Subsidy.AmountRemainingCents)/100.0)
						}
					}
					subs = append(subs, parsedSubsidy{
						AccountName: s.AccountName,
						Name:        s.Name,
						Code:        s.Code,
						Coverage:    s.Coverage,
						State:       s.State,
						ValidAt:     s.ValidAt,
						Remaining:   remainingStr,
					})
				}

				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), subs, flags)
				}

				w := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintln(w, "Account\tName\tCode\tCoverage\tRemaining\tState\tValid At")
				for _, s := range subs {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", s.AccountName, s.Name, s.Code, s.Coverage, s.Remaining, s.State, s.ValidAt)
				}
				_ = w.Flush()
				return nil
			},
		}
		replaceCommand(subsidyCmd)

		// 5b. card
		cardCmd := &cobra.Command{
			Use:   "card",
			Short: "View Fooda Card balances and registered cards",
			Example: strings.Trim(`
  # Display registered Fooda cards and their outstanding balances
  fooda-pp-cli card
`, "\n"),
			Annotations: map[string]string{"pp:data-source": "live", "mcp:read-only": "true"},
			RunE: func(cmd *cobra.Command, args []string) error {
				if flags.dataSource == "local" {
					return fmt.Errorf("card is live-only; --data-source local is not supported")
				}

				if dryRunOK(flags) {
					return writeDryRun(cmd.OutOrStdout(), flags, "card")
				}

				c, err := flags.newClient()
				if err != nil {
					return err
				}

				rawBal, err := c.Query(cmd.Context(), client.OutstandingBalancesQuery, nil)
				if err != nil {
					return err
				}
				if len(rawBal) == 0 {
					return nil
				}

				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), rawBal, flags)
				}

				var envelopeBal struct {
					GetFoodaCardOutstandingBalances struct {
						Balances []struct {
							FoodaCardMembershipID string `json:"foodaCardMembershipId"`
							AmountCents           int    `json:"amountCents"`
						} `json:"balances"`
					} `json:"getFoodaCardOutstandingBalances"`
				}
				if err := json.Unmarshal(rawBal, &envelopeBal); err != nil {
					return fmt.Errorf("failed to parse Fooda Card balances: %w", err)
				}

				fmt.Fprintln(cmd.OutOrStdout(), "Outstanding Fooda Card Balances:")
				for _, b := range envelopeBal.GetFoodaCardOutstandingBalances.Balances {
					fmt.Fprintf(cmd.OutOrStdout(), "  Membership ID: %s, Outstanding: $%.2f\n", b.FoodaCardMembershipID, float64(b.AmountCents)/100.0)
				}
				if len(envelopeBal.GetFoodaCardOutstandingBalances.Balances) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "  No outstanding balances.")
				}
				return nil
			},
		}
		replaceCommand(cardCmd)

		// 5c. whoami
		whoamiCmd := &cobra.Command{
			Use:   "whoami",
			Short: "Display logged-in user profile, building, and account details",
			Example: strings.Trim(`
  fooda-pp-cli whoami
`, "\n"),
			Annotations: map[string]string{"pp:data-source": "live", "mcp:read-only": "true"},
			RunE: func(cmd *cobra.Command, args []string) error {
				if flags.dataSource == "local" {
					return fmt.Errorf("whoami is live-only; --data-source local is not supported")
				}

				if dryRunOK(flags) {
					return writeDryRun(cmd.OutOrStdout(), flags, "whoami")
				}

				if flags.platformSession != nil {
					return flags.printJSON(cmd, platformIdentityReport(flags.platformSession))
				}

				ctx, cancel := boundCtx(cmd.Context(), flags)
				defer cancel()

				c, err := flags.newClient()
				if err != nil {
					return err
				}

				// Cloudflare challenges /settings/profile for non-browser clients even
				// with a valid session; fall back to the greeting on /settings/orders.
				rawProfile, err := c.GetHTML(ctx, "/settings/profile")
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "note: profile page unavailable (%v); showing limited identity\n", err)
					rawProfile, err = c.GetHTML(ctx, "/settings/orders")
					if err != nil {
						return err
					}
				}

				email := "N/A"
				name := "N/A"
				userUUID := "N/A"

				emailReg := regexp.MustCompile(`<input\s+[^>]*id="user_email"\s+value="([^"]*)"`)
				fnReg := regexp.MustCompile(`<input\s+[^>]*id="user_first_name"\s+value="([^"]*)"`)
				lnReg := regexp.MustCompile(`<input\s+[^>]*id="user_last_name"\s+value="([^"]*)"`)

				emailMatch := emailReg.FindStringSubmatch(string(rawProfile))
				if len(emailMatch) >= 2 {
					email = emailMatch[1]
				}
				fnMatch := fnReg.FindStringSubmatch(string(rawProfile))
				lnMatch := lnReg.FindStringSubmatch(string(rawProfile))
				if len(fnMatch) >= 2 || len(lnMatch) >= 2 {
					var names []string
					if len(fnMatch) >= 2 {
						names = append(names, fnMatch[1])
					}
					if len(lnMatch) >= 2 {
						names = append(names, lnMatch[1])
					}
					name = strings.Join(names, " ")
				}

				if name == "N/A" {
					if m := regexp.MustCompile(`Hi,\s*([^!<]+)!`).FindStringSubmatch(string(rawProfile)); len(m) >= 2 {
						name = strings.TrimSpace(m[1])
					}
				}

				uuidReg := regexp.MustCompile(`user_uuid:\s*'([^']*)'`)
				uuidMatch := uuidReg.FindStringSubmatch(string(rawProfile))
				if len(uuidMatch) >= 2 {
					userUUID = uuidMatch[1]
				}

				acct, bldg, _ := getAccountAndBuilding(ctx, c, flags)

				type whoamiInfo struct {
					Name       string `json:"name"`
					Email      string `json:"email"`
					UserUUID   string `json:"user_uuid"`
					AccountID  string `json:"account_id"`
					BuildingID string `json:"building_id"`
				}
				info := whoamiInfo{
					Name:       name,
					Email:      email,
					UserUUID:   userUUID,
					AccountID:  acct,
					BuildingID: bldg,
				}

				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), info, flags)
				}

				fmt.Fprintf(cmd.OutOrStdout(), "Name:        %s\n", info.Name)
				fmt.Fprintf(cmd.OutOrStdout(), "Email:       %s\n", info.Email)
				fmt.Fprintf(cmd.OutOrStdout(), "User UUID:   %s\n", info.UserUUID)
				fmt.Fprintf(cmd.OutOrStdout(), "Account ID:  %s\n", info.AccountID)
				fmt.Fprintf(cmd.OutOrStdout(), "Building ID: %s\n", info.BuildingID)
				return nil
			},
		}
		replaceCommand(whoamiCmd)

		// 5d. recommend
		recommendCmd := &cobra.Command{
			Use:   "recommend <event-id>",
			Short: "Get restaurant recommendations for an event",
			Example: strings.Trim(`
  fooda-pp-cli recommend 12345
`, "\n"),
			Args:        cobra.ExactArgs(1),
			Annotations: map[string]string{"pp:data-source": "live", "mcp:read-only": "true"},
			RunE: func(cmd *cobra.Command, args []string) error {
				if flags.dataSource == "local" {
					return fmt.Errorf("recommend is live-only; --data-source local is not supported")
				}

				if dryRunOK(flags) {
					return writeDryRun(cmd.OutOrStdout(), flags, "recommend")
				}

				c, err := flags.newClient()
				if err != nil {
					return err
				}

				eventID := args[0]
				variables := map[string]any{
					"input": map[string]any{
						"eventId": eventID,
					},
				}
				raw, err := c.Query(cmd.Context(), client.RecommendationsGetQuery, variables)
				if err != nil {
					return err
				}
				if len(raw) == 0 {
					return nil
				}

				return printJSONFiltered(cmd.OutOrStdout(), raw, flags)
			},
		}
		replaceCommand(recommendCmd)

		// 6. order
		orderCmd := &cobra.Command{
			Use:   "order",
			Short: "View cart, plan budget, and place or cancel food orders",
		}

		// order add
		var orderAddQty int
		var orderAddOptions []string
		var orderAddNote string
		var orderAddVendor string
		var orderAddConfirm bool
		var orderAddEvent string

		orderAddCmd := &cobra.Command{
			Use:   "add <item-id-or-name>",
			Short: "Add a menu item to your cart (dry-run by default)",
			Example: strings.Trim(`
  # Dry-run: resolve and inspect adding item
  fooda-pp-cli order add "Beef Bulgogi Bowl" --event S609348
  # Confirm adding item to cart
  fooda-pp-cli order add "Beef Bulgogi Bowl" --event S609348 --confirm
`, "\n"),
			Args:        cobra.ExactArgs(1),
			Annotations: map[string]string{"pp:data-source": "live"},
			RunE: func(cmd *cobra.Command, args []string) error {
				if flags.dataSource == "local" {
					return fmt.Errorf("order add is live-only; --data-source local is not supported")
				}
				if dryRunOK(flags) {
					return writeDryRun(cmd.OutOrStdout(), flags, "order add")
				}
				if orderAddConfirm && cliutil.IsAnyHarness() {
					return writeHarnessRefusal(cmd.OutOrStdout(), flags, "order add")
				}

				ctx, cancel := boundCtx(cmd.Context(), flags)
				defer cancel()

				c, err := flags.newClient()
				if err != nil {
					return err
				}

				acct, _, err := getAccountAndBuilding(ctx, c, flags)
				if err != nil {
					return err
				}

				eventID := orderAddEvent
				if eventID == "" {
					eventID, err = getUpcomingEvent(ctx, c, flags)
					if err != nil {
						return err
					}
				}

				// Resolve URL path for event
				var eventPath string
				if strings.HasPrefix(eventID, "S") || strings.HasPrefix(eventID, "s") {
					eventPath = fmt.Sprintf("/accounts/%s/select_events/%s/items", acct, eventID)
				} else {
					eventPath = urlToPath(eventID)
				}

				// Fetch menu to find the item
				rawMenu, err := c.GetHTML(ctx, eventPath)
				if err != nil {
					return err
				}
				menuItems, err := client.ParseMenuItems(string(rawMenu))
				if err != nil {
					return err
				}

				if orderAddVendor != "" {
					var filtered []client.MenuItem
					for _, it := range menuItems {
						if strings.Contains(strings.ToLower(it.VendorName), strings.ToLower(orderAddVendor)) {
							filtered = append(filtered, it)
						}
					}
					if len(filtered) == 0 {
						return notFoundErr(fmt.Errorf("no vendors matching %q found on the menu", orderAddVendor))
					}
					menuItems = filtered
				}

				itemQuery := args[0]
				var matched []client.MenuItem
				if isNumeric(itemQuery) {
					for _, it := range menuItems {
						if it.ID == itemQuery {
							matched = append(matched, it)
							break
						}
					}
				} else {
					for _, it := range menuItems {
						if strings.Contains(strings.ToLower(it.Name), strings.ToLower(itemQuery)) {
							matched = append(matched, it)
						}
					}
				}

				if len(matched) == 0 {
					return notFoundErr(fmt.Errorf("no menu items matching %q for event %s", itemQuery, eventID))
				}
				if len(matched) > 1 {
					var candidates []string
					for _, m := range matched {
						candidates = append(candidates, fmt.Sprintf("- %s (ID: %s, Vendor: %s, Price: $%.2f)", m.Name, m.ID, m.VendorName, m.Price))
					}
					return usageErr(fmt.Errorf("ambiguous item name %q; matches:\n%s", itemQuery, strings.Join(candidates, "\n")))
				}

				targetItem := matched[0]

				// Fetch individual menu item page to get form and parent_id, csrf token, options
				itemPageURL := fmt.Sprintf("/accounts/%s/select_events/%s/items/%s", acct, strings.TrimPrefix(eventID, "S"), targetItem.ID)
				if strings.Contains(eventPath, "/select_events/") {
					parts := strings.Split(eventPath, "/select_events/")
					if len(parts) > 1 {
						sID := strings.Split(parts[1], "/")[0]
						itemPageURL = fmt.Sprintf("/accounts/%s/select_events/%s/items/%s", acct, sID, targetItem.ID)
					}
				}

				rawPage, err := c.GetHTML(ctx, itemPageURL)
				if err != nil {
					return err
				}

				form, err := parseItemPageHTML(string(rawPage))
				if err != nil {
					return err
				}

				// Resolve selected options
				var selectedOptionIDs []string
				var selectedOptionLabels []string
				var optionsPriceDelta float64

				for _, optArg := range orderAddOptions {
					var matches []parsedItemOption
					// 1. exact ID match
					for _, o := range form.Options {
						if o.ID == optArg {
							matches = []parsedItemOption{o}
							break
						}
					}
					// 2. exact label match (case-insensitive)
					if len(matches) == 0 {
						for _, o := range form.Options {
							cleanLabel := strings.TrimSpace(o.Label)
							if idx := strings.Index(cleanLabel, " ("); idx != -1 {
								cleanLabel = strings.TrimSpace(cleanLabel[:idx])
							}
							if strings.EqualFold(cleanLabel, optArg) || strings.EqualFold(strings.TrimSpace(o.Label), optArg) {
								matches = append(matches, o)
							}
						}
					}
					// 3. unique substring match
					if len(matches) == 0 {
						for _, o := range form.Options {
							if strings.Contains(strings.ToLower(o.Label), strings.ToLower(optArg)) {
								matches = append(matches, o)
							}
						}
					}

					if len(matches) == 0 {
						return usageErr(fmt.Errorf("option %q not found on menu item; available options:\n%s", optArg, formatOptions(form.Options)))
					}
					if len(matches) > 1 {
						var candidates []string
						for _, m := range matches {
							candidates = append(candidates, fmt.Sprintf("- ID: %s, Label: %s (+$%.2f)", m.ID, m.Label, m.PriceDelta))
						}
						return usageErr(fmt.Errorf("ambiguous option %q; matches multiple options:\n%s", optArg, strings.Join(candidates, "\n")))
					}

					selectedOpt := matches[0]
					selectedOptionIDs = append(selectedOptionIDs, selectedOpt.ID)
					selectedOptionLabels = append(selectedOptionLabels, selectedOpt.Label)
					optionsPriceDelta += selectedOpt.PriceDelta
				}

				totalItemPrice := targetItem.Price + optionsPriceDelta

				// Build url-encoded body preview
				formVals := url.Values{}
				formVals.Set("authenticity_token", form.CSRFToken)
				formVals.Set("info[parent_id]", form.ParentID)
				formVals.Set("info[parent_type]", form.ParentType)
				formVals.Set("info[item_id]", form.ItemID)
				formVals.Set("info[item_type]", form.ItemType)
				formVals.Set("quantity", strconv.Itoa(orderAddQty))
				for _, id := range selectedOptionIDs {
					formVals.Add("info[options][]", id)
				}
				if orderAddNote != "" {
					formVals.Set("info[instructions]", orderAddNote)
				}

				postURL := fmt.Sprintf("/accounts/%s/select_events/%s/items", acct, strings.TrimPrefix(eventID, "S"))
				if strings.Contains(eventPath, "/select_events/") {
					parts := strings.Split(eventPath, "/select_events/")
					if len(parts) > 1 {
						sID := strings.Split(parts[1], "/")[0]
						postURL = fmt.Sprintf("/accounts/%s/select_events/%s/items", acct, sID)
					}
				}

				if flags.asJSON {
					type jsonItem struct {
						ID         string   `json:"id"`
						Name       string   `json:"name"`
						Vendor     string   `json:"vendor"`
						PriceCents int      `json:"price_cents"`
						Options    []string `json:"options"`
					}

					ji := jsonItem{
						ID:         targetItem.ID,
						Name:       targetItem.Name,
						Vendor:     targetItem.VendorName,
						PriceCents: int(math.Round(totalItemPrice * 100)),
						Options:    selectedOptionLabels,
					}

					if !orderAddConfirm {
						type dryRunAddResult struct {
							DryRun  bool     `json:"dry_run"`
							Action  string   `json:"action"`
							Item    jsonItem `json:"item"`
							Request string   `json:"request"`
						}
						out := dryRunAddResult{
							DryRun:  true,
							Action:  "order add",
							Item:    ji,
							Request: fmt.Sprintf("POST %s with %s", postURL, formVals.Encode()),
						}
						return printJSONFiltered(cmd.OutOrStdout(), out, flags)
					}

					_, statusCode, err := makeAPIRequest(ctx, c, "POST", postURL, []byte(formVals.Encode()), "application/x-www-form-urlencoded")
					if err != nil {
						return err
					}
					if statusCode >= 400 {
						return fmt.Errorf("failed to add item to cart (HTTP %d)", statusCode)
					}

					items, pricing, err := getCartAndPricing(ctx, c)
					if err != nil {
						return err
					}

					type confirmedAddResult struct {
						Added bool     `json:"added"`
						Item  jsonItem `json:"item"`
						Cart  struct {
							Items   []map[string]any `json:"items"`
							Pricing any              `json:"pricing"`
						} `json:"cart"`
					}
					out := confirmedAddResult{
						Added: true,
						Item:  ji,
					}
					out.Cart.Items = items
					out.Cart.Pricing = pricing

					return printJSONFiltered(cmd.OutOrStdout(), out, flags)
				}

				if !orderAddConfirm {
					// Dry-run mode plain text
					fmt.Fprintf(cmd.OutOrStdout(), "Resolved Item:  %s\n", targetItem.Name)
					fmt.Fprintf(cmd.OutOrStdout(), "Vendor:         %s\n", targetItem.VendorName)
					fmt.Fprintf(cmd.OutOrStdout(), "Base Price:     $%.2f\n", targetItem.Price)
					if len(selectedOptionLabels) > 0 {
						fmt.Fprintln(cmd.OutOrStdout(), "Selected Options:")
						for _, label := range selectedOptionLabels {
							fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", label)
						}
					}
					fmt.Fprintf(cmd.OutOrStdout(), "Total Price:    $%.2f (qty: %d)\n", totalItemPrice*float64(orderAddQty), orderAddQty)
					fmt.Fprintln(cmd.OutOrStdout(), "\nThis is a DRY-RUN. Run with --confirm to add this item to your cart.")
					return nil
				}

				_, statusCode, err := makeAPIRequest(ctx, c, "POST", postURL, []byte(formVals.Encode()), "application/x-www-form-urlencoded")
				if err != nil {
					return err
				}
				if statusCode >= 400 {
					return fmt.Errorf("failed to add item to cart (HTTP %d)", statusCode)
				}

				fmt.Fprintln(cmd.OutOrStdout(), "Successfully added item to cart!")
				return showCartAndPricing(ctx, c, cmd.OutOrStdout(), flags)
			},
		}
		orderAddCmd.Flags().StringVar(&orderAddEvent, "event", "", "Event S-ID or URL")
		orderAddCmd.Flags().IntVar(&orderAddQty, "qty", 1, "Quantity to add")
		orderAddCmd.Flags().StringSliceVar(&orderAddOptions, "option", []string{}, "Item option ID or label substring (can be repeated)")
		orderAddCmd.Flags().StringVar(&orderAddNote, "note", "", "Special instructions")
		orderAddCmd.Flags().StringVar(&orderAddVendor, "vendor", "", "Filter matches by vendor name")
		orderAddCmd.Flags().BoolVar(&orderAddConfirm, "confirm", false, "Confirm and execute the add to cart")

		// order cart
		var orderCartEvent string
		orderCartCmd := &cobra.Command{
			Use:   "cart",
			Short: "View current cart items and pricing",
			Example: strings.Trim(`
  fooda-pp-cli order cart
`, "\n"),
			Annotations: map[string]string{"pp:data-source": "live", "mcp:read-only": "true"},
			RunE: func(cmd *cobra.Command, args []string) error {
				if flags.dataSource == "local" {
					return fmt.Errorf("order cart is live-only; --data-source local is not supported")
				}
				if dryRunOK(flags) {
					return writeDryRun(cmd.OutOrStdout(), flags, "order cart")
				}

				ctx, cancel := boundCtx(cmd.Context(), flags)
				defer cancel()

				c, err := flags.newClient()
				if err != nil {
					return err
				}

				return showCartAndPricing(ctx, c, cmd.OutOrStdout(), flags)
			},
		}
		orderCartCmd.Flags().StringVar(&orderCartEvent, "event", "", "Event S-ID")

		// order place
		var orderPlaceEvent string
		var orderPlaceLocation string
		var orderPlaceAllowCharge bool
		var orderPlaceConfirm bool

		orderPlaceCmd := &cobra.Command{
			Use:   "place",
			Short: "Place your food order (dry-run by default)",
			Example: strings.Trim(`
  # Dry-run: preview order details, pricing and request body
  fooda-pp-cli order place --event S609348
  # Confirm and place the order
  fooda-pp-cli order place --event S609348 --confirm
`, "\n"),
			Annotations: map[string]string{"pp:data-source": "live"},
			RunE: func(cmd *cobra.Command, args []string) error {
				if flags.dataSource == "local" {
					return fmt.Errorf("order place is live-only; --data-source local is not supported")
				}
				if dryRunOK(flags) {
					return writeDryRun(cmd.OutOrStdout(), flags, "order place")
				}
				if orderPlaceConfirm && cliutil.IsAnyHarness() {
					return writeHarnessRefusal(cmd.OutOrStdout(), flags, "order place")
				}

				ctx, cancel := boundCtx(cmd.Context(), flags)
				defer cancel()

				c, err := flags.newClient()
				if err != nil {
					return err
				}

				_, _, err = getAccountAndBuilding(ctx, c, flags)
				if err != nil {
					return err
				}

				eventID := orderPlaceEvent
				if eventID == "" {
					eventID, err = getUpcomingEvent(ctx, c, flags)
					if err != nil {
						return err
					}
				}

				numericID := strings.TrimPrefix(eventID, "S")
				numericID = strings.TrimPrefix(numericID, "s")
				if !isNumeric(numericID) {
					u, err := url.Parse(eventID)
					if err == nil && u.Path != "" {
						parts := strings.Split(u.Path, "/select_events/")
						if len(parts) > 1 {
							numericID = strings.TrimPrefix(strings.Split(parts[1], "/")[0], "S")
						}
					}
				}

				rawEventInfo, statusCode, err := makeAPIRequest(ctx, c, "GET", "/api/v2/select_events/by?id="+numericID, nil, "")
				if err != nil {
					return err
				}
				if statusCode >= 400 {
					return fmt.Errorf("failed to fetch event info (HTTP %d)", statusCode)
				}

				var eventEnvelope struct {
					SelectEvent struct {
						ID        int `json:"id"`
						Locations []struct {
							ID                   int    `json:"id"`
							DeliveryBuildingName string `json:"delivery_building_name"`
							DeliveryLocationName string `json:"delivery_location_name"`
						} `json:"locations"`
						DeliveryTime          string `json:"delivery_time"`
						OrderingWindowEndTime string `json:"ordering_window_end_time"`
						Status                string `json:"status"`
					} `json:"select_event"`
				}
				if err := json.Unmarshal(rawEventInfo, &eventEnvelope); err != nil {
					return err
				}

				ev := eventEnvelope.SelectEvent
				if ev.ID == 0 {
					return notFoundErr(fmt.Errorf("select event %s not found on server", eventID))
				}

				if ev.OrderingWindowEndTime != "" {
					windowEnd, err := time.Parse(time.RFC3339, ev.OrderingWindowEndTime)
					if err == nil && time.Now().After(windowEnd) {
						return usageErr(fmt.Errorf("ordering window closed at %s; cannot place order", ev.OrderingWindowEndTime))
					}
				}

				var locationID int
				if len(ev.Locations) == 0 {
					return fmt.Errorf("no delivery locations found for this select event")
				} else if len(ev.Locations) == 1 {
					locationID = ev.Locations[0].ID
				} else {
					if orderPlaceLocation == "" {
						var locs []string
						for _, loc := range ev.Locations {
							locs = append(locs, fmt.Sprintf("  - ID: %d (%s at %s)", loc.ID, loc.DeliveryLocationName, loc.DeliveryBuildingName))
						}
						return usageErr(fmt.Errorf("multiple delivery locations available; please specify --location ID from:\n%s", strings.Join(locs, "\n")))
					}
					parsedLoc, err := strconv.Atoi(orderPlaceLocation)
					if err != nil {
						return usageErr(fmt.Errorf("invalid --location ID: %s", orderPlaceLocation))
					}
					locationID = parsedLoc
				}

				rawPrice, statusCode, err := makeAPIRequest(ctx, c, "GET", "/api/v1/cart/price", nil, "")
				if err != nil {
					return apiErr(err)
				}
				if statusCode >= 400 {
					return apiErr(fmt.Errorf("failed to fetch cart pricing: HTTP status %d", statusCode))
				}
				var priceEnvelope struct {
					Data struct {
						Pricing struct {
							TotalCents *int `json:"total_cents"`
						} `json:"pricing"`
					} `json:"data"`
				}
				if err := json.Unmarshal(rawPrice, &priceEnvelope); err != nil {
					return apiErr(fmt.Errorf("malformed pricing JSON response: %w", err))
				}
				if priceEnvelope.Data.Pricing.TotalCents == nil || *priceEnvelope.Data.Pricing.TotalCents < 0 {
					return apiErr(fmt.Errorf("pricing total_cents is missing, null, or negative in response"))
				}

				rawItems, statusCode, err := makeAPIRequest(ctx, c, "GET", "/api/v1/cart/items", nil, "")
				if err != nil {
					return apiErr(err)
				}
				if statusCode >= 400 {
					return apiErr(fmt.Errorf("failed to fetch cart items: HTTP status %d", statusCode))
				}
				var cartEnvelope struct {
					Data struct {
						Items []map[string]any `json:"items"`
					} `json:"data"`
				}
				if err := json.Unmarshal(rawItems, &cartEnvelope); err != nil {
					return apiErr(fmt.Errorf("malformed cart items JSON response: %w", err))
				}

				if len(cartEnvelope.Data.Items) == 0 {
					return usageErr(fmt.Errorf("your cart is empty; please add items using 'order add' before placing an order"))
				}

				totalCents := *priceEnvelope.Data.Pricing.TotalCents
				if totalCents > 0 && !orderPlaceAllowCharge {
					return usageErr(fmt.Errorf("order requires an out-of-pocket payment of $%.2f; please specify --allow-charge to authorize this charge", float64(totalCents)/100.0))
				}

				type orderLocation struct {
					LocationID    int `json:"location_id"`
					SelectEventID int `json:"select_event_id"`
				}
				type orderPayload struct {
					Locations            []orderLocation `json:"locations"`
					CardID               any             `json:"card_id"`
					TotalCentsAuthorized int             `json:"total_cents_authorized"`
				}

				payload := orderPayload{
					CardID:               nil,
					TotalCentsAuthorized: totalCents,
				}
				payload.Locations = append(payload.Locations, orderLocation{
					LocationID:    locationID,
					SelectEventID: ev.ID,
				})

				rawPayload, _ := json.Marshal(payload)

				if flags.asJSON {
					if !orderPlaceConfirm {
						type placePreviewResult struct {
							DryRun  bool `json:"dry_run"`
							Cart    any  `json:"cart"`
							Pricing any  `json:"pricing"`
							Event   struct {
								DeliveryTime string `json:"delivery_time"`
								WindowEnd    string `json:"window_end"`
							} `json:"event"`
							Request string `json:"request"`
						}

						out := placePreviewResult{
							DryRun: true,
						}
						var cartData struct {
							Data struct {
								Items []map[string]any `json:"items"`
							} `json:"data"`
						}
						_ = json.Unmarshal(rawItems, &cartData)
						var pricingData struct {
							Data struct {
								Pricing any `json:"pricing"`
							} `json:"data"`
						}
						_ = json.Unmarshal(rawPrice, &pricingData)

						out.Cart = cartData.Data.Items
						out.Pricing = pricingData.Data.Pricing
						out.Event.DeliveryTime = ev.DeliveryTime
						out.Event.WindowEnd = ev.OrderingWindowEndTime
						out.Request = fmt.Sprintf("POST /api/v1/orders with %s", string(rawPayload))

						return printJSONFiltered(cmd.OutOrStdout(), out, flags)
					}

					rawResponse, statusCode, err := makeAPIRequest(ctx, c, "POST", "/api/v1/orders", rawPayload, "application/json")
					if err != nil {
						return err
					}
					if statusCode >= 400 {
						return fmt.Errorf("failed to place order: POST /api/v1/orders returned HTTP %d", statusCode)
					}

					type confirmedPlaceResult struct {
						Placed      bool            `json:"placed"`
						Response    json.RawMessage `json:"response"`
						LatestOrder any             `json:"latest_order,omitempty"`
					}

					out := confirmedPlaceResult{
						Placed:   true,
						Response: json.RawMessage(rawResponse),
					}

					rawOrders, err := c.GetHTML(ctx, "/settings/orders")
					if err == nil {
						props, err := client.ParsePastOrders(string(rawOrders))
						if err == nil && len(props.Presenter) > 0 {
							out.LatestOrder = props.Presenter[0].Order
						}
					}

					return printJSONFiltered(cmd.OutOrStdout(), out, flags)
				}

				if !orderPlaceConfirm {
					fmt.Fprintln(cmd.OutOrStdout(), "=== Order Placement Preview ===")
					_ = showCartAndPricing(ctx, c, cmd.OutOrStdout(), flags)
					fmt.Fprintf(cmd.OutOrStdout(), "\nDelivery Time:         %s\n", ev.DeliveryTime)
					fmt.Fprintf(cmd.OutOrStdout(), "Ordering Window Ends:  %s\n", ev.OrderingWindowEndTime)
					fmt.Fprintf(cmd.OutOrStdout(), "Location ID:           %d\n", locationID)
					fmt.Fprintf(cmd.OutOrStdout(), "Request Body:          %s\n", string(rawPayload))
					fmt.Fprintln(cmd.OutOrStdout(), "\nThis is a DRY-RUN. Run with --confirm to place this order.")
					return nil
				}

				rawResponse, statusCode, err := makeAPIRequest(ctx, c, "POST", "/api/v1/orders", rawPayload, "application/json")
				if err != nil {
					return err
				}
				if statusCode >= 400 {
					return fmt.Errorf("failed to place order: POST /api/v1/orders returned HTTP %d", statusCode)
				}

				fmt.Fprintln(cmd.OutOrStdout(), "Order successfully placed!")
				fmt.Fprintf(cmd.OutOrStdout(), "Response: %s\n", string(rawResponse))

				rawOrders, err := c.GetHTML(ctx, "/settings/orders")
				if err == nil {
					props, err := client.ParsePastOrders(string(rawOrders))
					if err == nil && len(props.Presenter) > 0 {
						newest := props.Presenter[0].Order
						fmt.Fprintf(cmd.OutOrStdout(), "\nNewest Order Details (from Server):\n")
						fmt.Fprintf(cmd.OutOrStdout(), "  UUID:     %s\n", newest.RequestID)
						fmt.Fprintf(cmd.OutOrStdout(), "  Vendor:   %s\n", strings.Join(newest.VendorNames, ", "))
						fmt.Fprintf(cmd.OutOrStdout(), "  Delivery: %s\n", newest.Delivery)
					}
				}

				return nil
			},
		}
		orderPlaceCmd.Flags().StringVar(&orderPlaceEvent, "event", "", "Event S-ID")
		orderPlaceCmd.Flags().StringVar(&orderPlaceLocation, "location", "", "Specific delivery location ID")
		orderPlaceCmd.Flags().BoolVar(&orderPlaceAllowCharge, "allow-charge", false, "Authorize paying an out-of-pocket total price > $0.00")
		orderPlaceCmd.Flags().BoolVar(&orderPlaceConfirm, "confirm", false, "Confirm and execute the order placement")

		// order plan
		var orderPlanEvent string
		var orderPlanBudget float64
		var orderPlanAnchor string

		orderPlanCmd := &cobra.Command{
			Use:   "plan",
			Short: "Suggest a budget-optimized add-on item for an event",
			Example: strings.Trim(`
  # Plan order with $20 budget and 'Beef Bulgogi Bowl' as anchor
  fooda-pp-cli order plan --event S609348 --budget 20 --anchor "Beef Bulgogi Bowl"
`, "\n"),
			Annotations: map[string]string{"pp:data-source": "live", "mcp:read-only": "true", "pp:happy-args": "event=S609348,anchor=Beef Bulgogi Bowl"},
			RunE: func(cmd *cobra.Command, args []string) error {
				if flags.dataSource == "local" {
					return fmt.Errorf("order plan is live-only; --data-source local is not supported")
				}
				if dryRunOK(flags) {
					return writeDryRun(cmd.OutOrStdout(), flags, "order plan")
				}

				if orderPlanEvent == "" {
					return usageErr(fmt.Errorf("missing required flag --event"))
				}
				if orderPlanAnchor == "" {
					return usageErr(fmt.Errorf("missing required flag --anchor"))
				}

				ctx, cancel := boundCtx(cmd.Context(), flags)
				defer cancel()

				c, err := flags.newClient()
				if err != nil {
					return err
				}

				acct, _, err := getAccountAndBuilding(ctx, c, flags)
				if err != nil {
					return err
				}

				var eventPath string
				if strings.HasPrefix(orderPlanEvent, "S") || strings.HasPrefix(orderPlanEvent, "s") {
					eventPath = fmt.Sprintf("/accounts/%s/select_events/%s/items", acct, orderPlanEvent)
				} else {
					eventPath = urlToPath(orderPlanEvent)
				}

				rawMenu, err := c.GetHTML(ctx, eventPath)
				if err != nil {
					return err
				}
				menuItems, err := client.ParseMenuItems(string(rawMenu))
				if err != nil {
					return err
				}

				var anchorItem client.MenuItem
				foundAnchor := false
				for _, it := range menuItems {
					if strings.Contains(strings.ToLower(it.Name), strings.ToLower(orderPlanAnchor)) {
						anchorItem = it
						foundAnchor = true
						break
					}
				}

				if !foundAnchor {
					return notFoundErr(fmt.Errorf("anchor item %q not found in menu", orderPlanAnchor))
				}

				addOn, foundAddOn := planOrder(orderPlanBudget, anchorItem.Name, menuItems)

				subtotal := anchorItem.Price
				remaining := orderPlanBudget - anchorItem.Price
				if foundAddOn {
					subtotal = anchorItem.Price + addOn.Price
					remaining = orderPlanBudget - subtotal
				}

				if flags.asJSON {
					type planOutput struct {
						Anchor         client.MenuItem  `json:"anchor"`
						Addon          *client.MenuItem `json:"addon,omitempty"`
						SubtotalCents  int              `json:"subtotal_cents"`
						RemainingCents int              `json:"remaining_cents"`
						Commands       []string         `json:"commands"`
						Note           string           `json:"note"`
					}

					out := planOutput{
						Anchor:         anchorItem,
						SubtotalCents:  int(math.Round(subtotal * 100)),
						RemainingCents: int(math.Round(remaining * 100)),
						Note:           "Subtotal does not include tax/fees.",
					}
					out.Commands = append(out.Commands, fmt.Sprintf("fooda-pp-cli order add %q --event %s", anchorItem.ID, orderPlanEvent))
					if foundAddOn {
						out.Addon = &addOn
						out.Commands = append(out.Commands, fmt.Sprintf("fooda-pp-cli order add %q --event %s", addOn.ID, orderPlanEvent))
					}

					return printJSONFiltered(cmd.OutOrStdout(), out, flags)
				}

				fmt.Fprintln(cmd.OutOrStdout(), "=== Budget Plan Suggestion ===")
				fmt.Fprintf(cmd.OutOrStdout(), "  Budget:       $%.2f\n", orderPlanBudget)
				fmt.Fprintf(cmd.OutOrStdout(), "  Anchor Item:  %s (Vendor: %s, Price: $%.2f)\n", anchorItem.Name, anchorItem.VendorName, anchorItem.Price)

				if !foundAddOn {
					fmt.Fprintln(cmd.OutOrStdout(), "  No suitable add-on item found within budget.")
					fmt.Fprintf(cmd.OutOrStdout(), "\nTo order the anchor item, run:\n")
					fmt.Fprintf(cmd.OutOrStdout(), "  fooda-pp-cli order add %q --event %s\n", anchorItem.ID, orderPlanEvent)
					return nil
				}

				fmt.Fprintf(cmd.OutOrStdout(), "  Add-on Item:  %s (Vendor: %s, Price: $%.2f)\n", addOn.Name, addOn.VendorName, addOn.Price)
				fmt.Fprintf(cmd.OutOrStdout(), "  Subtotal:     $%.2f (Remaining: $%.2f)\n", subtotal, remaining)
				fmt.Fprintln(cmd.OutOrStdout(), "  Note: Subtotal does not include tax/fees.")

				fmt.Fprintln(cmd.OutOrStdout(), "\nTo add these to your cart, run:")
				fmt.Fprintf(cmd.OutOrStdout(), "  fooda-pp-cli order add %q --event %s\n", anchorItem.ID, orderPlanEvent)
				fmt.Fprintf(cmd.OutOrStdout(), "  fooda-pp-cli order add %q --event %s\n", addOn.ID, orderPlanEvent)
				return nil
			},
		}
		orderPlanCmd.Flags().StringVar(&orderPlanEvent, "event", "", "Event S-ID")
		orderPlanCmd.Flags().Float64Var(&orderPlanBudget, "budget", 20.0, "Budget limit")
		orderPlanCmd.Flags().StringVar(&orderPlanAnchor, "anchor", "", "Anchor item name")

		orderCmd.AddCommand(orderAddCmd, orderCartCmd, orderPlaceCmd, orderPlanCmd)
		replaceCommand(orderCmd)

		// 7. sync
		syncCmd := &cobra.Command{
			Use:   "sync",
			Short: "Synchronize live events and past orders to the local SQLite database",
			Example: strings.Trim(`
  fooda-pp-cli sync
`, "\n"),
			RunE: func(cmd *cobra.Command, args []string) error {
				if dryRunOK(flags) {
					return writeDryRun(cmd.OutOrStdout(), flags, "sync")
				}

				ctx, cancel := boundCtx(cmd.Context(), flags)
				defer cancel()

				c, err := flags.newClient()
				if err != nil {
					return err
				}

				acct, bldg, err := getAccountAndBuilding(ctx, c, flags)
				if err != nil {
					return err
				}

				dbPath := defaultDBPath("fooda-pp-cli")
				db, err := store.OpenWithContext(ctx, dbPath)
				if err != nil {
					return fmt.Errorf("failed to open local store: %w", err)
				}
				defer db.Close()

				fmt.Fprintln(cmd.OutOrStderr(), "Syncing past orders...")
				rawOrders, err := c.GetHTML(ctx, "/settings/orders")
				if err != nil {
					return fmt.Errorf("failed to fetch orders: %w", err)
				}
				if len(rawOrders) == 0 {
					return nil
				}

				ordersProps, err := client.ParsePastOrders(string(rawOrders))
				if err != nil {
					return fmt.Errorf("failed to parse orders: %w", err)
				}

				orderCount := 0
				failedDetails := 0
				attemptedDetails := 0
				for _, pres := range ordersProps.Presenter {
					o := pres.Order
					uuid := o.RequestID
					if uuid == "" {
						continue
					}

					// Store each order individually by its own ID
					var singleProp client.PastOrdersProps
					singleProp.Presenter = []struct {
						Order struct {
							ID                     int      `json:"id"`
							EventType              string   `json:"event_type"`
							ItemNames              []string `json:"item_names"`
							OrderFulfilledTimeUnix int64    `json:"order_fulfilled_time_unix"`
							VendorNames            []string `json:"vendor_names"`
							Status                 string   `json:"status"`
							Delivery               string   `json:"delivery"`
							RequestID              string   `json:"request_id"`
							PaymentCents           int      `json:"payment_cents"`
						} `json:"order"`
					}{pres}

					singleJSON, err := json.Marshal(singleProp)
					if err != nil {
						return fmt.Errorf("failed to marshal order %s: %w", uuid, err)
					}
					err = db.Upsert("order", uuid, singleJSON)
					if err != nil {
						return fmt.Errorf("failed to upsert order %s: %w", uuid, err)
					}

					existing, getErr := db.Get("order_detail", uuid)
					if getErr != nil || len(existing) == 0 {
						attemptedDetails++
						fmt.Fprintf(cmd.OutOrStderr(), "Fetching detail for order %s...\n", uuid)
						rawDet, err := c.GetHTML(ctx, "/settings/select_order/"+uuid)
						if err != nil {
							failedDetails++
						} else {
							detProps, err := client.ParseOrderDetail(string(rawDet))
							if err != nil {
								failedDetails++
							} else {
								detJSON, err := json.Marshal(detProps)
								if err != nil {
									failedDetails++
								} else {
									err = db.Upsert("order_detail", uuid, detJSON)
									if err != nil {
										failedDetails++
									}
								}
							}
						}
					}
					orderCount++
				}

				if failedDetails > 0 {
					fmt.Fprintf(cmd.OutOrStderr(), "Warning: failed to sync %d of %d order details\n", failedDetails, attemptedDetails)
				}

				fmt.Fprintln(cmd.OutOrStderr(), "Syncing upcoming events...")
				startStr := time.Now().Format("2006-01-02")
				endStr := time.Now().AddDate(0, 0, 7).Format("2006-01-02")
				variables := map[string]any{
					"input": map[string]any{
						"statuses":      []string{"ACTIVE", "PROPOSED"},
						"productTypes":  []string{"POPUP", "CAFE", "DELIVERY", "CATERING"},
						"accountId":     acct,
						"buildingId":    bldg,
						"endTimeAfter":  startStr,
						"endTimeBefore": endStr,
					},
				}

				rawEv, err := c.Query(ctx, client.SearchPublicEventsQuery, variables)
				if err != nil {
					return fmt.Errorf("failed to query events: %w", err)
				}

				events, err := parseEventsFromRaw(rawEv)
				if err != nil {
					return fmt.Errorf("failed to parse events: %w", err)
				}

				for _, ev := range events {
					evJSON, err := json.Marshal(ev)
					if err != nil {
						return fmt.Errorf("failed to marshal event %s: %w", ev.ID, err)
					}
					err = db.Upsert("event", ev.ID, evJSON)
					if err != nil {
						return fmt.Errorf("failed to upsert event %s: %w", ev.ID, err)
					}
				}

				err = db.SaveSyncState("order", "complete", orderCount)
				if err != nil {
					return fmt.Errorf("failed to save order sync state: %w", err)
				}
				err = db.SaveSyncState("event", "complete", len(events))
				if err != nil {
					return fmt.Errorf("failed to save event sync state: %w", err)
				}

				fmt.Fprintf(cmd.OutOrStdout(), "Sync complete. Synced %d orders and %d upcoming events.\n", orderCount, len(events))
				return nil
			},
		}
		replaceCommand(syncCmd)

		// TRANSCENDENCE COMMANDS

		// T1: served-history
		servedHistCmd := &cobra.Command{
			Use:   "served-history",
			Short: "Show food history based on synced orders",
			Example: strings.Trim(`
  fooda-pp-cli served-history --since 90d
`, "\n"),
			Annotations: map[string]string{"pp:data-source": "local", "mcp:read-only": "true"},
			RunE: func(cmd *cobra.Command, args []string) error {
				if flags.dataSource == "live" {
					return fmt.Errorf("served-history is local-only; --data-source live is not supported")
				}

				if dryRunOK(flags) {
					return writeDryRun(cmd.OutOrStdout(), flags, "served-history")
				}

				db, err := store.OpenReadOnlyContext(cmd.Context(), defaultDBPath("fooda-pp-cli"))
				if err != nil || db == nil {
					return fmt.Errorf("local database not found. Run 'sync' first")
				}
				defer db.Close()

				sinceStr, _ := cmd.Flags().GetString("since")
				cutoff, err := parseSince(sinceStr)
				if err != nil {
					return err
				}

				orders, err := loadAndDeduplicateOrders(db)
				if err != nil {
					return err
				}

				type historyRow struct {
					Date     string  `json:"date"`
					Vendor   string  `json:"vendor"`
					Items    string  `json:"items"`
					Price    float64 `json:"price"`
					TimeUnix int64   `json:"time_unix"`
				}

				var rows []historyRow
				for _, props := range orders {
					for _, pres := range props.Presenter {
						o := pres.Order
						orderTime := time.Unix(o.OrderFulfilledTimeUnix, 0)
						if orderTime.Before(cutoff) {
							continue
						}
						rows = append(rows, historyRow{
							Date:     orderTime.Format("2006-01-02"),
							Vendor:   strings.Join(o.VendorNames, ", "),
							Items:    strings.Join(o.ItemNames, "; "),
							Price:    float64(o.PaymentCents) / 100.0,
							TimeUnix: o.OrderFulfilledTimeUnix,
						})
					}
				}

				sort.Slice(rows, func(i, j int) bool {
					return rows[i].TimeUnix > rows[j].TimeUnix
				})

				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
				}

				w := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintln(w, "Date\tVendor\tItems Ordered\tPrice Paid")
				for _, r := range rows {
					fmt.Fprintf(w, "%s\t%s\t%s\t$%.2f\n", r.Date, r.Vendor, r.Items, r.Price)
				}
				_ = w.Flush()
				return nil
			},
		}
		var servedHistSince string
		servedHistCmd.Flags().StringVar(&servedHistSince, "since", "90d", "Filter history (e.g. 90d, 120d, 6mo)")
		replaceCommand(servedHistCmd)

		// T2: venue-rotation
		venueRotCmd := &cobra.Command{
			Use:   "venue-rotation",
			Short: "Analyze vendor order frequency and last ordered date",
			Example: strings.Trim(`
  # Analyze vendor rotation using default 90-day history
  fooda-pp-cli venue-rotation

  # Analyze rotation over a different timeframe (e.g., 6 months)
  fooda-pp-cli venue-rotation --since 6mo
`, "\n"),
			Annotations: map[string]string{"pp:data-source": "local", "mcp:read-only": "true"},
			RunE: func(cmd *cobra.Command, args []string) error {
				if flags.dataSource == "live" {
					return fmt.Errorf("venue-rotation is local-only; --data-source live is not supported")
				}

				if dryRunOK(flags) {
					return writeDryRun(cmd.OutOrStdout(), flags, "venue-rotation")
				}

				db, err := store.OpenReadOnlyContext(cmd.Context(), defaultDBPath("fooda-pp-cli"))
				if err != nil || db == nil {
					return fmt.Errorf("local database not found. Run 'sync' first")
				}
				defer db.Close()

				sinceStr, _ := cmd.Flags().GetString("since")
				cutoff, err := parseSince(sinceStr)
				if err != nil {
					return err
				}

				orders, err := loadAndDeduplicateOrders(db)
				if err != nil {
					return err
				}

				type rotInfo struct {
					Vendor string
					Count  int
					Last   time.Time
				}

				rotMap := map[string]*rotInfo{}
				for _, props := range orders {
					for _, pres := range props.Presenter {
						o := pres.Order
						orderTime := time.Unix(o.OrderFulfilledTimeUnix, 0)
						if orderTime.Before(cutoff) {
							continue
						}
						for _, name := range o.VendorNames {
							ri := rotMap[name]
							if ri == nil {
								ri = &rotInfo{Vendor: name}
								rotMap[name] = ri
							}
							ri.Count++
							if orderTime.After(ri.Last) {
								ri.Last = orderTime
							}
						}
					}
				}

				var results []*rotInfo
				for _, ri := range rotMap {
					results = append(results, ri)
				}

				sort.Slice(results, func(i, j int) bool {
					return results[i].Count > results[j].Count
				})

				type printableRot struct {
					Vendor      string `json:"vendor"`
					OrderCount  int    `json:"order_count"`
					LastOrdered string `json:"last_ordered"`
				}

				var prList []printableRot
				for _, ri := range results {
					prList = append(prList, printableRot{
						Vendor:      ri.Vendor,
						OrderCount:  ri.Count,
						LastOrdered: ri.Last.Format("2006-01-02"),
					})
				}

				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), prList, flags)
				}

				w := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintln(w, "Vendor Name\tFrequency (Count)\tLast Ordered")
				for _, pr := range prList {
					fmt.Fprintf(w, "%s\t%d\t%s\n", pr.Vendor, pr.OrderCount, pr.LastOrdered)
				}
				_ = w.Flush()
				return nil
			},
		}
		var venueRotSince string
		venueRotCmd.Flags().StringVar(&venueRotSince, "since", "120d", "Filter range (e.g. 90d, 120d, 6mo)")
		replaceCommand(venueRotCmd)

		// T3: spend-trends
		spendTrendsCmd := &cobra.Command{
			Use:   "spend-trends",
			Short: "Generate time-series spend reports showing subsidy and out-of-pocket costs",
			Example: strings.Trim(`
  fooda-pp-cli spend-trends --since 6mo --group-by month
`, "\n"),
			Annotations: map[string]string{"pp:data-source": "local", "mcp:read-only": "true"},
			RunE: func(cmd *cobra.Command, args []string) error {
				if flags.dataSource == "live" {
					return fmt.Errorf("spend-trends is local-only; --data-source live is not supported")
				}

				if dryRunOK(flags) {
					return writeDryRun(cmd.OutOrStdout(), flags, "spend-trends")
				}

				db, err := store.OpenReadOnlyContext(cmd.Context(), defaultDBPath("fooda-pp-cli"))
				if err != nil || db == nil {
					return fmt.Errorf("local database not found. Run 'sync' first")
				}
				defer db.Close()

				sinceStr, _ := cmd.Flags().GetString("since")
				groupBy, _ := cmd.Flags().GetString("group-by")
				cutoff, err := parseSince(sinceStr)
				if err != nil {
					return err
				}

				orders, err := loadAndDeduplicateOrders(db)
				if err != nil {
					return err
				}

				type trendBucket struct {
					Period     string  `json:"period"`
					Subsidized float64 `json:"subsidized"`
					Paid       float64 `json:"paid"`
					Total      float64 `json:"total"`
				}

				buckets := map[string]*trendBucket{}

				for _, props := range orders {
					for _, pres := range props.Presenter {
						o := pres.Order
						orderTime := time.Unix(o.OrderFulfilledTimeUnix, 0)
						if orderTime.Before(cutoff) {
							continue
						}

						period := orderTime.Format("2006-01")
						if groupBy == "week" {
							y, w := orderTime.ISOWeek()
							period = fmt.Sprintf("%d-W%02d", y, w)
						}

						tb := buckets[period]
						if tb == nil {
							tb = &trendBucket{Period: period}
							buckets[period] = tb
						}

						paid := float64(o.PaymentCents) / 100.0
						subsidized := 0.0

						if rawDet, getErr := db.Get("order_detail", o.RequestID); getErr == nil && len(rawDet) > 0 {
							var parsedDet client.OrderDetailProps
							if json.Unmarshal(rawDet, &parsedDet) == nil {
								subsidized = parseAmount(parsedDet.OrderData.FormattedSubsidyAmount)
								parsedPaid := parseAmount(parsedDet.OrderData.FormattedTotalAmount)
								if parsedPaid > 0 {
									paid = parsedPaid
								}
							}
						}

						tb.Paid += paid
						tb.Subsidized += subsidized
						tb.Total += (paid + subsidized)
					}
				}

				var results []*trendBucket
				for _, tb := range buckets {
					results = append(results, tb)
				}

				sort.Slice(results, func(i, j int) bool {
					return results[i].Period < results[j].Period
				})

				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), results, flags)
				}

				w := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintln(w, "Period\tSubsidy-Covered\tOut-Of-Pocket\tTotal Spend")
				for _, r := range results {
					fmt.Fprintf(w, "%s\t$%.2f\t$%.2f\t$%.2f\n", r.Period, r.Subsidized, r.Paid, r.Total)
				}
				_ = w.Flush()
				return nil
			},
		}
		var spendTrendsSince, spendTrendsGroupBy string
		spendTrendsCmd.Flags().StringVar(&spendTrendsSince, "since", "6mo", "Filter range (e.g. 90d, 120d, 6mo)")
		spendTrendsCmd.Flags().StringVar(&spendTrendsGroupBy, "group-by", "month", "Group spending by 'month' or 'week'")
		replaceCommand(spendTrendsCmd)

		// T4: week-ahead
		var weekAheadDays int
		weekAheadCmd := &cobra.Command{
			Use:   "week-ahead",
			Short: "View live events for the next 7 days in a daily one-line digest",
			Example: strings.Trim(`
  fooda-pp-cli week-ahead
`, "\n"),
			Annotations: map[string]string{"pp:data-source": "live", "mcp:read-only": "true"},
			RunE: func(cmd *cobra.Command, args []string) error {
				if flags.dataSource == "local" {
					return fmt.Errorf("week-ahead is live-only; --data-source local is not supported")
				}

				if dryRunOK(flags) {
					return writeDryRun(cmd.OutOrStdout(), flags, "week-ahead")
				}

				ctx, cancel := boundCtx(cmd.Context(), flags)
				defer cancel()

				c, err := flags.newClient()
				if err != nil {
					return err
				}

				acct, bldg, err := getAccountAndBuilding(ctx, c, flags)
				if err != nil {
					return err
				}

				variables := map[string]any{
					"input": map[string]any{
						"statuses":      []string{"ACTIVE", "PROPOSED"},
						"productTypes":  []string{"POPUP", "CAFE", "DELIVERY", "CATERING"},
						"accountId":     acct,
						"buildingId":    bldg,
						"endTimeAfter":  time.Now().Format("2006-01-02"),
						"endTimeBefore": time.Now().AddDate(0, 0, weekAheadDays).Format("2006-01-02"),
					},
				}

				raw, err := c.Query(ctx, client.SearchPublicEventsQuery, variables)
				if err != nil {
					return err
				}
				if len(raw) == 0 {
					return nil
				}

				events, err := parseEventsFromRaw(raw)
				if err != nil {
					return err
				}

				type dayDigest struct {
					Date        string   `json:"date"`
					EventType   string   `json:"event_type"`
					Restaurants []string `json:"restaurants"`
				}

				type dateGroup struct {
					Date        string
					Types       map[string]bool
					Restaurants []string
				}

				groups := map[string]*dateGroup{}
				for _, ev := range events {
					serviceDate, _, err := getEventServiceDate(ev)
					if err != nil {
						continue
					}

					serviceDateStr := serviceDate.Format("2006-01-02")
					tMidnight, _ := time.ParseInLocation("2006-01-02", serviceDateStr, time.Local)
					todayMidnight, _ := time.ParseInLocation("2006-01-02", time.Now().Format("2006-01-02"), time.Local)
					maxMidnight := todayMidnight.AddDate(0, 0, weekAheadDays)

					if tMidnight.Before(todayMidnight) || tMidnight.After(maxMidnight) {
						continue
					}

					g := groups[serviceDateStr]
					if g == nil {
						g = &dateGroup{
							Date:        serviceDate.Format("2006-01-02 (Monday)"),
							Types:       map[string]bool{},
							Restaurants: []string{},
						}
						groups[serviceDateStr] = g
					}

					cleanType := strings.TrimSuffix(ev.Typename, "EventPublic")
					g.Types[cleanType] = true

					for _, r := range ev.Restaurants {
						name := strings.TrimSpace(r.Name)
						seen := false
						for _, ex := range g.Restaurants {
							if ex == name {
								seen = true
								break
							}
						}
						if !seen && name != "" {
							g.Restaurants = append(g.Restaurants, name)
						}
					}
				}

				var keys []string
				for k := range groups {
					keys = append(keys, k)
				}
				sort.Strings(keys)

				var results []dayDigest
				for _, k := range keys {
					g := groups[k]
					var types []string
					for t := range g.Types {
						types = append(types, t)
					}
					sort.Strings(types)
					typeStr := strings.Join(types, ", ")

					results = append(results, dayDigest{
						Date:        g.Date,
						EventType:   typeStr,
						Restaurants: g.Restaurants,
					})
				}

				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), results, flags)
				}

				for _, d := range results {
					fmt.Fprintf(cmd.OutOrStdout(), "%s [%s]: %s\n", d.Date, d.EventType, strings.Join(d.Restaurants, ", "))
				}
				if len(results) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No upcoming events found.")
				}
				return nil
			},
		}
		weekAheadCmd.Flags().IntVar(&weekAheadDays, "days", 7, "Number of days to check ahead")
		replaceCommand(weekAheadCmd)

		// T5: subsidy-status
		subsidyStatusCmd := &cobra.Command{
			Use:   "subsidy-status",
			Short: "Display today's remaining dollar subsidy and validity details",
			Example: strings.Trim(`
  fooda-pp-cli subsidy-status
`, "\n"),
			Annotations: map[string]string{"pp:data-source": "live", "mcp:read-only": "true"},
			RunE: func(cmd *cobra.Command, args []string) error {
				if flags.dataSource == "local" {
					return fmt.Errorf("subsidy-status is live-only; --data-source local is not supported")
				}

				if dryRunOK(flags) {
					return writeDryRun(cmd.OutOrStdout(), flags, "subsidy-status")
				}

				c, err := flags.newClient()
				if err != nil {
					return err
				}

				raw, err := c.Query(cmd.Context(), client.SubsidiesListQuery, nil)
				if err != nil {
					return err
				}
				if len(raw) == 0 {
					return nil
				}

				var envelope struct {
					GetUserSubsidies struct {
						Subsidies []struct {
							AccountName string `json:"accountName"`
							QrInfo      string `json:"qrInfo"`
							Code        string `json:"code"`
							Coverage    string `json:"coverage"`
							Name        string `json:"name"`
							State       string `json:"state"`
							ValidAt     string `json:"validAt"`
						} `json:"subsidies"`
					} `json:"getUserSubsidies"`
				}

				if err := json.Unmarshal(raw, &envelope); err != nil {
					return err
				}

				type subsStatus struct {
					Code      string `json:"code"`
					Remaining string `json:"remaining"`
					Coverage  string `json:"coverage"`
					ValidAt   string `json:"validAt"`
				}

				var results []subsStatus
				for _, s := range envelope.GetUserSubsidies.Subsidies {
					if s.State != "ACTIVE" {
						continue
					}
					remainingStr := "N/A"
					if s.QrInfo != "" {
						var qr struct {
							Subsidy struct {
								AmountRemainingCents int `json:"amount_remaining_cents"`
							} `json:"subsidy"`
						}
						if json.Unmarshal([]byte(s.QrInfo), &qr) == nil {
							remainingStr = fmt.Sprintf("$%.2f", float64(qr.Subsidy.AmountRemainingCents)/100.0)
						}
					}
					results = append(results, subsStatus{
						Code:      s.Code,
						Remaining: remainingStr,
						Coverage:  s.Coverage,
						ValidAt:   s.ValidAt,
					})
				}

				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), results, flags)
				}

				for _, ss := range results {
					fmt.Fprintf(cmd.OutOrStdout(), "Subsidy Code: %s\n", ss.Code)
					fmt.Fprintf(cmd.OutOrStdout(), "  Remaining:  %s\n", ss.Remaining)
					fmt.Fprintf(cmd.OutOrStdout(), "  Coverage:   %s\n", ss.Coverage)
					fmt.Fprintf(cmd.OutOrStdout(), "  Valid At:   %s\n", ss.ValidAt)
				}
				if len(results) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No active subsidies found.")
				}
				return nil
			},
		}
		replaceCommand(subsidyStatusCmd)

		// T6: menu-search
		var menuSearchDays, menuSearchMaxEvents int
		menuSearchCmd := &cobra.Command{
			Use:   "menu-search <keyword>",
			Short: "Filter and search menu items across today's events",
			Example: strings.Trim(`
  fooda-pp-cli menu-search vegan
`, "\n"),
			Args:        cobra.ExactArgs(1),
			Annotations: map[string]string{"pp:data-source": "live", "mcp:read-only": "true", "pp:happy-args": "keyword=chicken"},
			RunE: func(cmd *cobra.Command, args []string) error {
				if flags.dataSource == "local" {
					return fmt.Errorf("menu-search is live-only; --data-source local is not supported")
				}

				if dryRunOK(flags) {
					return writeDryRun(cmd.OutOrStdout(), flags, "menu-search")
				}

				ctx, cancel := boundCtx(cmd.Context(), flags)
				defer cancel()

				c, err := flags.newClient()
				if err != nil {
					return err
				}

				keyword := strings.ToLower(args[0])
				typeFlag, _ := cmd.Flags().GetString("type")

				acct, bldg, err := getAccountAndBuilding(ctx, c, flags)
				if err != nil {
					return err
				}

				productTypes := []string{"POPUP", "CAFE", "DELIVERY"}
				if typeFlag != "" {
					productTypes = []string{strings.ToUpper(typeFlag)}
				}

				todayStr := time.Now().Format("2006-01-02")
				variables := map[string]any{
					"input": map[string]any{
						"statuses":      []string{"ACTIVE"},
						"productTypes":  productTypes,
						"accountId":     acct,
						"buildingId":    bldg,
						"endTimeAfter":  todayStr,
						"endTimeBefore": time.Now().AddDate(0, 0, menuSearchDays).Format("2006-01-02"),
					},
				}

				rawEv, err := c.Query(ctx, client.SearchPublicEventsQuery, variables)
				if err != nil {
					return err
				}
				if len(rawEv) == 0 {
					return notFoundErr(fmt.Errorf("no upcoming events found to search in the next %d days", menuSearchDays))
				}

				events, err := parseEventsFromRaw(rawEv)
				if err != nil {
					return err
				}

				type matchedItem struct {
					EventID    string          `json:"event_id"`
					Restaurant string          `json:"restaurant"`
					Item       client.MenuItem `json:"item"`
				}

				type menuSearchGroup struct {
					Date  string        `json:"date"`
					Items []matchedItem `json:"items"`
				}

				eventDateMap := map[string]string{}
				eventFormattedDateMap := map[string]string{}
				for _, ev := range events {
					serviceDate, _, err := getEventServiceDate(ev)
					if err == nil {
						eventDateMap[ev.ID] = serviceDate.Format("2006-01-02")
						eventFormattedDateMap[ev.ID] = serviceDate.Format("2006-01-02 (Monday)")
					}
				}

				var matched []matchedItem
				failedFetches := 0
				totalFetches := 0
				for _, ev := range events {
					serviceDate, _, err := getEventServiceDate(ev)
					if err != nil {
						continue
					}

					serviceDateStr := serviceDate.Format("2006-01-02")
					tMidnight, _ := time.ParseInLocation("2006-01-02", serviceDateStr, time.Local)
					todayMidnight, _ := time.ParseInLocation("2006-01-02", time.Now().Format("2006-01-02"), time.Local)
					maxMidnight := todayMidnight.AddDate(0, 0, menuSearchDays)

					if tMidnight.Before(todayMidnight) || tMidnight.After(maxMidnight) {
						continue
					}

					if totalFetches >= menuSearchMaxEvents {
						break
					}

					totalFetches++
					if ev.URL == "" {
						failedFetches++
						continue
					}

					urlPath := urlToPath(ev.URL)
					rawHtml, err := c.GetHTML(ctx, urlPath)
					if err != nil {
						failedFetches++
						continue
					}
					items, err := client.ParseMenuItems(string(rawHtml))
					if err != nil {
						failedFetches++
						continue
					}
					for _, it := range items {
						if strings.Contains(strings.ToLower(it.Name), keyword) || strings.Contains(strings.ToLower(it.Description), keyword) {
							matched = append(matched, matchedItem{
								EventID:    ev.ID,
								Restaurant: strings.TrimSpace(it.VendorName),
								Item:       it,
							})
						}
					}
				}

				if failedFetches > 0 {
					fmt.Fprintf(cmd.OutOrStderr(), "Warning: failed to fetch menu for %d of %d events\n", failedFetches, totalFetches)
				}

				if failedFetches == totalFetches && totalFetches > 0 {
					return fmt.Errorf("failed to fetch any event menus: all %d attempts failed", totalFetches)
				}

				if len(matched) == 0 {
					return notFoundErr(fmt.Errorf("no menu items matching %q across %d events", args[0], totalFetches))
				}

				// Group matched items by date
				groups := map[string][]matchedItem{}
				for _, m := range matched {
					dStr := eventDateMap[m.EventID]
					groups[dStr] = append(groups[dStr], m)
				}

				var keys []string
				for k := range groups {
					keys = append(keys, k)
				}
				sort.Strings(keys)

				var results []menuSearchGroup
				for _, k := range keys {
					fDate := eventFormattedDateMap[groups[k][0].EventID]
					if fDate == "" {
						fDate = k
					}
					results = append(results, menuSearchGroup{
						Date:  fDate,
						Items: groups[k],
					})
				}

				if flags.asJSON {
					out := map[string]any{
						"results":        results,
						"fetch_failures": failedFetches,
					}
					return printJSONFiltered(cmd.OutOrStdout(), out, flags)
				}

				w := newTabWriter(cmd.OutOrStdout())
				for _, g := range results {
					fmt.Fprintf(w, "\nDate: %s\n", g.Date)
					fmt.Fprintln(w, "  Event ID\tRestaurant\tCategory\tItem Name\tPrice\tRestrictions")
					for _, m := range g.Items {
						restr := strings.Join(m.Item.DietaryRestrictions, ", ")
						fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t$%.2f\t%s\n", m.EventID, m.Restaurant, m.Item.Category, m.Item.Name, m.Item.Price, restr)
					}
				}
				_ = w.Flush()
				return nil
			},
		}
		menuSearchCmd.Flags().String("type", "", "Filter by product type (POPUP, CAFE, DELIVERY)")
		menuSearchCmd.Flags().IntVar(&menuSearchDays, "days", 7, "Number of days to check ahead")
		menuSearchCmd.Flags().IntVar(&menuSearchMaxEvents, "max-events", 20, "Maximum number of events to scan")
		replaceCommand(menuSearchCmd)
	})
}

// HELPERS

type parsedEvent struct {
	ID          string `json:"id"`
	StartTime   string `json:"startTime"`
	EndTime     string `json:"endTime"`
	Status      string `json:"status"`
	URL         string `json:"url"`
	Typename    string `json:"__typename"`
	Restaurants []struct {
		ID             string   `json:"id"`
		VendorID       string   `json:"vendorId"`
		Name           string   `json:"name"`
		Cuisines       []string `json:"cuisines"`
		MealPeriod     string   `json:"mealPeriod"`
		DefaultCuisine string   `json:"defaultCuisine"`
	} `json:"restaurants"`
	DeliveryWindow struct {
		StartTime string `json:"startTime"`
		EndTime   string `json:"endTime"`
	} `json:"deliveryWindow"`
}

func parseEventsFromRaw(raw json.RawMessage) ([]parsedEvent, error) {
	var envelope struct {
		SearchPublicEvents struct {
			Events struct {
				Nodes []parsedEvent `json:"nodes"`
			} `json:"events"`
		} `json:"searchPublicEvents"`
	}

	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	return envelope.SearchPublicEvents.Events.Nodes, nil
}

var (
	memoizedAccountID  string
	memoizedBuildingID string
)

func resetMemoizedAccountAndBuilding() {
	memoizedAccountID = ""
	memoizedBuildingID = ""
}

func getAccountAndBuilding(ctx context.Context, c *client.Client, flags *rootFlags) (string, string, error) {
	var acctOverride, bldgOverride string
	if flags != nil {
		acctOverride = flags.accountID
		bldgOverride = flags.buildingID
	}

	var finalAcct, finalBldg string
	if acctOverride != "" {
		finalAcct = acctOverride
	} else if memoizedAccountID != "" {
		finalAcct = memoizedAccountID
	}

	if bldgOverride != "" {
		finalBldg = bldgOverride
	} else if memoizedBuildingID != "" {
		finalBldg = memoizedBuildingID
	}

	if finalAcct != "" && finalBldg != "" {
		return finalAcct, finalBldg, nil
	}

	// Try discovery
	acct, bldg, err := c.DiscoverAccountAndBuilding(ctx)
	if err == nil && acct != "" && bldg != "" {
		memoizedAccountID = acct
		memoizedBuildingID = bldg

		if finalAcct == "" {
			finalAcct = acct
		}
		if finalBldg == "" {
			finalBldg = bldg
		}
		return finalAcct, finalBldg, nil
	}

	if err == nil {
		err = fmt.Errorf("account_id or building_id not found in response")
	}

	return "", "", fmt.Errorf("failed to discover Fooda account and building: %w; log in to app.fooda.com in Chrome and re-run 'auth login --chrome', or pass --account and --building flags directly", err)
}

func parseSince(since string) (time.Time, error) {
	now := time.Now()
	since = strings.TrimSpace(since)
	if since == "" {
		return time.Time{}, fmt.Errorf("invalid --since %q", since)
	}

	var suffix string
	if strings.HasSuffix(since, "mo") {
		suffix = "mo"
	} else if strings.HasSuffix(since, "d") {
		suffix = "d"
	} else if strings.HasSuffix(since, "w") {
		suffix = "w"
	} else if strings.HasSuffix(since, "y") {
		suffix = "y"
	} else {
		return time.Time{}, fmt.Errorf("invalid --since %q", since)
	}

	numStr := strings.TrimSuffix(since, suffix)
	val, err := strconv.Atoi(numStr)
	if err != nil || val < 0 {
		return time.Time{}, fmt.Errorf("invalid --since %q", since)
	}

	switch suffix {
	case "d":
		return now.AddDate(0, 0, -val), nil
	case "w":
		return now.AddDate(0, 0, -7*val), nil
	case "mo":
		return now.AddDate(0, -val, 0), nil
	case "y":
		return now.AddDate(-val, 0, 0), nil
	}
	return time.Time{}, fmt.Errorf("invalid --since %q", since)
}

func parseAmount(formatted string) float64 {
	// e.g. "-$16.25" -> 16.25 or "$16.25" -> 16.25
	formatted = strings.ReplaceAll(formatted, "-", "")
	formatted = strings.ReplaceAll(formatted, "$", "")
	formatted = strings.TrimSpace(formatted)
	val, _ := strconv.ParseFloat(formatted, 64)
	return val
}

func isNumeric(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}

func urlToPath(rawURL string) string {
	if !strings.Contains(rawURL, "/accounts/") && !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Path == "" {
		return rawURL
	}
	p := u.Path
	if !strings.HasSuffix(p, "/items") {
		p = strings.TrimSuffix(p, "/") + "/items"
	}
	return p
}

func getEventServiceDate(ev parsedEvent) (time.Time, string, error) {
	dateStr := ev.StartTime
	if ev.Typename == "DeliveryEventPublic" && ev.DeliveryWindow.StartTime != "" {
		dateStr = ev.DeliveryWindow.StartTime
	}
	if dateStr == "" {
		return time.Time{}, "", fmt.Errorf("missing start time")
	}
	t, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		return time.Time{}, "", err
	}
	return t, t.Format("2006-01-02"), nil
}

func loadAndDeduplicateOrders(db *store.Store) ([]client.PastOrdersProps, error) {
	rawOrders, err := db.List("order", 0)
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var deduplicated []client.PastOrdersProps

	for _, ro := range rawOrders {
		var prop client.PastOrdersProps
		if json.Unmarshal(ro, &prop) == nil {
			var filteredPres []struct {
				Order struct {
					ID                     int      `json:"id"`
					EventType              string   `json:"event_type"`
					ItemNames              []string `json:"item_names"`
					OrderFulfilledTimeUnix int64    `json:"order_fulfilled_time_unix"`
					VendorNames            []string `json:"vendor_names"`
					Status                 string   `json:"status"`
					Delivery               string   `json:"delivery"`
					RequestID              string   `json:"request_id"`
					PaymentCents           int      `json:"payment_cents"`
				} `json:"order"`
			}
			for _, pres := range prop.Presenter {
				uuid := pres.Order.RequestID
				if uuid != "" {
					if seen[uuid] {
						continue
					}
					seen[uuid] = true
				}
				filteredPres = append(filteredPres, pres)
			}
			if len(filteredPres) > 0 {
				prop.Presenter = filteredPres
				deduplicated = append(deduplicated, prop)
			}
		}
	}
	return deduplicated, nil
}

func makeAPIRequest(ctx context.Context, c *client.Client, method, path string, body []byte, contentType string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}

	clientToken, sessionToken, err := c.EnsureHandshakeTokens(ctx)
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/155.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", "https://app.fooda.com")
	req.Header.Set("Referer", "https://app.fooda.com/my")
	req.Header.Set("x-clienttoken", clientToken)
	req.Header.Set("x-sessiontoken", sessionToken)

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	} else if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	return respBody, resp.StatusCode, nil
}

type parsedItemForm struct {
	ParentID   string
	ParentType string
	ItemID     string
	ItemType   string
	CSRFToken  string
	Options    []parsedItemOption
}

type parsedItemOption struct {
	ID         string
	Label      string
	PriceDelta float64
}

func parseItemPageHTML(htmlStr string) (parsedItemForm, error) {
	doc, err := nethtml.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return parsedItemForm{}, err
	}

	var form parsedItemForm
	form.Options = []parsedItemOption{}

	var f func(*nethtml.Node)
	f = func(n *nethtml.Node) {
		if n.Type == nethtml.ElementNode && n.Data == "meta" {
			var name, content string
			for _, a := range n.Attr {
				if a.Key == "name" {
					name = a.Val
				} else if a.Key == "content" {
					content = a.Val
				}
			}
			if name == "csrf-token" {
				form.CSRFToken = content
			}
		}

		if n.Type == nethtml.ElementNode && n.Data == "input" {
			var name, val string
			for _, a := range n.Attr {
				if a.Key == "name" {
					name = a.Val
				} else if a.Key == "value" {
					val = a.Val
				}
			}
			switch name {
			case "info[parent_id]":
				form.ParentID = val
			case "info[parent_type]":
				form.ParentType = val
			case "info[item_id]":
				form.ItemID = val
			case "info[item_type]":
				form.ItemType = val
			case "authenticity_token":
				if form.CSRFToken == "" {
					form.CSRFToken = val
				}
			}

			if name == "info[options][]" {
				opt := parsedItemOption{ID: val}
				if n.Parent != nil {
					text := getNodeText(n.Parent)
					opt.Label = text
					opt.PriceDelta = parsePriceDelta(text)
				}
				form.Options = append(form.Options, opt)
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	return form, nil
}

func parsePriceDelta(text string) float64 {
	text = strings.ToLower(text)
	if !strings.Contains(text, "(+") && !strings.Contains(text, "(-") {
		return 0.0
	}
	idx := strings.LastIndex(text, "(")
	if idx == -1 {
		return 0.0
	}
	sub := text[idx:]
	end := strings.Index(sub, ")")
	if end == -1 {
		return 0.0
	}
	bracketed := sub[1:end]
	bracketed = strings.ReplaceAll(bracketed, "$", "")
	bracketed = strings.TrimSpace(bracketed)
	multiplier := 1.0
	if strings.HasPrefix(bracketed, "-") {
		multiplier = -1.0
		bracketed = bracketed[1:]
	} else if strings.HasPrefix(bracketed, "+") {
		bracketed = bracketed[1:]
	}
	val, _ := strconv.ParseFloat(bracketed, 64)
	return val * multiplier
}

func getNodeText(n *nethtml.Node) string {
	var b strings.Builder
	var f func(*nethtml.Node)
	f = func(node *nethtml.Node) {
		if node.Type == nethtml.TextNode {
			b.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)
	return strings.TrimSpace(b.String())
}

func getUpcomingEvent(ctx context.Context, c *client.Client, flags *rootFlags) (string, error) {
	acct, bldg, err := getAccountAndBuilding(ctx, c, flags)
	if err != nil {
		return "", err
	}
	variables := map[string]any{
		"input": map[string]any{
			"statuses":      []string{"ACTIVE"},
			"productTypes":  []string{"POPUP", "CAFE", "DELIVERY"},
			"accountId":     acct,
			"buildingId":    bldg,
			"endTimeAfter":  time.Now().Format("2006-01-02"),
			"endTimeBefore": time.Now().AddDate(0, 0, 7).Format("2006-01-02"),
		},
	}
	raw, err := c.Query(ctx, client.SearchPublicEventsQuery, variables)
	if err != nil {
		return "", err
	}
	events, err := parseEventsFromRaw(raw)
	if err != nil || len(events) == 0 {
		return "", fmt.Errorf("no upcoming events found")
	}

	var deliveryEvents []parsedEvent
	for _, ev := range events {
		cleanType := strings.TrimSuffix(ev.Typename, "EventPublic")
		if cleanType == "Delivery" {
			deliveryEvents = append(deliveryEvents, ev)
		}
	}
	if len(deliveryEvents) == 0 {
		return "", fmt.Errorf("no upcoming delivery events found; please specify --event explicitly")
	}
	sort.Slice(deliveryEvents, func(i, j int) bool {
		tI, _, _ := getEventServiceDate(deliveryEvents[i])
		tJ, _, _ := getEventServiceDate(deliveryEvents[j])
		return tI.Before(tJ)
	})

	firstTime, _, _ := getEventServiceDate(deliveryEvents[0])
	var earliestEvents []parsedEvent
	for _, ev := range deliveryEvents {
		t, _, _ := getEventServiceDate(ev)
		if t.Equal(firstTime) {
			earliestEvents = append(earliestEvents, ev)
		}
	}
	if len(earliestEvents) > 1 {
		var ids []string
		for _, ev := range earliestEvents {
			ids = append(ids, "S"+ev.ID)
		}
		return "", fmt.Errorf("ambiguous upcoming events on %s: %s; please specify --event explicitly", firstTime.Format("2006-01-02"), strings.Join(ids, ", "))
	}
	return "S" + earliestEvents[0].ID, nil
}

func formatOptions(opts []parsedItemOption) string {
	var lines []string
	for _, o := range opts {
		lines = append(lines, fmt.Sprintf("  - ID: %s, %s", o.ID, o.Label))
	}
	return strings.Join(lines, "\n")
}

func showCartAndPricing(ctx context.Context, c *client.Client, w io.Writer, flags *rootFlags) error {
	rawItems, statusCode, err := makeAPIRequest(ctx, c, "GET", "/api/v1/cart/items", nil, "")
	if err != nil {
		return apiErr(err)
	}
	if statusCode >= 400 {
		return apiErr(fmt.Errorf("failed to fetch cart items: HTTP status %d", statusCode))
	}

	var cartEnvelope struct {
		Data struct {
			Items []map[string]any `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rawItems, &cartEnvelope); err != nil {
		return apiErr(fmt.Errorf("malformed cart items JSON response: %w", err))
	}

	rawPrice, statusCode, err := makeAPIRequest(ctx, c, "GET", "/api/v1/cart/price", nil, "")
	if err != nil {
		return apiErr(err)
	}
	if statusCode >= 400 {
		return apiErr(fmt.Errorf("failed to fetch cart pricing: HTTP status %d", statusCode))
	}

	var priceEnvelope struct {
		Data struct {
			Pricing struct {
				SubtotalCents        *int   `json:"subtotal_cents"`
				TaxCents             *int   `json:"tax_cents"`
				DeliveryFeeCents     *int   `json:"delivery_fee_cents"`
				SubsidyCents         *int   `json:"subsidy_cents"`
				SubsidyCode          string `json:"subsidy_code"`
				PromotionCents       *int   `json:"promotion_cents"`
				GratuityCents        *int   `json:"gratuity_cents"`
				TotalCents           *int   `json:"total_cents"`
				TotalCommissionCents *int   `json:"total_commission_cents"`
			} `json:"pricing"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rawPrice, &priceEnvelope); err != nil {
		return apiErr(fmt.Errorf("malformed pricing JSON response: %w", err))
	}

	p := priceEnvelope.Data.Pricing
	if p.TotalCents == nil || *p.TotalCents < 0 || p.SubtotalCents == nil || *p.SubtotalCents < 0 {
		return apiErr(fmt.Errorf("pricing total_cents or subtotal_cents is missing, null, or negative in response"))
	}

	if flags.asJSON {
		out := map[string]any{
			"items":   cartEnvelope.Data.Items,
			"pricing": p,
		}
		return printJSONFiltered(w, out, flags)
	}

	fmt.Fprintln(w, "Cart Items:")
	for _, it := range cartEnvelope.Data.Items {
		name := it["name"]
		vendor := it["vendor"]
		if vendor == nil {
			vendor = it["vendor_name"]
		}
		price := it["price"]
		qty := it["quantity"]
		if qty == nil {
			qty = "1"
		}
		fmt.Fprintf(w, "  - %sx %v (Vendor: %v, Price: %v)\n", qty, name, vendor, price)
	}
	if len(cartEnvelope.Data.Items) == 0 {
		fmt.Fprintln(w, "  (Cart is empty)")
	}

	fmt.Fprintln(w, "\nPricing Summary:")
	fmt.Fprintf(w, "  Subtotal:     $%.2f\n", float64(*p.SubtotalCents)/100.0)
	if p.TaxCents != nil {
		fmt.Fprintf(w, "  Tax:          $%.2f\n", float64(*p.TaxCents)/100.0)
	}
	if p.DeliveryFeeCents != nil {
		fmt.Fprintf(w, "  Delivery Fee: $%.2f\n", float64(*p.DeliveryFeeCents)/100.0)
	}
	if p.SubsidyCents != nil {
		fmt.Fprintf(w, "  Subsidy:      -$%.2f (Code: %s)\n", float64(*p.SubsidyCents)/100.0, p.SubsidyCode)
	}
	fmt.Fprintf(w, "  Total Paid:   $%.2f\n", float64(*p.TotalCents)/100.0)

	return nil
}

func planOrder(budget float64, anchorName string, items []client.MenuItem) (client.MenuItem, bool) {
	var anchor client.MenuItem
	foundAnchor := false
	for _, it := range items {
		if strings.Contains(strings.ToLower(it.Name), strings.ToLower(anchorName)) {
			anchor = it
			foundAnchor = true
			break
		}
	}
	if !foundAnchor {
		return client.MenuItem{}, false
	}

	leftoverBudget := budget - anchor.Price
	if leftoverBudget < 0 {
		return client.MenuItem{}, false
	}

	var bestAddOn client.MenuItem
	hasAddOn := false

	for _, it := range items {
		if strings.ToLower(it.Name) == strings.ToLower(anchor.Name) {
			continue
		}
		if it.Price <= leftoverBudget {
			if !hasAddOn {
				bestAddOn = it
				hasAddOn = true
			} else {
				if it.Price > bestAddOn.Price {
					bestAddOn = it
				}
			}
		}
	}

	return bestAddOn, hasAddOn
}

func getCartAndPricing(ctx context.Context, c *client.Client) ([]map[string]any, any, error) {
	rawItems, statusCode, err := makeAPIRequest(ctx, c, "GET", "/api/v1/cart/items", nil, "")
	if err != nil {
		return nil, nil, apiErr(err)
	}
	if statusCode >= 400 {
		return nil, nil, apiErr(fmt.Errorf("failed to fetch cart items: HTTP status %d", statusCode))
	}

	rawPrice, statusCode, err := makeAPIRequest(ctx, c, "GET", "/api/v1/cart/price", nil, "")
	if err != nil {
		return nil, nil, apiErr(err)
	}
	if statusCode >= 400 {
		return nil, nil, apiErr(fmt.Errorf("failed to fetch cart pricing: HTTP status %d", statusCode))
	}

	var cartEnvelope struct {
		Data struct {
			Items []map[string]any `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rawItems, &cartEnvelope); err != nil {
		return nil, nil, apiErr(fmt.Errorf("malformed cart items JSON response: %w", err))
	}

	var priceEnvelope struct {
		Data struct {
			Pricing map[string]any `json:"pricing"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rawPrice, &priceEnvelope); err != nil {
		return nil, nil, apiErr(fmt.Errorf("malformed pricing JSON response: %w", err))
	}

	return cartEnvelope.Data.Items, priceEnvelope.Data.Pricing, nil
}
