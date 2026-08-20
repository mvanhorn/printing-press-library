// Hand-authored. Do not regenerate over this file with `printing-press generate`
// without merging — it owns ClickUp-specific sync hierarchy traversal.
//
// ClickUp's API requires a parent ID for almost every list operation
// (`/team/{team_id}/space`, `/space/{space_id}/folder`, etc.). The generator's
// default sync layer assumes global list endpoints, so without this file most
// resources fail with "unknown sync resource". This file:
//
//  1. Defines a syncHandler that returns N HTTP fetches per resource (one per
//     parent record already in the store).
//  2. Registers handlers for the full ClickUp hierarchy.
//  3. Orders default resources so parents land before children (team -> space
//     -> folder -> list -> task; team -> doc; team -> channel).
//  4. Exposes hasHierarchicalResource so sync.go can force concurrency=1 when
//     hierarchical resources are in play (children need their parents synced
//     first).

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/project-management/clickup/internal/store"
)

// fetchTarget is one HTTP call to make for a resource sync.
// ParentID is recorded so callers can attribute progress events to a specific
// parent record (e.g., "synced 87 tasks under list 901706411455").
type fetchTarget struct {
	Path     string
	ParentID string
}

// syncHandler returns the list of fetches required to sync a resource.
// db is provided so handlers for hierarchical resources can enumerate
// already-synced parent IDs.
type syncHandler func(db *store.Store) ([]fetchTarget, error)

// hierarchicalResources are resources whose handlers depend on parent records
// already being present in the store. Used to detect when sync must run
// sequentially in dependency order rather than via the parallel worker pool.
var hierarchicalResources = map[string]bool{
	"space":   true,
	"folder":  true,
	"list":    true,
	"task":    true,
	"doc":     true,
	"channel": true,
}

// syncHandlers maps each resource name to a function that returns the HTTP
// fetches required. Globals return one target; hierarchical resources fan out
// across parent records read from the store.
var syncHandlers = map[string]syncHandler{
	"team": func(_ *store.Store) ([]fetchTarget, error) {
		return []fetchTarget{{Path: "/v2/team"}}, nil
	},
	"user": func(_ *store.Store) ([]fetchTarget, error) {
		return []fetchTarget{{Path: "/v2/user"}}, nil
	},
	"space": func(db *store.Store) ([]fetchTarget, error) {
		return parentTargets(db, "team", "/v2/team/{id}/space")
	},
	"folder": func(db *store.Store) ([]fetchTarget, error) {
		return parentTargets(db, "space", "/v2/space/{id}/folder")
	},
	// list pulls from both folder-bound lists and folder-less (space-bound)
	// lists. Both shapes are stored under "list" so downstream consumers
	// (task handler, search) treat them uniformly. The list object itself
	// carries embedded `folder` and `space` fields, so the distinction is
	// preserved in the JSON.
	"list": func(db *store.Store) ([]fetchTarget, error) {
		return unionTargets(db,
			parentSource{parent: "folder", template: "/v2/folder/{id}/list"},
			parentSource{parent: "space", template: "/v2/space/{id}/list"},
		)
	},
	"task": func(db *store.Store) ([]fetchTarget, error) {
		return parentTargets(db, "list", "/v2/list/{id}/task")
	},
	"doc": func(db *store.Store) ([]fetchTarget, error) {
		return parentTargets(db, "team", "/v3/workspaces/{id}/docs")
	},
	"channel": func(db *store.Store) ([]fetchTarget, error) {
		return parentTargets(db, "team", "/v3/workspaces/{id}/chat/channels")
	},
}

// orderedDefaultResources is the ClickUp dependency order: each entry's parents
// come before it. Sync must respect this order when hierarchical resources
// are present.
var orderedDefaultResources = []string{
	"team",
	"user",
	"space",
	"folder",
	"list",
	"task",
	"doc",
	"channel",
}

