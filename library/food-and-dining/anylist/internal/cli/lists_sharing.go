// Copyright 2026 Jeeves and contributors. Licensed under Apache-2.0.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
	"github.com/spf13/cobra"
)

func newListsSharingCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "sharing",
		Short:  "Inspect shopping list sharing state and invite a user (live-proven; removal and household writes stay fail-closed)",
		Hidden: true,
	}
	cmd.AddCommand(newListsSharingListCmd(flags))
	cmd.AddCommand(newListsSharingAddCmd(flags))
	return cmd
}

func newListsSharingListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List shopping lists with the shared users already present in user data",
		Example:     "  anylist-pp-cli lists sharing list --json",
		Annotations: map[string]string{"pp:endpoint": "lists.sharing.list", "pp:method": "POST", "pp:path": "/data/user-data/get", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return printJSONOrText(cmd, flags, map[string]any{"dry_run": true, "lists": []any{}}, "Dry run: would read shopping list sharing state\n")
			}
			_, client, err := openAuthedRecipeClient(flags)
			if err != nil {
				return err
			}
			data, err := client.GetUserData(cmd.Context())
			if err != nil {
				return err
			}
			lists := listsSharingView(data)
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), lists, flags)
			}
			return writeListsSharingText(cmd, lists)
		},
	}
	return cmd
}

// listsSharingView projects the shared users already present in the
// PBUserDataResponse shopping-list payload. It never invents users: missing
// optional fields stay empty and nil entries project to empty strings.
func listsSharingView(data *pb.PBUserDataResponse) []map[string]any {
	byID := listsShoppingListsByIndex(data)
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	lists := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		list := byID[id]
		shared := make([]map[string]any, 0, len(list.GetSharedUsers()))
		for _, user := range list.GetSharedUsers() {
			shared = append(shared, map[string]any{
				"email":     user.GetEmail(),
				"user_id":   user.GetUserId(),
				"full_name": user.GetFullName(),
			})
		}
		lists = append(lists, map[string]any{
			"id":           list.GetIdentifier(),
			"name":         list.GetName(),
			"creator":      list.GetCreator(),
			"shared_users": shared,
		})
	}
	return lists
}

func listsShoppingListsByIndex(data *pb.PBUserDataResponse) map[string]*pb.ShoppingList {
	byID := make(map[string]*pb.ShoppingList)
	for _, list := range data.GetShoppingListsResponse().GetNewLists() {
		if list == nil || list.GetIdentifier() == "" {
			continue
		}
		byID[list.GetIdentifier()] = list
	}
	for _, list := range data.GetShoppingListsResponse().GetModifiedLists() {
		if list == nil || list.GetIdentifier() == "" {
			continue
		}
		byID[list.GetIdentifier()] = list
	}
	return byID
}

