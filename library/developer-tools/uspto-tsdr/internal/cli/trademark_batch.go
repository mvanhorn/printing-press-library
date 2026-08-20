package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// tmBatchEntry is a single mark's summary in a batch status check.
type tmBatchEntry struct {
	SerialNumber   string `json:"serialNumber"`
	MarkText       string `json:"markText,omitempty"`
	Status         string `json:"status"`
	FilingDate     string `json:"filingDate,omitempty"`
	RegistrationNo string `json:"registrationNumber,omitempty"`
	Owner          string `json:"owner,omitempty"`
}

func newTrademarkBatchCmd(flags *rootFlags) *cobra.Command {
	var useType string

	cmd := &cobra.Command{
		Use:   "batch <serial1> [serial2] ...",
		Short: "Batch status check for multiple trademarks",
		Long: `Looks up the status of multiple trademarks in one command using the
multi-case status endpoint. More efficient than individual lookups
for large portfolios.

For detailed per-mark data (owner, classes, attorney), use
'trademark status' on individual marks.`,
		Example: strings.Trim(`
  uspto-tsdr-pp-cli trademark batch 97123456 97654321
  uspto-tsdr-pp-cli trademark batch 97123456 97654321 --json
  uspto-tsdr-pp-cli trademark batch --type rn 1234567 2345678`, "\n"),
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

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// PATCH: use GetJSON (plain HTTP) — surf overrides Accept header.
			ids := strings.Join(args, ",")
			path := replacePathParam("/caseMultiStatus/{type}", "type", useType)
			params := map[string]string{"ids": ids}
			data, err := c.GetJSON(path, params)
			if err != nil {
				// Fallback: fetch individually
				return batchFetchIndividual(cmd, c, flags, args)
			}

			// Try to parse multi-status response
			entries := parseBatchResponse(data, args)
			if len(entries) == 0 {
				// Fallback to individual lookups
				return batchFetchIndividual(cmd, c, flags, args)
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), entries, flags)
			}

			// Table output
			headers2 := []string{"Serial", "Mark", "Status", "Filed", "Reg#", "Owner"}
			rows := make([][]string, len(entries))
			for i, e := range entries {
				rows[i] = []string{
					e.SerialNumber,
					truncate(e.MarkText, 25),
					truncate(e.Status, 25),
					e.FilingDate,
					e.RegistrationNo,
					truncate(e.Owner, 25),
				}
			}
			return flags.printTable(cmd, headers2, rows)
		},
	}
	cmd.Flags().StringVar(&useType, "type", "sn", "Case ID type: sn (serial), rn (registration), ref, ir")
	return cmd
}

