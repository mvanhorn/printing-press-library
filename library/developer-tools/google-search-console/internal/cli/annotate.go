// Copyright 2026 james-frewin. Licensed under Apache-2.0. See LICENSE.

// Agent-owned annotations layer. Persists arbitrary notes and tags on
// pages, queries, sites, and triage items in the local store so an agent
// has memory between sessions. Triage state (resolved / snoozed) is
// expressed as annotations on triage items with reserved tags
// ("resolved", "snoozed") and an expires_at value.
//
// All writes here are local-only — no Google API impact. MCP annotations
// mark them mcp:read-only=false so hosts surface a permission prompt the
// first time, but they're not destructive (the annotation table is the
// agent's workspace, not user-visible Search Console state).
package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var validAnnotationTypes = map[string]bool{
	"page": true, "query": true, "site": true, "triage": true,
}

func newAnnotateCmd(flags *rootFlags) *cobra.Command {
	var (
		note   string
		tags   []string
		dbPath string
	)
	cmd := &cobra.Command{
		Use:   "annotate <type> <target>",
		Short: "Attach a note and/or tags to a page, query, site, or triage item",
		Long: strings.TrimSpace(`
Stores a note and/or tags against a target in the local store. Targets
are addressed by (type, identifier) where type is one of: page, query,
site, triage. Examples:

  annotate page https://example.com/post --note "Q2 priority" --tag q2 --tag landing
  annotate query "branded term" --tag branded
  annotate site sc-domain:example.com --note "Owned by SEO team"

Annotations persist in the local SQLite store and survive across runs.
Use the annotations command to list them and annotation-remove to delete
by id. Pure local — no GSC API impact.
`),
		Example:     `  google-search-console-pp-cli annotate page https://example.com/post --note "redesign launched" --tag launch`,
		Args:        cobra.ExactArgs(2),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			targetType := strings.ToLower(args[0])
			target := args[1]
			if !validAnnotationTypes[targetType] {
				return usageErr(fmt.Errorf("type must be one of page|query|site|triage, got %q", targetType))
			}
			s, err := openStoreFromFlag(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			id, err := s.AddAnnotation(cmd.Context(), targetType, target, note, strings.Join(tags, ","), "")
			if err != nil {
				return apiErr(err)
			}
			return emit(cmd, flags, map[string]any{
				"id":          id,
				"target_type": targetType,
				"target":      target,
				"note":        note,
				"tags":        tags,
				"status":      "created",
			})
		},
	}
	cmd.Flags().StringVar(&note, "note", "", "Free-text note (optional).")
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "Tag(s), repeatable.")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite path (default ~/.config/google-search-console-pp-cli/store.sqlite).")
	return cmd
}

func newAnnotationsCmd(flags *rootFlags) *cobra.Command {
	var (
		targetType     string
		target         string
		tagFilter      string
		includeExpired bool
		dbPath         string
	)
	cmd := &cobra.Command{
		Use:   "annotations",
		Short: "List annotations, optionally filtered by type, target, or tag",
		Long: strings.TrimSpace(`
Lists annotations from the local store. Filters compose: --type narrows
to one of page|query|site|triage; --target matches the entity
identifier exactly; --tag matches a substring of the comma-joined tags
column. Expired snoozes are hidden unless --include-expired is set.
`),
		Example: strings.Join([]string{
			"  google-search-console-pp-cli annotations",
			"  google-search-console-pp-cli annotations --type page --tag launch",
			"  google-search-console-pp-cli annotations --type triage --include-expired",
		}, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if targetType != "" && !validAnnotationTypes[targetType] {
				return usageErr(fmt.Errorf("--type must be one of page|query|site|triage, got %q", targetType))
			}
			s, err := openStoreFromFlag(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			anns, err := s.ListAnnotations(cmd.Context(), targetType, target, tagFilter, includeExpired)
			if err != nil {
				return apiErr(err)
			}
			out := make([]map[string]any, 0, len(anns))
			for _, a := range anns {
				rec := map[string]any{
					"id":          a.ID,
					"target_type": a.TargetType,
					"target":      a.Target,
					"note":        a.Note,
					"tags":        a.Tags,
					"created_at":  a.CreatedAt,
					"updated_at":  a.UpdatedAt,
				}
				if a.ExpiresAt != "" {
					rec["expires_at"] = a.ExpiresAt
				}
				out = append(out, rec)
			}
			return emit(cmd, flags, map[string]any{
				"count":       len(out),
				"annotations": out,
			})
		},
	}
	cmd.Flags().StringVar(&targetType, "type", "", "Filter: page|query|site|triage.")
	cmd.Flags().StringVar(&target, "target", "", "Filter: exact target identifier.")
	cmd.Flags().StringVar(&tagFilter, "tag", "", "Filter: tag substring.")
	cmd.Flags().BoolVar(&includeExpired, "include-expired", false, "Include expired entries (e.g. lapsed snoozes).")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite path (default ~/.config/google-search-console-pp-cli/store.sqlite).")
	return cmd
}

func newAnnotationRemoveCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "annotation-remove <id>",
		Short:       "Delete an annotation by id",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("id must be an integer, got %q", args[0]))
			}
			s, err := openStoreFromFlag(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			n, err := s.RemoveAnnotation(cmd.Context(), id)
			if err != nil {
				return apiErr(err)
			}
			status := "removed"
			if n == 0 {
				status = "not-found"
			}
			return emit(cmd, flags, map[string]any{"id": id, "rows_affected": n, "status": status})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite path (default ~/.config/google-search-console-pp-cli/store.sqlite).")
	return cmd
}

