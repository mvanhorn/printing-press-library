// Copyright 2026 Jeeves and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist"
	"github.com/spf13/cobra"
)

func newCategoriesDeleteCmd(flags *rootFlags) *cobra.Command {
	var listToken, categoryToken string
	var stdinBody, apply bool

	cmd := &cobra.Command{
		Use:         "delete",
		Short:       "Delete a custom category from a shopping list (preview unless --apply)",
		Long:        "Delete one custom category from the selected shopping list. The target category is resolved by a fresh live read using its stable ID or exact name; ambiguous or missing names fail closed, and system categories (such as the default Other category) cannot be deleted. Preview is the default; --apply performs the live write on the proven /data/shopping-lists/update-v2 wire contract, fresh-reads the list, and verifies the category is absent by stable ID before reporting success. Category-group create, rename, and delete (group CRUD) remain unsupported.",
		Example:     "  anylist-pp-cli categories delete --list Groceries --category \"Pantry Aisle\" --apply",
		Annotations: map[string]string{"pp:endpoint": "categories.delete", "pp:method": "POST", "pp:path": "/data/shopping-lists/update-v2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if stdinBody {
				body, err := readStdinJSONMap()
				if err != nil {
					return err
				}
				listToken = stringFromBody(body, "list")
				categoryToken = stringFromBody(body, "category")
				apply = boolFromBody(body, "apply")
			}
			listToken = strings.TrimSpace(listToken)
			categoryToken = strings.TrimSpace(categoryToken)
			if listToken == "" && !flags.dryRun {
				return fmt.Errorf("required flag \"list\" not set")
			}
			if categoryToken == "" && !flags.dryRun {
				return fmt.Errorf("required flag \"category\" not set")
			}
			if !apply || flags.dryRun {
				preview := map[string]any{
					"dry_run":  true,
					"list":     listToken,
					"category": categoryToken,
					"apply":    apply,
				}
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), preview, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Dry run: would delete category %q from list %q (pass --apply to write)\n", categoryToken, listToken)
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
			category, err := resolveCategoryRecordInList(userData, listID, categoryToken)
			if err != nil {
				return err
			}
			if err := ensureDeletableCategory(category); err != nil {
				return err
			}
			group, err := findCategoryGroupForCategory(userData, listID, category)
			if err != nil {
				return err
			}
			if err := alClient.DeleteListCategory(ctx, listID, group, category); err != nil {
				return fmt.Errorf("deleting category %q: %w", category.GetName(), err)
			}
			verifiedData, err := alClient.GetUserData(ctx)
			if err != nil {
				return fmt.Errorf("verifying deleted category %q: %w", category.GetName(), err)
			}
			if err := verifyLiveCategoryDelete(verifiedData, listID, category); err != nil {
				return err
			}
			if err := st.SyncFromUserData(verifiedData); err != nil {
				return fmt.Errorf("updating local cache after deleting category: %w", err)
			}
			if flags.quiet {
				return nil
			}
			result := map[string]any{
				"deleted":  true,
				"id":       category.GetIdentifier(),
				"name":     category.GetName(),
				"list":     listID,
				"group":    group.GetIdentifier(),
				"verified": true,
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted category %q (ID %s) from list %q\n", category.GetName(), category.GetIdentifier(), list.GetName())
			return nil
		},
	}
	cmd.Flags().StringVar(&listToken, "list", "", "Target list (stable ID or exact name)")
	cmd.Flags().StringVar(&categoryToken, "category", "", "Category to delete (stable ID or exact name)")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply the deletion; preview is the default")
	return cmd
}
