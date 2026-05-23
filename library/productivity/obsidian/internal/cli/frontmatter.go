package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/vault"
)

func newFrontmatterCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "frontmatter",
		Short: "Read, update, and delete frontmatter fields on a note.",
	}
	cmd.AddCommand(newFrontmatterGetCmd(flags))
	cmd.AddCommand(newFrontmatterSetCmd(flags))
	cmd.AddCommand(newFrontmatterDelCmd(flags))
	return cmd
}

func newFrontmatterGetCmd(flags *rootFlags) *cobra.Command {
	var key string
	cmd := &cobra.Command{
		Use:         "get [path]",
		Short:       "Print the frontmatter (or one field) of a note.",
		Example:     "  obsidian-pp-cli frontmatter get 'People/Jeff Smith.md' --key role",
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
				return notFoundErr(fmt.Errorf("note has no frontmatter: %s", args[0]))
			}
			if key != "" {
				val := frontmatterFieldValue(n.Frontmatter, key)
				if flags.asJSON {
					return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{key: val})
				}
				fmt.Fprintln(cmd.OutOrStdout(), val)
				return nil
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(n.Frontmatter)
			}
			out, _ := n.Frontmatter.Encode()
			fmt.Fprint(cmd.OutOrStdout(), string(out))
			return nil
		},
	}
	cmd.Flags().StringVar(&key, "key", "", "Print only this field's value")
	return cmd
}

func newFrontmatterSetCmd(flags *rootFlags) *cobra.Command {
	var key, value string
	var force bool
	cmd := &cobra.Command{
		Use:         "set [path]",
		Short:       "Set a frontmatter field value (validated against the protocol).",
		Example:     "  obsidian-pp-cli frontmatter set 'People/Jeff Smith.md' --key status --value paused",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || key == "" {
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
				n.Frontmatter = vault.Frontmatter{Extra: map[string]interface{}{}}
				n.HasFM = true
			}
			applyFieldSet(&n.Frontmatter, key, value)
			findings := vault.Validate(n.Path, n.Frontmatter, true)
			if hasErrors(findings) && !force {
				return usageErr(fmt.Errorf("validation failed after setting %s=%s:\n%s", key, value, formatFindings(findings)))
			}
			data, err := vault.AssembleNote(n.Frontmatter, []byte(n.Body))
			if err != nil {
				return apiErr(err)
			}
			if err := vault.AtomicWrite(abs, data, 0o644); err != nil {
				return apiErr(err)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]string{"path": n.Path, "key": key, "value": value, "status": "updated"})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "updated %s: %s=%s\n", n.Path, key, value)
			return nil
		},
	}
	cmd.Flags().StringVar(&key, "key", "", "Field name (required)")
	_ = cmd.MarkFlagRequired("key")
	cmd.Flags().StringVar(&value, "value", "", "Field value")
	cmd.Flags().BoolVar(&force, "force", false, "Write even if protocol validation fails")
	return cmd
}

func newFrontmatterDelCmd(flags *rootFlags) *cobra.Command {
	var key string
	var force bool
	cmd := &cobra.Command{
		Use:         "del [path]",
		Short:       "Delete a frontmatter field (refuses required fields without --force).",
		Example:     "  obsidian-pp-cli frontmatter del 'People/Jeff Smith.md' --key superseded_by",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || key == "" {
				return cmd.Help()
			}
			if cliutil.IsVerifyEnv() {
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}
			required := map[string]bool{"type": true, "date": true, "description": true, "status": true}
			if required[key] && !force {
				return usageErr(fmt.Errorf("%q is a protocol-required field; pass --force to delete anyway", key))
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
				return notFoundErr(fmt.Errorf("note has no frontmatter: %s", args[0]))
			}
			applyFieldDel(&n.Frontmatter, key)
			data, err := vault.AssembleNote(n.Frontmatter, []byte(n.Body))
			if err != nil {
				return apiErr(err)
			}
			if err := vault.AtomicWrite(abs, data, 0o644); err != nil {
				return apiErr(err)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]string{"path": n.Path, "key": key, "status": "deleted"})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted %s.%s\n", n.Path, key)
			return nil
		},
	}
	cmd.Flags().StringVar(&key, "key", "", "Field name to delete")
	cmd.Flags().BoolVar(&force, "force", false, "Delete required fields too (type/date/description/status)")
	return cmd
}

func frontmatterFieldValue(fm vault.Frontmatter, key string) interface{} {
	switch strings.ToLower(key) {
	case "type":
		return fm.Type
	case "date":
		return fm.Date
	case "description":
		return fm.Description
	case "status":
		return fm.Status
	case "superseded_by":
		return fm.SupersededBy
	case "facts_file":
		return fm.FactsFile
	case "tags":
		return fm.Tags
	case "facts":
		return fm.Facts
	}
	return fm.Extra[key]
}

func applyFieldSet(fm *vault.Frontmatter, key, value string) {
	switch strings.ToLower(key) {
	case "type":
		fm.Type = value
	case "date":
		fm.Date = value
	case "description":
		fm.Description = value
	case "status":
		fm.Status = value
	case "superseded_by":
		fm.SupersededBy = value
	case "facts_file":
		fm.FactsFile = value
	default:
		if fm.Extra == nil {
			fm.Extra = map[string]interface{}{}
		}
		fm.Extra[key] = value
	}
}

func applyFieldDel(fm *vault.Frontmatter, key string) {
	switch strings.ToLower(key) {
	case "type":
		fm.Type = ""
	case "date":
		fm.Date = ""
	case "description":
		fm.Description = ""
	case "status":
		fm.Status = ""
	case "superseded_by":
		fm.SupersededBy = ""
	case "facts_file":
		fm.FactsFile = ""
	case "tags":
		fm.Tags = nil
	default:
		delete(fm.Extra, key)
	}
}
