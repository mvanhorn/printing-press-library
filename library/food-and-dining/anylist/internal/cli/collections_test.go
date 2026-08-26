package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
	"github.com/spf13/cobra"
)

func TestCollectionWritesPreviewUnlessApply(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		new  func(*rootFlags) *cobra.Command
		args []string
	}{
		{name: "create", new: newCollectionsCreateCmd, args: []string{"--name", "Temporary collection"}},
		{name: "add", new: newCollectionsAddCmd, args: []string{"--collection", "Temporary collection", "--recipe", "Temporary recipe"}},
		{name: "remove", new: newCollectionsRemoveCmd, args: []string{"--collection", "Temporary collection", "--recipe", "Temporary recipe"}},
		{name: "delete", new: newCollectionsDeleteCmd, args: []string{"--name", "Temporary collection"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			flags := &rootFlags{asJSON: true}
			cmd := tc.new(flags)
			if flag := cmd.Flags().Lookup("apply"); flag == nil || flag.DefValue != "false" {
				t.Fatalf("apply flag = %#v, want a false default", flag)
			}
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetArgs(tc.args)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("preview returned error: %v", err)
			}
			var preview map[string]any
			if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
				t.Fatalf("preview output is not JSON: %v\n%s", err, out.String())
			}
			if preview["status"] != "preview" || preview["apply"] != false {
				t.Fatalf("preview gate = %#v, want status=preview/apply=false", preview)
			}
		})
	}
}

// executeCollectionsStdin drives a collection write command with --stdin,
// swapping os.Stdin for a pipe the way the other stdin tests do.
func executeCollectionsStdin(t *testing.T, flags *rootFlags, new func(*rootFlags) *cobra.Command, body string, args ...string) (string, error) {
	t.Helper()
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating stdin pipe: %v", err)
	}
	os.Stdin = r
	defer func() {
		os.Stdin = oldStdin
		r.Close()
		w.Close()
	}()
	if _, err := w.WriteString(body); err != nil {
		t.Fatalf("writing stdin: %v", err)
	}
	w.Close()

	cmd := new(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs(append(append([]string{}, args...), "--stdin"))
	err = cmd.Execute()
	return out.String(), err
}

// TestCollectionWritesStdinApplyPrecedence pins the final apply decision for
// every combination of the --apply flag and a stdin apply value (omitted,
// false, true) across all four collection write commands. An explicit --apply
// must survive stdin (including stdin apply:false); stdin apply:true may
// enable the operation; and only a true final decision reaches the
// fail-closed live-write gate instead of the preview.
func TestCollectionWritesStdinApplyPrecedence(t *testing.T) {
	commands := []struct {
		name string
		new  func(*rootFlags) *cobra.Command
		args []string
	}{
		{name: "create", new: newCollectionsCreateCmd, args: []string{"--name", "Temporary collection"}},
		{name: "add", new: newCollectionsAddCmd, args: []string{"--collection", "Temporary collection", "--recipe", "Temporary recipe"}},
		{name: "remove", new: newCollectionsRemoveCmd, args: []string{"--collection", "Temporary collection", "--recipe", "Temporary recipe"}},
		{name: "delete", new: newCollectionsDeleteCmd, args: []string{"--name", "Temporary collection"}},
	}
	stdinCases := []struct {
		name       string
		body       string
		stdinApply bool
	}{
		{name: "stdin-omitted", body: "{}", stdinApply: false},
		{name: "stdin-false", body: `{"apply":false}`, stdinApply: false},
		{name: "stdin-true", body: `{"apply":true}`, stdinApply: true},
	}
	for _, command := range commands {
		for _, stdinCase := range stdinCases {
			for _, flagApply := range []bool{false, true} {
				name := fmt.Sprintf("%s/%s/apply-flag=%v", command.name, stdinCase.name, flagApply)
				t.Run(name, func(t *testing.T) {
					args := append([]string{}, command.args...)
					if flagApply {
						args = append(args, "--apply")
					}
					out, err := executeCollectionsStdin(t, &rootFlags{asJSON: true}, command.new, stdinCase.body, args...)
					if wantApply := flagApply || stdinCase.stdinApply; wantApply {
						// Final apply=true: the preview gate is passed and the
						// command must fail closed at the live-write gate.
						if err == nil || !strings.Contains(err.Error(), "live writes are disabled") {
							t.Fatalf("expected fail-closed live-write gate error, got err=%v out=%s", err, out)
						}
						return
					}
					if err != nil {
						t.Fatalf("preview returned error: %v", err)
					}
					var preview map[string]any
					if err := json.Unmarshal([]byte(out), &preview); err != nil {
						t.Fatalf("preview output is not JSON: %v\n%s", err, out)
					}
					if preview["status"] != "preview" || preview["apply"] != false || preview["dry_run"] != true {
						t.Fatalf("preview = %#v, want status=preview/apply=false/dry_run=true", preview)
					}
				})
			}
		}
	}
}

