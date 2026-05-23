package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/vault"
)

func newTagCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag",
		Short: "List tags, find notes by tag, and add/remove tags from notes.",
	}
	cmd.AddCommand(newTagListCmd(flags))
	cmd.AddCommand(newTagGetCmd(flags))
	cmd.AddCommand(newTagAddCmd(flags))
	cmd.AddCommand(newTagRmCmd(flags))
	return cmd
}

func newTagListCmd(flags *rootFlags) *cobra.Command {
	var prefix string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List every tag in the vault (frontmatter + inline), with usage counts.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}
			vc, err := openVaultAndStore(cmd.Context(), flags)
			if err != nil {
				return err
			}
			defer vc.Close()
			q := `SELECT tag, COUNT(DISTINCT path) FROM tags`
			var argsList []interface{}
			if prefix != "" {
				q += ` WHERE tag LIKE ?`
				argsList = append(argsList, prefix+"%")
			}
			q += ` GROUP BY tag ORDER BY 2 DESC, 1 ASC`
			rows, err := vc.S.DB().QueryContext(cmd.Context(), q, argsList...)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			type entry struct {
				Tag   string `json:"tag"`
				Count int    `json:"count"`
			}
			var out []entry
			for rows.Next() {
				var e entry
				if err := rows.Scan(&e.Tag, &e.Count); err != nil {
					return apiErr(err)
				}
				out = append(out, e)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
			}
			for _, e := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%d\n", e.Tag, e.Count)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&prefix, "match", "", "Match only tags with this prefix")
	return cmd
}

func newTagGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "get [tag]",
		Short:       "List notes carrying a tag.",
		Example:     "  obsidian-pp-cli tag get servosity --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if cliutil.IsVerifyEnv() {
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}
			vc, err := openVaultAndStore(cmd.Context(), flags)
			if err != nil {
				return err
			}
			defer vc.Close()
			tag := strings.TrimPrefix(args[0], "#")
			rows, err := vc.S.DB().QueryContext(cmd.Context(),
				`SELECT n.path, COALESCE(n.description,''), COALESCE(n.type,'')
				 FROM notes n JOIN tags t ON t.path = n.path
				 WHERE t.tag = ? GROUP BY n.path ORDER BY n.path`, tag)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			type entry struct {
				Path        string `json:"path"`
				Description string `json:"description,omitempty"`
				Type        string `json:"type,omitempty"`
			}
			var out []entry
			for rows.Next() {
				var e entry
				if err := rows.Scan(&e.Path, &e.Description, &e.Type); err != nil {
					return apiErr(err)
				}
				out = append(out, e)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
			}
			for _, e := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", e.Path, e.Description)
			}
			return nil
		},
	}
	return cmd
}

func newTagAddCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "add [path] [tag]",
		Short:       "Add a tag to a note's frontmatter (not inline #tag).",
		Example:     "  obsidian-pp-cli tag add 'People/Jeff Smith.md' compounding-teams\n  obsidian-pp-cli tag add 'Projects/CM.md' priority",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			if cliutil.IsVerifyEnv() {
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}
			v, _, err := openVaultOnly()
			if err != nil {
				return err
			}
			abs, err := v.ResolveAbs(args[0])
			if err != nil {
				return notFoundErr(err)
			}
			n, err := v.LoadNote(abs)
			if err != nil {
				return apiErr(err)
			}
			if !n.HasFM {
				return usageErr(fmt.Errorf("note has no frontmatter — can't safely append tag; create the note via `note new` first"))
			}
			tag := strings.TrimPrefix(args[1], "#")
			for _, t := range n.Frontmatter.Tags {
				if t == tag {
					if flags.asJSON {
						return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]string{"path": n.Path, "tag": tag, "status": "already-present"})
					}
					fmt.Fprintf(cmd.OutOrStdout(), "already present: %s on %s\n", tag, n.Path)
					return nil
				}
			}
			n.Frontmatter.Tags = append(n.Frontmatter.Tags, tag)
			data, err := vault.AssembleNote(n.Frontmatter, []byte(n.Body))
			if err != nil {
				return apiErr(err)
			}
			if err := vault.AtomicWrite(abs, data, 0o644); err != nil {
				return apiErr(err)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]string{"path": n.Path, "tag": tag, "status": "added"})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added: %s on %s\n", tag, n.Path)
			return nil
		},
	}
	return cmd
}

func newTagRmCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "rm [path] [tag]",
		Short:       "Remove a tag from a note's frontmatter list.",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			if cliutil.IsVerifyEnv() {
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}
			v, _, err := openVaultOnly()
			if err != nil {
				return err
			}
			abs, err := v.ResolveAbs(args[0])
			if err != nil {
				return notFoundErr(err)
			}
			n, err := v.LoadNote(abs)
			if err != nil {
				return apiErr(err)
			}
			if !n.HasFM {
				return notFoundErr(fmt.Errorf("note has no frontmatter"))
			}
			tag := strings.TrimPrefix(args[1], "#")
			var kept []string
			removed := false
			for _, t := range n.Frontmatter.Tags {
				if t == tag {
					removed = true
					continue
				}
				kept = append(kept, t)
			}
			if !removed {
				if flags.asJSON {
					return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]string{"path": n.Path, "tag": tag, "status": "not-present"})
				}
				fmt.Fprintf(cmd.OutOrStdout(), "not present: %s on %s\n", tag, n.Path)
				return nil
			}
			n.Frontmatter.Tags = kept
			data, err := vault.AssembleNote(n.Frontmatter, []byte(n.Body))
			if err != nil {
				return apiErr(err)
			}
			if err := vault.AtomicWrite(abs, data, 0o644); err != nil {
				return apiErr(err)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]string{"path": n.Path, "tag": tag, "status": "removed"})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed: %s from %s\n", tag, n.Path)
			return nil
		},
	}
	return cmd
}
