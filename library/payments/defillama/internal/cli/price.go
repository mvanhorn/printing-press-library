package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/defillama/internal/client"
	"github.com/mvanhorn/printing-press-library/library/payments/defillama/internal/format"
)

func newPriceCmd() *cobra.Command {
	var at string
	cmd := &cobra.Command{
		Use:   "price <coins>",
		Short: "Live token price lookup (pass-through to api.llama.fi)",
		Long:  "Coins are comma-separated, each `chain:address` or `coingecko:id` (e.g. ethereum:0xdac17f958d2ee523a2206206994597c13d831ec7,coingecko:bitcoin)",
		Args:  cobra.ExactArgs(1),
		RunE: withCtx(func(cmd *cobra.Command, args []string, cx *Ctx) error {
			coins := url.PathEscape(args[0])
			path := "/prices/current/" + coins
			if at != "" {
				path = "/prices/historical/" + at + "/" + coins
			}
			body, err := cx.C.Get(context.Background(), client.HostCoins, path, nil)
			if err != nil {
				return err
			}
			defer body.Close()
			var resp struct {
				Coins map[string]struct {
					Decimals   int     `json:"decimals"`
					Symbol     string  `json:"symbol"`
					Price      float64 `json:"price"`
					Timestamp  int64   `json:"timestamp"`
					Confidence float64 `json:"confidence"`
				} `json:"coins"`
			}
			if err := json.NewDecoder(body).Decode(&resp); err != nil {
				return err
			}
			if len(resp.Coins) == 0 {
				return fmt.Errorf("no prices returned for %q", args[0])
			}
			rec := format.NewRecorder([]string{"COIN", "SYMBOL", "PRICE", "DECIMALS", "CONFIDENCE"})
			for id, c := range resp.Coins {
				rec.Append([]format.Cell{
					format.Str(id), format.Str(c.Symbol),
					format.USD(c.Price), format.Int(int64(c.Decimals)),
					format.Raw(c.Confidence),
				})
			}
			return format.RenderRec(os.Stdout, mode(), rec, format.Options{NoHeader: G.NoHeader, RightAlign: rightSet([]int{2, 3, 4})})
		}),
	}
	cmd.Flags().StringVar(&at, "at", "", "historical timestamp (unix seconds)")
	return cmd
}
