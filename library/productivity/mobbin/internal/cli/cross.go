// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newCrossCmd(flags *rootFlags) *cobra.Command {
	var appsCSV, platformsCSV string
	var limit int
	cmd := &cobra.Command{
		Use:         "cross <pattern>",
		Short:       "Compare pattern coverage across apps and platforms.",
		Example:     "  mobbin-pp-cli cross paywall --apps stripe,linear,figma --platforms web,ios",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
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
			apps := splitCSV(appsCSV)
			platforms := splitCSV(platformsCSV)
			byApp := map[string]map[string][]string{}
			for _, platform := range platforms {
				hits, err := searchScreensAPI(cmd.Context(), c, platform, args[0], "", limit)
				if err != nil {
					return err
				}
				for _, h := range hits {
					key := appNameSlug(h.AppSlug)
					if fields := strings.Fields(h.App); len(fields) > 0 {
						key = strings.ToLower(fields[0])
					}
					if key == "" {
						continue
					}
					// Bucket each hit under the requested app token(s) it
					// matches so the output loop (which keys on the raw
					// --apps tokens) finds it regardless of case or
					// multi-word display names.
					for _, app := range apps {
						if !appMatches(key, app) {
							continue
						}
						if byApp[app] == nil {
							byApp[app] = map[string][]string{}
						}
						byApp[app][platform] = append(byApp[app][platform], h.ID)
					}
				}
			}
			rows := []map[string]any{}
			for _, app := range apps {
				row := map[string]any{"app": app}
				for _, platform := range platforms {
					ids := byApp[app][platform]
					row[platform+"_screens"] = len(ids)
					row[platform+"_ids"] = ids
				}
				rows = append(rows, row)
			}
			return flags.printJSON(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&appsCSV, "apps", "stripe,linear,figma", "Comma-separated app names to compare")
	cmd.Flags().StringVar(&platformsCSV, "platforms", "web,ios", "Comma-separated platforms")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum search results per platform")
	return cmd
}

// appMatches reports whether a derived app key (lowercased first word of the
// display name, or the slug stem) corresponds to a requested --apps token,
// case-insensitively and in either containment direction.
func appMatches(key, app string) bool {
	a := strings.ToLower(app)
	return strings.Contains(key, a) || strings.Contains(a, key)
}
