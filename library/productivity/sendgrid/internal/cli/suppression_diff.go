// Hand-coded — do not add "DO NOT EDIT" header. Persists across regenerations.

package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/sendgrid/internal/store"
	"github.com/spf13/cobra"
)

func newSuppressionDiffCmd(flags *rootFlags) *cobra.Command {
	var flagAgainst string

	cmd := &cobra.Command{
		Use:   "diff <type>",
		Short: "Three-way diff: live API ↔ local store ↔ external CSV/URL",
		Long: `Performs a three-way comparison of suppressions between:
  1. The live SendGrid API (paginated fetch)
  2. The local SQLite mirror (from a previous sync)
  3. An external CSV file or URL (text/csv)

Reports emails in each exclusive zone: only_in_api, only_in_local,
only_in_external, and cases where the reason field differs (mismatched_reason).`,
		Example:     "  sendgrid-pp-cli suppression diff bounces --against external.csv",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			typeName := args[0]
			info, ok := suppressionTypes[typeName]
			if !ok {
				return usageErr(fmt.Errorf("unknown suppression type %q: must be one of bounces, blocks, spam_reports, invalid_emails", typeName))
			}

			if dryRunOK(flags) {
				return nil
			}

			// Load external source
			var external map[string]suppressionEntry
			var extErr error
			if flagAgainst != "" {
				if strings.HasPrefix(flagAgainst, "http://") || strings.HasPrefix(flagAgainst, "https://") {
					external, extErr = fetchURLCSV(cmd.Context(), flagAgainst)
				} else {
					external, extErr = loadSuppressionCSV(flagAgainst)
				}
				if extErr != nil {
					return fmt.Errorf("loading external source: %w", extErr)
				}
			}

			// Fetch live API state across all pages. A single 500-record
			// window silently truncates large accounts, producing false-positive
			// only_in_local entries and missing only_in_api / missing_from_external
			// rows. Mirror the offset/limit loop used by suppression sync.
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			const pageSize = 500
			const maxPages = 200 // safety cap: 100k entries per type
			apiEntries := map[string]suppressionEntry{}
			for page := 0; page < maxPages; page++ {
				offset := page * pageSize
				params := map[string]string{
					"limit":  fmt.Sprintf("%d", pageSize),
					"offset": fmt.Sprintf("%d", offset),
				}
				data, err := c.Get(info.apiPath, params)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				// Count raw rows before dedup-into-map so duplicate emails on
				// one page (different case) don't trigger an early break.
				var rawList []map[string]any
				if err := json.Unmarshal(data, &rawList); err != nil {
					return fmt.Errorf("parsing %s page %d: %w", typeName, page, err)
				}
				for k, v := range parseSuppressionList(data) {
					apiEntries[k] = v
				}
				if len(rawList) < pageSize {
					break
				}
			}

			// Load local store state
			var localEntries map[string]suppressionEntry
			db, err := openStoreForRead(cmd.Context(), "sendgrid-pp-cli")
			if err == nil && db != nil {
				defer func() { _ = db.Close() }()
				localEntries = loadLocalSuppressions(db, info.resourceType)
			} else {
				localEntries = map[string]suppressionEntry{}
			}

			// Three-way diff. Each email gets exactly one category based on
			// presence in (API, local, external). When --against isn't supplied,
			// external is empty and we only emit only_in_api / only_in_local.
			type diffResult struct {
				Category string `json:"category"`
				Email    string `json:"email"`
				Reason   string `json:"reason,omitempty"`
			}

			hasExternal := flagAgainst != ""

			// Union of all email keys we know about.
			allEmails := map[string]struct{}{}
			for e := range apiEntries {
				allEmails[e] = struct{}{}
			}
			for e := range localEntries {
				allEmails[e] = struct{}{}
			}
			for e := range external {
				allEmails[e] = struct{}{}
			}

			var results []diffResult
			for email := range allEmails {
				apiE, inAPI := apiEntries[email]
				localE, inLocal := localEntries[email]
				extE, inExt := external[email]

				if hasExternal {
					switch {
					case inAPI && !inLocal && !inExt:
						results = append(results, diffResult{Category: "only_in_api", Email: email, Reason: apiE.Reason})
					case !inAPI && inLocal && !inExt:
						results = append(results, diffResult{Category: "only_in_local", Email: email, Reason: localE.Reason})
					case !inAPI && !inLocal && inExt:
						results = append(results, diffResult{Category: "only_in_external", Email: email, Reason: extE.Reason})
					case inAPI && inLocal && !inExt:
						results = append(results, diffResult{Category: "missing_from_external", Email: email, Reason: apiE.Reason})
					case inAPI && !inLocal && inExt:
						results = append(results, diffResult{Category: "missing_from_local", Email: email, Reason: apiE.Reason})
					case !inAPI && inLocal && inExt:
						results = append(results, diffResult{Category: "missing_from_api", Email: email, Reason: localE.Reason})
					case inAPI && inLocal && inExt:
						if apiE.Reason != localE.Reason || apiE.Reason != extE.Reason {
							results = append(results, diffResult{
								Category: "mismatched_reason",
								Email:    email,
								Reason:   fmt.Sprintf("api=%q local=%q external=%q", apiE.Reason, localE.Reason, extE.Reason),
							})
						}
					}
				} else {
					// Two-way diff against the local store.
					switch {
					case inAPI && !inLocal:
						results = append(results, diffResult{Category: "only_in_api", Email: email, Reason: apiE.Reason})
					case !inAPI && inLocal:
						results = append(results, diffResult{Category: "only_in_local", Email: email, Reason: localE.Reason})
					case inAPI && inLocal && apiE.Reason != localE.Reason:
						results = append(results, diffResult{
							Category: "mismatched_reason",
							Email:    email,
							Reason:   fmt.Sprintf("api=%q local=%q", apiE.Reason, localE.Reason),
						})
					}
				}
			}

			if flags.asJSON {
				raw, _ := json.Marshal(results)
				return printOutput(cmd.OutOrStdout(), raw, true)
			}

			if len(results) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No differences found.")
				return nil
			}

			items := make([]map[string]any, len(results))
			for i, r := range results {
				items[i] = map[string]any{
					"category": r.Category,
					"email":    r.Email,
					"reason":   r.Reason,
				}
			}
			return printAutoTable(cmd.OutOrStdout(), items)
		},
	}

	cmd.Flags().StringVar(&flagAgainst, "against", "", "CSV file path or HTTP URL (text/csv) to diff against")

	return cmd
}