// Triage-state shortcuts. Each one is sugar over annotate / annotation-
// remove with the conventional ("resolved" | "snoozed") tag and an
// optional expiry. Agents are free to query/clear them directly through
// the annotations table; these commands exist because resolve/snooze are
// the dominant triage workflows and deserve discoverable names.

func newTriageResolveCmd(flags *rootFlags) *cobra.Command {
	var (
		note   string
		dbPath string
	)
	cmd := &cobra.Command{
		Use:         "triage-resolve <triage-id>",
		Short:       "Mark a triage item as resolved (persists in the local store)",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			s, err := openStoreFromFlag(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			id, err := s.AddAnnotation(cmd.Context(), "triage", args[0], note, "resolved", "")
			if err != nil {
				return apiErr(err)
			}
			return emit(cmd, flags, map[string]any{
				"triage_id":     args[0],
				"annotation_id": id,
				"state":         "resolved",
			})
		},
	}
	cmd.Flags().StringVar(&note, "note", "", "Optional note about the resolution.")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite path (default ~/.config/google-search-console-pp-cli/store.sqlite).")
	return cmd
}

func newTriageSnoozeCmd(flags *rootFlags) *cobra.Command {
	var (
		until  string
		note   string
		dbPath string
	)
	cmd := &cobra.Command{
		Use:   "triage-snooze <triage-id>",
		Short: "Snooze a triage item for a period (Nd, Nw, Nm)",
		Long: strings.TrimSpace(`
Stores a snooze annotation on the triage item with expires_at set to
now + --until. Subsequent annotations list calls hide the snooze until
it expires (unless --include-expired is passed).
`),
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			days := parseWindow(until, 7)
			expiresAt := time.Now().UTC().AddDate(0, 0, days).Format(time.RFC3339)
			s, err := openStoreFromFlag(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			id, err := s.AddAnnotation(cmd.Context(), "triage", args[0], note, "snoozed", expiresAt)
			if err != nil {
				return apiErr(err)
			}
			return emit(cmd, flags, map[string]any{
				"triage_id":     args[0],
				"annotation_id": id,
				"state":         "snoozed",
				"expires_at":    expiresAt,
			})
		},
	}
	cmd.Flags().StringVar(&until, "until", "7d", "Snooze window: Nd, Nw, or Nm.")
	cmd.Flags().StringVar(&note, "note", "", "Optional note about the snooze.")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite path (default ~/.config/google-search-console-pp-cli/store.sqlite).")
	return cmd
}

func newTriageUnresolveCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "triage-unresolve <triage-id>",
		Short:       "Remove resolved/snoozed annotations from a triage item",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			s, err := openStoreFromFlag(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			// Find every resolved or snoozed annotation on this triage id
			// (including expired snoozes so unresolve cleans up history too).
			anns, err := s.ListAnnotations(cmd.Context(), "triage", args[0], "", true)
			if err != nil {
				return apiErr(err)
			}
			removed := int64(0)
			for _, a := range anns {
				if a.Tags == "resolved" || a.Tags == "snoozed" {
					n, err := s.RemoveAnnotation(cmd.Context(), a.ID)
					if err != nil {
						return apiErr(err)
					}
					removed += n
				}
			}
			return emit(cmd, flags, map[string]any{
				"triage_id":     args[0],
				"removed_count": removed,
				"state":         "unresolved",
			})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite path (default ~/.config/google-search-console-pp-cli/store.sqlite).")
	return cmd
}
