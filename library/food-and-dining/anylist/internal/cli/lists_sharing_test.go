// Copyright 2026 Jeeves and contributors. Licensed under Apache-2.0.

package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
	"github.com/spf13/cobra"
)

func TestListsSharingCommandsAreRegistered(t *testing.T) {
	t.Parallel()

	root := newListsCmd(&rootFlags{})
	sharing, _, err := root.Find([]string{"sharing"})
	if err != nil || sharing == nil {
		t.Fatalf("Find(sharing) = %#v, %v", sharing, err)
	}
	if !sharing.Hidden {
		t.Fatal("lists sharing parent command should stay hidden")
	}
	list, _, err := root.Find([]string{"sharing", "list"})
	if err != nil || list == nil {
		t.Fatalf("Find(sharing list) = %#v, %v", list, err)
	}
	if list.Short == "" || !strings.Contains(strings.ToLower(list.Short), "list") {
		t.Fatalf("sharing list Short = %q", list.Short)
	}
}

func TestListsSharingAddCommandIsRegistered(t *testing.T) {
	t.Parallel()
	root := newListsCmd(&rootFlags{})
	add, _, err := root.Find([]string{"sharing", "add"})
	if err != nil || add == nil {
		t.Fatalf("Find(sharing add) = %#v, %v", add, err)
	}
	if !strings.Contains(strings.ToLower(add.Short), "invite") {
		t.Fatalf("sharing add Short = %q, want invite wording", add.Short)
	}
	if got := add.Annotations["pp:path"]; got != "/data/shopping-lists/share-list" {
		t.Fatalf("add annotation pp:path = %q, want /data/shopping-lists/share-list", got)
	}
	if _, err := add.Flags().GetString("email"); err != nil {
		t.Fatalf("email flag missing: %v", err)
	}
	apply, err := add.Flags().GetBool("apply")
	if err != nil || apply {
		t.Fatalf("apply flag = %v, %v; default must be false", apply, err)
	}
}

func TestListsSharingAddPreviewsByDefaultAndDryRunStaysOffline(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		flags *rootFlags
	}{
		{name: "default preview", flags: &rootFlags{}},
		{name: "dry run", flags: &rootFlags{dryRun: true}},
		{name: "dry run overrides apply", flags: &rootFlags{dryRun: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			flags := tc.flags
			flags.asJSON = true
			cmd := newListsSharingAddCmd(flags)
			var out bytes.Buffer
			cmd.SetOut(&out)
			args := []string{"GROCERIES", "--email", "person@example.com"}
			if tc.name == "dry run overrides apply" {
				args = append(args, "--apply")
			}
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("preview returned error: %v", err)
			}
			var result map[string]any
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatalf("preview output is not JSON: %v\n%s", err, out.String())
			}
			if result["dry_run"] != true || result["email"] != "person@example.com" || result["list_selector"] != "GROCERIES" {
				t.Fatalf("preview result = %#v", result)
			}
		})
	}
}

func TestListsSharingAddApplyGoesLiveWithoutPreview(t *testing.T) {
	t.Parallel()
	flags := &rootFlags{asJSON: true, configPath: filepath.Join(t.TempDir(), "config.toml")}
	cmd := newListsSharingAddCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"GROCERIES", "--email", "person@example.com", "--apply"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("--apply without authentication returned nil error")
	}
	if strings.Contains(err.Error(), "Dry run") {
		t.Fatalf("--apply must not fall back to a preview: %v", err)
	}
}

func TestListsSharingAddRejectsInvalidEmailAndMissingSelector(t *testing.T) {
	t.Parallel()
	for name, email := range map[string]string{"invalid email": "person", "empty email": "", "blank email": "  "} {
		t.Run(name, func(t *testing.T) {
			flags := &rootFlags{asJSON: true}
			cmd := newListsSharingAddCmd(flags)
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetArgs([]string{"GROCERIES", "--email", email})
			if err := cmd.Execute(); err == nil {
				t.Fatalf("--email %q accepted in preview", email)
			}
		})
	}
	t.Run("blank list selector", func(t *testing.T) {
		flags := &rootFlags{asJSON: true}
		cmd := newListsSharingAddCmd(flags)
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"   ", "--email", "person@example.com"})
		if err := cmd.Execute(); err == nil {
			t.Fatal("blank list selector accepted in preview")
		}
	})
}

