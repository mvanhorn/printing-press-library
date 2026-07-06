// Copyright 2026 Ryan Gravette and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command group: course authoring (read-modify-write on the Dawn concept).
//
// Dawn persists the entire course as one concept document. Every authoring
// operation is a read-modify-write: GET /concept/{id}, mutate the JSON, then
// PUT /api/concept with the full body (the server increments `version`).
//
// SAFETY: every write command previews by default and only mutates the live
// course when given --apply. Under the verify/dogfood harness they never write.

package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/productivity/agilix-dawn/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/agilix-dawn/internal/cliutil"
	"github.com/spf13/cobra"
)

// concept is fetched and PUT as a raw map so no field is lost on write.
func fetchConceptRaw(ctx context.Context, c *client.Client, id string) (map[string]any, error) {
	data, err := c.Get(ctx, "/concept/"+pathEscape(id), nil)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing concept %s: %w", id, err)
	}
	if _, ok := m["id"]; !ok {
		return nil, fmt.Errorf("no concept found for id %q", id)
	}
	return m, nil
}

func pathEscape(s string) string {
	// concept ids are c_<hex>; keep this local so authoring has no import cycle.
	out := make([]byte, 0, len(s))
	for _, r := range []byte(s) {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			out = append(out, r)
		} else {
			out = append(out, '%')
			out = append(out, "0123456789ABCDEF"[r>>4], "0123456789ABCDEF"[r&0xf])
		}
	}
	return string(out)
}

