// Copyright 2026 Ryan Gravette and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel commands: enrollment management via Dawn enrollment groups.
//
// In Dawn, enrollment is group membership: a group (g_...) has an `object` list
// of course ids and a `member` list of enrolled user ids. Enrolling a learner =
// adding their user id to a group's member[] and PUT /api/group. All writes use
// the shared preview-by-default / --apply safety gate.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/agilix-dawn/internal/client"
	"github.com/spf13/cobra"
)

type enrollGroup struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Body         string   `json:"body,omitempty"`
	Object       []string `json:"object"`
	Member       []string `json:"member"`
	MemberCount  int      `json:"memberCount"`
	MaxGroupSize int      `json:"maxGroupSize"`
	Private      bool     `json:"private"`
	Status       string   `json:"status"`
	Type         string   `json:"type"`
}

func fetchGroupByID(ctx context.Context, c *client.Client, groupID string) (map[string]any, error) {
	search := fmt.Sprintf(`{"query":"_id:%s","limit":1}`, groupID)
	// A just-created group can lag in the search index; retry briefly.
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Second):
			}
		}
		data, err := c.Get(ctx, "/group", map[string]string{"search": search})
		if err != nil {
			lastErr = err
			continue
		}
		var env struct {
			Matches []map[string]any `json:"matches"`
		}
		if err := json.Unmarshal(data, &env); err != nil {
			return nil, fmt.Errorf("parsing group: %w", err)
		}
		if len(env.Matches) > 0 {
			return env.Matches[0], nil
		}
		lastErr = fmt.Errorf("no group found for id %q", groupID)
	}
	return nil, lastErr
}

func fetchMyUserID(ctx context.Context, c *client.Client) string {
	data, err := c.Get(ctx, "/user/me", nil)
	if err != nil {
		return ""
	}
	var me struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(data, &me)
	return me.ID
}

func stringSlice(v any) []string {
	out := []string{}
	for _, e := range arrOf(v) {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func newEnrollmentCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enrollment",
		Short: "Manage course enrollments via enrollment groups",
		Long: "Manage enrollments. In Dawn, learners are enrolled through groups: a group is\n" +
			"tied to a course and holds the enrolled users. Use 'group' to manage groups and\n" +
			"'add'/'remove' to enroll/unenroll users. Writes preview by default; pass --apply.",
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newEnrollmentGroupCmd(flags))
	cmd.AddCommand(newEnrollmentMembersCmd(flags))
	cmd.AddCommand(newEnrollmentAddCmd(flags))
	cmd.AddCommand(newEnrollmentRemoveCmd(flags))
	return cmd
}

func newEnrollmentGroupCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group",
		Short: "Manage enrollment groups (list, create, delete)",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newEnrollmentGroupListCmd(flags))
	cmd.AddCommand(newEnrollmentGroupCreateCmd(flags))
	cmd.AddCommand(newEnrollmentGroupDeleteCmd(flags))
	return cmd
}

// enrollment group list <course-id>
func newEnrollmentGroupListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list <course-id>",
		Short:   "List enrollment groups for a course",
		Example: "  agilix-dawn-pp-cli enrollment group list c_216daf6f76024e43b03b229895686555 --json",
		// An unknown course id returns an empty group list (exit 0), not an error —
		// there is no way to distinguish a bad id from a course with no groups.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "course-id=c_216daf6f76024e43b03b229895686555", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list enrollment groups")
				return nil
			}
			if len(args) < 1 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("course-id is required (a concept id, c_...)"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			matches, total, err := fetchAllSearch(ctx, c, "group", map[string]any{
				"query": fmt.Sprintf("object:%s", args[0]),
				"sort":  []map[string]string{{"name.raw": "asc"}},
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			warnTruncated(cmd.ErrOrStderr(), "group", len(matches), total)
			groups := make([]enrollGroup, 0, len(matches))
			for _, m := range matches {
				var g enrollGroup
				if json.Unmarshal(m, &g) == nil {
					groups = append(groups, g)
				}
			}
			if flags.asJSON {
				return flags.printJSON(cmd, groups)
			}
			w := cmd.OutOrStdout()
			if len(groups) == 0 {
				fmt.Fprintln(w, "no enrollment groups for this course")
				return nil
			}
			for _, g := range groups {
				fmt.Fprintf(w, "%s\t%s\t%d/%d members\t%s\n", g.ID, g.Name, g.MemberCount, g.MaxGroupSize, g.Status)
			}
			return nil
		},
	}
	return cmd
}

