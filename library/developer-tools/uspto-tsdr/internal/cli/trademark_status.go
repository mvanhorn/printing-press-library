package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// trademarkSnapshot is the combined one-look view of a trademark.
type trademarkSnapshot struct {
	SerialNumber   string `json:"serialNumber"`
	MarkText       string `json:"markText,omitempty"`
	Status         string `json:"status,omitempty"`
	StatusDate     string `json:"statusDate,omitempty"`
	FilingDate     string `json:"filingDate,omitempty"`
	RegistrationNo string `json:"registrationNumber,omitempty"`
	RegistrationDt string `json:"registrationDate,omitempty"`
	Owner          string `json:"owner,omitempty"`
	DrawingCode    string `json:"drawingCode,omitempty"`
	Classes        string `json:"classes,omitempty"`
	Attorney       string `json:"attorney,omitempty"`
	EventCount     int    `json:"prosecutionEventCount"`
}

func newTrademarkStatusCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status <serialNumber>",
		Short: "Full current state of a trademark in one command",
		Long: `Fetches the TSDR case status with JSON content negotiation and renders
a clean one-screen snapshot: mark text, status, owner, classes, dates,
and attorney of record.`,
		Example: strings.Trim(`
  uspto-tsdr-pp-cli trademark status 97123456
  uspto-tsdr-pp-cli trademark status 97123456 --json
  uspto-tsdr-pp-cli trademark status 97123456 --json --select status,owner`, "\n"),
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

			// Fetch status with Accept: application/json
			path := replacePathParam("/casestatus/{caseid}/info", "caseid", caseID)
			headers := map[string]string{"Accept": "application/json"}
			data, err := c.GetWithHeaders(path, nil, headers)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			snap := parseTrademarkStatus(data, serial)

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), snap, flags)
			}

			// Human-readable output
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Trademark Status: %s\n\n", serial)
			if snap.MarkText != "" {
				fmt.Fprintf(w, "  Mark:             %s\n", snap.MarkText)
			}
			fmt.Fprintf(w, "  Status:           %s\n", snap.Status)
			if snap.StatusDate != "" {
				fmt.Fprintf(w, "  Status Date:      %s\n", snap.StatusDate)
			}
			fmt.Fprintf(w, "  Filing Date:      %s\n", snap.FilingDate)
			if snap.RegistrationNo != "" {
				fmt.Fprintf(w, "  Registration:     %s\n", snap.RegistrationNo)
			}
			if snap.RegistrationDt != "" {
				fmt.Fprintf(w, "  Registered:       %s\n", snap.RegistrationDt)
			}
			if snap.Owner != "" {
				fmt.Fprintf(w, "  Owner:            %s\n", snap.Owner)
			}
			if snap.Classes != "" {
				fmt.Fprintf(w, "  Classes:          %s\n", snap.Classes)
			}
			if snap.Attorney != "" {
				fmt.Fprintf(w, "  Attorney:         %s\n", snap.Attorney)
			}
			if snap.DrawingCode != "" {
				fmt.Fprintf(w, "  Drawing Code:     %s\n", snap.DrawingCode)
			}
			fmt.Fprintf(w, "  Prosecution Events: %d\n", snap.EventCount)
			return nil
		},
	}
	return cmd
}

// normalizeCaseID prepends "sn" if the input is digits only.
func normalizeCaseID(id string) string {
	if id == "" {
		return id
	}
	// If it already starts with sn, rn, ref, or ir, leave it
	for _, prefix := range []string{"sn", "rn", "ref", "ir"} {
		if strings.HasPrefix(strings.ToLower(id), prefix) {
			return id
		}
	}
	// Default: treat digits-only as serial number
	allDigits := true
	for _, c := range id {
		if c < '0' || c > '9' {
			allDigits = false
			break
		}
	}
	if allDigits {
		return "sn" + id
	}
	return id
}