func parseSuppressionList(data json.RawMessage) map[string]suppressionEntry {
	entries := map[string]suppressionEntry{}
	var list []map[string]any
	if err := json.Unmarshal(data, &list); err != nil {
		return entries
	}
	for _, item := range list {
		email, _ := item["email"].(string)
		reason, _ := item["reason"].(string)
		if email != "" {
			entries[strings.ToLower(email)] = suppressionEntry{Email: strings.ToLower(email), Reason: reason}
		}
	}
	return entries
}

func loadLocalSuppressions(db *store.Store, resourceType string) map[string]suppressionEntry {
	entries := map[string]suppressionEntry{}
	items, err := db.List(resourceType, 10000)
	if err != nil {
		return entries
	}
	// suppression sync writes one blob per type containing a JSON array of
	// records. Fall back to single-object parsing for any rows written by
	// other code paths (defensive — current writers always emit arrays).
	for _, raw := range items {
		var arr []map[string]any
		if json.Unmarshal(raw, &arr) == nil {
			for _, item := range arr {
				email, _ := item["email"].(string)
				reason, _ := item["reason"].(string)
				if email != "" {
					entries[strings.ToLower(email)] = suppressionEntry{Email: strings.ToLower(email), Reason: reason}
				}
			}
			continue
		}
		var item map[string]any
		if json.Unmarshal(raw, &item) != nil {
			continue
		}
		email, _ := item["email"].(string)
		reason, _ := item["reason"].(string)
		if email != "" {
			entries[strings.ToLower(email)] = suppressionEntry{Email: strings.ToLower(email), Reason: reason}
		}
	}
	return entries
}

func fetchURLCSV(ctx context.Context, url string) (map[string]suppressionEntry, error) {
	// Bound the wait: a slow/unresponsive host shouldn't pin a CLI invocation
	// forever. 30s covers reasonable corporate CSV exports without being
	// generous enough to hide a real outage.
	httpClient := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building request for %q: %w", url, err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %q: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("fetching %q: HTTP %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	r := csv.NewReader(strings.NewReader(string(body)))
	r.TrimLeadingSpace = true
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parsing CSV: %w", err)
	}

	entries := map[string]suppressionEntry{}
	for i, rec := range records {
		if i == 0 && len(rec) > 0 && strings.EqualFold(rec[0], "email") {
			continue
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