func TestListsSharingResolveTarget(t *testing.T) {
	t.Parallel()
	data := &pb.PBUserDataResponse{ShoppingListsResponse: &pb.ShoppingListsResponse{
		NewLists:      []*pb.ShoppingList{{Identifier: "id-a", Name: "Groceries"}, {Identifier: "id-b", Name: "Dinner"}, {Identifier: "id-c", Name: "Dinner"}, nil},
		ModifiedLists: []*pb.ShoppingList{{Identifier: "id-a", Name: "Groceries renamed"}, {Identifier: "", Name: "no-id"}},
	}}
	list, err := resolveListsSharingTarget(data, "id-a")
	if err != nil || list.GetName() != "Groceries renamed" {
		t.Fatalf("resolve by ID = %q, %v; want modified Groceries renamed", list.GetName(), err)
	}
	list, err = resolveListsSharingTarget(data, "Groceries renamed")
	if err != nil || list.GetIdentifier() != "id-a" {
		t.Fatalf("resolve by unique name = %q, %v; want id-a", list.GetIdentifier(), err)
	}
	if _, err = resolveListsSharingTarget(data, "Dinner"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("ambiguous name error = %v, want refusal", err)
	}
	if _, err = resolveListsSharingTarget(data, "Nope"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing selector error = %v, want not-found refusal", err)
	}
	view := listsSharingView(data)
	if len(view) != 3 || view[0]["id"] != "id-a" || view[0]["name"] != "Groceries renamed" {
		t.Fatalf("view = %#v, want modified id-a plus two dinner lists", view)
	}
}

func TestListsSharingInviteVerificationReportsPendingHonestly(t *testing.T) {
	t.Parallel()
	requested := &pb.ShoppingList{Identifier: "list-1", Name: "Groceries"}
	response := &pb.PBShareListOperationResponse{SharedUser: &pb.PBEmailUserIDPair{Email: "person@example.com"}}
	fresh := &pb.PBUserDataResponse{ShoppingListsResponse: &pb.ShoppingListsResponse{ModifiedLists: []*pb.ShoppingList{{Identifier: "list-1", Name: "Groceries", SharedUsers: []*pb.PBEmailUserIDPair{{Email: "person@example.com"}}}}}}
	result, message, err := listsSharingInviteVerification(fresh, requested, "person@example.com", response)
	if err != nil || result["status"] != "pending-invitation" || result["shared_user_present"] != false || !strings.Contains(message, "pending invitation") || strings.Contains(message, "now lists") {
		t.Fatalf("pending verification = %#v, %q, %v", result, message, err)
	}
	accepted := &pb.PBUserDataResponse{ShoppingListsResponse: &pb.ShoppingListsResponse{ModifiedLists: []*pb.ShoppingList{{Identifier: "list-1", Name: "Groceries", SharedUsers: []*pb.PBEmailUserIDPair{{Email: "Person@Example.com", UserId: "user-2"}}}}}}
	result, message, err = listsSharingInviteVerification(accepted, requested, "person@example.com", response)
	if err != nil || result["status"] != "shared-user-present" || result["shared_user_present"] != true || strings.Contains(message, "pending") {
		t.Fatalf("accepted verification = %#v, %q, %v", result, message, err)
	}
	if _, _, err := listsSharingInviteVerification(&pb.PBUserDataResponse{}, requested, "person@example.com", response); err == nil || !strings.Contains(err.Error(), "no longer present") {
		t.Fatalf("missing-list verification = %v, want failure", err)
	}
	wrong := &pb.PBShareListOperationResponse{SharedUser: &pb.PBEmailUserIDPair{Email: "someone-else@example.com"}}
	if _, _, err := listsSharingInviteVerification(fresh, requested, "person@example.com", wrong); err == nil || !strings.Contains(err.Error(), "identifies") {
		t.Fatalf("wrong-address verification = %v, want failure", err)
	}
	if _, _, err := listsSharingInviteVerification(fresh, requested, "person@example.com", &pb.PBShareListOperationResponse{}); err == nil {
		t.Fatal("verification accepted a response without a shared user")
	}
}

