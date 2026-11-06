// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/circle/internal/client"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type spacesApplyFile struct {
	Groups []map[string]any `yaml:"space_groups"`
	Spaces []map[string]any `yaml:"spaces"`
}

type spacesApplyChange struct {
	From any `json:"from"`
	To   any `json:"to"`
}

type spacesApplyEntry struct {
	Kind    string                       `json:"kind"`
	Action  string                       `json:"action"`
	Name    string                       `json:"name"`
	ID      any                          `json:"id,omitempty"`
	Applied bool                         `json:"applied"`
	Changes map[string]spacesApplyChange `json:"changes"`
}

type spacesApplyResult struct {
	Applied   bool                `json:"applied"`
	Plan      []spacesApplyEntry  `json:"plan"`
	Summary   map[string]int      `json:"summary"`
	Warnings  []string            `json:"warnings"`
	Unmanaged *[]spacesApplyEntry `json:"unmanaged,omitempty"`
}

func newNovelSpacesApplyCmd(flags *rootFlags) *cobra.Command {
	var apply, prune bool
	cmd := &cobra.Command{
		Use:   "apply <file.yaml>",
		Short: "Make Circle's space groups and spaces match a YAML file, previewing changes first.",
		Long: strings.Trim(`
Use this command to make Circle's space structure match a YAML file. Do NOT use it to read structure; use 'spaces list' instead.

Minimal YAML:
space_groups:
  - name: Spaces
    slug: spaces
spaces:
  - name: Start Here
    slug: start-here
    group: Spaces
    type: basic
    is_private: false

Supported keys are space_groups (name, slug, is_hidden_from_non_members, hide_members_count, allow_members_to_create_spaces, automatically_add_members_to_new_spaces) and spaces (name, slug, group, type, emoji, default_sort, is_private, is_hidden_from_non_members, is_hidden, is_post_disabled, hide_from_sidebar).
`, "\n"),
		Example: strings.Trim(`
  circle-pp-cli spaces apply spaces.yaml --json
  circle-pp-cli spaces apply spaces.yaml --apply
  circle-pp-cli spaces apply spaces.yaml --prune-report
`, "\n"),
		Annotations: map[string]string{"pp:data-source": "live", "pp:preview-happy-path": "true", "pp:happy-args": "file=testdata/spaces.example.yaml"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "spaces apply")
			}
			if len(args) != 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("expected one YAML file"))
			}
			data, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			desired, err := parseSpacesApply(data)
			if err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			groups, err := fetchSpacesApply(ctx, c, "/space_groups")
			if err != nil {
				return err
			}
			spaces, err := fetchSpacesApply(ctx, c, "/spaces")
			if err != nil {
				return err
			}
			result, err := diffSpacesApply(desired, groups, spaces, prune)
			if err != nil {
				return usageErr(err)
			}
			if apply {
				if err := executeSpacesApply(ctx, c, &result); err != nil {
					cmd.SilenceUsage = true
					outputFlags := *flags
					outputFlags.asJSON = true
					outputFlags.csv, outputFlags.plain, outputFlags.quiet = false, false, false
					if outputErr := printJSONFiltered(cmd.OutOrStdout(), result, &outputFlags); outputErr != nil {
						return fmt.Errorf("%w; print partial result: %w", err, outputErr)
					}
					return err
				}
				result.Applied = true
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			for _, entry := range result.Plan {
				if entry.Action != "noop" {
					if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s\n", entry.Action, entry.Kind, entry.Name); err != nil {
						return err
					}
				}
			}
			for _, warning := range result.Warnings {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), "Warning:", warning); err != nil {
					return err
				}
			}
			if result.Unmanaged != nil {
				for _, entry := range *result.Unmanaged {
					if _, err := fmt.Fprintf(cmd.OutOrStdout(), "unmanaged %s %s (%v)\n", entry.Kind, entry.Name, entry.ID); err != nil {
						return err
					}
				}
			}
			if result.Summary["create"]+result.Summary["update"] == 0 {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "Nothing to change.")
				return err
			} else if !apply {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "Re-run with --apply to make these changes.")
				return err
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply the planned creates and updates.")
	cmd.Flags().BoolVar(&prune, "prune-report", false, "Report unmanaged spaces and groups without deleting them.")
	return cmd
}

