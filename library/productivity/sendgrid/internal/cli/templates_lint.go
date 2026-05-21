// Hand-coded — do not add "DO NOT EDIT" header. Persists across regenerations.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var handlebarRE = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.-]+)\s*\}\}`)

type lintRow struct {
	Token      string `json:"token"`
	Status     string `json:"status"`
	Suggestion string `json:"suggestion,omitempty"`
}

func newTemplatesLintCmd(flags *rootFlags) *cobra.Command {
	var flagAgainst string

	cmd := &cobra.Command{
		Use:   "lint <template-id>",
		Short: "Lint a template's handlebars tokens against a contact or JSON payload",
		Long: `Fetches all versions of a template and extracts {{handlebars}} tokens from
html_content. Cross-checks each token against a SendGrid contact's fields
(when --against is a contact ID) or the top-level keys of a JSON file.
Exits with code 2 when any tokens are missing so CI pipelines can gate sends.`,
		Example:     "  sendgrid-pp-cli templates lint d-abc123 --against contact-id-or-file.json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			templateID := args[0]

			if dryRunOK(flags) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Fetch template
			path := replacePathParam("/v3/templates/{template_id}", "template_id", templateID)
			data, err := c.Get(path, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Extract tokens from all versions
			tokens := extractHandlebarTokens(data)

			// Build known-fields set from --against
			knownFields := map[string]bool{}
			if flagAgainst != "" {
				knownFields, err = resolveAgainstFields(cmd, c, flagAgainst)
				if err != nil {
					return err
				}
			}

			// Build lint results
			rows := make([]lintRow, 0, len(tokens))
			anyMissing := false

			for token := range tokens {
				row := lintRow{Token: token}
				if len(knownFields) == 0 {
					row.Status = "ok"
				} else if knownFields[token] {
					row.Status = "ok"
				} else {
					row.Status = "missing"
					anyMissing = true
					if sug := typoSuggest(token, knownFields); sug != "" {
						row.Suggestion = sug
						row.Status = "typo-suggestion"
					}
				}
				rows = append(rows, row)
			}

			// Output
			if flags.asJSON {
				raw, _ := json.Marshal(rows)
				if err := printOutput(cmd.OutOrStdout(), raw, true); err != nil {
					return err
				}
			} else {
				items := make([]map[string]any, len(rows))
				for i, r := range rows {
					items[i] = map[string]any{
						"token":      r.Token,
						"status":     r.Status,
						"suggestion": r.Suggestion,
					}
				}
				if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
					return err
				}
			}

			if anyMissing {
				// Return a code-2 exit so CI pipelines can gate sends, while still
				// allowing the Cobra command + deferred cleanups (db handles, etc.)
				// to unwind cleanly. main.go maps cliError.code to os.Exit.
				return &cliError{code: 2, err: fmt.Errorf("template %s: missing tokens", templateID)}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagAgainst, "against", "", "Contact ID or path to a JSON file to validate tokens against")

	return cmd
}

func extractHandlebarTokens(templateData json.RawMessage) map[string]bool {
	tokens := map[string]bool{}

	var tmpl map[string]json.RawMessage
	if err := json.Unmarshal(templateData, &tmpl); err != nil {
		return tokens
	}

	versionsRaw, ok := tmpl["versions"]
	if !ok {
		return tokens
	}

	var versions []map[string]json.RawMessage
	if err := json.Unmarshal(versionsRaw, &versions); err != nil {
		return tokens
	}

	for _, v := range versions {
		for _, field := range []string{"html_content", "plain_content", "subject"} {
			if raw, ok := v[field]; ok {
				var s string
				if json.Unmarshal(raw, &s) == nil {
					for _, m := range handlebarRE.FindAllStringSubmatch(s, -1) {
						if len(m) > 1 {
							tokens[m[1]] = true
						}
					}
				}
			}
		}
	}
	return tokens
}

func resolveAgainstFields(cmd *cobra.Command, c interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}, against string) (map[string]bool, error) {
	// Try as JSON file first
	if strings.HasSuffix(against, ".json") || strings.Contains(against, "/") || strings.Contains(against, "\\") {
		return loadJSONFields(against)
	}

	// Check if file exists
	if _, err := os.Stat(against); err == nil {
		return loadJSONFields(against)
	}

	// Otherwise treat as contact ID
	path := replacePathParam("/v3/marketing/contacts/{id}", "id", against)
	data, err := c.Get(path, nil)
	if err != nil {
		return nil, fmt.Errorf("fetching contact %q: %w", against, err)
	}

	fields := map[string]bool{}
	var contact map[string]any
	if err := json.Unmarshal(data, &contact); err != nil {
		return nil, fmt.Errorf("parsing contact: %w", err)
	}

	for k := range contact {
		fields[k] = true
	}

	// Also flatten custom_fields
	if cf, ok := contact["custom_fields"]; ok {
		if cfMap, ok := cf.(map[string]any); ok {
			for k := range cfMap {
				fields[k] = true
			}
		}
	}

	return fields, nil
}

func loadJSONFields(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %q: %w", path, err)
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("parsing JSON %q: %w", path, err)
	}
	fields := map[string]bool{}
	for k := range obj {
		fields[k] = true
	}
	return fields, nil
}

// typoSuggest returns the closest field name if edit distance is small.
func typoSuggest(token string, known map[string]bool) string {
	best := ""
	bestDist := 3 // max edit distance to suggest
	for k := range known {
		if d := editDistance(token, k); d < bestDist {
			bestDist = d
			best = k
		}
	}
	return best
}

func editDistance(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	row := make([]int, lb+1)
	for j := range row {
		row[j] = j
	}
	for i := 1; i <= la; i++ {
		prev := i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			next := min3(row[j]+1, prev+1, row[j-1]+cost)
			row[j-1] = prev
			prev = next
		}
		row[lb] = prev
	}
	return row[lb]
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}
