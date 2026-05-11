package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

// oneLookSummary combines data from 6 parallel API calls into a single view.
type oneLookSummary struct {
	ApplicationNumber string `json:"applicationNumber"`
	Status            string `json:"status,omitempty"`
	FilingDate        string `json:"filingDate,omitempty"`
	GrantDate         string `json:"grantDate,omitempty"`
	PatentNumber      string `json:"patentNumber,omitempty"`
	PTADays           int    `json:"ptaDays"`
	CurrentOwner      string `json:"currentOwner,omitempty"`
	AttorneyOfRecord  string `json:"attorneyOfRecord,omitempty"`
	FamilyDepth       int    `json:"familyDepth"`
	PTABExposure      bool   `json:"ptabExposure"`
	PTABCount         int    `json:"ptabCount,omitempty"`
}

func newOneLookCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "one-look <applicationNumber>",
		Short: "Full current state of a patent in one command",
		Long: `Calls 6 endpoints in parallel to produce a comprehensive snapshot:
status, filing/grant dates, PTA days, current owner, attorney of record,
family depth, and PTAB exposure.`,
		Example: strings.Trim(`
  uspto-patents-pp-cli patent one-look 14412875
  uspto-patents-pp-cli patent one-look 14412875 --json
  uspto-patents-pp-cli patent one-look 14412875 --json --select status,currentOwner`, "\n"),
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

			summary := oneLookSummary{ApplicationNumber: appNum}
			var mu sync.Mutex
			var wg sync.WaitGroup

			type fetchResult struct {
				name string
				data json.RawMessage
				err  error
			}

			results := make(chan fetchResult, 6)

			// 1. Meta-data
			wg.Add(1)
			go func() {
				defer wg.Done()
				path := replacePathParam("/api/v1/patent/applications/{num}/meta-data", "num", appNum)
				data, err := c.Get(path, nil)
				results <- fetchResult{name: "meta", data: data, err: err}
			}()

			// 2. PTA adjustment
			wg.Add(1)
			go func() {
				defer wg.Done()
				path := replacePathParam("/api/v1/patent/applications/{num}/adjustment", "num", appNum)
				data, err := c.Get(path, nil)
				results <- fetchResult{name: "adjustment", data: data, err: err}
			}()

			// 3. Assignment
			wg.Add(1)
			go func() {
				defer wg.Done()
				path := replacePathParam("/api/v1/patent/applications/{num}/assignment", "num", appNum)
				data, err := c.Get(path, nil)
				results <- fetchResult{name: "assignment", data: data, err: err}
			}()

			// 4. Attorney
			wg.Add(1)
			go func() {
				defer wg.Done()
				path := replacePathParam("/api/v1/patent/applications/{num}/attorney", "num", appNum)
				data, err := c.Get(path, nil)
				results <- fetchResult{name: "attorney", data: data, err: err}
			}()

			// 5. Continuity (family depth)
			wg.Add(1)
			go func() {
				defer wg.Done()
				path := replacePathParam("/api/v1/patent/applications/{num}/continuity", "num", appNum)
				data, err := c.Get(path, nil)
				results <- fetchResult{name: "continuity", data: data, err: err}
			}()

			// 6. PTAB proceedings
			wg.Add(1)
			go func() {
				defer wg.Done()
				body := map[string]interface{}{
					"q": fmt.Sprintf("patentNumber:%s", appNum),
				}
				data, _, err := c.Post("/api/v1/patent/trials/proceedings/search", body)
				results <- fetchResult{name: "ptab", data: data, err: err}
			}()

			// Close channel after all goroutines finish
			go func() {
				wg.Wait()
				close(results)
			}()

			// Process results
			for r := range results {
				if r.err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s fetch failed: %v\n", r.name, r.err)
					continue
				}

				mu.Lock()
				switch r.name {
				case "meta":
					parseMetaData(r.data, &summary)
				case "adjustment":
					parseAdjustment(r.data, &summary)
				case "assignment":
					parseAssignment(r.data, &summary)
				case "attorney":
					parseAttorney(r.data, &summary)
				case "continuity":
					parseContinuityDepth(r.data, &summary)
				case "ptab":
					parsePTAB(r.data, &summary)
				}
				mu.Unlock()
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), summary, flags)
			}

			// Human-readable output
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "One-Look: %s\n\n", appNum)
			fmt.Fprintf(w, "  Status:           %s\n", summary.Status)
			fmt.Fprintf(w, "  Filing Date:      %s\n", summary.FilingDate)
			if summary.GrantDate != "" {
				fmt.Fprintf(w, "  Grant Date:       %s\n", summary.GrantDate)
			}
			if summary.PatentNumber != "" {
				fmt.Fprintf(w, "  Patent Number:    US %s\n", summary.PatentNumber)
			}
			fmt.Fprintf(w, "  PTA Days:         %d\n", summary.PTADays)
			fmt.Fprintf(w, "  Current Owner:    %s\n", summary.CurrentOwner)
			fmt.Fprintf(w, "  Attorney:         %s\n", summary.AttorneyOfRecord)
			fmt.Fprintf(w, "  Family Depth:     %d\n", summary.FamilyDepth)
			if summary.PTABExposure {
				fmt.Fprintf(w, "  PTAB Exposure:    Yes (%d proceedings)\n", summary.PTABCount)
			} else {
				fmt.Fprintf(w, "  PTAB Exposure:    No\n")
			}

			return nil
		},
	}
	return cmd
}

