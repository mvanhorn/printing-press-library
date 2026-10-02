// pp:data-source live
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/toyota-rentacar/internal/toyota"
	"github.com/spf13/cobra"
)

func newNovelShopsSearchCmd(flags *rootFlags) *cobra.Command {
	var keyword string
	var limit int
	cmd := &cobra.Command{Use: "search", Short: "Resolve nearby Toyota shops with stable IDs and Japanese names",
		Long:        "Search Toyota's public location keyword service. It returns at most ten nearby shops; --limit bounds output. Operating hours are distinct from dated class availability.",
		Example:     "  toyota-rentacar-pp-cli shops search --keyword \"Kyoto Station\" --limit 3 --agent --select shops.id,shops.name,shops.name_jp,shops.oneway_returns",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--keyword=Kyoto Station;--limit=3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if stop, err := toyotaGuard(cmd, args, flags, "shops search"); stop {
				return err
			}
			if keyword == "" {
				return usageErr(fmt.Errorf("--keyword is required; e.g. --keyword \"Kyoto Station\""))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			out, err := toyota.NewClient(flags.rateLimit).SearchShops(ctx, keyword, limit)
			if err != nil {
				return toyotaError(err)
			}
			return toyotaPrint(cmd, flags, out)
		}}
	cmd.Flags().StringVar(&keyword, "keyword", "", "Place,station or airport name (1–80 characters)")
	cmd.Flags().IntVar(&limit, "limit", 5, "Maximum shops returned,1–10; source search is capped at ten")
	toyotaRequireFlags(cmd, flags, "keyword")
	return cmd
}