func genID(prefix string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

// arrOf normalizes a JSON value that should be a list. Dawn's modern /api
// returns arrays even for single elements, but a defensive normalization means
// a single-object serialization can never cause a read-modify-write to silently
// drop the lone existing child.
func arrOf(v any) []any {
	switch t := v.(type) {
	case []any:
		return t
	case map[string]any:
		return []any{t}
	default:
		return nil
	}
}

func sectionsOf(m map[string]any) []any {
	return arrOf(m["section"])
}

func asMap(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}

// previewOrApply prints the planned change and, only when apply is true and the
// harness is not in verify/dogfood mode, PUTs the full concept back.
func previewOrApply(cmd *cobra.Command, flags *rootFlags, apply bool, c *client.Client, obj map[string]any, summary string) error {
	return previewOrApplyPath(cmd, flags, apply, c, "/concept", obj, summary)
}

// previewBlocked reports the preview state and prints/returns the preview
// message when a write should not happen. ok is true when the caller should
// stop (preview only); it is false when the caller may proceed to write.
func previewBlocked(cmd *cobra.Command, flags *rootFlags, apply bool, summary string) (stop bool, err error) {
	blocked := cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv()
	if apply && !blocked {
		return false, nil
	}
	reason := "preview — re-run with --apply to write"
	if apply && blocked {
		reason = "preview — writes are disabled under the verify/dogfood harness"
	}
	if flags.asJSON {
		return true, flags.printJSON(cmd, map[string]any{"applied": false, "summary": summary, "reason": reason})
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s\n(%s)\n", summary, reason)
	return true, nil
}

// previewOrApplyPath is previewOrApply for an arbitrary document endpoint
// (e.g. /concept or /group) that persists via PUT of the full object.
func previewOrApplyPath(cmd *cobra.Command, flags *rootFlags, apply bool, c *client.Client, path string, obj map[string]any, summary string) error {
	if stop, err := previewBlocked(cmd, flags, apply, summary); stop {
		return err
	}
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	raw, status, err := c.Put(ctx, path, obj)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	if status < 200 || status >= 300 {
		if status == 409 || status == 412 {
			return fmt.Errorf("write failed: HTTP %d — the document changed since it was read (version conflict); re-run to fetch the latest and retry", status)
		}
		return fmt.Errorf("write failed: HTTP %d", status)
	}
	newVersion := extractVersion(raw)
	if flags.asJSON {
		return flags.printJSON(cmd, map[string]any{"applied": true, "summary": summary, "version": newVersion})
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s\napplied — new version: %v\n", summary, newVersion)
	return nil
}

// extractVersion reads `version` from a write response. Dawn returns the
// version at the top level for some resources (concept) and wrapped in a
// single-key envelope for others ({"group": {"version": ...}}).
func extractVersion(raw json.RawMessage) any {
	var resp map[string]any
	if json.Unmarshal(raw, &resp) != nil {
		return nil
	}
	if v, ok := resp["version"]; ok {
		return v
	}
	for _, val := range resp {
		if inner, ok := val.(map[string]any); ok {
			if v, ok := inner["version"]; ok {
				return v
			}
		}
	}
	return nil
}

// previewOrApplyDelete is the same preview/apply gate for a DELETE endpoint.
func previewOrApplyDelete(cmd *cobra.Command, flags *rootFlags, apply bool, c *client.Client, path, summary string) error {
	if stop, err := previewBlocked(cmd, flags, apply, summary); stop {
		return err
	}
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	_, status, err := c.Delete(ctx, path)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("delete failed: HTTP %d", status)
	}
	if flags.asJSON {
		return flags.printJSON(cmd, map[string]any{"applied": true, "summary": summary})
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s\napplied\n", summary)
	return nil
}

func newEditCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Author course content (metadata, sections, instructions, publish state)",
		Long: "Author course content on the Dawn API.\n\n" +
			"Every subcommand is a read-modify-write on the whole course document and PREVIEWS by\n" +
			"default; pass --apply to actually write. Creating brand-new courses is not supported\n" +
			"(the API returns 403 for POST /concept unless you have publisher permissions).",
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newEditConceptCmd(flags))
	cmd.AddCommand(newEditPublishCmd(flags, true))
	cmd.AddCommand(newEditPublishCmd(flags, false))
	cmd.AddCommand(newEditSectionCmd(flags))
	cmd.AddCommand(newEditInstructionCmd(flags))
	cmd.AddCommand(newEditInteractionCmd(flags))
	cmd.AddCommand(newEditPatchCmd(flags))
	return cmd
}

// edit concept <id> — update top-level metadata.
func newEditConceptCmd(flags *rootFlags) *cobra.Command {
	var title, desc, shortDesc, status string
	var price int
	var priceSet, apply bool
	cmd := &cobra.Command{
		Use:         "concept <id>",
		Short:       "Update a course's metadata (title, description, price, status)",
		Example:     "  agilix-dawn-pp-cli edit concept c_216daf6f76024e43b03b229895686555 --title \"New Title\" --apply",
		Annotations: map[string]string{"pp:happy-args": "id=c_216daf6f76024e43b03b229895686555"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would update concept metadata")
				return nil
			}
			if len(args) < 1 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("id is required (a concept id, c_...)"))
			}
			priceSet = cmd.Flags().Changed("price")
			if title == "" && desc == "" && shortDesc == "" && status == "" && !priceSet {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("nothing to change: pass at least one of --title/--description/--short-description/--price/--status"))
			}
			if status != "" && status != "enabled" && status != "disabled" {
				return usageErr(fmt.Errorf("--status must be 'enabled' or 'disabled'"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			obj, err := fetchConceptRaw(ctx, c, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			changes := ""
			set := func(field, val string) {
				obj[field] = val
				changes += fmt.Sprintf("  %s → %q\n", field, val)
			}
			if title != "" {
				set("title", title)
			}
			if desc != "" {
				set("description", desc)
			}
			if shortDesc != "" {
				set("shortDescription", shortDesc)
			}
			if status != "" {
				set("status", status)
			}
			if priceSet {
				obj["price"] = price
				changes += fmt.Sprintf("  price → %d\n", price)
			}
			summary := fmt.Sprintf("edit concept %s:\n%s", args[0], changes)
			return previewOrApply(cmd, flags, apply, c, obj, summary)
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "New course title")
	cmd.Flags().StringVar(&desc, "description", "", "New long description")
	cmd.Flags().StringVar(&shortDesc, "short-description", "", "New short description")
	cmd.Flags().IntVar(&price, "price", 0, "New price (in cents)")
	cmd.Flags().StringVar(&status, "status", "", "Set status: enabled or disabled")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually write the change (default previews only)")
	return cmd
}

// edit publish / unpublish <id>.
func newEditPublishCmd(flags *rootFlags, publish bool) *cobra.Command {
	use, target := "unpublish", "disabled"
	if publish {
		use, target = "publish", "enabled"
	}
	var apply bool
	cmd := &cobra.Command{
		Use:         use + " <id>",
		Short:       fmt.Sprintf("Set a course's status to %q", target),
		Example:     "  agilix-dawn-pp-cli edit " + use + " c_216daf6f76024e43b03b229895686555 --apply",
		Annotations: map[string]string{"pp:happy-args": "id=c_216daf6f76024e43b03b229895686555"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would set status to %s\n", target)
				return nil
			}
			if len(args) < 1 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("id is required (a concept id, c_...)"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			obj, err := fetchConceptRaw(ctx, c, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			obj["status"] = target
			summary := fmt.Sprintf("%s concept %s: status → %q", use, args[0], target)
			return previewOrApply(cmd, flags, apply, c, obj, summary)
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually write the change (default previews only)")
	return cmd
}

func newEditSectionCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "section",
		Short: "Manage sections within a course (add, rename, remove)",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newEditSectionAddCmd(flags))
	cmd.AddCommand(newEditSectionRenameCmd(flags))
	cmd.AddCommand(newEditSectionRemoveCmd(flags))
	return cmd
}

// edit section add <course-id> --title
func newEditSectionAddCmd(flags *rootFlags) *cobra.Command {
	var title string
	var apply bool
	cmd := &cobra.Command{
		Use:         "add <course-id>",
		Short:       "Append a new section to a course",
		Example:     "  agilix-dawn-pp-cli edit section add c_216daf6f76024e43b03b229895686555 --title \"Module 1\" --apply",
		Annotations: map[string]string{"pp:happy-args": "course-id=c_216daf6f76024e43b03b229895686555;--title=Module 1"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would add a section")
				return nil
			}
			if len(args) < 1 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("course-id is required (a concept id, c_...)"))
			}
			if title == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--title is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			obj, err := fetchConceptRaw(ctx, c, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			newID := genID("s_")
			sec := map[string]any{"id": newID, "title": title, "status": "enabled", "instruction": []any{}}
			obj["section"] = append(sectionsOf(obj), sec)
			summary := fmt.Sprintf("edit section add %s: new section %s (%q)", args[0], newID, title)
			return previewOrApply(cmd, flags, apply, c, obj, summary)
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "Section title")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually write the change (default previews only)")
	return cmd
}

// edit section rename <course-id> <section-id> --title
func newEditSectionRenameCmd(flags *rootFlags) *cobra.Command {
	var title string
	var apply bool
	cmd := &cobra.Command{
		Use:     "rename <course-id> <section-id>",
		Short:   "Rename a section",
		Example: "  agilix-dawn-pp-cli edit section rename c_216... s_dd819... --title \"New Name\" --apply",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rename a section")
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("course-id and section-id are required"))
			}
			if title == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--title is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			obj, err := fetchConceptRaw(ctx, c, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			found := false
			for _, s := range sectionsOf(obj) {
				if sm, ok := asMap(s); ok && sm["id"] == args[1] {
					sm["title"] = title
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("section %s not found in course %s", args[1], args[0])
			}
			summary := fmt.Sprintf("edit section rename %s/%s: title → %q", args[0], args[1], title)
			return previewOrApply(cmd, flags, apply, c, obj, summary)
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "New section title")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually write the change (default previews only)")
	return cmd
}

// edit section remove <course-id> <section-id>
func newEditSectionRemoveCmd(flags *rootFlags) *cobra.Command {
	var apply bool
	cmd := &cobra.Command{
		Use:     "remove <course-id> <section-id>",
		Short:   "Remove a section (and its instructions) from a course",
		Example: "  agilix-dawn-pp-cli edit section remove c_216... s_dd819... --apply",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would remove a section")
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("course-id and section-id are required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			obj, err := fetchConceptRaw(ctx, c, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			kept := make([]any, 0)
			found := false
			for _, s := range sectionsOf(obj) {
				if sm, ok := asMap(s); ok && sm["id"] == args[1] {
					found = true
					continue
				}
				kept = append(kept, s)
			}
			if !found {
				return fmt.Errorf("section %s not found in course %s", args[1], args[0])
			}
			obj["section"] = kept
			summary := fmt.Sprintf("edit section remove %s/%s", args[0], args[1])
			return previewOrApply(cmd, flags, apply, c, obj, summary)
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually write the change (default previews only)")
	return cmd
}

func newEditInstructionCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "instruction",
		Short: "Manage instructions (lessons) within a section (add, remove)",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newEditInstructionAddCmd(flags))
	cmd.AddCommand(newEditInstructionRemoveCmd(flags))
	return cmd
}

// edit instruction add <course-id> <section-id> --title
func newEditInstructionAddCmd(flags *rootFlags) *cobra.Command {
	var title, itype string
	var duration float64
	var points int
	var apply bool
	cmd := &cobra.Command{
		Use:     "add <course-id> <section-id>",
		Short:   "Append a new instruction (lesson) to a section",
		Example: "  agilix-dawn-pp-cli edit instruction add c_216... s_dd819... --title \"Lesson 1\" --type activity --apply",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would add an instruction")
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("course-id and section-id are required"))
			}
			if title == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--title is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			obj, err := fetchConceptRaw(ctx, c, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			newID := genID("i_")
			var target map[string]any
			for _, s := range sectionsOf(obj) {
				if sm, ok := asMap(s); ok && sm["id"] == args[1] {
					target = sm
					break
				}
			}
			if target == nil {
				return fmt.Errorf("section %s not found in course %s", args[1], args[0])
			}
			instr := map[string]any{
				"id": newID, "title": title, "type": itype, "status": "enabled",
				"duration": duration, "points": points, "interaction": []any{},
			}
			target["instruction"] = append(arrOf(target["instruction"]), instr)
			summary := fmt.Sprintf("edit instruction add %s/%s: new instruction %s (%q, type=%s)", args[0], args[1], newID, title, itype)
			return previewOrApply(cmd, flags, apply, c, obj, summary)
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "Instruction (lesson) title")
	cmd.Flags().StringVar(&itype, "type", "activity", "Instruction type (e.g. activity, video, survey)")
	cmd.Flags().Float64Var(&duration, "duration", 0, "Estimated duration in seconds")
	cmd.Flags().IntVar(&points, "points", 0, "Points awarded")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually write the change (default previews only)")
	return cmd
}

// edit instruction remove <course-id> <section-id> <instruction-id>
func newEditInstructionRemoveCmd(flags *rootFlags) *cobra.Command {
	var apply bool
	cmd := &cobra.Command{
		Use:     "remove <course-id> <section-id> <instruction-id>",
		Short:   "Remove an instruction (lesson) from a section",
		Example: "  agilix-dawn-pp-cli edit instruction remove c_216... s_dd819... i_abc... --apply",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would remove an instruction")
				return nil
			}
			if len(args) < 3 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("course-id, section-id and instruction-id are required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			obj, err := fetchConceptRaw(ctx, c, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var target map[string]any
			for _, s := range sectionsOf(obj) {
				if sm, ok := asMap(s); ok && sm["id"] == args[1] {
					target = sm
					break
				}
			}
			if target == nil {
				return fmt.Errorf("section %s not found in course %s", args[1], args[0])
			}
			existing := arrOf(target["instruction"])
			kept := make([]any, 0, len(existing))
			found := false
			for _, in := range existing {
				if im, ok := asMap(in); ok && im["id"] == args[2] {
					found = true
					continue
				}
				kept = append(kept, in)
			}
			if !found {
				return fmt.Errorf("instruction %s not found in section %s", args[2], args[1])
			}
			target["instruction"] = kept
			summary := fmt.Sprintf("edit instruction remove %s/%s/%s", args[0], args[1], args[2])
			return previewOrApply(cmd, flags, apply, c, obj, summary)
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually write the change (default previews only)")
	return cmd
}

// edit patch <id> --file — power-user escape hatch for advanced structure
// (interactions, mastery, per-instruction content) that the typed commands
// above do not model. Reads a full concept JSON and PUTs it verbatim.
func newEditPatchCmd(flags *rootFlags) *cobra.Command {
	var file string
	var apply bool
	cmd := &cobra.Command{
		Use:     "patch <id>",
		Short:   "Apply an edited full-concept JSON document (advanced authoring)",
		Long:    "Read a full concept JSON from --file and PUT it verbatim.\n\nUse this for advanced edits (interactions, mastery, per-instruction content) the typed edit commands do not model. Fetch the current document with 'concept get <id> --json', edit it, then patch it back.",
		Example: "  agilix-dawn-pp-cli edit patch c_216... --file course.json --apply",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would patch a concept from a JSON file")
				return nil
			}
			if len(args) < 1 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("id is required (a concept id, c_...)"))
			}
			if file == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--file is required"))
			}
			raw, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("reading %s: %w", file, err)
			}
			var obj map[string]any
			if err := json.Unmarshal(raw, &obj); err != nil {
				return usageErr(fmt.Errorf("%s is not valid JSON: %w", file, err))
			}
			if obj["id"] != args[0] {
				return usageErr(fmt.Errorf("the JSON document's id (%v) does not match <id> (%s)", obj["id"], args[0]))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			secs := len(sectionsOf(obj))
			summary := fmt.Sprintf("edit patch %s from %s (%d sections)", args[0], file, secs)
			return previewOrApply(cmd, flags, apply, c, obj, summary)
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "Path to a full concept JSON document")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually write the change (default previews only)")
	return cmd
}