// getSyncTargets returns the HTTP fetches required for a resource.
// Returns "unknown sync resource" for unregistered names so the failure
// surfaces in sync_error events rather than disappearing into the count.
//
// An empty target slice with a nil error is a legitimate result: the parent
// was synced but has zero children (e.g. a workspace with no folders). The
// caller iterates targets, so zero targets means the resource finishes
// cleanly with zero records.
func getSyncTargets(resource string, db *store.Store) ([]fetchTarget, error) {
	h, ok := syncHandlers[resource]
	if !ok {
		return nil, fmt.Errorf("unknown sync resource %q (known: %s)",
			resource, strings.Join(knownResourceNames(), ", "))
	}
	return h(db)
}

// hasHierarchicalResource reports whether any resource in the slice depends
// on parent enumeration. Used to force sequential dependency-ordered sync.
func hasHierarchicalResource(resources []string) bool {
	for _, r := range resources {
		if hierarchicalResources[r] {
			return true
		}
	}
	return false
}

// orderResourcesByDependency reorders the requested resources so parents
// come before children. Unknown resources keep their original relative
// position at the end.
func orderResourcesByDependency(requested []string) []string {
	want := make(map[string]bool, len(requested))
	for _, r := range requested {
		want[r] = true
	}
	out := make([]string, 0, len(requested))
	for _, r := range orderedDefaultResources {
		if want[r] {
			out = append(out, r)
			delete(want, r)
		}
	}
	// Anything not in the dependency table appended at end in caller-order
	for _, r := range requested {
		if want[r] {
			out = append(out, r)
			delete(want, r)
		}
	}
	return out
}

// parentSource describes one parent->child relationship for the union helper.
type parentSource struct {
	parent   string // store resource type to list (e.g. "space", "folder")
	template string // path template with `{id}` placeholder for the parent's ID
}

// parentTargets reads parent records from the store and templates a path.
// Distinguishes "parent never synced" (return error) from "parent synced and
// has zero records" (return empty slice — legitimate result, not an error).
func parentTargets(db *store.Store, parentResource, pathTemplate string) ([]fetchTarget, error) {
	parents, err := db.List(parentResource, 10000)
	if err != nil {
		return nil, fmt.Errorf("listing %s from store: %w", parentResource, err)
	}
	if len(parents) == 0 {
		// Distinguish "haven't run sync yet" from "synced and got zero".
		// sync_state has a row for any resource that has been attempted.
		_, lastSynced, _, _ := db.GetSyncState(parentResource)
		if lastSynced.IsZero() {
			return nil, fmt.Errorf("no %s records in store; sync %s first", parentResource, parentResource)
		}
		// Parent was synced and is genuinely empty. Return an empty slice;
		// callers should treat this as "0 children, success."
		return []fetchTarget{}, nil
	}
	targets := make([]fetchTarget, 0, len(parents))
	for _, raw := range parents {
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			continue
		}
		id := extractID(parentResource, obj)
		if id == "" {
			continue
		}
		targets = append(targets, fetchTarget{
			Path:     strings.ReplaceAll(pathTemplate, "{id}", id),
			ParentID: id,
		})
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("found %d %s records in store but none had extractable IDs", len(parents), parentResource)
	}
	return targets, nil
}

// unionTargets merges fetch targets from multiple parent sources. Used when a
// child resource lives under more than one parent type (e.g. ClickUp lists
// live under both folders and spaces).
func unionTargets(db *store.Store, sources ...parentSource) ([]fetchTarget, error) {
	var out []fetchTarget
	var firstErr error
	for _, src := range sources {
		targets, err := parentTargets(db, src.parent, src.template)
		if err != nil {
			// Record the first error but continue trying other sources;
			// "no folder records yet" should not block lists from spaces.
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		out = append(out, targets...)
	}
	if len(out) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return out, nil
}

func knownResourceNames() []string {
	names := make([]string, 0, len(syncHandlers))
	for n := range syncHandlers {
		names = append(names, n)
	}
	return names
}