func parseMetaData(data json.RawMessage, s *oneLookSummary) {
	// USPTO wraps in patentFileWrapperDataBag[0].applicationMetaData
	obj := unwrapUSPTOObj(data, "applicationMetaData")
	if obj == nil {
		var flat map[string]interface{}
		if json.Unmarshal(data, &flat) == nil {
			obj = flat
		} else {
			return
		}
	}
	s.Status = extractStringField(obj, "applicationStatusDescriptionText", "applicationStatus", "status", "applicationStatusCategory")
	s.FilingDate = extractStringField(obj, "filingDate", "applicationFilingDate")
	s.GrantDate = extractStringField(obj, "grantDate", "patentGrantDate")
	s.PatentNumber = extractStringField(obj, "patentNumber")

	// Trim dates to YYYY-MM-DD
	if len(s.FilingDate) > 10 {
		s.FilingDate = s.FilingDate[:10]
	}
	if len(s.GrantDate) > 10 {
		s.GrantDate = s.GrantDate[:10]
	}
}

func parseAdjustment(data json.RawMessage, s *oneLookSummary) {
	// USPTO wraps in patentFileWrapperDataBag[0].patentTermAdjustmentData
	obj := unwrapUSPTOObj(data, "patentTermAdjustmentData")
	if obj == nil {
		var flat map[string]interface{}
		if json.Unmarshal(data, &flat) == nil {
			obj = flat
		} else {
			return
		}
	}
	for _, key := range []string{"totalPtaDays", "adjustmentTotalDays", "totalDays", "totalAdjustmentDays"} {
		if v, ok := obj[key]; ok {
			if f, ok := v.(float64); ok {
				s.PTADays = int(f)
				return
			}
		}
	}
}

func parseAssignment(data json.RawMessage, s *oneLookSummary) {
	// USPTO wraps in patentFileWrapperDataBag[0].assignmentBag[]
	items := unwrapUSPTOArray(data, "assignmentBag")

	// Fallback: try plain array
	if len(items) == 0 {
		json.Unmarshal(data, &items)
	}
	// Fallback: try wrapper with known keys
	if len(items) == 0 {
		var wrapper map[string]json.RawMessage
		if json.Unmarshal(data, &wrapper) == nil {
			for _, key := range []string{"assignments", "data", "results", "items"} {
				if raw, ok := wrapper[key]; ok {
					if json.Unmarshal(raw, &items) == nil && len(items) > 0 {
						break
					}
				}
			}
		}
	}

	if len(items) > 0 {
		last := items[len(items)-1]
		// USPTO nests assignees in assigneeBag[].assigneeNameText
		if bag, ok := last["assigneeBag"]; ok {
			if arr, ok := bag.([]interface{}); ok && len(arr) > 0 {
				if m, ok := arr[0].(map[string]interface{}); ok {
					s.CurrentOwner = extractStringField(m, "assigneeNameText", "assigneeName", "name")
					return
				}
			}
		}
		s.CurrentOwner = extractStringField(last, "assigneeName", "assignee", "ownerName", "name")
		return
	}

	// Single object fallback
	var obj map[string]interface{}
	if json.Unmarshal(data, &obj) == nil {
		s.CurrentOwner = extractStringField(obj, "assigneeName", "assignee", "ownerName", "name")
	}
}

