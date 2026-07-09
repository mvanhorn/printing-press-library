// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/mobbin/internal/appscrape"
	"github.com/mvanhorn/printing-press-library/library/productivity/mobbin/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/mobbin/internal/imagecache"
	"github.com/spf13/cobra"
)

var fullMobbinAppSlugRE = regexp.MustCompile(`^[a-z0-9-]+-(ios|android|web)-[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`)

func newAppCmd(flags *rootFlags) *cobra.Command {
	var saveImages bool
	var limit int
	cmd := &cobra.Command{
		Use:         "app <slug>",
		Short:       "Scrape one Mobbin app page for flows, screens, and versions.",
		Example:     "  mobbin-pp-cli app stripe-web\n  mobbin-pp-cli app airbnb-ios-86063168-143f-43c3-ba8d-e01208120883 screens --save-images",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			mode := "summary"
			if len(args) > 1 && (args[1] == "flows" || args[1] == "screens" || args[1] == "versions") {
				mode = args[1]
			}
			return runAppScrape(cmd, flags, args[0], mode, limit, saveImages)
		},
	}
	cmd.Flags().BoolVar(&saveImages, "save-images", false, "Download screen images into the local image cache")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum flows/screens to print")
	for _, mode := range []string{"flows", "screens", "versions"} {
		mode := mode
		example := "  mobbin-pp-cli app " + mode + " stripe-web --limit 25"
		if mode == "screens" {
			example = "  mobbin-pp-cli app screens stripe-web --save-images --limit 25"
		}
		sub := &cobra.Command{
			Use:         mode + " <slug>",
			Short:       "Print app " + mode + ".",
			Example:     example,
			Annotations: map[string]string{"mcp:read-only": "true"},
			RunE: func(cmd *cobra.Command, args []string) error {
				if len(args) == 0 {
					return usageErr(fmt.Errorf("app slug is required for %s", mode))
				}
				return runAppScrape(cmd, flags, args[0], mode, limit, saveImages)
			},
		}
		cmd.AddCommand(sub)
	}
	return cmd
}

func runAppScrape(cmd *cobra.Command, flags *rootFlags, slug, mode string, limit int, saveImages bool) error {
	if dryRunOK(flags) {
		return nil
	}
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	full, err := resolveAppSlug(cmd.Context(), c, slug, flags)
	if err != nil {
		return err
	}
	payload, err := appscrape.Fetch(cmd.Context(), c, full)
	if err != nil {
		return err
	}
	if saveImages && len(payload.Screens) > 0 {
		cache, err := imagecache.New("")
		if err != nil {
			return err
		}
		items := []imagecache.FetchItem{}
		for _, s := range payload.Screens {
			items = append(items, imagecache.FetchItem{ImageURL: val(s, "imageUrlFull", "imageUrl", "image_url"), Platform: val(s, "platform"), AppSlug: full, ScreenID: val(s, "id", "screenId")})
		}
		paths, errs := cache.FetchMany(cmd.Context(), items, imagecache.CDNOpts{}, 8)
		for _, s := range payload.Screens {
			s["local_path"] = paths[val(s, "id", "screenId")]
		}
		if len(errs) > 0 {
			// Best-effort, matching deck/grab: a failed image download must not
			// discard the successfully-scraped flows/screens payload.
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d image(s) failed to download; app data returned without them\n", len(errs), len(items))
		}
	}
	if limit > 0 {
		if len(payload.Flows) > limit {
			payload.Flows = payload.Flows[:limit]
		}
		if len(payload.Screens) > limit {
			payload.Screens = payload.Screens[:limit]
		}
	}
	switch mode {
	case "flows":
		return flags.printJSON(cmd, payload.Flows)
	case "screens":
		return flags.printJSON(cmd, payload.Screens)
	case "versions":
		return flags.printJSON(cmd, []map[string]any{})
	default:
		return flags.printJSON(cmd, map[string]any{"slug": payload.Slug, "app_name": payload.AppName, "flows": payload.Flows, "screens": payload.Screens})
	}
}