func resolveListsSharingTarget(data *pb.PBUserDataResponse, selector string) (*pb.ShoppingList, error) {
	byID := listsShoppingListsByIndex(data)
	if list, ok := byID[selector]; ok {
		return list, nil
	}
	var matches []*pb.ShoppingList
	for _, list := range byID {
		if list.GetName() == selector {
			matches = append(matches, list)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("list %q not found (expected an exact list ID or a unique list name)", selector)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("list name %q is ambiguous: %d lists match; use the exact list ID", selector, len(matches))
	}
	return matches[0], nil
}

func listsSharingTargetPresent(list *pb.ShoppingList, targetEmail string) bool {
	for _, user := range list.GetSharedUsers() {
		// AnyList also returns an email-only PBEmailUserIDPair while an
		// unregistered invitation is pending. A stable user ID is the
		// observable boundary between that state and an accepted share.
		if strings.EqualFold(user.GetEmail(), targetEmail) && strings.TrimSpace(user.GetUserId()) != "" {
			return true
		}
	}
	return false
}

func newListsSharingAddCmd(flags *rootFlags) *cobra.Command {
	var email string
	var apply bool
	cmd := &cobra.Command{
		Use:         "add LIST",
		Short:       "Invite an exact email to a shopping list (live-proven; preview by default, --apply to write)",
		Example:     "  anylist-pp-cli lists sharing add GROCERIES --email person@example.com --apply --json",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"pp:endpoint": "lists.sharing.add", "pp:method": "POST", "pp:path": "/data/shopping-lists/share-list"},
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := validateRecipeLinkEmail(email)
			if err != nil {
				return err
			}
			selector := strings.TrimSpace(args[0])
			if selector == "" {
				return fmt.Errorf("a list ID or an unambiguous list name is required")
			}
			if !apply || flags.dryRun {
				return printJSONOrText(cmd, flags, map[string]any{"dry_run": true, "list_selector": selector, "email": target, "apply": apply}, fmt.Sprintf("Dry run: would invite %q to list %s (pass --apply to write)\n", target, selector))
			}
			_, client, err := openAuthedRecipeClient(flags)
			if err != nil {
				return err
			}
			data, err := client.GetUserData(cmd.Context())
			if err != nil {
				return fmt.Errorf("reading current shopping lists: %w", err)
			}
			list, err := resolveListsSharingTarget(data, selector)
			if err != nil {
				return err
			}
			if listsSharingTargetPresent(list, target) {
				return fmt.Errorf("%q already appears as an accepted shared user on list %q; re-inviting is not supported", target, list.GetIdentifier())
			}
			response, err := client.ShareListInvite(cmd.Context(), list.GetIdentifier(), target)
			if err != nil {
				return err
			}
			verified, err := client.GetUserData(cmd.Context())
			if err != nil {
				return fmt.Errorf("verifying share-list invitation: %w", err)
			}
			result, message, err := listsSharingInviteVerification(verified, list, target, response)
			if err != nil {
				return err
			}
			return printWriteResult(cmd, flags, result, message+"\n")
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "Invitee's exact email address")
	cmd.Flags().BoolVar(&apply, "apply", false, "Enable the live invitation")
	return cmd
}

func listsSharingInviteVerification(data *pb.PBUserDataResponse, list *pb.ShoppingList, targetEmail string, response *pb.PBShareListOperationResponse) (map[string]any, string, error) {
	verifiedList, ok := listsShoppingListsByIndex(data)[list.GetIdentifier()]
	if !ok || verifiedList == nil {
		return nil, "", fmt.Errorf("share-list invitation verification failed: list %q is no longer present in fresh user data", list.GetIdentifier())
	}
	identified := response.GetSharedUser().GetEmail()
	if !strings.EqualFold(identified, targetEmail) {
		return nil, "", fmt.Errorf("share-list invitation verification failed: response identifies %q, want the requested address %q", identified, targetEmail)
	}
	present := listsSharingTargetPresent(verifiedList, targetEmail)
	result := map[string]any{
		"invited": targetEmail, "list_id": list.GetIdentifier(), "list_name": verifiedList.GetName(),
		"shared_user_present": present, "status": "pending-invitation",
		"response": map[string]any{"shared_user_email": identified, "status_code": response.GetStatusCode()},
	}
	if present {
		result["status"] = "shared-user-present"
		return result, fmt.Sprintf("List %q (%s) now lists %q as an accepted shared user", verifiedList.GetName(), list.GetIdentifier(), targetEmail), nil
	}
	return result, fmt.Sprintf("Invitation sent to %q for list %q (%s) — pending invitation; %q does not yet appear as an accepted shared user until the invite is accepted", targetEmail, verifiedList.GetName(), list.GetIdentifier(), targetEmail), nil
}

func writeListsSharingText(cmd *cobra.Command, lists []map[string]any) error {
	if len(lists) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No lists with sharing data present")
		return nil
	}
	tw := newTabWriter(cmd.OutOrStdout())
	fmt.Fprintln(tw, "NAME\tID\tCREATOR\tSHARED USERS")
	for _, list := range lists {
		shared := list["shared_users"].([]map[string]any)
		users := make([]string, 0, len(shared))
		for _, user := range shared {
			label := user["email"].(string)
			if user["full_name"].(string) != "" {
				label = user["full_name"].(string) + " <" + label + ">"
			}
			if user["user_id"].(string) != "" {
				label += " (" + user["user_id"].(string) + ")"
			}
			users = append(users, label)
		}
		if len(users) == 0 {
			users = append(users, "no shared users")
		}
		fmt.Fprintf(tw, "%v\t%v\t%v\t%s\n", list["name"], list["id"], list["creator"], strings.Join(users, ", "))
	}
	return tw.Flush()
}
