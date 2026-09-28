package cli

// pp:data-source auto

import (
	"github.com/spf13/cobra"
	"strings"
)

func newPlacesShowCmd(flags *rootFlags, factory tenkiFactory) *cobra.Command {
	var place string
	var read tenkiReadFlags
	cmd := &cobra.Command{Use: "show", Short: "Show destination identity separately from its forecast municipality", Example: strings.Trim(`
  tenki-pp-cli places show --place https://tenki.jp/forecast/3/16/4410/13101/ --agent
`, "\n"), Annotations: tenkiAnnotations("--place=" + tenkiTokyo)}
	cmd.Flags().StringVar(&place, "place", "", "Canonical tenki.jp place URL (required)")
	read.attach(cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if done, err := tenkiPrelude(cmd, args, flags); done {
			return err
		}
		if err := validateTenkiPlace(place); err != nil {
			return err
		}
		client, err := read.source(flags, factory)
		if err != nil {
			return err
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		result, err := client.Resolve(ctx, place)
		if err != nil {
			return tenkiProductError(err)
		}
		tenkiWarnings(cmd, result.Warnings)
		return emitTenki(cmd, flags, client, result, result.Source)
	}
	return cmd
}
