package cli

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/hostfully/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/hostfully/internal/config"
	"github.com/mvanhorn/printing-press-library/library/travel/hostfully/internal/store"
	"github.com/spf13/cobra"
)

const hostfullyLeadPageSize = 200

const hostfullyGraphQLLeadFields = `
uid propertyUid unitUid checkInLocalDateTime checkOutLocalDateTime status
type source externalBookingId bookedUtcDateTime notes extraNotes
metadata { updatedUtcDateTime }
guestInformation {
  firstName lastName email phoneNumber cellPhoneNumber
  adultCount childrenCount petCount
}`

const hostfullyGraphQLSecureFields = `
uid
pinCode { pinCode }
order {
  currency totalAmount amountPaid balance
  transactions { status }
}`

type hostfullyLeadAPI interface {
	GetNoRetry(path string, params map[string]string) (json.RawMessage, error)
	Post(path string, body any) (json.RawMessage, int, error)
}

type hostfullyLeadFetch struct {
	Leads     []json.RawMessage
	Recovered int
}

func hostfullyOffsetCursor(offset int) string {
	payload, _ := json.Marshal(map[string]int{"offset": offset})
	return base64.StdEncoding.EncodeToString(payload)
}

func hostfullyIsHTTP500(err error) bool {
	var apiErr *client.APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == 500
}

