// Copyright 2026 Derick Ng and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: generic JSON:API write commands (create/update/delete).
//
// The generator emits GET/POST/PUT/DELETE endpoint commands but not PATCH, and
// its flat body-flags don't build the JSON:API `{data:{type,attributes,
// relationships}}` envelope Productive requires. These three commands provide a
// uniform write surface across every resource type using the client's
// Post/Patch/Delete methods with the correct vnd.api+json content type.

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/productive/internal/cliutil"
)

// dogfoodGuard prevents live mutation during `dogfood --live` (PRINTING_PRESS_DOGFOOD=1).
// The read-side matrix is safe to hit live; create/update/delete must not touch the
// operator's real org, so they report intent and return without calling the API.
func dogfoodGuard(cmd *cobra.Command, action, resType, id string) bool {
	if !cliutil.IsDogfoodEnv() {
		return false
	}
	if id != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "would %s %s/%s (dogfood: no live mutation)\n", action, resType, id)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "would %s %s (dogfood: no live mutation)\n", action, resType)
	}
	return true
}

// jsonAPIContentType is required on Productive write requests.
const jsonAPIContentType = "application/vnd.api+json"

func jsonAPIWriteHeaders() map[string]string {
	return map[string]string{
		"Content-Type": jsonAPIContentType,
		"Accept":       jsonAPIContentType,
	}
}

// buildJSONAPIBody assembles a JSON:API request document from --set/--set-json/
// --rel flags, or from a raw --data payload (file path, "-" for stdin, or inline
// JSON). A raw payload already containing a top-level "data" key is sent as-is;
// otherwise it is treated as the attributes object and wrapped.
func buildJSONAPIBody(resType, id string, sets, setsJSON, rels []string, dataArg string) (map[string]any, error) {
	if dataArg != "" {
		raw, err := readDataArg(dataArg)
		if err != nil {
			return nil, err
		}
		var parsed map[string]json.RawMessage
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return nil, fmt.Errorf("parsing --data JSON: %w", err)
		}
		if _, ok := parsed["data"]; ok {
			var doc map[string]any
			if err := json.Unmarshal(raw, &doc); err != nil {
				return nil, fmt.Errorf("parsing --data JSON document: %w", err)
			}
			return doc, nil
		}
		// Treat the payload as the attributes object.
		attrs := map[string]any{}
		if err := json.Unmarshal(raw, &attrs); err != nil {
			return nil, fmt.Errorf("parsing --data attributes: %w", err)
		}
		return wrapJSONAPI(resType, id, attrs, nil), nil
	}

	attrs := map[string]any{}
	for _, kv := range sets {
		k, v, ok := splitKV(kv)
		if !ok {
			return nil, fmt.Errorf("--set expects key=value, got %q", kv)
		}
		attrs[k] = v
	}
	for _, kv := range setsJSON {
		k, v, ok := splitKV(kv)
		if !ok {
			return nil, fmt.Errorf("--set-json expects key=json, got %q", kv)
		}
		var val any
		if err := json.Unmarshal([]byte(v), &val); err != nil {
			return nil, fmt.Errorf("--set-json %q value is not valid JSON: %w", k, err)
		}
		attrs[k] = val
	}
	relationships := map[string]any{}
	for _, r := range rels {
		name, rest, ok := splitKV(r)
		if !ok {
			return nil, fmt.Errorf("--rel expects name=type:id, got %q", r)
		}
		rtype, rid, ok := strings.Cut(rest, ":")
		if !ok || rtype == "" || rid == "" {
			return nil, fmt.Errorf("--rel %q value must be type:id", name)
		}
		relationships[name] = map[string]any{"data": map[string]any{"type": rtype, "id": rid}}
	}
	if len(attrs) == 0 && len(relationships) == 0 {
		return nil, fmt.Errorf("provide at least one --set/--set-json/--rel, or --data")
	}
	return wrapJSONAPI(resType, id, attrs, relationships), nil
}

func wrapJSONAPI(resType, id string, attrs, rels map[string]any) map[string]any {
	data := map[string]any{"type": resType}
	if id != "" {
		data["id"] = id
	}
	if len(attrs) > 0 {
		data["attributes"] = attrs
	}
	if len(rels) > 0 {
		data["relationships"] = rels
	}
	return map[string]any{"data": data}
}

func splitKV(s string) (string, string, bool) {
	k, v, ok := strings.Cut(s, "=")
	if !ok || k == "" {
		return "", "", false
	}
	return k, v, true
}

func readDataArg(arg string) ([]byte, error) {
	switch {
	case arg == "-":
		return io.ReadAll(os.Stdin)
	case strings.HasPrefix(arg, "@"):
		return os.ReadFile(arg[1:])
	default:
		return []byte(arg), nil
	}
}

