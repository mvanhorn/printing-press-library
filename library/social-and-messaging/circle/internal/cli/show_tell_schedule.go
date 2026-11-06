// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/circle/internal/client"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/circle/internal/cliutil"

	"github.com/spf13/cobra"
)

type showTellOccurrence struct {
	Date     string      `json:"date"`
	StartsAt string      `json:"starts_at"`
	Action   string      `json:"action"`
	EventID  json.Number `json:"event_id,omitempty"`
}

type showTellSchedule struct {
	Applied     bool                 `json:"applied"`
	TemplateID  string               `json:"template_id"`
	Every       string               `json:"every"`
	Until       string               `json:"until"`
	Occurrences []showTellOccurrence `json:"occurrences"`
	Warnings    []string             `json:"warnings,omitempty"`
}

func showTellCadence(every string) (time.Duration, error) {
	d, err := cliutil.ParseDurationLoose(every)
	if err != nil || d <= 0 || d%(24*time.Hour) != 0 {
		return 0, fmt.Errorf("--every must be a positive whole number of days (for example 2w)")
	}
	return d, nil
}

func showTellDates(start time.Time, cadence time.Duration, from, until string) ([]time.Time, error) {
	dates := make([]time.Time, 0)
	if cadence <= 0 || cadence%(24*time.Hour) != 0 {
		return nil, fmt.Errorf("cadence must be a positive whole number of days")
	}
	last, err := time.ParseInLocation(time.DateOnly, until, start.Location())
	if err != nil {
		return nil, fmt.Errorf("--until: %w", err)
	}
	days := int(cadence / (24 * time.Hour))
	first := start.AddDate(0, 0, days)
	if from != "" {
		day, err := time.ParseInLocation(time.DateOnly, from, start.Location())
		if err != nil {
			return nil, fmt.Errorf("--from: %w", err)
		}
		first = time.Date(day.Year(), day.Month(), day.Day(), start.Hour(), start.Minute(), start.Second(), start.Nanosecond(), start.Location())
	}
	for next := first; next.Format(time.DateOnly) <= last.Format(time.DateOnly); next = next.AddDate(0, 0, days) {
		dates = append(dates, next)
	}
	return dates, nil
}

type showTellLedgerEntry struct {
	Date    string      `json:"date"`
	EventID json.Number `json:"event_id"`
	Status  string      `json:"status,omitempty"`
	SpaceID string      `json:"space_id,omitempty"`
}

type showTellLedger map[string][]showTellLedgerEntry

func readShowTellLedger(path string) (showTellLedger, error) {
	ledger := make(showTellLedger)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ledger, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &ledger); err != nil {
		return nil, fmt.Errorf("invalid schedule ledger: %w", err)
	}
	if ledger == nil {
		return nil, fmt.Errorf("schedule ledger must be an object")
	}
	for template, entries := range ledger {
		if err := circlePositiveID(template); err != nil {
			return nil, fmt.Errorf("invalid ledger template: %w", err)
		}
		for _, entry := range entries {
			if _, err := time.Parse(time.DateOnly, entry.Date); err != nil {
				return nil, fmt.Errorf("invalid ledger date: %w", err)
			}
			if err := circlePositiveID(entry.EventID.String()); err != nil {
				return nil, fmt.Errorf("invalid ledger event: %w", err)
			}
		}
	}
	return ledger, nil
}

func writeShowTellLedger(path string, ledger showTellLedger) error {
	data, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		return err
	}
	return cliutil.AtomicWritePrivateFile(path, data, 0o600, 0o700)
}

func validShowTellEntries(entries []showTellLedgerEntry, get func(string) error) ([]showTellLedgerEntry, error) {
	valid := make([]showTellLedgerEntry, 0, len(entries))
	for _, entry := range entries {
		err := get(entry.EventID.String())
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("verify ledger event %s: %w", entry.EventID, err)
		}
		valid = append(valid, entry)
	}
	return valid, nil
}

func showTellPlan(name string, dates []time.Time, events []circleEvent, now time.Time, entries ...showTellLedgerEntry) []showTellOccurrence {
	existing := make(map[string]json.Number)
	for _, event := range events {
		if event.Name == name && !event.StartsAt.IsZero() {
			existing[event.StartsAt.UTC().Format(time.DateOnly)] = event.ID
		}
	}
	for _, entry := range entries {
		existing[entry.Date] = entry.EventID
	}
	plan := make([]showTellOccurrence, 0, len(dates))
	for _, date := range dates {
		item := showTellOccurrence{Date: date.UTC().Format(time.DateOnly), StartsAt: date.Format(time.RFC3339), Action: "create"}
		if id, ok := existing[item.Date]; ok {
			item.Action, item.EventID = "exists", id
		}
		if date.Before(now) {
			item.Action = "past"
		}
		plan = append(plan, item)
	}
	return plan
}

