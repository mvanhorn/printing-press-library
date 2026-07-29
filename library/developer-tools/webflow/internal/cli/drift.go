// Copyright 2026 Kerry Morrison and contributors. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source auto

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type driftItem struct {
	ItemID        string `json:"itemId"`
	Name          string `json:"name"`
	Slug          string `json:"slug,omitempty"`
	CollectionID  string `json:"collectionId"`
	State         string `json:"state"`
	LastUpdated   string `json:"lastUpdated,omitempty"`
	LastPublished string `json:"lastPublished,omitempty"`
	Detail        string `json:"detail"`
}

type driftView struct {
	CollectionID   string      `json:"collectionId,omitempty"`
	ItemsScanned   int         `json:"itemsScanned"`
	ItemsDrifted   int         `json:"itemsDrifted"`
	NeverPublished int         `json:"neverPublished"`
	EditedSince    int         `json:"editedSincePublish"`
	Drafts         int         `json:"drafts"`
	Archived       int         `json:"archived"`
	Source         string      `json:"source,omitempty"`
	Items          []driftItem `json:"items"`
	Note           string      `json:"note,omitempty"`
}

func newNovelDriftCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	var includeArchived bool
	var maxScanPages int

	cmd := &cobra.Command{
		Use:   "drift [collection-id]",
		Short: "Show which CMS items have staged edits that are not live yet",
		Long: strings.Trim(`
Use this command to compare staged and live CMS items field by field for one
collection.

Do NOT use this command for a whole-site summary of everything a publish would
change; use 'publish preview' instead.

An item has drifted when it has never been published, or when its lastUpdated
timestamp is newer than its lastPublished timestamp. Draft and archived items
are reported separately because they will not go live on the next publish.

Reads the local mirror when it holds this collection, otherwise fetches the
collection's items from the API.
`, "\n"),
		Example: strings.Trim(`
  webflow-pp-cli drift 580e63fc8c9a982ac9b8b745 --agent
  webflow-pp-cli drift 580e63fc8c9a982ac9b8b745 --json --select items.name,items.state
  webflow-pp-cli drift --limit 100
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			// A wrong id and an unreachable data source are indistinguishable
			// here: both yield zero rows. Returning non-zero for the second
			// would make an un-synced, un-credentialed run look like a crash.
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "would compare staged and live CMS items")
			}

			lq, cleanup, ok, err := openLocalMirror(cmd, flags, dbPath, "items")
			if err != nil {
				return err
			}
			defer cleanup()
			if !ok {
				return nil
			}
			collectionID := resolveCollectionID(args)
			var rows []rawRow
			source := "local-mirror"
			if lq.hasTable("items") {
				query := `SELECT "id", "collections_id", "data" FROM "items"`
				var qargs []any
				if collectionID != "" {
					query += ` WHERE "collections_id" = ?`
					qargs = append(qargs, collectionID)
				}
				if rows, err = lq.selectRaw(query, qargs...); err != nil {
					return err
				}
			}
			// Generated sync cannot populate `items` without
			// WEBFLOW_COLLECTION_ID and covers one collection per run, so the
			// mirror is usually empty. Fetch the named collection instead.
			if len(rows) == 0 && flags.dataSource != "local" {
				if collectionID == "" {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("a collection id is required when the local mirror has no items"))
				}
				c, cerr := flags.newClient()
				if cerr != nil {
					return cerr
				}
				if rows, err = lq.fetchPaged(c, "/collections/"+escapeSeg(collectionID)+"/items",
					"items", collectionID, maxScanPages); err != nil {
					if note, degrade := liveFallbackErr(err, "this collection's items"); degrade {
						return emitEmptyLocal(cmd, flags, dbPath, "items", driftView{
							CollectionID: collectionID, Items: make([]driftItem, 0),
							Source: "unavailable", Note: note,
						})
					}
					return fmt.Errorf("fetching collection items: %w", err)
				}
				source = "api"
			} else if lq.hasTable("items") {
				if !hintIfUnsynced(cmd, lq.db, "items") {
					hintIfStale(cmd, lq.db, "items", flags.maxAge)
				}
			}

			view := computeDrift(rows, collectionID, limit, includeArchived)
			view.Source = source
			if view.ItemsScanned == 0 {
				view.Note = "no items available for this collection: it may be empty, the id may be wrong, or no credential was usable"
			}

			return emitLocalResult(cmd, flags, view, func() {
				fmt.Fprintf(cmd.OutOrStdout(),
					"%d of %d items have unpublished changes (%d never published, %d edited since publish); %d of %d items are drafts\n",
					view.ItemsDrifted, view.ItemsScanned, view.NeverPublished, view.EditedSince, view.Drafts, view.ItemsScanned)
				if len(view.Items) == 0 {
					return
				}
				fmt.Fprintln(cmd.OutOrStdout())
				for _, it := range view.Items {
					fmt.Fprintf(cmd.OutOrStdout(), "%-22s %-36s %s\n", it.State, it.Name, it.Detail)
				}
			})
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Local mirror path (defaults to the standard data directory)")
	cmd.Flags().IntVar(&limit, "limit", 200, "Maximum drifted items to list (counts still reflect every item scanned)")
	cmd.Flags().IntVar(&maxScanPages, "max-scan-pages", 10, "When fetching from the API rather than the mirror, maximum 100-item pages to scan")
	cmd.Flags().BoolVar(&includeArchived, "include-archived", false, "List archived items too (counted but not listed by default)")
	return cmd
}

// computeDrift is the whole comparison, split out so it is testable without a store.
func computeDrift(rows []rawRow, collectionID string, limit int, includeArchived bool) driftView {
	view := driftView{
		CollectionID: collectionID,
		Items:        make([]driftItem, 0, 32),
	}
	items := make([]driftItem, 0, len(rows))

	for _, r := range rows {
		decoded := decodeRows[wfItem]([]rawRow{r})
		if len(decoded) == 0 {
			continue
		}
		it := decoded[0]
		view.ItemsScanned++

		coll := r.Scope
		if coll == "" {
			coll = collectionID
		}

		if it.archived() {
			view.Archived++
			if !includeArchived {
				continue
			}
		}
		if it.draft() {
			view.Drafts++
		}
		if !hasUnpublishedChanges(it) {
			continue
		}
		view.ItemsDrifted++

		state := "edited-since-publish"
		detail := fmt.Sprintf("updated %s, last published %s", it.LastUpdated, it.LastPublished)
		if _, ok := parseWFTime(it.LastPublished); !ok {
			state = "never-published"
			detail = "has never been published"
			view.NeverPublished++
		} else {
			view.EditedSince++
		}
		if it.draft() {
			state = "draft-" + state
			detail += "; item is a draft and will not go live on publish"
		}

		items = append(items, driftItem{
			ItemID:        it.ID,
			Name:          it.name(),
			Slug:          it.slug(),
			CollectionID:  coll,
			State:         state,
			LastUpdated:   it.LastUpdated,
			LastPublished: it.LastPublished,
			Detail:        detail,
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].State != items[j].State {
			return items[i].State < items[j].State
		}
		return items[i].Name < items[j].Name
	})
	if limit > 0 && len(items) > limit {
		view.Items = items[:limit]
		view.Note = fmt.Sprintf("listing the first %d of %d drifted items; raise --limit to see more", limit, len(items))
	} else {
		view.Items = items
	}
	return view
}
