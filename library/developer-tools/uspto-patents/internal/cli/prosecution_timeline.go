package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// timelineEvent is a single event in the prosecution timeline.
type timelineEvent struct {
	Date        string `json:"date"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

func newProsecutionTimelineCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "timeline <applicationNumber>",
		Short: "Prosecution timeline — all events in chronological order",
		Long: `Fetches transactions, continuity events, and PTAB proceedings for
an application and merges them into a single chronological stream.`,
		Example: strings.Trim(`
  uspto-patents-pp-cli patent timeline 14412875
  uspto-patents-pp-cli patent timeline 14412875 --json
  uspto-patents-pp-cli patent timeline 14412875 --json --select date,type`, "\n"),
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

			appNum := args[0]
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			var events []timelineEvent

			// 1. Transactions
			txPath := replacePathParam("/api/v1/patent/applications/{num}/transactions", "num", appNum)
			txData, err := c.Get(txPath, nil)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not fetch transactions: %v\n", err)
			} else {
				events = append(events, extractTimelineEvents(txData, "transaction")...)
			}

			// 2. Continuity
			contPath := replacePathParam("/api/v1/patent/applications/{num}/continuity", "num", appNum)
			contData, err := c.Get(contPath, nil)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not fetch continuity: %v\n", err)
			} else {
				events = append(events, extractTimelineEvents(contData, "continuity")...)
			}

			// 3. PTAB proceedings
			ptabBody := map[string]interface{}{
				"q": fmt.Sprintf("patentNumber:%s", appNum),
			}
			ptabData, _, err := c.Post("/api/v1/patent/trials/proceedings/search", ptabBody)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not fetch PTAB proceedings: %v\n", err)
			} else {
				events = append(events, extractTimelineEvents(ptabData, "ptab")...)
			}

			// Sort by date ascending
			sort.Slice(events, func(i, j int) bool {
				return events[i].Date < events[j].Date
			})

			if len(events) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no timeline events found for application %s\n", appNum)
				return nil
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), events, flags)
			}

			// Table output
			headers := []string{"Date", "Type", "Description"}
			rows := make([][]string, len(events))
			for i, ev := range events {
				rows[i] = []string{ev.Date, ev.Type, truncate(ev.Description, 60)}
			}
			return flags.printTable(cmd, headers, rows)
		},
	}
	return cmd
}

// extractTimelineEvents pulls date+description pairs from a JSON response.
// Handles the USPTO patentFileWrapperDataBag envelope, plain arrays, and
// object-wrapped arrays.
func extractTimelineEvents(data json.RawMessage, eventType string) []timelineEvent {
	var events []timelineEvent

	// USPTO responses wrap data in patentFileWrapperDataBag[0].<subBag>.
	// Unwrap that first, then fall through to generic extraction.
	var allItems []map[string]interface{}
	allItems = append(allItems, unwrapUSPTOBags(data, eventType)...)

	// Generic fallback: try as plain array
	if len(allItems) == 0 {
		var items []map[string]interface{}
		if err := json.Unmarshal(data, &items); err != nil {
			// Try as object with nested array
			var wrapper map[string]json.RawMessage
			if json.Unmarshal(data, &wrapper) == nil {
				for _, key := range []string{"results", "data", "items", "transactionHistory", "continuityData", "proceedings"} {
					if raw, ok := wrapper[key]; ok {
						if json.Unmarshal(raw, &items) == nil && len(items) > 0 {
							break
						}
					}
				}
			}
		}
		allItems = append(allItems, items...)
	}

	dateFields := []string{"eventDate", "date", "filingDate", "recordDate", "actionDate",
		"transactionDate", "effectiveDate", "dispositionDate", "institutionDate",
		"parentApplicationFilingDate", "childApplicationFilingDate"}
	descFields := []string{"eventDescriptionText", "description", "transactionDescription",
		"code", "eventCode", "title", "proceedingNumber",
		"claimParentageTypeCodeDescriptionText", "relationshipType", "status"}

	for _, item := range allItems {
		date := ""
		for _, f := range dateFields {
			if v, ok := item[f]; ok && v != nil {
				date = fmt.Sprintf("%v", v)
				if len(date) > 10 {
					date = date[:10]
				}
				break
			}
		}
		desc := ""
		for _, f := range descFields {
			if v, ok := item[f]; ok && v != nil {
				desc = fmt.Sprintf("%v", v)
				break
			}
		}
		if date == "" && desc == "" {
			continue
		}
		events = append(events, timelineEvent{
			Date:        date,
			Type:        eventType,
			Description: desc,
		})
	}
	return events
}

// unwrapUSPTOBags extracts items from the USPTO patentFileWrapperDataBag
// envelope. The response shape is:
//
//	{
//	  "count": 1,
//	  "patentFileWrapperDataBag": [{
//	    "eventDataBag": [...],          // transactions
//	    "parentContinuityBag": [...],   // continuity parents
//	    "childContinuityBag": [...]     // continuity children
//	  }]
//	}
func unwrapUSPTOBags(data json.RawMessage, eventType string) []map[string]interface{} {
	var envelope struct {
		Bags []map[string]json.RawMessage `json:"patentFileWrapperDataBag"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil || len(envelope.Bags) == 0 {
		return nil
	}

	var bagKeys []string
	switch eventType {
	case "transaction":
		bagKeys = []string{"eventDataBag"}
	case "continuity":
		bagKeys = []string{"parentContinuityBag", "childContinuityBag"}
	default:
		// For PTAB or unknown types, try all known bag keys
		bagKeys = []string{"eventDataBag", "parentContinuityBag", "childContinuityBag"}
	}

	var items []map[string]interface{}
	for _, bag := range envelope.Bags {
		for _, key := range bagKeys {
			if raw, ok := bag[key]; ok {
				var arr []map[string]interface{}
				if json.Unmarshal(raw, &arr) == nil {
					items = append(items, arr...)
				}
			}
		}
	}
	return items
}
