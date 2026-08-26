package cli

import (
	"bytes"
	"encoding/json"
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
