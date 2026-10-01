// pp:data-source local
package cli

import (
	"context"
	"github.com/mvanhorn/printing-press-library/library/travel/jma/internal/jma"
	"github.com/spf13/cobra"
)

func newJMAAreaSearchCmd(run jmaRunner) *cobra.Command {
	var query, kind string
	var offset, limit int
	cmd := &cobra.Command{Use: "search", Short: "Search JMA area names or source IDs by query and kind; return bounded inventory records", Example: "  jma-pp-cli areas search --query Kyoto --kind municipality", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"}, RunE: run(func(ctx context.Context, c *jma.Client) (jma.Envelope, error) {
		return c.AreaSearch(ctx, query, kind, offset, limit)
	})}
	cmd.Flags().StringVar(&query, "query", "", "Source name or identifier substring to search")
	cmd.Flags().StringVar(&kind, "kind", "all", "Area kind: all, office, district, subdivision, municipality")
	cmd.Flags().IntVar(&offset, "offset", 0, "Local result offset, between zero and 100000")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum returned source records, between 1 and 100")
	return cmd
}
