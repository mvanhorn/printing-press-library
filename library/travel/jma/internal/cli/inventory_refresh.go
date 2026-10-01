// pp:data-source live
package cli

import (
	"context"
	"github.com/mvanhorn/printing-press-library/library/travel/jma/internal/jma"
	"github.com/spf13/cobra"
)

func newJMAInventoryRefreshCmd(run jmaRunner) *cobra.Command {
	return &cobra.Command{Use: "refresh", Short: "Validate eight first-party source catalogs and atomically replace local inventory", Example: "  jma-pp-cli inventory refresh", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"}, RunE: run(func(ctx context.Context, c *jma.Client) (jma.Envelope, error) { return c.RefreshInventory(ctx) })}
}
