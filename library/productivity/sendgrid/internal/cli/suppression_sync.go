// Hand-coded — do not add "DO NOT EDIT" header. Persists across regenerations.

package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/sendgrid/internal/store"
	"github.com/spf13/cobra"
)

// suppressionTypes maps the --type flag values to their API path segment and
// store resource type name.
var suppressionTypes = map[string]struct {
	apiPath      string
	resourceType string
}{
	"bounces":        {"/v3/suppression/bounces", "suppression_bounces"},
	"blocks":         {"/v3/suppression/blocks", "suppression_blocks"},
	"spam_reports":   {"/v3/suppression/spam_reports", "suppression_spam_reports"},
	"invalid_emails": {"/v3/suppression/invalid_emails", "suppression_invalid_emails"},
}

type suppressionEntry struct {
	Email  string
	Reason string
}

type suppressionSyncRow struct {
	Action string `json:"action"`
	Type   string `json:"type"`
	Email  string `json:"email"`
	Reason string `json:"reason"`
}

func newSuppressionSyncCmd(flags *rootFlags) *cobra.Command {
	var flagFrom string
	var flagType string
	var flagApply bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync suppressions between SendGrid and a CSV source",
		Long: `Bidirectional sync between SendGrid suppressions and an external CSV/CRM source.
Fetches current suppressions from the API (paginated), diffs against the supplied
CSV (columns: email,reason), then prints an add/remove plan. With --apply the plan
is executed via POST (adds) and DELETE (drops). Mirrors current API state into the
local SQLite store under suppression_bounces, suppression_blocks, etc.`,
		Example:     "  sendgrid-pp-cli suppression sync --from contacts.csv --type bounces --apply",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			types := resolveSuppressionTypes(flagType)
			if len(types) == 0 {
				return usageErr(fmt.Errorf("--type must be one of: bounces, blocks, spam_reports, invalid_emails, all"))
			}

			// Load CSV if --from supplied. Without --from, we just mirror current API state
			// into the local store (a read-only refresh) — useful as a happy-path probe.
			// Guard: --apply without --from would mark every live suppression as "in API
			// but not in CSV" and remove the entire list. Refuse that combination.
			if flagFrom == "" && flagApply {
				return usageErr(fmt.Errorf("--apply requires --from: applying with no CSV would remove every live suppression"))
			}
			var csvEntries map[string]suppressionEntry
			if flagFrom != "" {
				loaded, err := loadSuppressionCSV(flagFrom)
				if err != nil {
					return fmt.Errorf("loading CSV %q: %w", flagFrom, err)
				}
				csvEntries = loaded
			} else {
				csvEntries = map[string]suppressionEntry{}
			}

			if dryRunOK(flags) {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "dry-run: would sync %d CSV entries against types: %s\n",
					len(csvEntries), strings.Join(types, ", "))
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("sendgrid-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer func() { _ = db.Close() }()

			var allRows []suppressionSyncRow

			for _, typeName := range types {
				info := suppressionTypes[typeName]
				rows, err := syncOneSuppressionType(cmd.Context(), c, db, typeName, info.apiPath, info.resourceType, csvEntries, flagApply)
				if err != nil {
					return fmt.Errorf("syncing %s: %w", typeName, err)
				}
				allRows = append(allRows, rows...)
			}

			if flags.asJSON {
				if allRows == nil {
					return printOutput(cmd.OutOrStdout(), json.RawMessage("[]"), true)
				}
				raw, _ := json.Marshal(allRows)
				return printOutput(cmd.OutOrStdout(), raw, true)
			}

			if len(allRows) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No changes.")
				return nil
			}

			items := make([]map[string]any, len(allRows))
			for i, r := range allRows {
				items[i] = map[string]any{
					"action": r.Action,
					"type":   r.Type,
					"email":  r.Email,
					"reason": r.Reason,
				}
			}
			return printAutoTable(cmd.OutOrStdout(), items)
		},
	}

	cmd.Flags().StringVar(&flagFrom, "from", "", "Path to CSV file with columns: email,reason (required)")
	cmd.Flags().StringVar(&flagType, "type", "bounces", "Suppression type: bounces|blocks|spam_reports|invalid_emails|all")
	cmd.Flags().BoolVar(&flagApply, "apply", false, "Execute the plan (default is dry-run / print-only)")

	return cmd
}

func resolveSuppressionTypes(t string) []string {
	if t == "all" {
		return []string{"bounces", "blocks", "spam_reports", "invalid_emails"}
	}
	if _, ok := suppressionTypes[t]; ok {
		return []string{t}
	}
	return nil
}

