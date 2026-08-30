package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
)

func newRecipesPhotoCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "photo", Short: "Manage recipe photos"}
	cmd.AddCommand(newRecipesPhotoAttachCmd(flags))
	cmd.AddCommand(newRecipesPhotoClearCmd(flags))
	return cmd
}

func newRecipesPhotoAttachCmd(flags *rootFlags) *cobra.Command {
	var recipeName string
	var file string
	var apply bool

	cmd := &cobra.Command{
		Use:     "attach",
		Short:   "Upload and attach a photo to a recipe",
		Example: "  anylist-pp-cli recipes photo attach --name Pancakes --file pancakes.jpg --apply",
		Annotations: map[string]string{
			"pp:endpoint": "recipes.photo.attach",
			"pp:method":   "POST",
			"pp:path":     "/data/photos/upload + /data/user-recipe-data/update",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(recipeName) == "" {
				return fmt.Errorf("required flag \"name\" not set")
			}
			if strings.TrimSpace(file) == "" {
				return fmt.Errorf("required flag \"file\" not set")
			}
			info, err := anylist.InspectPhotoFile(file)
			if err != nil {
				return err
			}
			if !apply || flags.dryRun {
				preview := map[string]any{
					"dry_run":      true,
					"recipe":       recipeName,
					"file":         file,
					"size":         info.Size,
					"content_type": info.ContentType,
					"apply":        apply,
				}
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), preview, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Dry run: would attach a %s photo (%d bytes) to recipe %q (pass --apply to write)\n", info.ContentType, info.Size, recipeName)
				return nil
			}
			ctx := cmd.Context()
			cfg, st, err := openAuthedLocalStore(flags)
			if err != nil {
				return err
			}
			defer st.Close()
			userData, recipeDataID, err := currentRecipeData(ctx, cfg)
			if err != nil {
				return fmt.Errorf("reading live recipe data: %w", err)
			}
			current, err := findLiveRecipeByName(userData, recipeName)
			if err != nil {
				return err
			}
			alClient := anylist.New(cfg)
			photoID, _, err := alClient.UploadPhoto(ctx, file)
			if err != nil {
				return err
			}
			updated := appendRecipePhoto(current, photoID)
			if err := alClient.SaveRecipe(ctx, recipeDataID, updated, false); err != nil {
				return fmt.Errorf("saving recipe photo: %w (uploaded photo %q may need cleanup)", err, photoID)
			}
			verifiedData, err := alClient.GetUserData(ctx)
			if err != nil {
				return fmt.Errorf("verifying recipe photo: %w", err)
			}
			verified, found := findLiveRecipeByID(verifiedData, current.GetIdentifier())
			if !found {
				return fmt.Errorf("recipe photo verification failed: recipe %q is not present", current.GetName())
			}
			if !recipeHasPhoto(verified, photoID) {
				return fmt.Errorf("recipe photo verification failed: photo %q did not read back", photoID)
			}
			if err := st.SyncFromUserData(verifiedData); err != nil {
				return fmt.Errorf("updating local cache after recipe photo attach: %w", err)
			}
			result := map[string]any{
				"attached":      true,
				"verified":      true,
				"cache_updated": true,
				"photo_id":      photoID,
				"recipe":        verified.GetName(),
			}
			if flags.quiet {
				return nil
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Attached photo %q to recipe %q\n", photoID, verified.GetName())
			return nil
		},
	}
	cmd.Flags().StringVarP(&recipeName, "name", "n", "", "Recipe name")
	cmd.Flags().StringVar(&file, "file", "", "Photo file to upload")
	cmd.Flags().BoolVar(&apply, "apply", false, "Enable the live upload and recipe save")
	return cmd
}