func decodeHostfullyLeads(data json.RawMessage) ([]json.RawMessage, error) {
	var wrapper struct {
		Leads []json.RawMessage `json:"leads"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}
	if wrapper.Leads == nil {
		return nil, fmt.Errorf("Hostfully response has no leads array")
	}
	return wrapper.Leads, nil
}

// fetchHostfullyRESTRange recursively bisects a page that triggers Hostfully's
// REST serialization bug. Successful subranges retain the richer REST record;
// only singleton offsets that still return 500 are marked for GraphQL recovery.
func fetchHostfullyRESTRange(api hostfullyLeadAPI, agencyUID, propertyUID string, offset, limit int, filters ...map[string]string) ([]json.RawMessage, []int, error) {
	params := map[string]string{
		"agencyUid":   agencyUID,
		"propertyUid": propertyUID,
		"_limit":      strconv.Itoa(limit),
		"_cursor":     hostfullyOffsetCursor(offset),
	}
	for _, extra := range filters {
		for key, value := range extra {
			params[key] = value
		}
	}
	data, err := api.GetNoRetry("/leads", params)
	if err == nil {
		leads, decodeErr := decodeHostfullyLeads(data)
		return leads, nil, decodeErr
	}
	if !hostfullyIsHTTP500(err) {
		return nil, nil, err
	}
	if limit == 1 {
		return nil, []int{offset}, nil
	}

	leftSize := limit / 2
	rightSize := limit - leftSize
	left, leftGaps, err := fetchHostfullyRESTRange(api, agencyUID, propertyUID, offset, leftSize, filters...)
	if err != nil {
		return nil, nil, err
	}
	right, rightGaps, err := fetchHostfullyRESTRange(api, agencyUID, propertyUID, offset+leftSize, rightSize, filters...)
	if err != nil {
		return nil, nil, err
	}
	return append(left, right...), append(leftGaps, rightGaps...), nil
}

func hostfullyGraphQLBaseURL(restBaseURL string) string {
	base := strings.TrimRight(restBaseURL, "/")
	base = strings.TrimSuffix(base, "/v3.3")
	base = strings.TrimSuffix(base, "/v3")
	return base + "/v3.3"
}

func fetchHostfullyGraphQLLeads(api hostfullyLeadAPI, propertyUID string) ([]json.RawMessage, error) {
	var all []json.RawMessage
	cursor := ""
	seen := map[string]bool{}
	for page := 0; page < 1000; page++ {
		args := fmt.Sprintf("propertyUid:%q, _limit:100", propertyUID)
		if cursor != "" {
			args += fmt.Sprintf(", _cursor:%q", cursor)
		}
		query := fmt.Sprintf("query { leads(%s) { %s } }", args, hostfullyGraphQLLeadFields)
		data, _, err := api.Post("/graphql", map[string]any{"query": query})
		if err != nil {
			return nil, err
		}
		var response struct {
			Data struct {
				Leads []json.RawMessage `json:"leads"`
			} `json:"data"`
			Extensions struct {
				NextCursor string `json:"nextCursor"`
			} `json:"extensions"`
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}
		if err := json.Unmarshal(data, &response); err != nil {
			return nil, fmt.Errorf("parsing Hostfully GraphQL response: %w", err)
		}
		if len(response.Errors) > 0 {
			return nil, fmt.Errorf("Hostfully GraphQL error: %s", response.Errors[0].Message)
		}
		if response.Data.Leads == nil {
			return nil, fmt.Errorf("Hostfully GraphQL response has no leads array")
		}
		all = append(all, response.Data.Leads...)
		next := response.Extensions.NextCursor
		if len(response.Data.Leads) == 0 || next == "" {
			return all, nil
		}
		if seen[next] {
			return nil, fmt.Errorf("Hostfully GraphQL pagination repeated cursor")
		}
		seen[next] = true
		cursor = next
	}
	return nil, fmt.Errorf("Hostfully GraphQL pagination exceeded 1000 pages")
}

func hostfullyLeadUID(raw json.RawMessage) string {
	var item struct {
		UID string `json:"uid"`
	}
	if json.Unmarshal(raw, &item) != nil {
		return ""
	}
	return item.UID
}

func fetchAllHostfullyUnitLeads(restAPI, graphqlAPI hostfullyLeadAPI, agencyUID, propertyUID string) (hostfullyLeadFetch, error) {
	var leads []json.RawMessage
	var gaps []int
	for offset := 0; ; offset += hostfullyLeadPageSize {
		batch, pageGaps, err := fetchHostfullyRESTRange(
			restAPI, agencyUID, propertyUID, offset, hostfullyLeadPageSize,
		)
		if err != nil {
			return hostfullyLeadFetch{}, err
		}
		leads = append(leads, batch...)
		gaps = append(gaps, pageGaps...)
		if len(batch) == 0 && len(pageGaps) == 0 {
			break
		}
		if len(pageGaps) == 0 && len(batch) < hostfullyLeadPageSize {
			break
		}
	}
	if len(gaps) == 0 {
		return hostfullyLeadFetch{Leads: leads}, nil
	}

	graphQLLeads, err := fetchHostfullyGraphQLLeads(graphqlAPI, propertyUID)
	if err != nil {
		return hostfullyLeadFetch{}, fmt.Errorf("recovering %d REST serialization gaps through GraphQL: %w", len(gaps), err)
	}
	seenUIDs := make(map[string]bool, len(leads))
	for _, lead := range leads {
		if uid := hostfullyLeadUID(lead); uid != "" {
			seenUIDs[uid] = true
		}
	}
	recovered := 0
	for _, lead := range graphQLLeads {
		uid := hostfullyLeadUID(lead)
		if uid != "" && !seenUIDs[uid] {
			leads = append(leads, lead)
			seenUIDs[uid] = true
			recovered++
		}
	}
	if recovered < len(gaps) {
		return hostfullyLeadFetch{}, fmt.Errorf("GraphQL recovered %d of %d REST serialization gaps", recovered, len(gaps))
	}
	return hostfullyLeadFetch{Leads: leads, Recovered: recovered}, nil
}

// rawLeadOverlapsDate applies the property's local calendar date semantics.
// Checkout is included because the operations reports need that departure;
// occupancy itself remains check-in <= day < checkout in today.go.
func rawLeadOverlapsDate(raw json.RawMessage, day time.Time) bool {
	lead, err := parseReservation(raw)
	if err != nil || lead.CheckIn.IsZero() || lead.CheckOut.IsZero() {
		return false
	}
	want := day.Format("2006-01-02")
	return lead.CheckIn.Format("2006-01-02") <= want &&
		lead.CheckOut.Format("2006-01-02") >= want
}

// fetchHostfullyUnitLeadsForDate uses Hostfully's documented local-date
// filters, avoiding a full-history walk for every operational report. The
// per-property query retains the REST 500 isolation used by the full sync.
func fetchHostfullyUnitLeadsForDate(restAPI, graphqlAPI hostfullyLeadAPI, agencyUID, propertyUID string, day time.Time) (hostfullyLeadFetch, error) {
	dateText := day.Format("2006-01-02")
	filters := map[string]string{
		"checkInTo":    dateText,
		"checkOutFrom": dateText,
	}
	var leads []json.RawMessage
	var gaps []int
	for offset := 0; ; offset += hostfullyLeadPageSize {
		batch, pageGaps, err := fetchHostfullyRESTRange(
			restAPI, agencyUID, propertyUID, offset, hostfullyLeadPageSize, filters,
		)
		if err != nil {
			return hostfullyLeadFetch{}, err
		}
		leads = append(leads, batch...)
		gaps = append(gaps, pageGaps...)
		if len(batch) == 0 && len(pageGaps) == 0 {
			break
		}
		if len(pageGaps) == 0 && len(batch) < hostfullyLeadPageSize {
			break
		}
	}
	if len(gaps) == 0 {
		return hostfullyLeadFetch{Leads: leads}, nil
	}

	// GraphQL is the recovery path only. It does not need to support the REST
	// date-filter arguments: fetch the property and apply the same local-date
	// predicate here before reconciling the isolated gaps.
	graphQLLeads, err := fetchHostfullyGraphQLLeads(graphqlAPI, propertyUID)
	if err != nil {
		return hostfullyLeadFetch{}, fmt.Errorf("recovering %d date-filtered REST gaps through GraphQL: %w", len(gaps), err)
	}
	seenUIDs := make(map[string]bool, len(leads))
	for _, lead := range leads {
		if uid := hostfullyLeadUID(lead); uid != "" {
			seenUIDs[uid] = true
		}
	}
	recovered := 0
	for _, lead := range graphQLLeads {
		if !rawLeadOverlapsDate(lead, day) {
			continue
		}
		uid := hostfullyLeadUID(lead)
		if uid != "" && !seenUIDs[uid] {
			leads = append(leads, lead)
			seenUIDs[uid] = true
			recovered++
		}
	}
	if recovered < len(gaps) {
		return hostfullyLeadFetch{}, fmt.Errorf("GraphQL recovered %d of %d date-filtered REST gaps", recovered, len(gaps))
	}
	return hostfullyLeadFetch{Leads: leads, Recovered: recovered}, nil
}

func newHostfullySyncCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var full bool

	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "Fetch all reservations from Hostfully into local store (per-unit queries)",
		Long: `Fetches the complete reservation history for every unit using per-property API queries.
The command follows every page. If Hostfully's broad REST serializer returns
HTTP 500 for an otherwise valid lead, fetch isolates the affected offsets and
recovers only those missing records through the v3.3 GraphQL API.`,
		Example: `  hostfully-pp-cli fetch
  hostfully-pp-cli fetch --full`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "fetch")
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}

			agencyUID := cfg.AgencyUID()
			if agencyUID == "" {
				return fmt.Errorf("HOSTFULLY_AGENCY_ID environment variable is required")
			}

			apiKey := cfg.HostfullyApiKey
			if apiKey == "" {
				return fmt.Errorf("HOSTFULLY_API_KEY environment variable is required")
			}

			if dbPath == "" {
				dbPath = defaultDBPath("hostfully-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			c := client.New(cfg, flags.timeout, liveFanoutRateLimit(flags))
			c.NoCache = full
			graphQLCfg := *cfg
			graphQLCfg.BaseURL = hostfullyGraphQLBaseURL(cfg.BaseURL)
			graphQLClient := client.New(&graphQLCfg, flags.timeout, liveFanoutRateLimit(flags))
			graphQLClient.NoCache = full
			totalCount := 0
			startTime := time.Now()
			var failedUnits []int
			if _, err := requireUnits(); err != nil {
				return err
			}
			units := unitNumbers()
			progress := cmd.OutOrStdout()
			if flags.asJSON || flags.agent {
				progress = cmd.ErrOrStderr()
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			for _, unit := range units {
				uid, _ := unitUID(unit)
				result, err := fetchAllHostfullyUnitLeads(bindLeadAPI(ctx, c), bindLeadAPI(ctx, graphQLClient), agencyUID, uid)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warn: unit %d fetch failed: %v\n", unit, err)
					failedUnits = append(failedUnits, unit)
					continue
				}

				for _, lead := range result.Leads {
					var item map[string]any
					if err := json.Unmarshal(lead, &item); err != nil {
						continue
					}

					id, _ := item["uid"].(string)
					if id == "" {
						continue
					}

					if err := db.Upsert("leads", id, lead); err != nil {
						fmt.Fprintf(os.Stderr, "warn: upsert %s failed: %v\n", id, err)
						continue
					}
					totalCount++
				}

				if !flags.quiet {
					fmt.Fprintf(progress, "  unit #%d: %d leads", unit, len(result.Leads))
					if result.Recovered > 0 {
						fmt.Fprintf(progress, " (%d recovered through GraphQL)", result.Recovered)
					}
					fmt.Fprintln(progress)
				}
			}
			if len(failedUnits) > 0 {
				return fmt.Errorf("reservation fetch failed for units %v", failedUnits)
			}

			duration := time.Since(startTime)

			if flags.asJSON || flags.agent {
				result := map[string]any{
					"total_records": totalCount,
					"duration_ms":   duration.Milliseconds(),
					"status":        "ok",
				}
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\nSynced %d reservations in %v\n", totalCount, duration.Round(time.Millisecond))
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().BoolVar(&full, "full", false, "Full resync (same as default for Hostfully)")

	return cmd
}