func resolveAppSlug(ctx context.Context, c *client.Client, arg string, flags *rootFlags) (string, error) {
	if fullMobbinAppSlugRE.MatchString(strings.ToLower(arg)) {
		return arg, nil
	}
	normalized := slugify(arg)
	platform := platformFromSlugish(normalized)
	searchTerm := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(normalized, "-web"), "-ios"), "-android")
	if searchTerm == "" {
		searchTerm = normalized
	}
	if db, err := openStore(ctx, ""); err == nil && db != nil {
		defer db.Close()
		like := "%" + searchTerm + "%"
		rows, _ := db.RawQuery(ctx, `SELECT slug, app_name, platform FROM apps WHERE slug LIKE `+sqlQuote(like)+` OR app_name LIKE `+sqlQuote(like)+` LIMIT 10`)
		rows = filterSlugRows(rows, normalized, searchTerm, platform)
		if len(rows) == 1 {
			return fmt.Sprint(rows[0]["slug"]), nil
		}
		if len(rows) > 1 {
			return "", usageErr(fmt.Errorf("multiple matches: %s; pick one", slugList(rows)))
		}
	}
	raw, err := c.Get(ctx, "/api/searchable-apps/"+platform, map[string]string{})
	if err != nil {
		return "", classifyAPIError(err, flags)
	}
	matches := []map[string]any{}
	for _, row := range extractSyncItems(raw) {
		slug := fullSlugForSearchableApp(row, platform)
		name := val(row, "appName", "app_name", "name")
		row["slug"] = slug
		nameSlug := slugify(name)
		if slug != "" && (strings.Contains(slug, normalized) || strings.Contains(slug, searchTerm) || strings.Contains(nameSlug, searchTerm)) {
			matches = append(matches, row)
		}
	}
	if len(matches) == 0 {
		return "", usageErr(fmt.Errorf("no app matches %q; try a full Mobbin slug like %s-%s-<uuid>", arg, searchTerm, platform))
	}
	if len(matches) == 1 {
		return val(matches[0], "slug"), nil
	}
	return "", usageErr(fmt.Errorf("multiple matches: %s; pick one", slugList(matches)))
}

func slugList(rows []map[string]any) string {
	parts := []string{}
	for _, r := range rows {
		parts = append(parts, val(r, "slug"))
	}
	return strings.Join(parts, ", ")
}

func filterSlugRows(rows []map[string]any, normalized, searchTerm, platform string) []map[string]any {
	out := []map[string]any{}
	for _, row := range rows {
		rowPlatform := strings.ToLower(val(row, "platform"))
		if rowPlatform != "" && rowPlatform != platform {
			continue
		}
		slug := strings.ToLower(val(row, "slug"))
		if slug == "" {
			continue
		}
		name := slugify(val(row, "app_name", "appName", "name"))
		if strings.Contains(slug, normalized) || strings.Contains(slug, searchTerm) || strings.Contains(name, searchTerm) {
			out = append(out, row)
		}
	}
	return out
}

func platformFromSlugish(s string) string {
	switch {
	case strings.Contains(s, "-ios"):
		return "ios"
	case strings.Contains(s, "-android"):
		return "android"
	default:
		return "web"
	}
}

func fullSlugForSearchableApp(row map[string]any, platform string) string {
	slug := strings.ToLower(val(row, "slug"))
	if fullMobbinAppSlugRE.MatchString(slug) {
		return slug
	}
	id := val(row, "id", "appId", "app_id")
	if id == "" {
		return slug
	}
	nameSlug := slugify(val(row, "appName", "app_name", "name"))
	if nameSlug == "" {
		nameSlug = strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(slug, "-web"), "-ios"), "-android")
	}
	return fmt.Sprintf("%s-%s-%s", nameSlug, platform, id)
}
