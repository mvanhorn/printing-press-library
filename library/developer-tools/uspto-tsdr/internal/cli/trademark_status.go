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

			// PATCH: use GetJSON (plain HTTP) instead of GetWithHeaders — surf's
			// Chrome impersonation overrides Accept header, causing XML response.
			path := replacePathParam("/casestatus/{caseid}/info", "caseid", caseID)
			data, err := c.GetJSON(path, nil)
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

	// PATCH: correct TSDR API field names — actual JSON uses short names from
	// the live /casestatus/{caseid}/info endpoint, not ST96 XML-derived names.
	snap.MarkText = extractStringField(obj, "markElement",
		"MarkVerbalElementText", "markVerbalElementText",
		"MarkText", "markText", "wordMark")
	snap.Status = extractStringField(obj, "extStatusDesc",
		"MarkCurrentStatusExternalDescriptionText",
		"markCurrentStatusExternalDescriptionText", "Status", "status",
		"MarkCurrentStatusDescriptionText", "markCurrentStatusDescriptionText")
	snap.StatusDate = trimDate(extractStringField(obj, "statusDate",
		"MarkCurrentStatusDate", "markCurrentStatusDate", "StatusDate"))
	snap.FilingDate = trimDate(extractStringField(obj, "filingDate",
		"ApplicationDate", "applicationDate", "FilingDate"))
	snap.RegistrationNo = extractStringField(obj, "usRegistrationNumber",
		"RegistrationNumber", "registrationNumber", "RegNumber", "regNumber")
	snap.RegistrationDt = trimDate(extractStringField(obj, "usRegistrationDate",
		"registrationDate", "RegistrationDate"))
	snap.DrawingCode = extractStringField(obj, "markDrawingCd",
		"MarkDrawingCode", "markDrawingCode", "DrawingCode", "drawingCode")
	snap.Attorney = extractStringField(obj, "lawOffAssigned",
		"AttorneyName", "attorneyName",
		"StaffName", "staffName", "CorrespondentName", "correspondentName")

	// Extract owner from owner bag
	snap.Owner = extractTSDROwner(obj)

	// Extract classes
	snap.Classes = extractTSDRClasses(obj)

	// Count prosecution history events
	snap.EventCount = countTSDREvents(obj)

	return snap
}

// PATCH: rewrite envelope unwrap — TSDR API returns {"trademarks":[{status:{...}, parties:{...}, ...}]}.
// Flatten nested "status" and "parties" sub-objects into the returned map so
// extractStringField() finds fields without nested path traversal.
func extractTSDRObject(root map[string]json.RawMessage) map[string]interface{} {
	// Try TSDR "trademarks" array envelope (actual live API structure)
	if raw, ok := root["trademarks"]; ok {
		var tms []map[string]interface{}
		if json.Unmarshal(raw, &tms) == nil && len(tms) > 0 {
			return flattenTSDRTrademark(tms[0])
		}
	}

	// Try trademarkBag envelope (ST96 XML-derived, kept for backward compat)
	for _, key := range []string{"trademarkBag", "TrademarkBag"} {
		if raw, ok := root[key]; ok {
			var bags []map[string]interface{}
			if json.Unmarshal(raw, &bags) == nil && len(bags) > 0 {
				return bags[0]
			}
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

// PATCH: flatten TSDR trademark object — merges nested "status" and "parties"
// sub-objects into the top level so extractStringField sees all fields flat.
func flattenTSDRTrademark(tm map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(tm)+30)

	// Copy top-level keys first (gsList, prosecutionHistory, publication, etc.)
	for k, v := range tm {
		result[k] = v
	}

	// Flatten "status" sub-object — contains markElement, extStatusDesc, filingDate, etc.
	if statusObj, ok := tm["status"].(map[string]interface{}); ok {
		for k, v := range statusObj {
			if _, exists := result[k]; !exists {
				result[k] = v
			}
		}
	}

	// Flatten "parties" sub-object — contains ownerGroups
	if partiesObj, ok := tm["parties"].(map[string]interface{}); ok {
		for k, v := range partiesObj {
			if _, exists := result[k]; !exists {
				result[k] = v
			}
		}
	}

	return result
}

// PATCH: rewrite owner extraction — TSDR API uses parties.ownerGroups which is
// a dict keyed by party type code (e.g. "10"), values are arrays of owner objects.
func extractTSDROwner(obj map[string]interface{}) string {
	// Try TSDR ownerGroups structure (dict of party-type → []owner)
	if og, ok := obj["ownerGroups"]; ok {
		if groups, ok := og.(map[string]interface{}); ok {
			for _, groupVal := range groups {
				if arr, ok := groupVal.([]interface{}); ok && len(arr) > 0 {
					if m, ok := arr[0].(map[string]interface{}); ok {
						name := extractStringField(m, "name", "Name",
							"LegalEntityName", "legalEntityName",
							"EntityName", "entityName", "OwnerName", "ownerName")
						if name != "" {
							return name
						}
					}
				}
			}
		}
	}

	// Legacy: look for OwnerBag/ownerBag (ST96 XML envelope)
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

// PATCH: rewrite class extraction — TSDR API uses gsList[] with nested
// internationalClasses[].code for class codes.
func extractTSDRClasses(obj map[string]interface{}) string {
	// Try TSDR gsList structure
	if bag, ok := obj["gsList"]; ok {
		if arr, ok := bag.([]interface{}); ok {
			var classes []string
			for _, item := range arr {
				if m, ok := item.(map[string]interface{}); ok {
					// internationalClasses is an array of {code, description}
					if icRaw, ok := m["internationalClasses"]; ok {
						if icArr, ok := icRaw.([]interface{}); ok {
							for _, ic := range icArr {
								if icMap, ok := ic.(map[string]interface{}); ok {
									cls := extractStringField(icMap, "code", "Code")
									if cls != "" {
										classes = append(classes, cls)
									}
								}
							}
						}
					}
					// Fallback: direct class code on the gs entry
					if len(classes) == 0 {
						cls := extractStringField(m, "ClassNumber", "classNumber", "code")
						if cls != "" {
							classes = append(classes, cls)
						}
					}
				}
			}
			if len(classes) > 0 {
				return strings.Join(classes, ", ")
			}
		}
	}

	// Legacy: GoodsAndServicesBag (ST96 XML envelope)
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

// PATCH: prioritize TSDR API field name "prosecutionHistory" in key search.
func countTSDREvents(obj map[string]interface{}) int {
	for _, key := range []string{"prosecutionHistory",
		"ProsecutionHistoryBag", "prosecutionHistoryBag",
		"ProsecutionHistory",
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
// PATCH: handles float64 → integer string conversion for JSON-parsed numeric IDs
// that would otherwise render as scientific notation (e.g. 7.8787878e+07 → "78787878").
func extractStringField(obj map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := obj[k]; ok && v != nil {
			switch val := v.(type) {
			case float64:
				// Render whole-number floats as integers (JSON numbers → float64)
				if val == float64(int64(val)) {
					return fmt.Sprintf("%d", int64(val))
				}
				return fmt.Sprintf("%g", val)
			case string:
				if val != "" {
					return val
				}
			default:
				s := fmt.Sprintf("%v", v)
				if s != "" && s != "<nil>" {
					return s
				}
			}
		}
	}
	return ""
}