func parseAttorney(data json.RawMessage, s *oneLookSummary) {
	// USPTO wraps in patentFileWrapperDataBag[0].recordAttorney
	obj := unwrapUSPTOObj(data, "recordAttorney")
	if obj != nil {
		// Extract firm name from customerNumberCorrespondenceData.powerOfAttorneyAddressBag[0].nameLineOneText
		if cd, ok := obj["customerNumberCorrespondenceData"]; ok {
			if cdMap, ok := cd.(map[string]interface{}); ok {
				if bag, ok := cdMap["powerOfAttorneyAddressBag"]; ok {
					if arr, ok := bag.([]interface{}); ok && len(arr) > 0 {
						if m, ok := arr[0].(map[string]interface{}); ok {
							s.AttorneyOfRecord = extractStringField(m, "nameLineOneText", "correspondenceName", "firmName")
							return
						}
					}
				}
			}
		}
		// Fallback to powerOfAttorneyBag[0] name
		if bag, ok := obj["powerOfAttorneyBag"]; ok {
			if arr, ok := bag.([]interface{}); ok && len(arr) > 0 {
				if m, ok := arr[0].(map[string]interface{}); ok {
					first := extractStringField(m, "firstName")
					last := extractStringField(m, "lastName")
					if first != "" || last != "" {
						s.AttorneyOfRecord = strings.TrimSpace(first + " " + last)
						return
					}
				}
			}
		}
	}

	// Generic fallbacks
	var items []map[string]interface{}
	if json.Unmarshal(data, &items) == nil && len(items) > 0 {
		s.AttorneyOfRecord = extractStringField(items[0], "attorneyName", "name", "firmName", "correspondenceName")
		return
	}

	var wrapper map[string]json.RawMessage
	if json.Unmarshal(data, &wrapper) == nil {
		for _, key := range []string{"attorneys", "data", "results", "items"} {
			if raw, ok := wrapper[key]; ok {
				if json.Unmarshal(raw, &items) == nil && len(items) > 0 {
					s.AttorneyOfRecord = extractStringField(items[0], "attorneyName", "name", "firmName", "correspondenceName")
					return
				}
			}
		}
	}
}

// unwrapUSPTOObj extracts a named sub-object from the USPTO patentFileWrapperDataBag envelope.
// Returns nil if the envelope or target key is missing.
func unwrapUSPTOObj(data json.RawMessage, targetKey string) map[string]interface{} {
	var envelope struct {
		Bags []map[string]json.RawMessage `json:"patentFileWrapperDataBag"`
	}
	if json.Unmarshal(data, &envelope) != nil || len(envelope.Bags) == 0 {
		// Try one level deeper: maybe the data is already the "results" field
		var wrapper struct {
			Results json.RawMessage `json:"results"`
		}
		if json.Unmarshal(data, &wrapper) == nil && wrapper.Results != nil {
			if json.Unmarshal(wrapper.Results, &envelope) != nil || len(envelope.Bags) == 0 {
				return nil
			}
		} else {
			return nil
		}
	}
	if raw, ok := envelope.Bags[0][targetKey]; ok {
		var obj map[string]interface{}
		if json.Unmarshal(raw, &obj) == nil {
			return obj
		}
	}
	return nil
}

// unwrapUSPTOArray extracts a named array from the USPTO patentFileWrapperDataBag envelope.
func unwrapUSPTOArray(data json.RawMessage, targetKey string) []map[string]interface{} {
	var envelope struct {
		Bags []map[string]json.RawMessage `json:"patentFileWrapperDataBag"`
	}
	if json.Unmarshal(data, &envelope) != nil || len(envelope.Bags) == 0 {
		return nil
	}
	if raw, ok := envelope.Bags[0][targetKey]; ok {
		var arr []map[string]interface{}
		if json.Unmarshal(raw, &arr) == nil {
			return arr
		}
	}
	return nil
}

func parseContinuityDepth(data json.RawMessage, s *oneLookSummary) {
	// Count total family members (parents + children)
	members := extractFamilyMembers(data)
	s.FamilyDepth = len(members)
}

func parsePTAB(data json.RawMessage, s *oneLookSummary) {
	var items []json.RawMessage
	if json.Unmarshal(data, &items) == nil {
		s.PTABCount = len(items)
		s.PTABExposure = len(items) > 0
		return
	}

	var wrapper map[string]json.RawMessage
	if json.Unmarshal(data, &wrapper) == nil {
		for _, key := range []string{"results", "data", "items", "proceedings"} {
			if raw, ok := wrapper[key]; ok {
				if json.Unmarshal(raw, &items) == nil {
					s.PTABCount = len(items)
					s.PTABExposure = len(items) > 0
					return
				}
			}
		}
		// Check for totalCount or count field
		if countRaw, ok := wrapper["totalCount"]; ok {
			var count float64
			if json.Unmarshal(countRaw, &count) == nil && count > 0 {
				s.PTABCount = int(count)
				s.PTABExposure = true
				return
			}
		}
	}
}
