// steam_demo_annotate.go — opt-in Steam demo annotation for bulk commands.
//
// discover and similar keep no Steam store link, so annotating a result needs
// one RAWG /games/{id}/stores lookup to find its Steam appid, then one batched
// Steam GetItems call (200 ids per request) for the demo links. That is why the
// caller opts in via --with-demos.
package cli

import (
	"context"
	"errors"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/game-goat/internal/client"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/game-goat/internal/source/steam"
)

// withDemosHelp is the shared --with-demos help text on discover and similar.
const withDemosHelp = "Annotate each result with has_demo and demo_app_ids from Steam (opt-in: costs 1 extra RAWG request per result for its Steam store link, plus 1 Steam request per 200 games)"

// steamDemoAnnotation is the Steam demo data annotateSteamDemos resolves for
// one RAWG game.
type steamDemoAnnotation struct {
	SteamAppID int64
	HasDemo    bool
	DemoAppIDs []int64
}

// annotateSteamDemos resolves the Steam appid for each RAWG id and returns the
// demo annotation for every id whose Steam record was found, the count of RAWG
// /stores lookups that FAILED, and a transport error.
//
// A game whose store list is fine but contains no Steam link comes back as
// steam.ErrAppNotFound — that is a legitimate "no link" and is not counted as
// a failure; only real transport/parse failures are. The batching is
// deliberate: one /stores request per RAWG id, then a single Steam DemoLinks
// call that chunks itself at 200 appids.
func annotateSteamDemos(ctx context.Context, c *client.Client, rawgIDs []int) (map[int]steamDemoAnnotation, int, error) {
	out := make(map[int]steamDemoAnnotation)
	if c == nil || len(rawgIDs) == 0 {
		return out, 0, nil
	}
	type target struct {
		rawgID int
		appid  int64
	}
	targets := make([]target, 0, len(rawgIDs))
	appids := make([]int64, 0, len(rawgIDs))
	failed := 0
	for _, rawgID := range rawgIDs {
		if rawgID <= 0 {
			continue
		}
		appid, err := steamAppIDForRAWGGame(ctx, c, rawgID)
		if err != nil {
			if !errors.Is(err, steam.ErrAppNotFound) {
				failed++
			}
			continue
		}
		if appid <= 0 {
			continue
		}
		targets = append(targets, target{rawgID: rawgID, appid: appid})
		appids = append(appids, appid)
	}
	if len(appids) == 0 {
		return out, failed, nil
	}
	links, err := newSteamClient("", "").DemoLinks(ctx, appids)
	if err != nil {
		return nil, failed, err
	}
	for _, tgt := range targets {
		demos, ok := links[tgt.appid]
		if !ok {
			continue
		}
		if demos == nil {
			demos = []int64{}
		}
		out[tgt.rawgID] = steamDemoAnnotation{SteamAppID: tgt.appid, HasDemo: len(demos) > 0, DemoAppIDs: demos}
	}
	return out, failed, nil
}

// annotateRowsWithSteamDemos fills the Steam demo fields on discover rows in
// place. Individual misses are left absent, matching the JSON omitempty shape.
// It returns the number of failed RAWG /stores lookups so the caller can warn.
func annotateRowsWithSteamDemos(cmd *cobra.Command, c *client.Client, rows []gameRow) (int, error) {
	ids := make([]int, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	annotations, failed, err := annotateSteamDemos(cmd.Context(), c, ids)
	if err != nil {
		return failed, err
	}
	for i := range rows {
		ann, ok := annotations[rows[i].ID]
		if !ok {
			continue
		}
		has := ann.HasDemo
		rows[i].SteamAppID = ann.SteamAppID
		rows[i].HasDemo = &has
		rows[i].DemoAppIDs = ann.DemoAppIDs
	}
	return failed, nil
}

// annotateSimilarSteamDemos is the similar-row counterpart of
// annotateRowsWithSteamDemos.
func annotateSimilarSteamDemos(cmd *cobra.Command, c *client.Client, results []similarResult) (int, error) {
	ids := make([]int, 0, len(results))
	for _, r := range results {
		ids = append(ids, r.ID)
	}
	annotations, failed, err := annotateSteamDemos(cmd.Context(), c, ids)
	if err != nil {
		return failed, err
	}
	for i := range results {
		ann, ok := annotations[results[i].ID]
		if !ok {
			continue
		}
		has := ann.HasDemo
		results[i].SteamAppID = ann.SteamAppID
		results[i].HasDemo = &has
		results[i].DemoAppIDs = ann.DemoAppIDs
	}
	return failed, nil
}

// steamDemoCell renders a known demo state for a human table; unknown is "-".
func steamDemoCell(hasDemo *bool) string {
	if hasDemo == nil {
		return "-"
	}
	if *hasDemo {
		return "yes"
	}
	return "no"
}