// parseTrademarkStatus extracts key fields from the TSDR JSON response.
// TSDR returns ST96-derived JSON with varying envelope structures.
func parseTrademarkStatus(data json.RawMessage, serial string) trademarkSnapshot {
	snap := trademarkSnapshot{SerialNumber: serial}

	// Try to parse as the TSDR trademark bag structure
	var root map[string]json.RawMessage
	if json.Unmarshal(data, &root) != nil {
		return snap
	}

	// TSDR wraps in trademarkBag or directly at root level
	obj := extractTSDRObject(root)
	if obj == nil {
		// Flat fallback
		var flat map[string]interface{}
		if json.Unmarshal(data, &flat) == nil {
			obj = flat
		} else {
			return snap
		}
	}

	snap.MarkText = extractStringField(obj, "MarkVerbalElementText", "markVerbalElementText",
		"MarkText", "markText", "wordMark")
	snap.Status = extractStringField(obj, "MarkCurrentStatusExternalDescriptionText",
		"markCurrentStatusExternalDescriptionText", "Status", "status",
		"MarkCurrentStatusDescriptionText", "markCurrentStatusDescriptionText")
	snap.StatusDate = trimDate(extractStringField(obj, "MarkCurrentStatusDate",
		"markCurrentStatusDate", "StatusDate", "statusDate"))
	snap.FilingDate = trimDate(extractStringField(obj, "ApplicationDate",
		"applicationDate", "FilingDate", "filingDate"))
	snap.RegistrationNo = extractStringField(obj, "RegistrationNumber",
		"registrationNumber", "RegNumber", "regNumber")
	snap.RegistrationDt = trimDate(extractStringField(obj, "RegistrationDate",
		"registrationDate"))
	snap.DrawingCode = extractStringField(obj, "MarkDrawingCode",
		"markDrawingCode", "DrawingCode", "drawingCode")
	snap.Attorney = extractStringField(obj, "AttorneyName", "attorneyName",
		"StaffName", "staffName", "CorrespondentName", "correspondentName")

	// Extract owner from owner bag
	snap.Owner = extractTSDROwner(obj)

	// Extract classes
	snap.Classes = extractTSDRClasses(obj)

	// Count prosecution history events
	snap.EventCount = countTSDREvents(obj)

	return snap
}

func extractTSDRObject(root map[string]json.RawMessage) map[string]interface{} {
	// Try trademarkBag envelope
	for _, key := range []string{"trademarkBag", "TrademarkBag"} {
		if raw, ok := root[key]; ok {
			var bags []map[string]interface{}
			if json.Unmarshal(raw, &bags) == nil && len(bags) > 0 {
				return bags[0]
			}
			// Might be a single object
			var single map[string]interface{}
			if json.Unmarshal(raw, &single) == nil {
				return single
			}
		}
	}

	// Try direct trademark object
	for _, key := range []string{"trademark", "Trademark"} {
		if raw, ok := root[key]; ok {
			var obj map[string]interface{}
			if json.Unmarshal(raw, &obj) == nil {
				return obj
			}
		}
	}

	// Try flat root
	var flat map[string]interface{}
	rawAll, _ := json.Marshal(root)
	if json.Unmarshal(rawAll, &flat) == nil {
		return flat
	}
	return nil
}

func extractTSDROwner(obj map[string]interface{}) string {
	// Look for OwnerBag/ownerBag
	for _, key := range []string{"OwnerBag", "ownerBag", "Owners", "owners", "ApplicantBag", "applicantBag"} {
		if bag, ok := obj[key]; ok {
			if arr, ok := bag.([]interface{}); ok && len(arr) > 0 {
				if m, ok := arr[0].(map[string]interface{}); ok {
					name := extractStringField(m, "LegalEntityName", "legalEntityName",
						"EntityName", "entityName", "OwnerName", "ownerName", "Name", "name")
					if name != "" {
						return name
					}
				}
			}
		}
	}
	// Direct fields
	return extractStringField(obj, "OwnerName", "ownerName", "applicantName")
}

func extractTSDRClasses(obj map[string]interface{}) string {
	for _, key := range []string{"GoodsAndServicesBag", "goodsAndServicesBag",
		"GoodsAndServices", "goodsAndServices", "ClassificationBag", "classificationBag"} {
		if bag, ok := obj[key]; ok {
			if arr, ok := bag.([]interface{}); ok {
				var classes []string
				for _, item := range arr {
					if m, ok := item.(map[string]interface{}); ok {
						cls := extractStringField(m, "ClassNumber", "classNumber",
							"NiceClassNumber", "niceClassNumber", "ClassificationCode", "classificationCode")
						if cls != "" {
							classes = append(classes, cls)
						}
					}
				}
				if len(classes) > 0 {
					return strings.Join(classes, ", ")
				}
			}
		}
	}
	return ""
}

func countTSDREvents(obj map[string]interface{}) int {
	for _, key := range []string{"ProsecutionHistoryBag", "prosecutionHistoryBag",
		"ProsecutionHistory", "prosecutionHistory",
		"MarkEventBag", "markEventBag", "EventBag", "eventBag"} {
		if bag, ok := obj[key]; ok {
			if arr, ok := bag.([]interface{}); ok {
				return len(arr)
			}
		}
	}
	return 0
}

func trimDate(s string) string {
	if len(s) > 10 {
		return s[:10]
	}
	return s
}

// extractStringField looks for a value under any of the given keys, returns the first non-empty one.
func extractStringField(obj map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := obj[k]; ok && v != nil {
			s := fmt.Sprintf("%v", v)
			if s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}