func parseSpacesApply(data []byte) (spacesApplyFile, error) {
	var file spacesApplyFile
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&file); err != nil {
		return file, err
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return file, fmt.Errorf("expected a single YAML document")
	}
	for _, section := range []struct {
		name, strings, bools string
		rows                 []map[string]any
	}{
		{"space_groups", "name slug", "is_hidden_from_non_members hide_members_count allow_members_to_create_spaces automatically_add_members_to_new_spaces", file.Groups},
		{"spaces", "name slug group type emoji default_sort", "is_private is_hidden_from_non_members is_hidden is_post_disabled hide_from_sidebar", file.Spaces},
	} {
		for i, row := range section.rows {
			for key, value := range row {
				switch {
				case slices.Contains(strings.Fields(section.strings), key):
					text, ok := value.(string)
					if !ok || (strings.TrimSpace(text) == "" && key != "emoji") {
						return file, fmt.Errorf("%s[%d].%s must be a nonempty string", section.name, i, key)
					}
				case slices.Contains(strings.Fields(section.bools), key):
					if _, ok := value.(bool); !ok {
						return file, fmt.Errorf("%s[%d].%s must be a boolean", section.name, i, key)
					}
				default:
					return file, fmt.Errorf("unknown YAML key %q in %s[%d]", key, section.name, i)
				}
			}
			if row["name"] == nil && row["slug"] == nil {
				return file, fmt.Errorf("%s[%d] requires name or slug", section.name, i)
			}
			if typ, ok := row["type"]; ok && !slices.Contains([]string{"basic", "event", "course", "chat", "members", "image"}, typ.(string)) {
				return file, fmt.Errorf("unsupported space type %q", typ)
			}
		}
	}
	return file, nil
}

func fetchSpacesApply(ctx context.Context, c *client.Client, path string) ([]map[string]any, error) {
	rows := make([]map[string]any, 0)
	for page := 1; ; page++ {
		data, err := c.Get(ctx, path, map[string]string{"page": strconv.Itoa(page), "per_page": "100"})
		if err != nil {
			return nil, err
		}
		var envelope struct {
			Records []map[string]any `json:"records"`
			HasNext *bool            `json:"has_next_page"`
			Success *bool            `json:"success"`
			Message string           `json:"message"`
		}
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.UseNumber()
		if err := dec.Decode(&envelope); err != nil {
			return nil, err
		}
		if envelope.Success != nil && !*envelope.Success {
			return nil, fmt.Errorf("%s: %s", path, envelope.Message)
		}
		if envelope.HasNext == nil || envelope.Records == nil {
			return nil, fmt.Errorf("%s: missing records or pagination metadata", path)
		}
		rows = append(rows, envelope.Records...)
		if !*envelope.HasNext {
			return rows, nil
		}
		if len(envelope.Records) == 0 {
			return nil, fmt.Errorf("%s: empty page with has_next_page=true", path)
		}
	}
}

func spacesApplyMatch(want map[string]any, live []map[string]any) (map[string]any, error) {
	key := "name"
	if want["slug"] != nil {
		key = "slug"
	}
	var match map[string]any
	for _, row := range live {
		if strings.EqualFold(fmt.Sprint(want[key]), fmt.Sprint(row[key])) {
			if match != nil {
				return nil, fmt.Errorf("ambiguous %s %q", key, want[key])
			}
			match = row
		}
	}
	return match, nil
}

func spacesApplyGroup(ref string, groups []map[string]any) (any, error) {
	var id any
	for _, group := range groups {
		if strings.EqualFold(ref, fmt.Sprint(group["name"])) || (group["slug"] != nil && strings.EqualFold(ref, fmt.Sprint(group["slug"]))) {
			if id != nil && !reflect.DeepEqual(normalizeSpacesApplyValue(id), normalizeSpacesApplyValue(group["id"])) {
				return nil, fmt.Errorf("ambiguous group reference %q", ref)
			}
			id = group["id"]
		}
	}
	if id == nil {
		return nil, fmt.Errorf("unknown group reference %q", ref)
	}
	return id, nil
}

