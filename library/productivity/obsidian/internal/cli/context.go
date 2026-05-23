package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/cliutil"
)

// newContextListCmd is the user-facing `disclose` *domain* command for
// token-efficient progressive disclosure. Named `disclose` to avoid colliding
// with the generator's framework `context` command.
func newContextListCmd(flags *rootFlags) *cobra.Command {
	var layer string
	var limit int
	cmd := &cobra.Command{
		Use:   "disclose [folder]",
		Short: "Token-efficient progressive disclosure: list notes in a folder with path + description.",
		Long: "Layer-2 listing: returns path -> description pairs for every note in\n" +
			"the folder (or the whole vault if no folder is given). Designed for\n" +
			"agent reads — under 100 bytes per note, no body content.",
		Example:     "  obsidian-pp-cli disclose People/ --json\n  obsidian-pp-cli disclose Meetings/ --layer description --limit 30",
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
			folder := ""
			if len(args) > 0 {
				folder = strings.TrimRight(args[0], "/") + "/"
			}
			q := `SELECT path, COALESCE(description, '') FROM notes`
			var qargs []interface{}
			if folder != "" {
				q += ` WHERE path LIKE ?`
				qargs = append(qargs, folder+"%")
			}
			q += ` ORDER BY path`
			if limit > 0 {
				q += ` LIMIT ?`
				qargs = append(qargs, limit)
			}
			rows, err := vc.S.DB().QueryContext(cmd.Context(), q, qargs...)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			type entry struct {
				Path        string `json:"path"`
				Description string `json:"description,omitempty"`
			}
			var out []entry
			for rows.Next() {
				var e entry
				if err := rows.Scan(&e.Path, &e.Description); err != nil {
					return apiErr(err)
				}
				out = append(out, e)
			}
			_ = layer // currently only one layer mode; reserved for future expansion
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
			}
			for _, e := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", e.Path, e.Description)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&layer, "layer", "description", "Disclosure layer (description only for now)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max entries to return (0 = no limit)")
	return cmd
}
