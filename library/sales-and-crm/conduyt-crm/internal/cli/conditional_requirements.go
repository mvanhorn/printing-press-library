package cli

import (
	"fmt"
	"strings"
)

// PATCH(reprint-637-conditional-requirements): pre-request checks the fresh spec cannot express (it types the
// inputs as optional). Kept in a hand-written file so a reprint only has to restore the one-line calls.
// Under the global --dry-run a MISSING input is not an error (the preview prints, as for every generated
// required flag); a value that is passed and wrong still is.

// checkDeleteSessionsTarget: DELETE /auth/sessions revokes one session (--id) or every other session
// (--scope others); exactly one of the two.
func checkDeleteSessionsTarget(idSet bool, id string, scopeSet bool, scope string, dryRun bool) error {
	switch {
	case idSet && scopeSet:
		return usageErr(fmt.Errorf("--id and --scope cannot be combined: pass --id <session> or --scope others"))
	case scopeSet:
		if scope != "others" {
			return usageErr(fmt.Errorf("--scope must be \"others\" (got %q)", scope))
		}
	case idSet:
		if id == "" {
			return usageErr(fmt.Errorf("--id must not be empty"))
		}
	default:
		if !dryRun {
			return usageErr(fmt.Errorf("pass --id <session> or --scope others"))
		}
	}
	return nil
}

// checkNotificationPreferencesBody: PATCH /users/{id}/notifications needs preferences, sounds, or both.
func checkNotificationPreferencesBody(preferencesSet bool, preferences string, soundsSet bool, dryRun bool) error {
	if preferencesSet && strings.TrimSpace(preferences) == "" {
		// the serializer drops an empty value, which would send {} (r2 P3)
		return usageErr(fmt.Errorf("--preferences must not be empty: pass a JSON object such as {\"deal.won\":{\"email\":false}}"))
	}
	if !preferencesSet && !soundsSet && !dryRun {
		return usageErr(fmt.Errorf("send --preferences, --sounds-team-chat, or both (or the whole body with --stdin)"))
	}
	return nil
}