func spacesApplySlug(name string) string {
	var b strings.Builder
	separator := false
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if separator && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			separator = false
		} else {
			separator = true
		}
	}
	return b.String()
}

func normalizeSpacesApplyValue(value any) any {
	switch n := value.(type) {
	case json.Number:
		if integer, err := n.Int64(); err == nil {
			return integer
		}
	case int:
		return int64(n)
	case float64:
		if integer := int64(n); float64(integer) == n {
			return integer
		}
	}
	return value
}

func diffSpacesApply(file spacesApplyFile, groups, spaces []map[string]any, prune bool) (spacesApplyResult, error) {
	result := spacesApplyResult{Plan: make([]spacesApplyEntry, 0), Summary: map[string]int{"create": 0, "update": 0, "noop": 0}, Warnings: make([]string, 0)}
	resolved := make([]map[string]any, 0, len(groups)+len(file.Groups))
	// Overlay declared group names/slugs so spaces can reference groups being renamed or created.
	managed := map[string]bool{}
	for _, section := range []struct {
		kind          string
		desired, live []map[string]any
	}{{"space_group", file.Groups, groups}, {"space", file.Spaces, spaces}} {
		seen := map[string]bool{}
		for _, want := range section.desired {
			current, err := spacesApplyMatch(want, section.live)
			if err != nil {
				return result, err
			}
			name, _ := want["name"].(string)
			if name == "" {
				if current == nil {
					return result, fmt.Errorf("new %s with slug %q requires name", section.kind, want["slug"])
				}
				name = fmt.Sprint(current["name"])
			}
			entry := spacesApplyEntry{Kind: section.kind, Action: "noop", Name: name, Changes: map[string]spacesApplyChange{}}
			identity := fmt.Sprint(want["slug"])
			if want["slug"] == nil {
				identity = entry.Name
			}
			if current != nil {
				entry.ID = current["id"]
				if entry.ID == nil {
					return result, fmt.Errorf("%s %q has no live id", section.kind, entry.Name)
				}
				identity = fmt.Sprint(entry.ID)
				managed[section.kind+":"+identity] = true
			} else {
				entry.Action = "create"
			}
			identity = strings.ToLower(identity)
			if seen[identity] {
				return result, fmt.Errorf("duplicate %s %q", section.kind, entry.Name)
			}
			seen[identity] = true
			for field, to := range want {
				from := current[field]
				if field == "type" {
					field = "space_type"
					from = current[field]
					if current != nil {
						if !reflect.DeepEqual(from, to) {
							result.Warnings = append(result.Warnings, entry.Name+": type cannot change after creation")
						}
						continue
					}
				}
				if field == "group" {
					field = "space_group_id"
					to, err = spacesApplyGroup(to.(string), resolved)
					if err != nil {
						return result, err
					}
					from = current[field]
					if from == nil {
						if nested, ok := current["space_group"].(map[string]any); ok {
							from = nested["id"]
						}
					}
				}
				if current == nil || !reflect.DeepEqual(normalizeSpacesApplyValue(from), normalizeSpacesApplyValue(to)) {
					entry.Changes[field] = spacesApplyChange{From: from, To: to}
				}
			}
			if current == nil {
				if section.kind == "space" && want["group"] == nil {
					return result, fmt.Errorf("new space %q requires group", entry.Name)
				}
				if want["slug"] == nil {
					slug := spacesApplySlug(entry.Name)
					if slug == "" {
						return result, fmt.Errorf("%s %q requires an explicit slug", section.kind, entry.Name)
					}
					entry.Changes["slug"] = spacesApplyChange{To: slug}
				}
			}
			if current != nil && len(entry.Changes) > 0 {
				entry.Action = "update"
			}
			if section.kind == "space_group" {
				group := map[string]any{}
				for k, v := range current {
					group[k] = v
				}
				for k, v := range want {
					group[k] = v
				}
				if slug, ok := entry.Changes["slug"]; ok {
					group["slug"] = slug.To
				}
				if entry.ID == nil {
					group["id"] = "new-group:" + strconv.Itoa(len(result.Plan))
				}
				resolved = append(resolved, group)
			}
			result.Summary[entry.Action]++
			result.Plan = append(result.Plan, entry)
		}
		if section.kind == "space_group" {
			for _, group := range groups {
				if !managed["space_group:"+fmt.Sprint(group["id"])] {
					resolved = append(resolved, group)
				}
			}
		}
	}
	if prune {
		unmanaged := make([]spacesApplyEntry, 0)
		for _, section := range []struct {
			kind string
			rows []map[string]any
		}{{"space_group", groups}, {"space", spaces}} {
			for _, row := range section.rows {
				if !managed[section.kind+":"+fmt.Sprint(row["id"])] {
					unmanaged = append(unmanaged, spacesApplyEntry{Kind: section.kind, Name: fmt.Sprint(row["name"]), ID: row["id"], Changes: map[string]spacesApplyChange{}})
				}
			}
		}
		result.Unmanaged = &unmanaged
	}
	return result, nil
}

