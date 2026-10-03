// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/iko-yo/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/iko-yo/internal/store"
	"github.com/mvanhorn/printing-press-library/library/travel/iko-yo/internal/trip"
	"github.com/spf13/cobra"
)

func tripError(err error) error {
	var rate *cliutil.RateLimitError
	if errors.As(err, &rate) {
		return rateLimitErr(err)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return notFoundErr(fmt.Errorf("no saved Trip detail for this reference; use trip inspect with --data-source live"))
	}
	var source *trip.SourceError
	if errors.As(err, &source) && source.Status == 404 {
		return notFoundErr(err)
	}
	return apiErr(err)
}
func tripSave(ctx context.Context, flags *rootFlags, records []trip.Record) error {
	if flags.noCache || len(records) == 0 {
		return nil
	}
	db, err := store.OpenWithContext(ctx, defaultDBPath("iko-yo-pp-cli"))
	if err != nil {
		return err
	}
	defer db.Close()
	return db.SaveTripRecords(ctx, records)
}
func tripLocalRecord(ctx context.Context, ref string) (trip.Record, error) {
	db, err := openStoreForRead(ctx, "iko-yo-pp-cli")
	if err != nil {
		return trip.Record{}, err
	}
	if db == nil {
		return trip.Record{}, sql.ErrNoRows
	}
	defer db.Close()
	r, err := db.TripRecord(ctx, ref)
	if err == nil && !r.Detail {
		return r, fmt.Errorf("saved %s has listing facts only; use trip inspect --data-source live for family evidence", ref)
	}
	return r, err
}
func tripRead(ctx context.Context, cmd *cobra.Command, flags *rootFlags, c *trip.Client, ref string) (trip.Record, error) {
	if flags.dataSource == "local" {
		if flags.noCache {
			return trip.Record{}, fmt.Errorf("--no-cache conflicts with --data-source local")
		}
		return tripLocalRecord(ctx, ref)
	}
	r, err := c.Inspect(ctx, ref)
	if err == nil {
		if e := tripSave(ctx, flags, []trip.Record{r}); e != nil {
			return r, fmt.Errorf("save normalized Trip facts: %w", e)
		}
		return r, nil
	}
	if flags.dataSource == "auto" && !flags.noCache && ctx.Err() == nil && isNetworkError(err) {
		saved, localErr := tripLocalRecord(ctx, ref)
		if localErr == nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: Trip network read failed; using saved observation %s for %s\n", saved.ObservedAt, ref)
			return saved, nil
		}
	}
	return r, err
}
func tripAmenities(s string) []string {
	out := make([]string, 0)
	seen := map[string]bool{}
	if s == "" {
		return out
	}
	for _, x := range strings.Split(s, ",") {
		x = strings.TrimSpace(x)
		if !seen[x] {
			out = append(out, x)
			seen[x] = true
		}
	}
	return out
}
func tripToday() string { return time.Now().In(time.FixedZone("JST", 9*3600)).Format("2006-01-02") }
func tripSource(cmd *cobra.Command, flags *rootFlags, source string) {
	flags.agentSource = source
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations["pp:data-source"] = source
}
func hintIfUnsynced(cmd *cobra.Command, db *store.Store, resource string) bool {
	count, err := db.TripCacheCount(cmd.Context(), resource)
	if err != nil {
		return false
	}
	if count == 0 {
		fmt.Fprintln(cmd.ErrOrStderr(), "No saved Trip facts yet; run trip discover or trip inspect to populate the local cache.")
		return true
	}
	return false
}
func hintIfStale(cmd *cobra.Command, db *store.Store, resource string, maxAge time.Duration) {
	if maxAge <= 0 {
		return
	}
	var observed sql.NullString
	err := db.DB().QueryRowContext(cmd.Context(), "SELECT min(observed_at) FROM iko_yo_trip_records WHERE (?='' OR ?='all' OR kind=?)", resource, resource, resource).Scan(&observed)
	if err != nil || !observed.Valid {
		return
	}
	at, err := time.Parse(time.RFC3339, observed.String)
	if err == nil && time.Since(at) > maxAge {
		fmt.Fprintln(cmd.ErrOrStderr(), "warning: saved Trip facts include older observations; inspect observed_at and refresh selected references with trip inspect --data-source live")
	}
}
func tripPrintDiscovery(cmd *cobra.Command, flags *rootFlags, v trip.Discovery) error {
	if flags.quiet {
		for _, r := range v.Records {
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), r.Ref); err != nil {
				return err
			}
		}
		return nil
	}
	if flags.csv || flags.plain {
		fmt.Fprintf(cmd.ErrOrStderr(), "Trip scan: %d records; %d matches; %d unknown dates; %d unknown requirements. %s\n", v.ScannedRecords, v.MatchedRecords, v.UnknownDateRecords, v.UnknownRequirementRecords, v.Note)
		for _, c := range v.Coverage {
			fmt.Fprintf(cmd.ErrOrStderr(), "%s coverage: %d pages, next %d, complete for selected listing %t; source-wide complete false.\n", c.Kind, c.PagesScanned, c.NextPage, c.CompleteForListing)
		}
		selected := *flags
		selected.selectFields = strings.ReplaceAll(flags.selectFields, "records.", "")
		return selected.printJSON(cmd, v.Records)
	}
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return flags.printJSON(cmd, v)
	}
	rows := make([][]string, 0, len(v.Records))
	for _, r := range v.Records {
		rows = append(rows, []string{r.Ref, r.Name, r.Schedule.Raw, r.Schedule.Status, r.WindowMatch})
	}
	if err := flags.printTable(cmd, []string{"Reference", "Name", "Published dates", "Event state", "Window match"}, rows); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), v.Note)
	fmt.Fprintf(cmd.OutOrStdout(), "Scanned %d records; %d matches; %d unknown dates.\n", v.ScannedRecords, v.MatchedRecords, v.UnknownDateRecords)
	for _, c := range v.Coverage {
		fmt.Fprintf(cmd.OutOrStdout(), "%s: %d page(s), next page %d; complete for selected listing: %t.\n", c.Kind, c.PagesScanned, c.NextPage, c.CompleteForListing)
	}
	return nil
}
func tripPrintRecord(cmd *cobra.Command, flags *rootFlags, r trip.Record) error {
	if flags.quiet {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), r.Ref)
		return err
	}
	if flags.csv || flags.plain {
		return flags.printJSON(cmd, []trip.Record{r})
	}
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return flags.printJSON(cmd, r)
	}
	rows := [][]string{{"Reference", r.Ref}, {"Name", r.Name}, {"Address", r.Address}, {"Published dates", r.Schedule.Raw}, {"Fees", r.Fees.PublishedText}, {"Age description", r.Age.Status}, {"Indoor", r.Amenities["indoor"].Status}, {"Nursing", r.Amenities["nursing"].Status}, {"Changing", r.Amenities["changing"].Status}, {"Booking", r.Booking.Status}, {"Observed", r.ObservedAt}, {"Source", r.SourceURL}, {"Official", r.OfficialURL}}
	if err := flags.printTable(cmd, []string{"Fact", "Source evidence"}, rows); err != nil {
		return err
	}
	for _, e := range r.Evidence {
		fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", e.ID, e.Text)
	}
	fmt.Fprintln(cmd.OutOrStdout(), trip.ScopeNote)
	return nil
}

