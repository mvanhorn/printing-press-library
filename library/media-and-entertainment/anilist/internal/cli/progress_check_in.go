// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/anilist/internal/client"
	"github.com/spf13/cobra"
	"strconv"
	"strings"
)

func newNovelProgressCheckInCmd(flags *rootFlags) *cobra.Command {
	var episode int
	var apply bool
	cmd := &cobra.Command{Use: "check-in <media-id-or-exact-title>", Args: cobra.ExactArgs(1), Short: "Preview the next watched-episode update before explicitly applying it to AniList.", Example: "Cowboy Bebop --episode 3 --apply --agent", Annotations: map[string]string{"mcp:read-only": "false"}, RunE: func(cmd *cobra.Command, args []string) error {
		if episode <= 0 {
			return usageErr(fmt.Errorf("--episode must be positive"))
		}
		if flags.dryRun {
			return flags.printJSON(cmd, map[string]any{"media": args[0], "episode": episode, "apply": apply, "dry_run": true})
		}
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		u, err := viewerID(cmd.Context(), c)
		if err != nil {
			return err
		}
		mediaID, err := resolvePersonalMedia(cmd, c, args[0])
		if err != nil {
			return err
		}
		entry, err := getPersonalEntry(cmd, c, u, mediaID)
		if err != nil {
			return err
		}
		if entry == nil {
			return notFoundErr(fmt.Errorf("media %d has no existing AniList list entry", mediaID))
		}
		if episode < entry.Progress {
			return usageErr(fmt.Errorf("refusing progress regression from %d to %d", entry.Progress, episode))
		}
		if entry.Media.Episodes > 0 && episode > entry.Media.Episodes {
			return usageErr(fmt.Errorf("episode %d exceeds known total %d", episode, entry.Media.Episodes))
		}
		preview := map[string]any{"media_id": mediaID, "title": entry.Media.Title.UserPreferred, "before": entry.Progress, "after": episode, "apply": apply}
		if !apply {
			return flags.printJSON(cmd, preview)
		}
		fresh, err := getPersonalEntry(cmd, c, u, mediaID)
		if err != nil {
			return err
		}
		if fresh == nil || fresh.Progress != entry.Progress {
			return apiErr(fmt.Errorf("progress drift detected; rerun after reviewing current state"))
		}
		const mutation = `mutation($media:Int!,$progress:Int!){SaveMediaListEntry(mediaId:$media,progress:$progress){id progress media{id title{userPreferred} episodes}}}`
		var r struct {
			SaveMediaListEntry personalEntry `json:"SaveMediaListEntry"`
		}
		if err := anilistGraphQL(cmd.Context(), c, mutation, map[string]any{"media": mediaID, "progress": episode}, &r); err != nil {
			return err
		}
		if r.SaveMediaListEntry.Progress != episode {
			return apiErr(fmt.Errorf("AniList returned progress %d, expected %d", r.SaveMediaListEntry.Progress, episode))
		}
		preview["verified"] = true
		return flags.printJSON(cmd, preview)
	}}
	cmd.Flags().IntVar(&episode, "episode", 0, "Target watched episode number")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply the previewed mutation; otherwise only preview")
	return cmd
}
func resolvePersonalMedia(cmd *cobra.Command, c *client.Client, raw string) (int, error) {
	if id, err := strconv.Atoi(raw); err == nil && id > 0 {
		return id, nil
	}
	const q = `query($search:String!){Page(page:1,perPage:2){media(search:$search,type:ANIME){id}}}`
	var r struct {
		Page struct {
			Media []struct {
				ID int `json:"id"`
			} `json:"media"`
		} `json:"Page"`
	}
	if err := anilistGraphQL(cmd.Context(), c, q, map[string]any{"search": strings.TrimSpace(raw)}, &r); err != nil {
		return 0, err
	}
	if len(r.Page.Media) != 1 {
		return 0, usageErr(fmt.Errorf("title %q resolves ambiguously; use an AniList media ID", raw))
	}
	return r.Page.Media[0].ID, nil
}
func getPersonalEntry(cmd *cobra.Command, c *client.Client, user, id int) (*personalEntry, error) {
	const q = `query($user:Int!,$media:Int!){MediaList(userId:$user,mediaId:$media){id progress priority score status media{id title{userPreferred} episodes duration status}}}`
	var r struct {
		MediaList *personalEntry `json:"MediaList"`
	}
	if err := anilistGraphQL(cmd.Context(), c, q, map[string]any{"user": user, "media": id}, &r); err != nil {
		return nil, err
	}
	return r.MediaList, nil
}