func newCreateCmd(flags *rootFlags) *cobra.Command {
	var sets, setsJSON, rels []string
	var dataArg string
	cmd := &cobra.Command{
		Use:   "create <type>",
		Short: "Create a resource (JSON:API POST) — e.g. create tasks --set title=Hi --rel project=projects:5",
		Long: "Create any Productive resource by its JSON:API type. Build the body with repeatable " +
			"--set key=value (string attributes), --set-json key=<json> (typed attributes), and " +
			"--rel name=type:id (relationships); or pass a full payload with --data @file.json / --data - / --data '<json>'. " +
			"Sends application/vnd.api+json. Use --dry-run to preview the request without sending.",
		Example: "  productive-pp-cli create tasks --set title='Draft brief' --rel project=projects:12345 --rel task_list=task_lists:6789",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) && len(args) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "would POST a JSON:API resource")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("resource <type> is required (e.g. tasks, time_entries, deals)"))
			}
			resType := args[0]
			if dogfoodGuard(cmd, "create", resType, "") {
				return nil
			}
			body, err := buildJSONAPIBody(resType, "", sets, setsJSON, rels, dataArg)
			if err != nil {
				if dryRunOK(flags) {
					fmt.Fprintf(cmd.OutOrStdout(), "would POST /%s (application/vnd.api+json) — add --set/--set-json/--rel/--data for the body\n", resType)
					return nil
				}
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, _, err := c.PostWithHeaders(ctx, "/"+resType, body, jsonAPIWriteHeaders())
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlagsMeta(cmd.OutOrStdout(), data, flags, map[string]any{"source": "live", "action": "create", "type": resType})
		},
	}
	addWriteFlags(cmd, &sets, &setsJSON, &rels, &dataArg)
	return cmd
}

func newUpdateCmd(flags *rootFlags) *cobra.Command {
	var sets, setsJSON, rels []string
	var dataArg string
	cmd := &cobra.Command{
		Use:   "update <type> <id>",
		Short: "Update a resource (JSON:API PATCH) — e.g. update tasks 123 --set title=Renamed",
		Long: "Update any Productive resource by JSON:API type and id via PATCH. Same body flags as " +
			"create (--set / --set-json / --rel / --data). The resource id is injected into the JSON:API " +
			"document automatically. Sends application/vnd.api+json. Use --dry-run to preview.",
		Example: "  productive-pp-cli update deals 12345 --set probability=75",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) && len(args) < 2 {
				fmt.Fprintln(cmd.OutOrStdout(), "would PATCH a JSON:API resource")
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<type> and <id> are both required"))
			}
			resType, id := args[0], args[1]
			if dogfoodGuard(cmd, "update", resType, id) {
				return nil
			}
			body, err := buildJSONAPIBody(resType, id, sets, setsJSON, rels, dataArg)
			if err != nil {
				if dryRunOK(flags) {
					fmt.Fprintf(cmd.OutOrStdout(), "would PATCH /%s/%s (application/vnd.api+json) — add --set/--set-json/--rel/--data for the body\n", resType, id)
					return nil
				}
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, _, err := c.PatchWithHeaders(ctx, "/"+resType+"/"+id, body, jsonAPIWriteHeaders())
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlagsMeta(cmd.OutOrStdout(), data, flags, map[string]any{"source": "live", "action": "update", "type": resType, "id": id})
		},
	}
	addWriteFlags(cmd, &sets, &setsJSON, &rels, &dataArg)
	return cmd
}

func newDeleteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete <type> <id>",
		Short:   "Delete a resource (JSON:API DELETE) — e.g. delete tasks 123",
		Long:    "Delete any Productive resource by JSON:API type and id. Prompts for confirmation before deleting; pass --yes (or --agent) to skip the prompt in scripts, or --dry-run to preview the request without sending.",
		Example: "  productive-pp-cli delete time_entries 998877",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) && len(args) < 2 {
				fmt.Fprintln(cmd.OutOrStdout(), "would DELETE a JSON:API resource")
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<type> and <id> are both required"))
			}
			resType, id := args[0], args[1]
			if dogfoodGuard(cmd, "delete", resType, id) {
				return nil
			}
			// Destructive: confirm before deleting. --yes (and --agent, which
			// implies it) skips the prompt; --no-input without --yes refuses
			// rather than silently deleting in a non-interactive context. A
			// --dry-run never mutates, so it never prompts.
			if !flags.yes && !dryRunOK(flags) {
				if flags.noInput {
					return usageErr(fmt.Errorf("refusing to delete %s/%s without confirmation; pass --yes to proceed", resType, id))
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "Delete %s %s? This cannot be undone. [y/N]: ", resType, id)
				line, _ := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
				if ans := strings.ToLower(strings.TrimSpace(line)); ans != "y" && ans != "yes" {
					fmt.Fprintln(cmd.OutOrStdout(), "aborted")
					return nil
				}
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, _, err := c.DeleteWithHeaders(ctx, "/"+resType+"/"+id, jsonAPIWriteHeaders())
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if len(data) == 0 {
				data = json.RawMessage(fmt.Sprintf(`{"deleted":true,"type":%q,"id":%q}`, resType, id))
			}
			return printOutputWithFlagsMeta(cmd.OutOrStdout(), data, flags, map[string]any{"source": "live", "action": "delete", "type": resType, "id": id})
		},
	}
	return cmd
}

func addWriteFlags(cmd *cobra.Command, sets, setsJSON, rels *[]string, dataArg *string) {
	cmd.Flags().StringArrayVar(sets, "set", nil, "String attribute key=value (repeatable)")
	cmd.Flags().StringArrayVar(setsJSON, "set-json", nil, "Typed attribute key=<json> for numbers/bools/objects (repeatable)")
	cmd.Flags().StringArrayVar(rels, "rel", nil, "Relationship name=type:id, e.g. project=projects:5 (repeatable)")
	cmd.Flags().StringVar(dataArg, "data", "", "Full JSON:API document or attributes object: @file.json, - for stdin, or inline JSON")
}