func TestValidateRecipeCollectionReadBackChecksIdentityMembershipsAndSettings(t *testing.T) {
	t.Parallel()

	expected := &pb.PBRecipeCollection{
		Identifier: "collection-1",
		Name:       "Weeknight dinners",
		RecipeIds:  []string{"recipe-1", "recipe-2"},
		CollectionSettings: &pb.PBRecipeCollectionSettings{
			RecipesSortOrder:                2,
			ShowOnlyRecipesWithNoCollection: true,
		},
	}
	cases := []struct {
		name    string
		mutate  func(*pb.PBRecipeCollection)
		wantErr string
	}{
		{name: "identifier", mutate: func(actual *pb.PBRecipeCollection) { actual.Identifier = "other" }, wantErr: "ID"},
		{name: "name", mutate: func(actual *pb.PBRecipeCollection) { actual.Name = "Other" }, wantErr: "name"},
		{name: "memberships", mutate: func(actual *pb.PBRecipeCollection) { actual.RecipeIds = []string{"recipe-1"} }, wantErr: "memberships"},
		{name: "settings", mutate: func(actual *pb.PBRecipeCollection) { actual.CollectionSettings.RecipesSortOrder = 4 }, wantErr: "settings"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actual := &pb.PBRecipeCollection{
				Identifier:         expected.Identifier,
				Name:               expected.Name,
				RecipeIds:          append([]string(nil), expected.RecipeIds...),
				CollectionSettings: &pb.PBRecipeCollectionSettings{RecipesSortOrder: 2, ShowOnlyRecipesWithNoCollection: true},
			}
			tc.mutate(actual)
			if err := validateRecipeCollectionReadBack(expected, actual); err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("validateRecipeCollectionReadBack error = %v, want substring %q", err, tc.wantErr)
			}
		})
	}

	actual := &pb.PBRecipeCollection{
		Identifier: expected.Identifier,
		Name:       expected.Name,
		RecipeIds:  append([]string(nil), expected.RecipeIds...),
		CollectionSettings: &pb.PBRecipeCollectionSettings{
			RecipesSortOrder:                2,
			ShowOnlyRecipesWithNoCollection: true,
		},
		Timestamp: expected.Timestamp + 1,
	}
	if err := validateRecipeCollectionReadBack(expected, actual); err != nil {
		t.Fatalf("validateRecipeCollectionReadBack returned error for matching live collection: %v", err)
	}
}

func TestFindLiveRecipeCollectionByID(t *testing.T) {
	t.Parallel()

	data := &pb.PBUserDataResponse{RecipeDataResponse: &pb.PBRecipeDataResponse{
		RecipeCollections: []*pb.PBRecipeCollection{{Identifier: "collection-1", Name: "Dinners"}},
	}}
	collection, found := findLiveRecipeCollectionByID(data, "collection-1")
	if !found || collection.GetName() != "Dinners" {
		t.Fatalf("findLiveRecipeCollectionByID found = %v, collection = %#v", found, collection)
	}
	if _, found := findLiveRecipeCollectionByID(data, "missing"); found {
		t.Fatal("findLiveRecipeCollectionByID found a missing collection")
	}
}
