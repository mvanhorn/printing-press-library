// Copyright 2026 Jeeves and contributors. Licensed under Apache-2.0.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
	"github.com/spf13/cobra"
)

func TestRecipeSharingValidatesEmailAndSelectors(t *testing.T) {
	t.Parallel()

	if got, err := validateRecipeLinkEmail("Person@Example.com"); err != nil || got != "person@example.com" {
		t.Fatalf("validateRecipeLinkEmail = %q, %v", got, err)
	}
	for _, email := range []string{"person", "Person <person@example.com>", "person@example.com ", "@example.com"} {
		if _, err := validateRecipeLinkEmail(email); err == nil {
			t.Fatalf("validateRecipeLinkEmail accepted %q", email)
		}
	}

	data := &pb.PBUserDataResponse{RecipeDataResponse: &pb.PBRecipeDataResponse{
		PendingRecipeLinkRequests: []*pb.PBRecipeLinkRequest{
			{Identifier: "request-1", ConfirmingEmail: "person@example.com"},
			{Identifier: "request-2", ConfirmingEmail: "other@example.com"},
		},
		RecipeLinkRequestsToConfirm: []*pb.PBRecipeLinkRequest{{Identifier: "incoming-1", RequestingUserId: "user-1"}},
		LinkedUsers:                 []*pb.PBEmailUserIDPair{{UserId: "user-1", Email: "person@example.com"}},
	}}
	if request, err := resolveRecipeLinkRequest(data, "request-1", ""); err != nil || request.GetIdentifier() != "request-1" {
		t.Fatalf("resolveRecipeLinkRequest by ID = %#v, %v", request, err)
	}
	if request, err := resolveRecipeLinkRequest(data, "", "other@example.com"); err != nil || request.GetIdentifier() != "request-2" {
		t.Fatalf("resolveRecipeLinkRequest by email = %#v, %v", request, err)
	}
	if _, err := resolveRecipeLinkRequest(data, "", "missing@example.com"); err == nil {
		t.Fatal("resolveRecipeLinkRequest accepted a missing email")
	}
	if got, err := resolveLinkedRecipeUser(data, "", "person@example.com"); err != nil || got != "user-1" {
		t.Fatalf("resolveLinkedRecipeUser = %q, %v", got, err)
	}
}

func TestRecipeSharingWriteCommandsDefaultToOfflinePreview(t *testing.T) {
	t.Parallel()

	flags := &rootFlags{asJSON: true, dryRun: true}
	commands := []struct {
		name string
		cmd  *cobra.Command
		args []string
	}{
		{name: "request", cmd: newRecipesSharingRequestCmd(flags), args: []string{"--email", "person@example.com"}},
		{name: "cancel", cmd: newRecipesSharingCancelCmd(flags), args: []string{"--id", "request-1"}},
		{name: "accept", cmd: newRecipesSharingAcceptCmd(flags), args: []string{"--id", "incoming-1"}},
		{name: "unlink", cmd: newRecipesSharingUnlinkCmd(flags), args: []string{"--user-id", "user-1"}},
		{name: "list", cmd: newRecipesSharingListCmd(flags), args: nil},
	}
	for _, test := range commands {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			test.cmd.SetOut(&out)
			test.cmd.SetArgs(test.args)
			if err := test.cmd.Execute(); err != nil {
				t.Fatalf("preview returned error: %v", err)
			}
			var result map[string]any
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatalf("preview output is not JSON: %v\n%s", err, out.String())
			}
			if result["dry_run"] != true {
				t.Fatalf("result = %#v, want dry_run=true", result)
			}
		})
	}
}

func TestRecipeSharingCommandsAreRegistered(t *testing.T) {
	t.Parallel()

	root := newRecipesCmd(&rootFlags{})
	for _, path := range [][]string{{"link"}, {"sharing", "list"}, {"sharing", "request"}, {"sharing", "cancel"}, {"sharing", "accept"}, {"sharing", "unlink"}} {
		cmd, _, err := root.Find(path)
		if err != nil || cmd == nil {
			t.Fatalf("Find(%v) = %#v, %v", path, cmd, err)
		}
	}
}

func TestRecipeSharingRejectsUnsafeWriteSelectors(t *testing.T) {
	t.Parallel()

	cmd := newRecipesSharingCancelCmd(&rootFlags{})
	cmd.SetArgs([]string{"--id", "request-1", "--email", "person@example.com"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("cancel error = %v, want mutually exclusive selectors", err)
	}
	cmd = newRecipesSharingUnlinkCmd(&rootFlags{})
	cmd.SetArgs([]string{"--user-id", "user-1", "--email", "person@example.com"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("unlink error = %v, want mutually exclusive selectors", err)
	}
}

func TestRecipesLinkRequiresNameOffline(t *testing.T) {
	t.Parallel()

	cmd := newRecipesLinkCmd(&rootFlags{})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), `required flag "name" not set`) {
		t.Fatalf("link error = %v, want required name error", err)
	}
}