// The API accepts starts_at and duration_in_seconds inside event_setting_attributes, not ends_at.
func showTellEventUpdate(space json.Number, start time.Time, duration time.Duration) map[string]any {
	return map[string]any{"space_id": space, "event": map[string]any{
		"space_id": space,
		"status":   "draft",
		"event_setting_attributes": map[string]any{
			"starts_at": start.Format(time.RFC3339), "duration_in_seconds": int64(duration / time.Second),
			"send_email_confirmation": false, "send_email_reminder": false,
			"send_in_app_notification_confirmation": false, "send_in_app_notification_reminder": false,
			"send_publish_email": false,
		},
	}}
}

func newNovelShowTellScheduleCmd(flags *rootFlags) *cobra.Command {
	var template, every, until, from, space, stateFile, tz string
	cmd := &cobra.Command{
		Use:   "schedule",
		Short: "Create the next Show & Tell events on a cadence by duplicating a template event.",
		Long:  "Use this command to create future Show & Tell events from a template on a cadence. Do NOT use this command to gather what happened at a past session; use 'show-tell recap' instead.",
		Example: strings.Trim(`
  circle-pp-cli show-tell schedule --template 3859021 --every 2w --until 2027-01-17
  circle-pp-cli show-tell schedule --template 3859021 --every 2w --from 2026-11-19 --until 2027-01-17 --space 1000969 --yes
`, "\n"),
		Annotations: map[string]string{"pp:data-source": "live", "pp:preview-happy-path": "true", "pp:happy-args": "--template=30000788;--every=2w;--until=2026-12-17", "pp:no-error-path-probe": "true", "mcp:write-flags": "state-file"},
		RunE: func(cmd *cobra.Command, args []string) (returnErr error) {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "show-tell schedule")
			}
			if template == "" || every == "" || until == "" || len(args) != 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("requires --template, --every and --until, and no positional arguments"))
			}
			if err := circlePositiveID(template); err != nil {
				return usageErr(fmt.Errorf("--template: %w", err))
			}
			if space != "" {
				if err := circlePositiveID(space); err != nil {
					return usageErr(fmt.Errorf("--space: %w", err))
				}
			}
			cadence, err := showTellCadence(every)
			if err != nil {
				return usageErr(err)
			}
			location, err := time.LoadLocation(tz)
			if err != nil {
				return usageErr(fmt.Errorf("--tz: %w", err))
			}
			for _, input := range []struct{ flag, value string }{{"--until", until}, {"--from", from}} {
				if input.value != "" {
					if _, err := time.Parse(time.DateOnly, input.value); err != nil {
						return usageErr(fmt.Errorf("%s: %w", input.flag, err))
					}
				}
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			var event circleEvent
			if err := circleGet(ctx, c, "/events/"+template, nil, &event); err != nil {
				return err
			}
			start, duration, err := event.schedule()
			if err != nil {
				return err
			}
			sourceSpace := event.Space.ID.String()
			if sourceSpace == "" {
				sourceSpace = event.SpaceID.String()
			}
			if err := circlePositiveID(sourceSpace); err != nil {
				return fmt.Errorf("template has no valid space: %w", err)
			}
			if space == "" {
				space = sourceSpace
			}
			dates, err := showTellDates(start.In(location), cadence, from, until)
			if err != nil {
				return usageErr(err)
			}
			if stateFile == "" {
				dir, err := cliutil.StateDir()
				if err != nil {
					return err
				}
				stateFile = filepath.Join(dir, "show-tell-schedule.json")
			}
			run := func() (returnErr error) {
				events, err := circleList[circleEvent](ctx, c, "/events", map[string]string{"space_id": space})
				if err != nil {
					return err
				}
				_, statErr := os.Stat(stateFile)
				warnings := make([]string, 0)
				if errors.Is(statErr, os.ErrNotExist) {
					warning := fmt.Sprintf("no ledger at %s: drafts created from another machine or state file are invisible (GET /events lists published events only)", stateFile)
					warnings = append(warnings, warning)
					_, _ = fmt.Fprintln(cmd.ErrOrStderr(), warning)
				}
				ledger, err := readShowTellLedger(stateFile)
				if err != nil {
					return err
				}
				// Legacy entries belong to the template's original destination.
				destinationEntries := make([]showTellLedgerEntry, 0)
				otherEntries := make([]showTellLedgerEntry, 0)
				for i := range ledger[template] {
					if ledger[template][i].SpaceID == "" {
						ledger[template][i].SpaceID = sourceSpace
					}
					entry := ledger[template][i]
					if entry.SpaceID == space {
						destinationEntries = append(destinationEntries, entry)
					} else {
						otherEntries = append(otherEntries, entry)
					}
				}
				entries, err := validShowTellEntries(destinationEntries, func(id string) error {
					raw, err := c.GetNoCache(ctx, "/events/"+id, nil)
					if err != nil {
						return err
					}
					return circleDecode(raw, nil)
				})
				if err != nil {
					return err
				}
				if len(entries) != len(destinationEntries) {
					ledger[template] = append(otherEntries, entries...)
					if flags.yes {
						if err := writeShowTellLedger(stateFile, ledger); err != nil {
							return err
						}
					}
				}
				result := showTellSchedule{TemplateID: template, Every: every, Until: until, Warnings: warnings, Occurrences: showTellPlan(event.Name, dates, events, time.Now().UTC(), entries...)}
				defer func() {
					if returnErr != nil && flags.yes {
						returnErr = errors.Join(returnErr, printJSONFiltered(cmd.OutOrStdout(), result, flags))
					}
				}()
				if flags.yes {
					for i := range result.Occurrences {
						item := &result.Occurrences[i]
						if item.Action != "create" {
							continue
						}
						raw, _, err := c.Post(ctx, "/spaces/"+sourceSpace+"/events/"+template+"/duplicate", map[string]any{"event": map[string]any{"space_id": json.Number(space)}})
						if err != nil {
							return fmt.Errorf("duplicate for %s: %w", item.Date, err)
						}
						var copy circleEvent
						if err := circleDecode(raw, &copy); err != nil {
							return err
						}
						if err := circlePositiveID(copy.ID.String()); err != nil {
							return fmt.Errorf("duplicate for %s returned no valid event id: %w", item.Date, err)
						}
						item.EventID = copy.ID
						item.Action = "created-unscheduled"
						ledger[template] = append(ledger[template], showTellLedgerEntry{Date: item.Date, EventID: copy.ID, Status: "unscheduled", SpaceID: space})
						if err := writeShowTellLedger(stateFile, ledger); err != nil {
							return fmt.Errorf("draft event %s was duplicated but ledger save failed; record it before retrying: %w", copy.ID, err)
						}
						raw, _, err = c.Put(ctx, "/events/"+copy.ID.String(), showTellEventUpdate(json.Number(space), dates[i], duration))
						if err == nil {
							err = circleDecode(raw, nil)
						}
						if err != nil {
							return fmt.Errorf("draft event %s was duplicated but could not be rescheduled for %s; inspect it before retrying: %w", copy.ID, item.Date, err)
						}
						item.Action = "created"
						ledger[template][len(ledger[template])-1].Status = ""
						if err := writeShowTellLedger(stateFile, ledger); err != nil {
							return fmt.Errorf("draft event %s was scheduled for %s but ledger save failed; record it before retrying: %w", copy.ID, item.Date, err)
						}
					}
					result.Applied = true
				}
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), result, flags)
				}
				for _, item := range result.Occurrences {
					if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", item.StartsAt, item.Action, item.EventID); err != nil {
						return err
					}
				}
				if !result.Applied {
					_, err = fmt.Fprintln(cmd.OutOrStdout(), "Preview only. Pass --yes to create draft events.")
				}
				return err
			}
			if flags.yes {
				if err := os.MkdirAll(filepath.Dir(stateFile), 0o700); err != nil {
					return err
				}
				return cliutil.WithFileLock(stateFile, run)
			}
			return run()
		},
	}
	cmd.Flags().StringVar(&template, "template", "", "ID of the event to duplicate.")
	cmd.Flags().StringVar(&every, "every", "", "Cadence in whole days, such as 14d or 2w.")
	cmd.Flags().StringVar(&tz, "tz", "UTC", "IANA time zone; passing --tz keeps local wall-clock time across DST.")
	cmd.Flags().StringVar(&until, "until", "", "Last occurrence date, inclusive, in --tz (YYYY-MM-DD).")
	cmd.Flags().StringVar(&from, "from", "", "First occurrence date in --tz (YYYY-MM-DD); defaults to the template date plus the cadence. Past occurrences are skipped.")
	cmd.Flags().StringVar(&space, "space", "", "Destination space ID; defaults to the template space.")
	cmd.Flags().StringVar(&stateFile, "state-file", "", "Path to the occurrence ledger; defaults to show-tell-schedule.json in the CLI state directory.")
	return cmd
}