func spacesApplyResponseID(response map[string]any) any {
	if space, ok := response["space"].(map[string]any); ok && space["id"] != nil {
		return space["id"]
	}
	return response["id"]
}

func executeSpacesApply(ctx context.Context, c interface {
	Post(context.Context, string, any) (json.RawMessage, int, error)
	Put(context.Context, string, any) (json.RawMessage, int, error)
}, result *spacesApplyResult) error {
	created := map[string]any{}
	for _, phase := range []string{"space_group:create", "space:create", "update"} {
		for i := range result.Plan {
			entry := &result.Plan[i]
			if phase != entry.Kind+":"+entry.Action && (phase != "update" || entry.Action != "update") {
				continue
			}
			body := map[string]any{}
			for field, change := range entry.Changes {
				body[field] = change.To
			}
			if ref, ok := body["space_group_id"].(string); ok && strings.HasPrefix(ref, "new-group:") {
				id, ok := created[ref]
				if !ok {
					return fmt.Errorf("unresolved created group %q", ref)
				}
				body["space_group_id"] = id
				change := entry.Changes["space_group_id"]
				change.To = id
				entry.Changes["space_group_id"] = change
			}
			if id, ok := body["space_group_id"]; ok {
				body["space_group_id"] = normalizeSpacesApplyValue(id)
			}
			path := "/spaces"
			if entry.Kind == "space_group" {
				path = "/space_groups"
			}
			var raw json.RawMessage
			var err error
			if entry.Action == "create" {
				raw, _, err = c.Post(ctx, path, body)
			} else {
				raw, _, err = c.Put(ctx, path+"/"+fmt.Sprint(normalizeSpacesApplyValue(entry.ID)), body)
			}
			if err != nil {
				return fmt.Errorf("%s %s %q (earlier changes may have applied): %w", entry.Action, entry.Kind, entry.Name, err)
			}
			var response map[string]any
			dec := json.NewDecoder(bytes.NewReader(raw))
			dec.UseNumber()
			if err := dec.Decode(&response); err != nil {
				return err
			}
			if success, ok := response["success"].(bool); ok && !success {
				return fmt.Errorf("%s %q: %v", entry.Action, entry.Name, response["message"])
			}
			if entry.Action == "create" {
				entry.ID = spacesApplyResponseID(response)
				if entry.ID == nil {
					return fmt.Errorf("created %s %q but response has no id; inspect live state before retrying", entry.Kind, entry.Name)
				}
				if entry.Kind == "space_group" {
					created["new-group:"+strconv.Itoa(i)] = entry.ID
				}
			}
			entry.Applied = true
		}
	}
	return nil
}