func TestListsSharingJSONProjection(t *testing.T) {
	t.Parallel()

	data := &pb.PBUserDataResponse{ShoppingListsResponse: &pb.ShoppingListsResponse{
		NewLists: []*pb.ShoppingList{
			{
				Identifier: "list-b",
				Name:       "Dinner prep",
				Creator:    "creator@example.com",
				SharedUsers: []*pb.PBEmailUserIDPair{
					{Email: "friend@example.com", UserId: "user-2", FullName: "Friend Two"},
					nil,
				},
			},
		},
		ModifiedLists: []*pb.ShoppingList{
			{
				Identifier:  "list-a",
				Name:        "Groceries",
				Creator:     "creator@example.com",
				SharedUsers: []*pb.PBEmailUserIDPair{{Email: "mate@example.com", UserId: "user-1"}},
			},
			nil,
		},
	}}

	lists := listsSharingView(data)
	if len(lists) != 2 {
		t.Fatalf("listsSharingView returned %d lists, want 2 (nil entries skipped)", len(lists))
	}
	if lists[0]["id"] != "list-a" || lists[1]["id"] != "list-b" {
		t.Fatalf("view not sorted by id: %#v", lists)
	}
	if lists[0]["name"] != "Groceries" || lists[0]["creator"] != "creator@example.com" {
		t.Fatalf("list-a projection = %#v", lists[0])
	}

	encoded, err := json.Marshal(lists)
	if err != nil {
		t.Fatalf("marshaling view: %v", err)
	}
	var decoded []map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshaling view: %v", err)
	}
	firstUsers := decoded[0]["shared_users"].([]any)
	if len(firstUsers) != 1 {
		t.Fatalf("list-a shared_users = %#v, want exactly one user", firstUsers)
	}
	if u := firstUsers[0].(map[string]any); u["email"] != "mate@example.com" || u["user_id"] != "user-1" || u["full_name"] != "" {
		t.Fatalf("list-a user = %#v, want empty full_name when absent", u)
	}
	secondUsers := decoded[1]["shared_users"].([]any)
	if len(secondUsers) != 2 {
		t.Fatalf("list-b shared_users = %#v, want two entries (nil entry projected, not dropped)", secondUsers)
	}
	if u := secondUsers[1].(map[string]any); u["email"] != "" || u["user_id"] != "" || u["full_name"] != "" {
		t.Fatalf("nil shared user = %#v, want all-empty fields", u)
	}
}

func TestListsSharingEmptyUsersAndNilResponse(t *testing.T) {
	t.Parallel()

	if got := listsSharingView(nil); len(got) != 0 {
		t.Fatalf("listsSharingView(nil) = %#v, want empty slice", got)
	}

	data := &pb.PBUserDataResponse{ShoppingListsResponse: &pb.ShoppingListsResponse{
		NewLists: []*pb.ShoppingList{{Identifier: "list-a", Name: "Groceries", Creator: "creator@example.com"}},
	}}
	lists := listsSharingView(data)
	if len(lists) != 1 {
		t.Fatalf("listsSharingView returned %d lists, want 1", len(lists))
	}
	users := lists[0]["shared_users"].([]map[string]any)
	if len(users) != 0 {
		t.Fatalf("shared_users = %#v, want empty slice", users)
	}

	encoded, err := json.Marshal(lists)
	if err != nil {
		t.Fatalf("marshaling view: %v", err)
	}
	if !strings.Contains(string(encoded), `"shared_users":[]`) {
		t.Fatalf("empty shared users must marshal as []: %s", encoded)
	}

	var out bytes.Buffer
	textCmd := &cobra.Command{}
	textCmd.SetOut(&out)
	if err := writeListsSharingText(textCmd, lists); err != nil {
		t.Fatalf("writeListsSharingText: %v", err)
	}
	if !strings.Contains(out.String(), "no shared users") {
		t.Fatalf("text output = %q, want empty-users marker", out.String())
	}
}

func TestListsSharingDryRunOfflinePreview(t *testing.T) {
	t.Parallel()

	cmd := newListsSharingListCmd(&rootFlags{asJSON: true, dryRun: true})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry run returned error: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("dry-run output is not JSON: %v\n%s", err, out.String())
	}
	if result["dry_run"] != true {
		t.Fatalf("result = %#v, want dry_run=true", result)
	}
	if lists := result["lists"].([]any); len(lists) != 0 {
		t.Fatalf("dry-run lists = %#v, want empty", lists)
	}
}
