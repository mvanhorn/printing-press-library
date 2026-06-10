package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/worten/internal/worten"
)

func newWortenResolveCmd(flags *rootFlags) *cobra.Command {
	var raw bool
	cmd := &cobra.Command{
		Use:   "resolve <product-url-or-id>",
		Short: "Resolve a Worten product URL or slug to a canonical product identifier",
		Example: strings.Join([]string{
			"  worten-pp-cli resolve 11111111-1111-1111-1111-111111111111",
			"  worten-pp-cli resolve https://www.worten.pt/product/example --json",
		}, "\n"),
		Args: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return nil
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("resolve requires a product URL or product UUID"))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, map[string]any{"dry_run": true, "command": "resolve"})
			}
			svc, err := newWortenService(flags)
			if err != nil {
				return err
			}
			result, err := svc.Resolve(cmd.Context(), args[0], raw)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return flags.printJSON(cmd, result)
		},
	}
	cmd.Flags().BoolVar(&raw, "raw", false, "Return raw cache/update details")
	return cmd
}

func newWortenProductCmd(flags *rootFlags) *cobra.Command {
	var raw bool
	cmd := &cobra.Command{
		Use:   "product <product-url-or-id>",
		Short: "Fetch and normalize a Worten product",
		Example: strings.Join([]string{
			"  worten-pp-cli product 11111111-1111-1111-1111-111111111111",
			"  worten-pp-cli product 11111111-1111-1111-1111-111111111111 --raw --json",
		}, "\n"),
		Args: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return nil
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("product requires a product URL or product UUID"))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, map[string]any{"dry_run": true, "command": "product"})
			}
			svc, err := newWortenService(flags)
			if err != nil {
				return err
			}
			result, err := svc.Product(cmd.Context(), args[0], raw)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return flags.printJSON(cmd, result)
		},
	}
	cmd.Flags().BoolVar(&raw, "raw", false, "Return the raw product details payload")
	return cmd
}

func newWortenBuyerCmd(flags *rootFlags) *cobra.Command {
	var raw bool
	cmd := &cobra.Command{
		Use:   "buyer <product-url-or-id>",
		Short: "Fetch and normalize the buyer view for a Worten product",
		Example: strings.Join([]string{
			"  worten-pp-cli buyer 11111111-1111-1111-1111-111111111111",
			"  worten-pp-cli buyer 11111111-1111-1111-1111-111111111111 --raw --json",
		}, "\n"),
		Args: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return nil
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("buyer requires a product URL or product UUID"))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, map[string]any{"dry_run": true, "command": "buyer"})
			}
			svc, err := newWortenService(flags)
			if err != nil {
				return err
			}
			result, err := svc.Buyer(cmd.Context(), args[0], raw)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return flags.printJSON(cmd, result)
		},
	}
	cmd.Flags().BoolVar(&raw, "raw", false, "Return the raw details/specifications payloads")
	return cmd
}

func newWortenSpecsCmd(flags *rootFlags) *cobra.Command {
	var raw bool
	cmd := &cobra.Command{
		Use:   "specs <product-url-or-id>",
		Short: "Fetch Worten product technical specifications",
		Example: strings.Join([]string{
			"  worten-pp-cli specs 11111111-1111-1111-1111-111111111111",
			"  worten-pp-cli specs 11111111-1111-1111-1111-111111111111 --raw --json",
		}, "\n"),
		Args: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return nil
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("specs requires a product URL or product UUID"))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, map[string]any{"dry_run": true, "command": "specs"})
			}
			svc, err := newWortenService(flags)
			if err != nil {
				return err
			}
			result, err := svc.Specs(cmd.Context(), args[0], raw)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return flags.printJSON(cmd, result)
		},
	}
	cmd.Flags().BoolVar(&raw, "raw", false, "Return the raw technical specifications payload")
	return cmd
}

