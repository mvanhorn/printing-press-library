// Copyright 2026 github-actionsbot and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: see .printing-press-patches/2026-08-30-nextauth-cookie-surface.md.

package cli

import (
	"encoding/json"
	"testing"
)

// TestProjectsResponsePathExtractsRealShape locks in the response-path fix:
// the real project.searchUserProjects payload (confirmed live via curl
// against a real account) nests the projects array one level deeper than
// flow.projectInitialData does -- result.data.json.result.projects, not
// result.data.json.projects. Regression-guards resource_paths.go,
// sync.go's syncResourcePath/responsePathForResource, and
// promoted_projects.go all staying in sync on this path.
func TestProjectsResponsePathExtractsRealShape(t *testing.T) {
	t.Parallel()

	data := json.RawMessage(`{
		"result": {
			"data": {
				"json": {
					"result": {
						"projects": [
							{"projectId": "p1", "projectInfo": {"projectTitle": "Aug 07, 08:50 PM"}},
							{"projectId": "p2", "projectInfo": {"projectTitle": "Nov 29 - 22:00"}}
						]
					}
				}
			}
		}
	}`)

	path, err := resourceReadPath("projects")
	if err != nil {
		t.Fatalf("resourceReadPath: %v", err)
	}
	if path != "https://labs.google/fx/api/trpc/project.searchUserProjects" {
		t.Fatalf("resourceReadPath(\"projects\") = %q, want the absolute labs.google URL -- a relative path silently resolves against the wrong BaseURL (aisandbox-pa.googleapis.com)", path)
	}

	cfg, ok := resourceReadConfigs["projects"]
	if !ok {
		t.Fatalf("resourceReadConfigs missing \"projects\"")
	}
	if cfg.responsePath != "result.data.json.result.projects" {
		t.Fatalf("resourceReadConfigs[\"projects\"].responsePath = %q, want result.data.json.result.projects", cfg.responsePath)
	}

	items, _, _ := extractPageItemsWithPagination(data, cfg.cursorParam, cfg.nextCursorPath, responsePathForResource("projects", path)...)
	if len(items) != 2 {
		t.Fatalf("expected 2 projects extracted from the real response shape, got %d: %v", len(items), items)
	}
}

// TestProjectsPaginationContinuationIsANoOpByDesign documents a subtler
// consequence of the same tRPC input-envelope gap already flagged in
// .printing-press-patches/2026-08-30-nextauth-cookie-surface.md for
// `sync --resources projects` / `export projects`: project.searchUserProjects
// takes its cursor embedded inside a JSON-encoded `input=` query param, which
// the generic sync/pagination engine cannot construct. resourceReadConfigs
// and promoted_projects.go's resolvePaginatedReadWithStrategy call both set
// cursorParam to "" for this resource, and extractPaginationFromEnvelope
// short-circuits to ("", false) whenever cursorParam is empty -- so `--all`
// never attempts to detect or follow a second page. This was NOT verified
// against a real multi-page account (the test account only had 2 projects,
// never enough to need one) -- this test instead verifies the code-level
// fact directly: pagination continuation is a documented no-op for this
// resource, not a silent, unverified truncation risk.
func TestProjectsPaginationContinuationIsANoOpByDesign(t *testing.T) {
	t.Parallel()

	cfg, ok := resourceReadConfigs["projects"]
	if !ok {
		t.Fatalf("resourceReadConfigs missing \"projects\"")
	}
	if cfg.cursorParam != "" {
		t.Fatalf("expected cursorParam to remain empty for \"projects\" -- if this is no longer true, the pagination-is-a-no-op assumption in this test (and the patch doc) needs re-verifying against a real multi-page account before removing this guard")
	}

	envelope := map[string]json.RawMessage{
		"cursor":  json.RawMessage(`"some-real-cursor-value"`),
		"hasMore": json.RawMessage(`true`),
	}
	nextCursor, hasMore := extractPaginationFromEnvelope(envelope, cfg.cursorParam, cfg.nextCursorPath)
	if nextCursor != "" || hasMore {
		t.Fatalf("expected extractPaginationFromEnvelope to short-circuit to (\"\", false) with an empty cursorParam even when the envelope has real pagination fields, got (%q, %v)", nextCursor, hasMore)
	}
}
