// Copyright 2026 Jeeves and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
	"github.com/spf13/cobra"
)

func newCategoriesCreateCmd(flags *rootFlags) *cobra.Command {
	var listToken, categoryName, groupToken string
	var stdinBody, apply bool

	cmd := &cobra.Command{
		Use:         "create",
		Short:       "Create a custom category in a shopping list (preview unless --apply)",
		Long:        "Create a custom category in the selected shopping list. The list and category group are resolved by a fresh live read; the new category gets a non-conflicting stable ID and is appended after the group's current entries. Preview is the default; --apply performs the live write on the proven /data/shopping-lists/update-v2 wire contract and verifies the new category by stable ID before reporting success. Category-group create, rename, and delete (group CRUD) are not supported.",
		Example:     "  anylist-pp-cli categories create --list Groceries --name \"Pantry Aisle\" --apply",
		Annotations: map[string]string{"pp:endpoint": "categories.create", "pp:method": "POST", "pp:path": "/data/shopping-lists/update-v2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if stdinBody {
				body, err := readStdinJSONMap()
				if err != nil {
					return err
				}
				listToken = stringFromBody(body, "list")
				categoryName = stringFromBody(body, "name")
				groupToken = stringFromBody(body, "category_group")
				if groupToken == "" {
					groupToken = stringFromBody(body, "categoryGroup")
				}
				apply = boolFromBody(body, "apply")
			}
			listToken = strings.TrimSpace(listToken)
			categoryName = strings.TrimSpace(categoryName)
			if listToken == "" && !flags.dryRun {
				return fmt.Errorf("required flag \"list\" not set")
			}
			if categoryName == "" && !flags.dryRun {
				return fmt.Errorf("required flag \"name\" not set")
			}
			if !apply || flags.dryRun {
				preview := map[string]any{
					"dry_run":        true,
					"list":           listToken,
					"name":           categoryName,
					"category_group": strings.TrimSpace(groupToken),
					"apply":          apply,
				}
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), preview, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Dry run: would create category %q in list %q (pass --apply to write)\n", categoryName, listToken)
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}

			ctx := cmd.Context()
			cfg, st, err := openAuthedLocalStore(flags)
			if err != nil {
				return err
			}
			defer st.Close()

			alClient := anylist.New(cfg)
			userData, err := alClient.GetUserData(ctx)
			if err != nil {
				return fmt.Errorf("reading live lists: %w", err)
			}
			list, err := resolveCategoryListRecord(userData, listToken)
			if err != nil {
				return err
			}
			listID := list.GetIdentifier()
			group, err := selectCategoryGroupForCreate(userData, listID, groupToken)
			if err != nil {
				return err
			}
			if categoryNameConflictInList(userData, listID, categoryName, "") != nil {
				return fmt.Errorf("a category named %q already exists in list %q", categoryName, listID)
			}
			existing, err := allListCategories(userData, listID)
			if err != nil {
				return err
			}
			category := &pb.PBListCategory{
				Identifier:      newCategoryStableID(categoryName, existing),
				ListId:          listID,
				CategoryGroupId: group.GetIdentifier(),
				Name:            categoryName,
				SortIndex:       nextCategorySortIndex(group.GetCategories()),
			}
			if err := alClient.CreateListCategory(ctx, listID, category); err != nil {
				return fmt.Errorf("creating category %q: %w", categoryName, err)
			}
			verifiedData, err := alClient.GetUserData(ctx)
			if err != nil {
				return fmt.Errorf("verifying created category %q: %w", categoryName, err)
			}
			verified, err := verifyLiveCategoryCreate(verifiedData, listID, category)
			if err != nil {
				return err
			}
			if err := st.SyncFromUserData(verifiedData); err != nil {
				return fmt.Errorf("updating local cache after creating category: %w", err)
			}
			if flags.quiet {
				return nil
			}
			result := map[string]any{
				"created":    true,
				"id":         verified.GetIdentifier(),
				"name":       verified.GetName(),
				"list":       listID,
				"group":      verified.GetCategoryGroupId(),
				"sort_index": verified.GetSortIndex(),
				"verified":   true,
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created category %q (ID %s) in list %q\n", verified.GetName(), verified.GetIdentifier(), list.GetName())
			return nil
		},
	}
	cmd.Flags().StringVar(&listToken, "list", "", "Target list (stable ID or exact name)")
	cmd.Flags().StringVar(&categoryName, "name", "", "Name for the new category")
	cmd.Flags().StringVar(&groupToken, "category-group", "", "Target category group (stable ID or exact name); required when the list has more than one group")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply the creation; preview is the default")
	return cmd
}
