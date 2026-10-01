// pp:data-source live
package cli

import (
	"context"
	"github.com/mvanhorn/printing-press-library/library/travel/jma/internal/jma"
	"github.com/spf13/cobra"
)

func newJMAWarningsGetCmd(run jmaRunner, detail *bool) *cobra.Command {
	var area string
	var offset, limit int
	cmd := &cobra.Command{Use: "get", Short: "Read municipality warning lifecycle; report incomplete coverage", Example: "  jma-pp-cli warnings get --area 1310100 --detail", Annotations: map[string]string{"pp:endpoint": "warnings.get", "mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--area=1310100"}, RunE: run(func(ctx context.Context, c *jma.Client) (jma.Envelope, error) {
		return c.Warnings(ctx, area, *detail, offset, limit)
	})}
	cmd.Flags().StringVar(&area, "area", "", "JMA source area ID or exact Japanese/English name")
	cmd.Flags().IntVar(&offset, "offset", 0, "Municipality result offset; summary covers all resolved municipalities")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum returned municipalities, between 1 and 100")
	return cmd
}
