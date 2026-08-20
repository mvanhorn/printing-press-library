// PATCH (fix-sync-views-requires-scope-334): regression guard so a future
// regeneration or hand-edit cannot quietly re-add `views` to the default sync
// resource list. /v1/views requires a database_id or data_source_id scope, so
// including it in an unscoped sync errors every time. See
// .printing-press-patches.json entry fix-sync-views-requires-scope-334.

package cli

import "testing"

func TestDefaultSyncResourcesExcludesViews(t *testing.T) {
	for _, r := range defaultSyncResources() {
		if r == "views" {
			t.Fatalf("defaultSyncResources() must not include %q: "+
				"the Notion /v1/views endpoint rejects every unscoped call "+
				"with HTTP 400. See .printing-press-patches.json entry "+
				"fix-sync-views-requires-scope-334.", r)
		}
	}
}
