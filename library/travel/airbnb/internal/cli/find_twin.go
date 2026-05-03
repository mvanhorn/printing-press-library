package cli

import (
	"errors"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/searchbackend"
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/source/airbnb"
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/source/vrbo"
	"github.com/spf13/cobra"
)

func newFindTwinCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "find-twin <listing-url-or-photo-url>",
		Short:       "Use image or text search to find the same property elsewhere",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"method": "dry_run", "candidates": []any{}}, flags)
			}
			input := stripURLArg(args[0])
			backend := searchbackend.AutoSelect()
			photo, query, method := input, "", "image"
			if strings.Contains(input, "airbnb.") || strings.Contains(input, "vrbo.") {
				ref, err := parseListingURL(input)
				if err != nil {
					return usageErr(err)
				}
				if ref.Platform == "airbnb" {
					l, err := airbnb.Get(cmd.Context(), ref.ID, airbnb.GetParams{})
					if err != nil {
						return classifyAPIError(err)
					}
					if len(l.Photos) > 0 {
						photo = l.Photos[0]
					}
					query = strings.TrimSpace(l.City + " " + strings.Join(l.Amenities, " "))
				} else {
					return apiErr(vrbo.ErrDisabled)
				}
			}
			results, err := backend.ImageSearch(cmd.Context(), photo, searchbackend.SearchOpts{Limit: 10})
			if errors.Is(err, searchbackend.ErrUnsupported) {
				method = "text_fallback"
				if query == "" {
					query = photo
				}
				results, err = backend.Search(cmd.Context(), query+" vacation rental direct booking", searchbackend.SearchOpts{Limit: 10})
			}
			if err != nil {
				return classifyAPIError(err)
			}
			var candidates []searchbackend.Result
			for _, r := range results {
				text := strings.ToLower(r.Title + " " + r.Domain)
				if strings.Contains(text, "rental") || strings.Contains(text, "vacation") || strings.Contains(text, "cabin") || strings.Contains(text, "stay") {
					candidates = append(candidates, r)
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"method": method, "photo_url": photo, "candidates": candidates}, flags)
		},
	}
	return cmd
}
