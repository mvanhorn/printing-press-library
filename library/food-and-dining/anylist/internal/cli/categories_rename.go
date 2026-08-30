// Copyright 2026 Jeeves and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
)

func newCategoriesRenameCmd(flags *rootFlags) *cobra.Command {
	var listToken, categoryToken, newName string
	var stdinBody, apply bool

	cmd := &cobra.Command{
		Use:         "rename",
		Short:       "Rename a custom category in a shopping list (preview unless --apply)",
		Long:        "Rename one custom category in the selected shopping list. The target category is resolved by a fresh live read using its stable ID or exact name; ambiguous or missing names fail closed. The stable identifier, category group, list, and sort index are preserved. Preview is the default; --apply performs the live write on the proven /data/shopping-lists/update-v2 wire contract and verifies the new name by stable ID before reporting success. Category-group create, rename, and delete (group CRUD) are not supported.",
		Example:     "  anylist-pp-cli categories rename --list Groceries --category \"Pantry Aisle\" --new-name \"Pantry\" --apply",
		Annotations: map[string]string{"pp:endpoint": "categories.rename", "pp:method": "POST", "pp:path": "/data/shopping-lists/update-v2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if stdinBody {
				body, err := readStdinJSONMap()
				if err != nil {
					return err
				}
				listToken = stringFromBody(body, "list")
				categoryToken = stringFromBody(body, "category")
				newName = stringFromBody(body, "new_name")
				if newName == "" {
					newName = stringFromBody(body, "newName")
				}
				apply = boolFromBody(body, "apply")
			}
			listToken = strings.TrimSpace(listToken)
			categoryToken = strings.TrimSpace(categoryToken)
			newName = strings.TrimSpace(newName)
			if listToken == "" && !flags.dryRun {
				return fmt.Errorf("required flag \"list\" not set")
			}
			if categoryToken == "" && !flags.dryRun {
				return fmt.Errorf("required flag \"category\" not set")
			}
			if newName == "" && !flags.dryRun {
				return fmt.Errorf("required flag \"new-name\" not set")
			}
			if !apply || flags.dryRun {
				preview := map[string]any{
					"dry_run":  true,
					"list":     listToken,
					"category": categoryToken,
					"new_name": newName,
					"apply":    apply,
				}
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), preview, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Dry run: would rename category %q to %q in list %q (pass --apply to write)\n", categoryToken, newName, listToken)
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
			original, err := resolveCategoryRecordInList(userData, listID, categoryToken)
			if err != nil {
				return err
			}
			if strings.EqualFold(original.GetName(), newName) {
				return fmt.Errorf("new category name must differ from the current name")
			}
			if conflict := categoryNameConflictInList(userData, listID, newName, original.GetIdentifier()); conflict != nil {
				return fmt.Errorf("a different category in list %q is already named %q", listID, newName)
			}
			updated := proto.Clone(original).(*pb.PBListCategory)
			updated.Name = newName
			if err := alClient.RenameListCategory(ctx, listID, original, updated); err != nil {
				return fmt.Errorf("renaming category %q: %w", original.GetName(), err)
			}
			verifiedData, err := alClient.GetUserData(ctx)
			if err != nil {
				return fmt.Errorf("verifying renamed category %q: %w", original.GetName(), err)
			}
			verified, err := verifyLiveCategoryRename(verifiedData, listID, original, updated)
			if err != nil {
				return err
			}
			if err := st.SyncFromUserData(verifiedData); err != nil {
				return fmt.Errorf("updating local cache after renaming category: %w", err)
			}
			if flags.quiet {
				return nil
			}
			result := map[string]any{
				"renamed":  true,
				"id":       verified.GetIdentifier(),
				"old_name": original.GetName(),
				"name":     verified.GetName(),
				"list":     listID,
				"group":    verified.GetCategoryGroupId(),
				"verified": true,
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Renamed category %q to %q in list %q\n", original.GetName(), verified.GetName(), list.GetName())
			return nil
		},
	}
	cmd.Flags().StringVar(&listToken, "list", "", "Target list (stable ID or exact name)")
	cmd.Flags().StringVar(&categoryToken, "category", "", "Category to rename (stable ID or exact name)")
	cmd.Flags().StringVar(&newName, "new-name", "", "New category name")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply the rename; preview is the default")
	return cmd
}
