// Copyright 2026 Jeeves and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
	"github.com/spf13/cobra"
)

const saveInitialListSettingsHandler = "save-initial-list-settings"

func newListsSettingsSaveCmd(flags *rootFlags) *cobra.Command {
	var name, sortOrder, categoryGroupingID, listCategoryGroupID string
	var hideCategories, stdinBody, apply bool

	cmd := &cobra.Command{
		Use:         "save",
		Short:       "Create initial settings for a list (preview unless --apply)",
		Example:     "  anylist-pp-cli lists settings save --name Groceries --sort-order ALListItemSortOrderManual --apply",
		Annotations: map[string]string{"pp:endpoint": "lists.settings.save", "pp:method": "POST", "pp:path": "/data/list-settings/update"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if stdinBody {
				body, err := readStdinJSONMap()
				if err != nil {
					return err
				}
				name = stringFromBody(body, "name")
				if name == "" {
					name = stringFromBody(body, "list")
				}
				sortOrder = stringFromBody(body, "sort_order")
				if sortOrder == "" {
					sortOrder = stringFromBody(body, "list_item_sort_order")
				}
				categoryGroupingID = stringFromBody(body, "category_grouping_id")
				listCategoryGroupID = stringFromBody(body, "list_category_group_id")
				hideCategories = boolFromBody(body, "hide_categories")
				apply = boolFromBody(body, "apply")
			}
			name = strings.TrimSpace(name)
			sortOrder = strings.TrimSpace(sortOrder)
			categoryGroupingID = strings.TrimSpace(categoryGroupingID)
			listCategoryGroupID = strings.TrimSpace(listCategoryGroupID)
			if name == "" && !flags.dryRun {
				return fmt.Errorf("required flag \"name\" not set")
			}
			if sortOrder == "" {
				sortOrder = "ALListItemSortOrderManual"
			}
			if sortOrder != "ALListItemSortOrderManual" && sortOrder != "ALListItemSortOrderAlphabetical" {
				return fmt.Errorf("sort order must be ALListItemSortOrderManual or ALListItemSortOrderAlphabetical")
			}
			if !apply || flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run":                true,
					"name":                   name,
					"handler_id":             saveInitialListSettingsHandler,
					"sort_order":             sortOrder,
					"hide_categories":        hideCategories,
					"category_grouping_id":   categoryGroupingID,
					"list_category_group_id": listCategoryGroupID,
					"apply":                  apply,
				}, flags)
			}

			ctx := cmd.Context()
			cfg, st, err := openAuthedLocalStore(flags)
			if err != nil {
				return err
			}
			defer st.Close()

			client := anylist.New(cfg)
			data, err := client.GetUserData(ctx)
			if err != nil {
				return fmt.Errorf("reading live lists and settings: %w", err)
			}
			list, err := exactLiveShoppingListByName(data, name)
			if err != nil {
				return err
			}
			if existing, found := findLiveListSettingsByListID(data, list.GetIdentifier()); found {
				return fmt.Errorf("list %q already has settings %q; use a verified settings update or clear them first", list.GetName(), existing.GetIdentifier())
			}

			settings := &pb.PBListSettings{
				Identifier:           uuid.NewString(),
				UserId:               cfg.UserID,
				ListId:               list.GetIdentifier(),
				Timestamp:            float64(time.Now().Unix()),
				ListItemSortOrder:    sortOrder,
				ShouldHideCategories: hideCategories,
				CategoryGroupingId:   categoryGroupingID,
				ListCategoryGroupId:  listCategoryGroupID,
			}
			if err := client.UpdateListSettings(ctx, saveInitialListSettingsHandler, settings); err != nil {
				return fmt.Errorf("saving initial settings for %q: %w", list.GetName(), err)
			}
			verifiedData, err := client.GetUserData(ctx)
			if err != nil {
				return fmt.Errorf("verifying initial settings for %q: %w", list.GetName(), err)
			}
			verified, found := findLiveListSettingsByListID(verifiedData, list.GetIdentifier())
			if !found {
				return fmt.Errorf("initial settings verification failed: settings for list %q were not read back", list.GetName())
			}
			if err := st.SyncFromUserData(verifiedData); err != nil {
				return fmt.Errorf("updating local cache after saving initial settings: %w", err)
			}
			result := map[string]any{
				"saved":       true,
				"verified":    true,
				"id":          list.GetIdentifier(),
				"name":        list.GetName(),
				"settings_id": verified.GetIdentifier(),
			}
			if flags.quiet {
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Shopping list name")
	cmd.Flags().StringVar(&sortOrder, "sort-order", "", "Initial item sort order")
	cmd.Flags().BoolVar(&hideCategories, "hide-categories", false, "Hide categories")
	cmd.Flags().StringVar(&categoryGroupingID, "category-grouping-id", "", "Category grouping ID")
	cmd.Flags().StringVar(&listCategoryGroupID, "list-category-group-id", "", "List category-group ID")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply the save; preview is the default")
	return cmd
}
