package cli

import "testing"

// TestReadCommandResources_HomesNotRegistered locks the contract that the
// `homes` command is NOT registered for the generic auto-refresh path.
//
// Background: when `homes` was registered here, `ensureFreshForCommand`
// dispatched to `syncResource(c, db, "homes", ...)`, which builds params
// containing only pageSize/cursor/since — no `al`, no `region_id`, no
// `region_type`, none of the Stingray gis arguments. Every call to
// `redfin-pp-cli homes` therefore produced a noisy `HTTP 400: Request was
// missing a required argument named 'al'` from the auto-refresh hook,
// followed by a misleading `warning: using stale redfin-pp-cli cache`
// line on stderr — even on success.
//
// `homes` is a per-call search endpoint, not a list-all resource that
// can be backfilled into a local table. It must stay off this map.
func TestReadCommandResources_HomesNotRegistered(t *testing.T) {
	forbidden := []string{
		"redfin-pp-cli homes",
		"redfin-pp-cli homes list",
		"redfin-pp-cli homes get",
		"redfin-pp-cli homes search",
	}
	for _, path := range forbidden {
		if _, ok := readCommandResources[path]; ok {
			t.Errorf("readCommandResources[%q] is registered, but `homes` is a per-call search "+
				"endpoint and cannot be sync'd via the generic syncResource path "+
				"(the generated sync params lack al/region_id/region_type, which "+
				"Stingray rejects with HTTP 400). Remove the entry.", path)
		}
	}
}
