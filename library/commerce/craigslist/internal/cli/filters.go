// `filters show <category>` returns the per-category filter schema embedded
// in the sapi search response (data.filters). Categories like `apa` carry
// bedrooms/bathrooms/housing_type knobs that don't apply to `sof` jobs;
// agents need this to know which flags `search` will accept for the category.

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/craigslist/internal/source/craigslist"

	"github.com/spf13/cobra"
)

func newFiltersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "filters",
		Short:       "Inspect the search-filter schema available for a category",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newFiltersShowCmd(flags))
	return cmd
}

func newFiltersShowCmd(flags *rootFlags) *cobra.Command {
	var site string
	cmd := &cobra.Command{
		Use:         "show [category]",
		Short:       "Show search filters available for a category abbreviation",
		Long:        "Probe sapi for one page of results in the requested category and return the embedded data.filters block describing the filter knobs Craigslist exposes for that category.",
		Example:     "  craigslist-pp-cli filters show apa --json\n  craigslist-pp-cli filters show cta --site sfbay",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			cat := strings.TrimSpace(args[0])
			c := craigslist.New(1.0)
			params := url.Values{}
			params.Set("searchPath", cat)
			params.Set("cc", "US")
			params.Set("lang", "en")
			// sapi rejects tiny batch sizes ("bad details_length"); use the standard 360-item page.
			params.Set("batch", "1-0-360-0-0")
			body, err := c.RawGet(cmd.Context(), craigslist.HostSAPI, "/postings/search/full", params)
			if err != nil {
				return err
			}
			filters, err := extractFiltersBlock(body)
			if err != nil {
				return fmt.Errorf("parse filters for %q: %w", cat, err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), filters, flags)
		},
	}
	cmd.Flags().StringVar(&site, "site", "sfbay", "Site context to use when probing for filters")
	_ = site
	return cmd
}

// extractFiltersBlock pulls the data.filters block out of a sapi response. The
// shape is map[filterName]filterDef; we keep it as json.RawMessage so callers
// can hand the structure directly to a downstream consumer without a typed
// schema we'd have to maintain whenever Craigslist adds a new knob.
func extractFiltersBlock(body []byte) (map[string]json.RawMessage, error) {
	var wrap struct {
		Data struct {
			Filters json.RawMessage `json:"filters"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &wrap); err != nil {
		return nil, err
	}
	if len(wrap.Data.Filters) == 0 {
		return map[string]json.RawMessage{}, nil
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(wrap.Data.Filters, &out); err != nil {
		// Some categories return an array — pass through as-is.
		out = map[string]json.RawMessage{"filters": wrap.Data.Filters}
	}
	return out, nil
}
