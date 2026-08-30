// Copyright 2026 Jeeves and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
	"github.com/spf13/cobra"
)

func newCategoriesReorderCmd(flags *rootFlags) *cobra.Command {
	var listToken, groupToken, orderFlag string
	var stdinBody, apply bool

	cmd := &cobra.Command{
		Use:         "reorder",
		Short:       "Reorder the categories of a shopping-list category group (preview unless --apply)",
		Long:        "Reorder the categories of one category group in the selected shopping list. The list and group are resolved by a fresh live read (the list must have exactly one group unless --category-group selects one), and --order must name every category in the group exactly once, by stable ID or exact name, in the desired order. Duplicates, unknown entries, and orders that silently append or drop categories fail closed. Preview is the default; --apply performs the live write on the proven /data/shopping-lists/update-v2 wire contract, fresh-reads the group, and verifies the exact stable-ID order before reporting success. Category-group create, rename, and delete (group CRUD) remain unsupported.",
		Example:     "  anylist-pp-cli categories reorder --list Groceries --order Produce,Dairy,\"Pantry Aisle\" --apply",
		Annotations: map[string]string{"pp:endpoint": "categories.reorder", "pp:method": "POST", "pp:path": "/data/shopping-lists/update-v2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if stdinBody {
				body, err := readStdinJSONMap()
				if err != nil {
					return err
				}
				listToken = stringFromBody(body, "list")
				groupToken = stringFromBody(body, "category_group")
				if groupToken == "" {
					groupToken = stringFromBody(body, "categoryGroup")
				}
				orderFlag = stringFromBody(body, "order")
				apply = boolFromBody(body, "apply")
			}
			listToken = strings.TrimSpace(listToken)
			groupToken = strings.TrimSpace(groupToken)
			orderFlag = strings.TrimSpace(orderFlag)
			if listToken == "" && !flags.dryRun {
				return fmt.Errorf("required flag \"list\" not set")
			}
			if orderFlag == "" && !flags.dryRun {
				return fmt.Errorf("required flag \"order\" not set")
			}
			orderTokens := splitCommaList(orderFlag)
			if !apply || flags.dryRun {
				preview := map[string]any{
					"dry_run":        true,
					"list":           listToken,
					"category_group": groupToken,
					"order":          orderTokens,
					"apply":          apply,
				}
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), preview, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Dry run: would reorder %d categories in list %q (pass --apply to write)\n", len(orderTokens), listToken)
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
			ordered, err := resolveCategoryReorderInGroup(group, orderTokens)
			if err != nil {
				return err
			}
			if err := alClient.ReorderListCategories(ctx, listID, group, ordered); err != nil {
				return fmt.Errorf("reordering categories in group %q: %w", group.GetName(), err)
			}
			verifiedData, err := alClient.GetUserData(ctx)
			if err != nil {
				return fmt.Errorf("verifying reordered categories in group %q: %w", group.GetName(), err)
			}
			verifiedGroup, err := verifyLiveCategoryReorder(verifiedData, listID, group.GetIdentifier(), ordered)
			if err != nil {
				return err
			}
			if err := st.SyncFromUserData(verifiedData); err != nil {
				return fmt.Errorf("updating local cache after reordering categories: %w", err)
			}
			if flags.quiet {
				return nil
			}
			result := map[string]any{
				"reordered": true,
				"list":      listID,
				"group":     verifiedGroup.GetIdentifier(),
				"order":     orderedCategoryIDs(ordered),
				"verified":  true,
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Reordered %d categories in group %q of list %q\n", len(ordered), group.GetName(), list.GetName())
			return nil
		},
	}
	cmd.Flags().StringVar(&listToken, "list", "", "Target list (stable ID or exact name)")
	cmd.Flags().StringVar(&groupToken, "category-group", "", "Target category group (stable ID or exact name); required when the list has more than one group")
	cmd.Flags().StringVar(&orderFlag, "order", "", "Comma-separated order naming every category in the group exactly once, by stable ID or exact name")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply the reorder; preview is the default")
	return cmd
}

// splitCommaList splits a comma-separated flag value into trimmed tokens.
// Empty input yields no tokens; whitespace-only entries survive here and are
// rejected later by resolution, so preview and apply share one code path.
func splitCommaList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	tokens := make([]string, 0, len(parts))
	for _, part := range parts {
		tokens = append(tokens, strings.TrimSpace(part))
	}
	return tokens
}

func orderedCategoryIDs(ordered []*pb.PBListCategory) []string {
	ids := make([]string, len(ordered))
	for i, category := range ordered {
		ids[i] = category.GetIdentifier()
	}
	return ids
}