type tripCompareRow struct {
	Ref              string `json:"ref"`
	ID               string `json:"id"`
	Kind             string `json:"kind"`
	Name             string `json:"name"`
	Overall          string `json:"overall"`
	AgeCheck         string `json:"age_check"`
	IndoorCheck      string `json:"indoor_check"`
	NursingCheck     string `json:"nursing_check"`
	ChangingCheck    string `json:"changing_check"`
	StrollerCheck    string `json:"stroller_check"`
	DateCheck        string `json:"date_check"`
	ApplicationCheck string `json:"application_check"`
	ChildFees        string `json:"child_fees"`
	AdultFees        string `json:"adult_fees"`
	PublishedFees    string `json:"published_fees"`
	Seats            string `json:"seat_availability"`
	ObservedAt       string `json:"observed_at"`
	DataSource       string `json:"data_source"`
	SourceURL        string `json:"source_url"`
}

func tripPrintComparison(cmd *cobra.Command, flags *rootFlags, v tripComparison) error {
	if flags.quiet {
		for _, a := range v.Records {
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), a.Record.Ref); err != nil {
				return err
			}
		}
		return nil
	}
	if flags.csv || flags.plain {
		rows := make([]tripCompareRow, 0, len(v.Records))
		for _, a := range v.Records {
			r := a.Record
			rows = append(rows, tripCompareRow{Ref: r.Ref, ID: r.ID, Kind: r.Kind, Name: r.Name, Overall: a.Overall, AgeCheck: a.Age.Status, IndoorCheck: tripAmenityStatus(a, "indoor"), NursingCheck: tripAmenityStatus(a, "nursing"), ChangingCheck: tripAmenityStatus(a, "changing"), StrollerCheck: tripAmenityStatus(a, "stroller"), DateCheck: a.Schedule.Status, ApplicationCheck: a.Application.Status, ChildFees: r.Fees.Child, AdultFees: r.Fees.Adult, PublishedFees: r.Fees.PublishedText, Seats: r.Booking.Availability, ObservedAt: r.ObservedAt, DataSource: r.DataSource, SourceURL: r.SourceURL})
		}
		fmt.Fprintln(cmd.ErrOrStderr(), v.Note)
		selected := *flags
		selected.selectFields = strings.ReplaceAll(strings.ReplaceAll(flags.selectFields, "records.record.", ""), "records.", "")
		return selected.printJSON(cmd, rows)
	}
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return flags.printJSON(cmd, v)
	}
	rows := make([][]string, 0, len(v.Records))
	for _, a := range v.Records {
		rows = append(rows, []string{a.Record.Ref, a.Record.Name, a.Overall, a.Age.Status, a.Schedule.Status, a.Application.Status, a.Record.Fees.PublishedText, a.Record.Booking.Availability})
	}
	if err := flags.printTable(cmd, []string{"Reference", "Name", "Evidence fit", "Age", "Date", "Application", "Published fees", "Seats"}, rows); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), v.Note)
	return nil
}

func tripAmenityStatus(a trip.Assessment, key string) string {
	if c, ok := a.Amenities[key]; ok {
		return c.Status
	}
	return "not_requested"
}