func newRecipesPhotoClearCmd(flags *rootFlags) *cobra.Command {
	var recipeName string
	var photoID string
	var apply bool

	cmd := &cobra.Command{
		Use:     "clear",
		Short:   "Remove one or all photos from a recipe",
		Example: "  anylist-pp-cli recipes photo clear --name Pancakes --apply",
		Annotations: map[string]string{
			"pp:endpoint": "recipes.photo.clear",
			"pp:method":   "POST",
			"pp:path":     "/data/user-recipe-data/update",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(recipeName) == "" {
				return fmt.Errorf("required flag \"name\" not set")
			}
			if !apply || flags.dryRun {
				preview := map[string]any{"dry_run": true, "recipe": recipeName, "photo_id": photoID, "clear_all": photoID == "", "apply": apply}
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), preview, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Dry run: would remove %s photo(s) from recipe %q (pass --apply to write)\n", map[bool]string{true: "all", false: "the specified"}[photoID == ""], recipeName)
				return nil
			}
			ctx := cmd.Context()
			cfg, st, err := openAuthedLocalStore(flags)
			if err != nil {
				return err
			}
			defer st.Close()
			userData, recipeDataID, err := currentRecipeData(ctx, cfg)
			if err != nil {
				return fmt.Errorf("reading live recipe data: %w", err)
			}
			current, err := findLiveRecipeByName(userData, recipeName)
			if err != nil {
				return err
			}
			updated, removed := removeRecipePhoto(current, photoID)
			if photoID != "" && !removed {
				return fmt.Errorf("photo %q is not attached to recipe %q", photoID, current.GetName())
			}
			if photoID == "" && len(current.GetPhotoIds()) == 0 {
				return fmt.Errorf("recipe %q has no photos to clear", current.GetName())
			}
			if err := anylist.New(cfg).SaveRecipe(ctx, recipeDataID, updated, false); err != nil {
				return fmt.Errorf("clearing recipe photo: %w", err)
			}
			verifiedData, err := anylist.New(cfg).GetUserData(ctx)
			if err != nil {
				return fmt.Errorf("verifying recipe photo clear: %w", err)
			}
			verified, found := findLiveRecipeByID(verifiedData, current.GetIdentifier())
			if !found {
				return fmt.Errorf("recipe photo clear verification failed: recipe %q is not present", current.GetName())
			}
			if photoID == "" {
				if len(verified.GetPhotoIds()) != 0 {
					return fmt.Errorf("recipe photo clear verification failed: %d photos remain", len(verified.GetPhotoIds()))
				}
			} else if recipeHasPhoto(verified, photoID) {
				return fmt.Errorf("recipe photo clear verification failed: photo %q remains", photoID)
			}
			if err := st.SyncFromUserData(verifiedData); err != nil {
				return fmt.Errorf("updating local cache after recipe photo clear: %w", err)
			}
			result := map[string]any{"cleared": true, "verified": true, "cache_updated": true, "photo_id": photoID, "recipe": verified.GetName()}
			if flags.quiet {
				return nil
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Cleared recipe photo(s) from %q\n", verified.GetName())
			return nil
		},
	}
	cmd.Flags().StringVarP(&recipeName, "name", "n", "", "Recipe name")
	cmd.Flags().StringVar(&photoID, "photo-id", "", "Remove only this photo ID (default removes all)")
	cmd.Flags().BoolVar(&apply, "apply", false, "Enable the live recipe save")
	return cmd
}

func appendRecipePhoto(recipe *pb.PBRecipe, photoID string) *pb.PBRecipe {
	updated := proto.Clone(recipe).(*pb.PBRecipe)
	if !recipeHasPhoto(updated, photoID) {
		updated.PhotoIds = append(updated.PhotoIds, photoID)
	}
	return updated
}

func removeRecipePhoto(recipe *pb.PBRecipe, photoID string) (*pb.PBRecipe, bool) {
	updated := proto.Clone(recipe).(*pb.PBRecipe)
	if photoID == "" {
		removed := len(updated.GetPhotoIds()) > 0
		updated.PhotoIds = nil
		return updated, removed
	}
	kept := make([]string, 0, len(updated.GetPhotoIds()))
	removed := false
	for _, id := range updated.GetPhotoIds() {
		if id == photoID {
			removed = true
			continue
		}
		kept = append(kept, id)
	}
	updated.PhotoIds = kept
	return updated, removed
}

func recipeHasPhoto(recipe *pb.PBRecipe, photoID string) bool {
	if recipe == nil || photoID == "" {
		return false
	}
	for _, id := range recipe.GetPhotoIds() {
		if id == photoID {
			return true
		}
	}
	return false
}
