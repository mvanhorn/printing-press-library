// Copyright 2026 darin-kishore. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

func newCrossCmd(flags *rootFlags) *cobra.Command {
	var appsCSV, platformsCSV string
	var limit int
	cmd := &cobra.Command{
		Use:         "cross <pattern>",
		Short:       "Compare pattern coverage across apps and platforms.",
		Example:     "  mobbin-pp-cli cross paywall --apps stripe,linear,figma --platforms web,ios",
		Annotations: map[string]string{"mcp:read-only": "true"},
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
				hits, err := searchScreensAPI(c, platform, args[0], "", limit)
				if err != nil {
					return err
				}
				for _, h := range hits {
					key := appNameSlug(h.AppSlug)
					if h.App != "" {
						key = strings.ToLower(strings.Fields(h.App)[0])
					}
					if !wantedApp(key, apps) {
						continue
					}
					if byApp[key] == nil {
						byApp[key] = map[string][]string{}
					}
					byApp[key][platform] = append(byApp[key][platform], h.ID)
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

func wantedApp(key string, apps []string) bool {
	for _, app := range apps {
		if strings.Contains(key, strings.ToLower(app)) || strings.Contains(strings.ToLower(app), key) {
			return true
		}
	}
	return false
}