// PATCH: rewrite batch response parser for TSDR API's actual multi-status
// structure: {"transactionList":[{"trademarks":[{status:{...}, parties:{...}}]}], "size":N}
func parseBatchResponse(data json.RawMessage, serials []string) []tmBatchEntry {
	var entries []tmBatchEntry

	// Parse root object
	var root map[string]json.RawMessage
	if json.Unmarshal(data, &root) != nil {
		return entries
	}

	// Try TSDR multi-status: transactionList → each has trademarks[]
	for _, key := range []string{"transactionList", "TransactionBag"} {
		raw, ok := root[key]
		if !ok {
			continue
		}

		// If TransactionBag, unwrap one level deeper
		if key == "TransactionBag" {
			var bag map[string]json.RawMessage
			if json.Unmarshal(raw, &bag) == nil {
				if tl, ok2 := bag["transactionList"]; ok2 {
					raw = tl
				}
			}
		}

		var transactions []map[string]interface{}
		if json.Unmarshal(raw, &transactions) != nil {
			continue
		}

		for _, txn := range transactions {
			// Each transaction has a trademarks array
			tmsRaw, ok := txn["trademarks"]
			if !ok {
				continue
			}
			tms, ok := tmsRaw.([]interface{})
			if !ok || len(tms) == 0 {
				continue
			}
			tmObj, ok := tms[0].(map[string]interface{})
			if !ok {
				continue
			}

			// Flatten status sub-object (same as extractTSDRObject)
			flat := flattenTSDRTrademark(tmObj)
			entry := tmBatchEntry{}
			entry.SerialNumber = extractStringField(flat, "serialNumber",
				"ApplicationNumber", "applicationNumber", "SerialNumber")
			entry.MarkText = extractStringField(flat, "markElement",
				"MarkVerbalElementText", "markVerbalElementText", "MarkText", "markText")
			entry.Status = extractStringField(flat, "extStatusDesc",
				"MarkCurrentStatusExternalDescriptionText",
				"markCurrentStatusExternalDescriptionText", "Status", "status")
			entry.FilingDate = trimDate(extractStringField(flat, "filingDate",
				"ApplicationDate", "applicationDate", "FilingDate"))
			entry.RegistrationNo = extractStringField(flat, "usRegistrationNumber",
				"RegistrationNumber", "registrationNumber")
			entry.Owner = extractTSDROwner(flat)

			sn := entry.SerialNumber
			if sn != "" {
				// Strip "sn" prefix if present for display
				if strings.HasPrefix(strings.ToLower(sn), "sn") {
					// Keep as-is — the API returns the numeric serial
				}
				entries = append(entries, entry)
			}
		}

		if len(entries) > 0 {
			return entries
		}
	}

	// Legacy fallback: try as flat array or other wrapper structures
	var items []map[string]interface{}
	if json.Unmarshal(data, &items) == nil {
		for _, item := range items {
			entry := tmBatchEntry{}
			entry.SerialNumber = extractStringField(item, "serialNumber",
				"ApplicationNumber", "applicationNumber", "SerialNumber")
			entry.MarkText = extractStringField(item, "markElement",
				"MarkVerbalElementText", "MarkText", "markText")
			entry.Status = extractStringField(item, "extStatusDesc",
				"MarkCurrentStatusExternalDescriptionText", "Status", "status")
			entry.FilingDate = trimDate(extractStringField(item, "filingDate",
				"ApplicationDate", "applicationDate"))
			entry.RegistrationNo = extractStringField(item, "usRegistrationNumber",
				"RegistrationNumber", "registrationNumber")
			entry.Owner = extractStringField(item, "OwnerName", "ownerName")
			if entry.SerialNumber != "" {
				entries = append(entries, entry)
			}
		}
	}

	return entries
}

// PATCH: use GetJSON interface — surf overrides Accept header.
func batchFetchIndividual(cmd *cobra.Command, c interface {
	GetJSON(path string, params map[string]string) (json.RawMessage, error)
}, flags *rootFlags, serials []string) error {
	var entries []tmBatchEntry

	for _, serial := range serials {
		caseID := normalizeCaseID(serial)
		path := replacePathParam("/casestatus/{caseid}/info", "caseid", caseID)
		data, err := c.GetJSON(path, nil)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not fetch %s: %v\n", serial, err)
			entries = append(entries, tmBatchEntry{SerialNumber: serial, Status: "error"})
			continue
		}
		snap := parseTrademarkStatus(data, serial)
		entries = append(entries, tmBatchEntry{
			SerialNumber:   serial,
			MarkText:       snap.MarkText,
			Status:         snap.Status,
			FilingDate:     snap.FilingDate,
			RegistrationNo: snap.RegistrationNo,
			Owner:          snap.Owner,
		})
	}

	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		return printJSONFiltered(cmd.OutOrStdout(), entries, flags)
	}

	headers := []string{"Serial", "Mark", "Status", "Filed", "Reg#", "Owner"}
	rows := make([][]string, len(entries))
	for i, e := range entries {
		rows[i] = []string{
			e.SerialNumber,
			truncate(e.MarkText, 25),
			truncate(e.Status, 25),
			e.FilingDate,
			e.RegistrationNo,
			truncate(e.Owner, 25),
		}
	}
	return flags.printTable(cmd, headers, rows)
}
