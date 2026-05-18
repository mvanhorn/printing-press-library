package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// tmTimelineEvent is a single event in the trademark prosecution timeline.
type tmTimelineEvent struct {
	Date        string `json:"date"`
	Code        string `json:"code,omitempty"`
	Description string `json:"description"`
}

func newTrademarkTimelineCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "timeline <serialNumber>",
		Short: "Prosecution timeline — all events in chronological order",
		Long: `Fetches the full TSDR case status and extracts every prosecution history
entry into a chronological timeline. Shows office actions, examiner reviews,
publication events, and registration milestones.`,
		Example: strings.Trim(`
  uspto-tsdr-pp-cli trademark timeline 97123456
  uspto-tsdr-pp-cli trademark timeline 97123456 --json
  uspto-tsdr-pp-cli trademark timeline 97123456 --json --select date,description`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			serial := args[0]
			caseID := normalizeCaseID(serial)

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// PATCH: use GetJSON (plain HTTP) — surf overrides Accept header.
			path := replacePathParam("/casestatus/{caseid}/info", "caseid", caseID)
			data, err := c.GetJSON(path, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			events := extractTMTimeline(data)

			if len(events) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no prosecution events found for %s\n", serial)
				return nil
			}

			// Sort chronologically
			sort.Slice(events, func(i, j int) bool {
				return events[i].Date < events[j].Date
			})

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), events, flags)
			}

			// Table output
			headers2 := []string{"Date", "Code", "Description"}
			rows := make([][]string, len(events))
			for i, ev := range events {
				rows[i] = []string{ev.Date, ev.Code, truncate(ev.Description, 60)}
			}
			return flags.printTable(cmd, headers2, rows)
		},
	}
	return cmd
}

func extractTMTimeline(data json.RawMessage) []tmTimelineEvent {
	var events []tmTimelineEvent

	var root map[string]json.RawMessage
	if json.Unmarshal(data, &root) != nil {
		return events
	}

	obj := extractTSDRObject(root)
	if obj == nil {
		return events
	}

	// PATCH: prioritize TSDR API field names (entryDate, entryCode, entryDesc)
	// over ST96 XML names in prosecution history event extraction.
	for _, key := range []string{"prosecutionHistory",
		"ProsecutionHistoryBag", "prosecutionHistoryBag",
		"ProsecutionHistory",
		"MarkEventBag", "markEventBag", "EventBag", "eventBag"} {
		if bag, ok := obj[key]; ok {
			if arr, ok := bag.([]interface{}); ok {
				for _, item := range arr {
					if m, ok := item.(map[string]interface{}); ok {
						ev := tmTimelineEvent{}
						ev.Date = trimDate(extractStringField(m,
							"entryDate",
							"ProsecutionHistoryEntryDate", "prosecutionHistoryEntryDate",
							"MarkEventDate", "markEventDate",
							"EventDate", "eventDate", "Date", "date"))
						ev.Code = extractStringField(m,
							"entryCode",
							"ProsecutionHistoryEntryCodeDescriptionText", "prosecutionHistoryEntryCodeDescriptionText",
							"MarkEventEntryNumber", "markEventEntryNumber",
							"EventCode", "eventCode", "Code", "code")
						ev.Description = extractStringField(m,
							"entryDesc",
							"ProsecutionHistoryEntryDescriptionText", "prosecutionHistoryEntryDescriptionText",
							"MarkEventDescriptionText", "markEventDescriptionText",
							"EventDescription", "eventDescription",
							"Description", "description")
						if ev.Date != "" || ev.Description != "" {
							events = append(events, ev)
						}
					}
				}
				if len(events) > 0 {
					return events
				}
			}
		}
	}

	return events
}
