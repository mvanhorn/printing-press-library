// pp:data-source local
package cli

import (
	"context"
	"github.com/mvanhorn/printing-press-library/library/travel/jma/internal/jma"
	"github.com/spf13/cobra"
)

func newJMAAreaResolveCmd(run jmaRunner) *cobra.Command {
	var area string
	cmd := &cobra.Command{Use: "resolve", Short: "Resolve an exact JMA area name or ID and reject ambiguity", Example: "  jma-pp-cli areas resolve --area 1310100", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"}, RunE: run(func(ctx context.Context, c *jma.Client) (jma.Envelope, error) { return c.ResolveArea(ctx, area) })}
	cmd.Flags().StringVar(&area, "area", "", "Exact source name or canonical area identifier")
	return cmd
}