// enrollment group create <course-id> --name
func newEnrollmentGroupCreateCmd(flags *rootFlags) *cobra.Command {
	var name, description string
	var maxSize int
	var private, apply bool
	cmd := &cobra.Command{
		Use:         "create <course-id>",
		Short:       "Create an enrollment group for a course",
		Example:     "  agilix-dawn-pp-cli enrollment group create c_216... --name \"Fall 2026\" --apply",
		Annotations: map[string]string{"pp:happy-args": "course-id=c_216daf6f76024e43b03b229895686555;--name=Fall 2026"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would create an enrollment group")
				return nil
			}
			if len(args) < 1 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("course-id is required (a concept id, c_...)"))
			}
			if name == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--name is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			newID := genID("g_")
			grp := map[string]any{
				"id": newID, "name": name, "body": description,
				"object": []any{args[0]}, "member": []any{},
				"maxGroupSize": maxSize, "private": private,
				"status": "enabled", "type": "study", "coach": []any{},
			}
			if me := fetchMyUserID(ctx, c); me != "" {
				grp["admin"] = []any{me}
			}
			summary := fmt.Sprintf("enrollment group create for %s: new group %s (%q, max %d)", args[0], newID, name, maxSize)
			return previewOrApplyPath(cmd, flags, apply, c, "/group", grp, summary)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Group name")
	cmd.Flags().StringVar(&description, "description", "", "Group description")
	cmd.Flags().IntVar(&maxSize, "max-size", 500, "Maximum group size")
	cmd.Flags().BoolVar(&private, "private", false, "Make the group private")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually write the change (default previews only)")
	return cmd
}

// enrollment group delete <group-id>
func newEnrollmentGroupDeleteCmd(flags *rootFlags) *cobra.Command {
	var apply bool
	cmd := &cobra.Command{
		Use:     "delete <group-id>",
		Short:   "Delete an enrollment group",
		Example: "  agilix-dawn-pp-cli enrollment group delete g_05d557... --apply",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would delete an enrollment group")
				return nil
			}
			if len(args) < 1 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("group-id is required (g_...)"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			summary := fmt.Sprintf("enrollment group delete %s", args[0])
			return previewOrApplyDelete(cmd, flags, apply, c, "/group/"+pathEscape(args[0]), summary)
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually delete (default previews only)")
	return cmd
}

// enrollment members <group-id>
func newEnrollmentMembersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "members <group-id>",
		Short:       "List the user ids enrolled in a group",
		Example:     "  agilix-dawn-pp-cli enrollment members g_05d557... --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list group members")
				return nil
			}
			if len(args) < 1 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("group-id is required (g_...)"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			grp, err := fetchGroupByID(ctx, c, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			members := stringSlice(grp["member"])
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{"group": args[0], "member_count": len(members), "members": members})
			}
			w := cmd.OutOrStdout()
			if len(members) == 0 {
				fmt.Fprintln(w, "no members enrolled in this group")
				return nil
			}
			for _, m := range members {
				fmt.Fprintln(w, m)
			}
			fmt.Fprintf(w, "\n%d members\n", len(members))
			return nil
		},
	}
	return cmd
}

// enrollment add <group-id> --user <user-id>  (enroll)
func newEnrollmentAddCmd(flags *rootFlags) *cobra.Command {
	var user string
	var apply bool
	cmd := &cobra.Command{
		Use:     "add <group-id>",
		Short:   "Enroll a user into a group (add to member list)",
		Long:    "Enroll a user into a group by adding their user id to the group's member list.\n\nThis is a real enrollment — it previews by default and only writes with --apply.",
		Example: "  agilix-dawn-pp-cli enrollment add g_05d557... --user u_9a5fc7... --apply",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would enroll a user into the group")
				return nil
			}
			if len(args) < 1 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("group-id is required (g_...)"))
			}
			if user == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--user is required (a user id, u_...)"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			grp, err := fetchGroupByID(ctx, c, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			members := stringSlice(grp["member"])
			for _, m := range members {
				if m == user {
					return fmt.Errorf("user %s is already enrolled in group %s", user, args[0])
				}
			}
			members = append(members, user)
			grp["member"] = members
			grp["memberCount"] = len(members)
			summary := fmt.Sprintf("enrollment add %s → group %s (now %d members)", user, args[0], len(members))
			return previewOrApplyPath(cmd, flags, apply, c, "/group", grp, summary)
		},
	}
	cmd.Flags().StringVar(&user, "user", "", "User id to enroll (u_...)")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually enroll (default previews only)")
	return cmd
}

// enrollment remove <group-id> --user <user-id>  (unenroll)
func newEnrollmentRemoveCmd(flags *rootFlags) *cobra.Command {
	var user string
	var apply bool
	cmd := &cobra.Command{
		Use:     "remove <group-id>",
		Short:   "Unenroll a user from a group (remove from member list)",
		Example: "  agilix-dawn-pp-cli enrollment remove g_05d557... --user u_9a5fc7... --apply",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would unenroll a user from the group")
				return nil
			}
			if len(args) < 1 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("group-id is required (g_...)"))
			}
			if user == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--user is required (a user id, u_...)"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			grp, err := fetchGroupByID(ctx, c, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			members := stringSlice(grp["member"])
			kept := make([]any, 0, len(members))
			found := false
			for _, m := range members {
				if m == user {
					found = true
					continue
				}
				kept = append(kept, m)
			}
			if !found {
				return fmt.Errorf("user %s is not enrolled in group %s", user, args[0])
			}
			grp["member"] = kept
			grp["memberCount"] = len(kept)
			summary := fmt.Sprintf("enrollment remove %s from group %s (now %d members)", user, args[0], len(kept))
			return previewOrApplyPath(cmd, flags, apply, c, "/group", grp, summary)
		},
	}
	cmd.Flags().StringVar(&user, "user", "", "User id to unenroll (u_...)")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually unenroll (default previews only)")
	return cmd
}