func newWortenStockCmd(flags *rootFlags) *cobra.Command {
	var raw bool
	var postalCode string
	var radius int
	cmd := &cobra.Command{
		Use:   "stock <product-url-or-id>",
		Short: "Fetch normalized Worten stock context for a product",
		Example: strings.Join([]string{
			"  worten-pp-cli stock 11111111-1111-1111-1111-111111111111",
			"  worten-pp-cli stock 11111111-1111-1111-1111-111111111111 --postal-code 1000-001 --radius 25 --json",
		}, "\n"),
		Args: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return nil
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("stock requires a product URL or product UUID"))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, map[string]any{"dry_run": true, "command": "stock"})
			}
			svc, err := newWortenService(flags)
			if err != nil {
				return err
			}
			result, err := svc.Stock(cmd.Context(), args[0], worten.StockOptions{
				PostalCode: postalCode,
				RadiusKm:   radius,
			}, raw)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return flags.printJSON(cmd, result)
		},
	}
	cmd.Flags().BoolVar(&raw, "raw", false, "Return the raw product/store-search payloads")
	cmd.Flags().StringVar(&postalCode, "postal-code", "", "Postal code for nearby-store lookup")
	cmd.Flags().IntVar(&radius, "radius", 20, "Nearby-store search radius in km")
	return cmd
}

func newWortenSuggestCmd(flags *rootFlags) *cobra.Command {
	var raw bool
	var max int
	cmd := &cobra.Command{
		Use:   "suggest <query>",
		Short: "Fetch Worten search suggestions",
		Example: strings.Join([]string{
			"  worten-pp-cli suggest dishwasher",
			"  worten-pp-cli suggest dishwasher --max 10 --json",
		}, "\n"),
		Args: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return nil
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("suggest requires a query"))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, map[string]any{"dry_run": true, "command": "suggest"})
			}
			svc, err := newWortenService(flags)
			if err != nil {
				return err
			}
			result, err := svc.Suggest(cmd.Context(), joinArgs(args), max, raw)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return flags.printJSON(cmd, result)
		},
	}
	cmd.Flags().BoolVar(&raw, "raw", false, "Return the raw suggestion payload")
	cmd.Flags().IntVar(&max, "max", 5, "Maximum number of suggestions")
	return cmd
}

func newWortenSearchCmd(flags *rootFlags) *cobra.Command {
	var raw bool
	var contexts []string
	var page int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search Worten products with explicit context filters",
		Example: strings.Join([]string{
			"  worten-pp-cli search dishwasher --context appliances",
			"  worten-pp-cli search dishwasher --context appliances --page 2 --json",
		}, "\n"),
		Args: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return nil
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("search requires a query"))
			}
			if len(contexts) == 0 {
				return usageErr(fmt.Errorf("search requires at least one --context value"))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, map[string]any{"dry_run": true, "command": "search"})
			}
			svc, err := newWortenService(flags)
			if err != nil {
				return err
			}
			result, err := svc.Search(cmd.Context(), joinArgs(args), contexts, page, raw)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return flags.printJSON(cmd, result)
		},
	}
	cmd.Flags().BoolVar(&raw, "raw", false, "Return the raw search payload")
	cmd.Flags().StringArrayVar(&contexts, "context", nil, "Search context value; repeat the flag for multiple contexts")
	cmd.Flags().IntVar(&page, "page", 1, "Result page number")
	return cmd
}

func newWortenSnapshotCmd(flags *rootFlags) *cobra.Command {
	var raw bool
	var refresh bool
	var cacheOnly bool
	cmd := &cobra.Command{
		Use:   "snapshot <product-url-or-id>",
		Short: "Capture or read a normalized Worten snapshot",
		Example: strings.Join([]string{
			"  worten-pp-cli snapshot 11111111-1111-1111-1111-111111111111",
			"  worten-pp-cli snapshot 11111111-1111-1111-1111-111111111111 --refresh --json",
		}, "\n"),
		Args: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return nil
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("snapshot requires a product URL or product UUID"))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, map[string]any{"dry_run": true, "command": "snapshot"})
			}
			svc, err := newWortenService(flags)
			if err != nil {
				return err
			}
			result, err := svc.Snapshot(cmd.Context(), args[0], worten.SnapshotOptions{
				Refresh:   refresh,
				CacheOnly: cacheOnly,
			}, raw)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return flags.printJSON(cmd, result)
		},
	}
	cmd.Flags().BoolVar(&raw, "raw", false, "Return the full snapshot payload")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Force a live refresh before returning the snapshot")
	cmd.Flags().BoolVar(&cacheOnly, "cache-only", false, "Return only cached snapshots; do not hit the network")
	return cmd
}

func joinArgs(args []string) string {
	return strings.TrimSpace(strings.Join(args, " "))
}

func newWortenService(flags *rootFlags) (*worten.Service, error) {
	client, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	return worten.New(client)
}