// pp:data-source local

package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile"
	"github.com/spf13/cobra"
)

type tbProfileRow struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	IsDefault bool   `json:"is_default"`
	Exists    bool   `json:"exists"`
	Selected  bool   `json:"selected"`
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newTBProfilesCmd(flags))
	})
}

func newTBProfilesCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "profiles",
		Short: "List Thunderbird profiles from profiles.ini",
		Long: `List every Thunderbird profile declared in profiles.ini with its path, whether
it is the default, whether its directory exists, and which one this CLI would
read (selected). Select another with --profile <dir|name> or THUNDERBIRD_PROFILE.`,
		Example: strings.Trim(`
  thunderbird-pp-cli profiles
  thunderbird-pp-cli profiles --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "profiles")
			}
			root := tbprofile.RootDir()
			entries, err := tbprofile.ListProfiles(root)
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no profiles.ini found under %s\n", root)
			}
			selected, _ := resolveTBProfile(flags)
			rows := make([]tbProfileRow, 0, len(entries))
			for _, e := range entries {
				rows = append(rows, tbProfileRow{
					Name: e.Name, Path: e.Path, IsDefault: e.IsDefault, Exists: e.Exists,
					Selected: selected != "" && strings.EqualFold(filepath.Clean(selected), filepath.Clean(e.Path)),
				})
			}
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			tw := newTabWriter(tbHumanOut(cmd))
			fmt.Fprintln(tw, "NAME\tDEFAULT\tEXISTS\tSELECTED\tPATH")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%v\t%v\t%v\t%s\n", r.Name, r.IsDefault, r.Exists, r.Selected, r.Path)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum profiles to show (0 = all)")
	return cmd
}