func loadSuppressionCSV(path string) (map[string]suppressionEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	r := csv.NewReader(f)
	r.TrimLeadingSpace = true
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}

	entries := make(map[string]suppressionEntry)
	for i, rec := range records {
		if i == 0 {
			// skip header if present
			if len(rec) > 0 && strings.EqualFold(rec[0], "email") {
				continue
			}
		}
		if len(rec) == 0 {
			continue
		}
		email := strings.TrimSpace(strings.ToLower(rec[0]))
		reason := ""
		if len(rec) > 1 {
			reason = strings.TrimSpace(rec[1])
		}
		if email != "" {
			entries[email] = suppressionEntry{Email: email, Reason: reason}
		}
	}
	return entries, nil
}

func syncOneSuppressionType(
	ctx context.Context,
	c interface {
		Get(path string, params map[string]string) (json.RawMessage, error)
		Post(path string, body any) (json.RawMessage, int, error)
		Delete(path string) (json.RawMessage, int, error)
	},
	db *store.Store,
	typeName, apiPath, resourceType string,
	csvEntries map[string]suppressionEntry,
	apply bool,
) ([]suppressionSyncRow, error) {
	// Fetch current API state across all pages. SendGrid's suppression
	// endpoints support offset/limit pagination; stopping at a single
	// 500-record window silently truncates large accounts and produces
	// spurious "remove" rows for entries that exist beyond page 1.
	const pageSize = 500
	const maxPages = 200 // safety cap: 100k entries per type
	var apiList []map[string]any
	for page := 0; page < maxPages; page++ {
		offset := page * pageSize
		params := map[string]string{
			"limit":  fmt.Sprintf("%d", pageSize),
			"offset": fmt.Sprintf("%d", offset),
		}
		data, err := c.Get(apiPath, params)
		if err != nil {
			return nil, fmt.Errorf("fetching %s page %d: %w", typeName, page, err)
		}
		var batch []map[string]any
		if err := json.Unmarshal(data, &batch); err != nil {
			return nil, fmt.Errorf("parsing %s page %d: %w", typeName, page, err)
		}
		apiList = append(apiList, batch...)
		if len(batch) < pageSize {
			break
		}
	}

	// Mirror the full deduped list into the local store as a single JSON
	// array blob keyed by typeName. loadLocalSuppressions unmarshals it
	// as an array — storing only the last page (the prior implementation)
	// produced a store that the diff loader couldn't parse, making the
	// local arm of the three-way diff inoperative.
	if mirror, mErr := json.Marshal(apiList); mErr == nil {
		_ = db.Upsert(resourceType, typeName, mirror)
	}

	apiEntries := make(map[string]suppressionEntry, len(apiList))
	for _, item := range apiList {
		email, _ := item["email"].(string)
		reason, _ := item["reason"].(string)
		if email != "" {
			apiEntries[strings.ToLower(email)] = suppressionEntry{Email: strings.ToLower(email), Reason: reason}
		}
	}

	var rows []suppressionSyncRow

	// Additions: in CSV but not in API
	for email, entry := range csvEntries {
		if _, exists := apiEntries[email]; !exists {
			rows = append(rows, suppressionSyncRow{Action: "add", Type: typeName, Email: email, Reason: entry.Reason})
		}
	}

	// Removals: in API but not in CSV
	for email, entry := range apiEntries {
		if _, exists := csvEntries[email]; !exists {
			rows = append(rows, suppressionSyncRow{Action: "remove", Type: typeName, Email: email, Reason: entry.Reason})
		}
	}

	if !apply {
		return rows, nil
	}

	// Execute adds
	for _, row := range rows {
		if row.Action != "add" {
			continue
		}
		body := []map[string]string{{"email": row.Email}}
		if _, _, err := c.Post(apiPath, body); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "warn: add %s %s: %v\n", typeName, row.Email, err)
		}
	}

	// Execute removes. Path-escape the email so plus-tags (user+tag@…) and
	// other RFC-5321-legal characters survive the DELETE request — bare "+"
	// in a path segment may be interpreted as a space by intermediaries.
	for _, row := range rows {
		if row.Action != "remove" {
			continue
		}
		delPath := apiPath + "/" + url.PathEscape(row.Email)
		if _, _, err := c.Delete(delPath); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "warn: remove %s %s: %v\n", typeName, row.Email, err)
		}
	}

	return rows, nil
}
