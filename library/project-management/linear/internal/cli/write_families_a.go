package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

// Shared plumbing for the tier-2 write families (labels, workflow states,
// cycles, comment lifecycle, issue lifecycle). Everything here is mutation-side
// only: read paths keep using the existing provenance helpers.

// resolveWriteTeamID maps a team key, name, or UUID to the team UUID a mutation
// input needs. A UUID passes straight through. Otherwise the synced local store
// answers without a round trip, and only if that misses does the live lookup
// run. c may be nil, in which case an unresolved key returns the input
// unchanged so a --dry-run preview can still be rendered offline. Callers that
// are about to mutate for real must pass a client.
func resolveWriteTeamID(c graphqlQueryer, dbPath, teamRef string) (string, error) {
	teamRef = strings.TrimSpace(teamRef)
	if teamRef == "" || store.IsUUID(teamRef) {
		return teamRef, nil
	}
	if db, err := openStoreAt(resolveDBPath(dbPath)); err == nil && db != nil {
		defer db.Close()
		if resolved, ok := resolveTeam(db, teamRef); ok && resolved.ID != "" {
			return resolved.ID, nil
		}
	}
	if c == nil {
		return teamRef, nil
	}
	return resolveTeamIDLive(c, teamRef)
}

// confirmMutation gates a destructive, non-reversible mutation. --yes (and
// therefore --agent) skips it. --no-input without --yes is refused as a usage
// error rather than blocking forever on a prompt no one can answer.
func confirmMutation(cmd *cobra.Command, flags *rootFlags, prompt string) error {
	if flags != nil && flags.yes {
		return nil
	}
	if flags != nil && flags.noInput {
		return usageErr(fmt.Errorf("%s needs explicit confirmation: pass --yes (or remove --no-input)", cmd.CommandPath()))
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "%s [y/N] ", prompt)
	var answer string
	fmt.Fscanln(cmd.InOrStdin(), &answer)
	if !strings.EqualFold(answer, "y") && !strings.EqualFold(answer, "yes") {
		return usageErr(fmt.Errorf("aborted"))
	}
	return nil
}

// renderMutationEvent prints the result of a mutation that has no object to
// render, which is every DeletePayload in the API. JSON mode gets the same
// {event, ...} envelope shape renderMutationDryRun emits, so a caller can pair
// would_delete_label with label_deleted without reshaping its parser.
func renderMutationEvent(cmd *cobra.Command, flags *rootFlags, event string, fields map[string]any) error {
	out := map[string]any{"event": event}
	for key, value := range fields {
		out[key] = value
	}
	if flags != nil && flags.asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s\n", event)
	for _, key := range []string{"id", "entity_id", "identifier", "name"} {
		if value, ok := fields[key]; ok {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s: %v\n", key, value)
		}
	}
	return nil
}

// extractDeletedEntityID pulls entityId out of a DeletePayload. Success=false
// is an API error, not a silent no-op.
func extractDeletedEntityID(resp json.RawMessage, mutationKey string) (string, error) {
	var root map[string]struct {
		Success  bool   `json:"success"`
		EntityID string `json:"entityId"`
	}
	if err := json.Unmarshal(resp, &root); err != nil {
		return "", fmt.Errorf("parsing %s response: %w", mutationKey, err)
	}
	payload, ok := root[mutationKey]
	if !ok {
		return "", fmt.Errorf("%s response missing %q", mutationKey, mutationKey)
	}
	if !payload.Success {
		return "", apiErr(fmt.Errorf("Linear reported %s success=false", mutationKey))
	}
	return payload.EntityID, nil
}

// isGraphQLNotFound reports whether err is Linear telling us the target does
// not exist. GraphQL answers 200 with an errors array, so classifyDeleteError's
// HTTP 404 check never fires on this transport and --ignore-missing would be
// dead on every mutation in this package without this test.
//
// Matching is on the API's own error prose, which is workspace-independent.
func isGraphQLNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if !strings.HasPrefix(msg, "graphql:") {
		return false
	}
	return strings.Contains(msg, "not found") ||
		strings.Contains(msg, "could not find") ||
		strings.Contains(msg, "entity not found")
}

// classifyGraphQLMutationError is classifyMutationError plus --ignore-missing
// handling for the GraphQL transport. Delete-shaped commands route through it
// so a repeat delete is an exit-0 no-op when the caller asked for one.
func classifyGraphQLMutationError(operation string, err error, flags *rootFlags) error {
	if flags != nil && flags.ignoreMissing && isGraphQLNotFound(err) {
		return writeNoop(flags, "already_deleted", "already deleted (no-op)")
	}
	if isGraphQLNotFound(err) {
		return notFoundErr(err)
	}
	return classifyMutationError(operation, err, flags, nil)
}

// isGraphQLAlreadyExists reports whether err is Linear rejecting a create
// because an equivalent record is already there. Used only to honour
// --idempotent. An unrecognised message falls through to normal error
// handling, so a real failure is never swallowed.
func isGraphQLAlreadyExists(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if !strings.HasPrefix(msg, "graphql:") {
		return false
	}
	return strings.Contains(msg, "already exists") ||
		strings.Contains(msg, "already been taken") ||
		strings.Contains(msg, "duplicate")
}

// classifyGraphQLCreateError is classifyMutationError plus --idempotent
// handling: a create that collides with an existing record exits 0 as a no-op
// when the caller asked for that, matching the documented flag contract.
func classifyGraphQLCreateError(operation string, err error, flags *rootFlags) error {
	if flags != nil && flags.idempotent && isGraphQLAlreadyExists(err) {
		return writeNoop(flags, "already_exists", "already exists (no-op)")
	}
	return classifyMutationError(operation, err, flags, nil)
}

// setOptionalString adds key to input only when value is non-empty, so a
// partial update never sends a field the caller did not ask to change.
// Linear treats an explicit null as "clear this field", which is not what an
// unset flag means.
func setOptionalString(input map[string]any, key, value string) {
	if strings.TrimSpace(value) != "" {
		input[key] = value
	}
}

// setChangedString is setOptionalString for flags where the empty string is a
// legitimate value the caller may want to write (clearing a description, for
// example). It keys off cobra's Changed bit rather than emptiness.
func setChangedString(cmd *cobra.Command, input map[string]any, flagName, key, value string) {
	if cmd.Flags().Changed(flagName) {
		input[key] = value
	}
}
