package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/cliutil"
)

func newLinksCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "links",
		Short: "Query the vault's link graph (backlinks, outgoing, broken).",
	}
	cmd.AddCommand(newLinksToCmd(flags))
	cmd.AddCommand(newLinksFromCmd(flags))
	cmd.AddCommand(newLinksBrokenCmd(flags))
	return cmd
}

type linkRow struct {
	From   string `json:"from,omitempty"`
	To     string `json:"to,omitempty"`
	Target string `json:"target,omitempty"`
}

func newLinksToCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "to [path-or-stem]",
		Short:       "List notes linking to a target (backlinks).",
		Example:     "  obsidian-pp-cli links to 'People/Jeff Smith.md'",
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
			rows, err := vc.S.DB().QueryContext(cmd.Context(),
				`SELECT from_path, to_target FROM links WHERE resolved_path = ? OR to_target = ? ORDER BY from_path`,
				args[0], args[0])
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			var out []linkRow
			for rows.Next() {
				var r linkRow
				if err := rows.Scan(&r.From, &r.Target); err != nil {
					return apiErr(err)
				}
				out = append(out, r)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
			}
			for _, r := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%s -> [[%s]]\n", r.From, r.Target)
			}
			return nil
		},
	}
	return cmd
}

func newLinksFromCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "from [path]",
		Short:       "List wikilinks emanating from a note.",
		Example:     "  obsidian-pp-cli links from 'People/Jeff Smith.md'",
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
			rows, err := vc.S.DB().QueryContext(cmd.Context(),
				`SELECT to_target, COALESCE(resolved_path, '') FROM links WHERE from_path = ? ORDER BY to_target`,
				args[0])
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			type row struct {
				Target       string `json:"target"`
				ResolvedPath string `json:"resolved_path,omitempty"`
			}
			var out []row
			for rows.Next() {
				var r row
				if err := rows.Scan(&r.Target, &r.ResolvedPath); err != nil {
					return apiErr(err)
				}
				out = append(out, r)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
			}
			for _, r := range out {
				if r.ResolvedPath != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "[[%s]] -> %s\n", r.Target, r.ResolvedPath)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "[[%s]] (unresolved)\n", r.Target)
				}
			}
			return nil
		},
	}
	return cmd
}

func newLinksBrokenCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "broken",
		Short:       "List [[wikilinks]] that don't resolve to a note in the vault.",
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
			rows, err := vc.S.DB().QueryContext(cmd.Context(),
				`SELECT from_path, to_target FROM links WHERE resolved_path IS NULL OR resolved_path = '' ORDER BY from_path`)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			var out []linkRow
			for rows.Next() {
				var r linkRow
				if err := rows.Scan(&r.From, &r.Target); err != nil {
					return apiErr(err)
				}
				out = append(out, r)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
			}
			for _, r := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%s -> [[%s]] (broken)\n", r.From, r.Target)
			}
			return nil
		},
	}
	return cmd
}

func newOrphansCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "orphans",
		Short:       "List notes with zero incoming wikilinks.",
		Example:     "  obsidian-pp-cli orphans\n  obsidian-pp-cli orphans --json --select path",
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
			rows, err := vc.S.DB().QueryContext(cmd.Context(),
				`SELECT n.path, COALESCE(n.description, '')
				 FROM notes n LEFT JOIN links l ON l.resolved_path = n.path
				 WHERE l.from_path IS NULL ORDER BY n.path`)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			type row struct {
				Path        string `json:"path"`
				Description string `json:"description,omitempty"`
			}
			var out []row
			for rows.Next() {
				var r row
				if err := rows.Scan(&r.Path, &r.Description); err != nil {
					return apiErr(err)
				}
				out = append(out, r)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
			}
			for _, r := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", r.Path, r.Description)
			}
			return nil
		},
	}
	return cmd
}

func newDupesCmd(flags *rootFlags) *cobra.Command {
	var byTitle bool
	cmd := &cobra.Command{
		Use:         "dupes",
		Short:       "Find duplicate notes (same body hash, or with --by title, same filename).",
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
			var q string
			if byTitle {
				q = `SELECT path FROM notes
					 WHERE LOWER(SUBSTR(path, INSTR(path, '/') + 1)) IN (
						SELECT LOWER(SUBSTR(path, INSTR(path, '/') + 1))
						FROM notes GROUP BY LOWER(SUBSTR(path, INSTR(path, '/') + 1))
						HAVING COUNT(*) > 1
					 ) ORDER BY path`
			} else {
				q = `SELECT path FROM notes WHERE body_hash IN (
					SELECT body_hash FROM notes WHERE body_hash != '' GROUP BY body_hash HAVING COUNT(*) > 1
				) ORDER BY body_hash, path`
			}
			rows, err := vc.S.DB().QueryContext(cmd.Context(), q)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			var out []string
			for rows.Next() {
				var p string
				if err := rows.Scan(&p); err != nil {
					return apiErr(err)
				}
				out = append(out, p)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
			}
			for _, p := range out {
				fmt.Fprintln(cmd.OutOrStdout(), p)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&byTitle, "by", false, "Match by title (filename stem) instead of body hash")
	return cmd
}
