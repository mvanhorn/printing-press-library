// Copyright 2026 cathryn-lavery. Licensed under Apache-2.0. See LICENSE.
// Cobra command layout follows Printing Press CLI ergonomics while keeping unsafe operations deferred.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/tiktok-shop/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/tiktok-shop/internal/config"
	"github.com/spf13/cobra"
)

var version = "0.1.0"
var noColor bool

type rootFlags struct {
	asJSON       bool
	compact      bool
	quiet        bool
	dryRun       bool
	noInput      bool
	yes          bool
	agent        bool
	selectFields string
	configPath   string
	timeout      time.Duration
}

type cliError struct {
	code int
	err  error
}

func (e *cliError) Error() string { return e.err.Error() }
func (e *cliError) Unwrap() error { return e.err }

func usageErr(err error) error  { return &cliError{code: 2, err: err} }
func configErr(err error) error { return &cliError{code: 10, err: err} }
func authErr(err error) error   { return &cliError{code: 4, err: err} }
func apiErr(err error) error    { return &cliError{code: 5, err: err} }
func rateErr(err error) error   { return &cliError{code: 7, err: err} }

func RootCmd() *cobra.Command {
	var flags rootFlags
	return newRootCmd(&flags)
}

func Execute() error {
	var flags rootFlags
	return newRootCmd(&flags).Execute()
}

func newRootCmd(flags *rootFlags) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "tiktok-shop-pp-cli",
		Short: "Operate confirmed TikTok Shop seller APIs from Printing Press",
		Long: `Operate confirmed TikTok Shop seller APIs from Printing Press.

This v1 only encodes endpoints verified from official TikTok Shop Partner Center
docs. Read commands return raw upstream JSON. Mutations remain disabled unless
idempotency and retry safety are explicit.`,
		SilenceUsage: true,
		Version:      version,
	}
	rootCmd.SetVersionTemplate("tiktok-shop-pp-cli {{ .Version }}\n")

	rootCmd.PersistentFlags().BoolVar(&flags.asJSON, "json", false, "Output as JSON")
	rootCmd.PersistentFlags().BoolVar(&flags.compact, "compact", false, "Return only key fields for minimal token usage where supported")
	rootCmd.PersistentFlags().BoolVar(&flags.quiet, "quiet", false, "Bare output, one value per line")
	rootCmd.PersistentFlags().StringVar(&flags.configPath, "config", "", "Config file path")
	rootCmd.PersistentFlags().DurationVar(&flags.timeout, "timeout", 30*time.Second, "Request timeout")
	rootCmd.PersistentFlags().BoolVar(&flags.dryRun, "dry-run", false, "Show request intent without sending network requests")
	rootCmd.PersistentFlags().BoolVar(&flags.noInput, "no-input", false, "Disable all interactive prompts")
	rootCmd.PersistentFlags().StringVar(&flags.selectFields, "select", "", "Comma-separated fields to include in JSON output")
	rootCmd.PersistentFlags().BoolVar(&flags.yes, "yes", false, "Skip confirmation prompts")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().BoolVar(&flags.agent, "agent", false, "Set agent defaults (--json --compact --no-input --no-color --yes)")

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if flags.agent {
			if !cmd.Flags().Changed("json") {
				flags.asJSON = true
			}
			if !cmd.Flags().Changed("compact") {
				flags.compact = true
			}
			if !cmd.Flags().Changed("no-input") {
				flags.noInput = true
			}
			if !cmd.Flags().Changed("yes") {
				flags.yes = true
			}
			if !cmd.Flags().Changed("no-color") {
				noColor = true
			}
		}
		return nil
	}

	rootCmd.AddCommand(newDoctorCmd(flags))
	rootCmd.AddCommand(newAuthCmd(flags))
	rootCmd.AddCommand(newShopsCmd(flags))
	rootCmd.AddCommand(newOrdersCmd(flags))
	rootCmd.AddCommand(newProductsCmd(flags))
	rootCmd.AddCommand(newInventoryCmd(flags))
	rootCmd.AddCommand(newFulfillmentCmd(flags))
	rootCmd.AddCommand(newProfileCmd(flags))
	rootCmd.AddCommand(newWhichCmd(flags))
	rootCmd.AddCommand(newAgentContextCmd(rootCmd))
	return rootCmd
}

func (f *rootFlags) loadConfig() (*config.Config, error) {
	cfg, err := config.Load(f.configPath)
	if err != nil {
		return nil, configErr(err)
	}
	return cfg, nil
}

func (f *rootFlags) newClient() (*client.Client, error) {
	cfg, err := f.loadConfig()
	if err != nil {
		return nil, err
	}
	c := client.New(cfg, f.timeout)
	c.DryRun = f.dryRun
	return c, nil
}

func ExitCode(err error) int {
	var codeErr *cliError
	if errors.As(err, &codeErr) {
		return codeErr.code
	}
	return 1
}

func classifyErr(err error) error {
	if err == nil {
		return nil
	}
	var auth *client.AuthRequiredError
	if errors.As(err, &auth) {
		return authErr(err)
	}
	var rate *client.RateLimitError
	if errors.As(err, &rate) {
		return rateErr(err)
	}
	var upstream *client.APIError
	if errors.As(err, &upstream) {
		return apiErr(err)
	}
	return err
}

func printJSON(cmd *cobra.Command, v any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func printRawJSON(cmd *cobra.Command, raw json.RawMessage) error {
	if len(raw) == 0 {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "{}")
		return err
	}
	var pretty any
	if err := json.Unmarshal(raw, &pretty); err == nil {
		return printJSON(cmd, pretty)
	}
	_, err := fmt.Fprintln(cmd.OutOrStdout(), string(raw))
	return err
}

func printValue(cmd *cobra.Command, flags *rootFlags, v any) error {
	if flags.asJSON {
		return printJSON(cmd, v)
	}
	switch value := v.(type) {
	case string:
		_, err := fmt.Fprintln(cmd.OutOrStdout(), value)
		return err
	default:
		return printJSON(cmd, v)
	}
}

func pendingCommand(flags *rootFlags, use, short, operation, docURL string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPending(cmd, flags, operation, docURL)
		},
	}
}

func runPending(cmd *cobra.Command, flags *rootFlags, operation, docURL string) error {
	msg := fmt.Sprintf("%s not yet implemented; awaiting API confirmation from %s", operation, docURL)
	if flags.asJSON {
		return printJSON(cmd, map[string]any{
			"status":    "not_implemented",
			"operation": operation,
			"doc_url":   docURL,
			"message":   msg,
		})
	}
	_, err := fmt.Fprintln(cmd.OutOrStdout(), msg)
	return err
}

func newAuthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Inspect and refresh TikTok Shop auth material"}
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show configured auth material without revealing secrets",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := flags.loadConfig()
			if err != nil {
				return err
			}
			return printValue(cmd, flags, map[string]any{
				"app_credentials_configured": cfg.HasAppCredentials(),
				"token_bundle_configured":    cfg.HasTokenBundle(),
				"shop_selector_configured":   cfg.HasShopSelector(),
				"auth_source":                cfg.AuthSource,
				"access_token_expires_at":    zeroTimeOmit(cfg.TokenExpiry),
				"oauth_flow_status":          "confirmed for auth code exchange and refresh; authorization link is generated in Partner Center",
				"doc_url":                    client.AuthorizationOverviewURL,
			})
		},
	})
	cmd.AddCommand(newAuthExchangeCmd(flags))
	cmd.AddCommand(newAuthRefreshCmd(flags))
	return cmd
}

func newAuthExchangeCmd(flags *rootFlags) *cobra.Command {
	var authCode string
	var save bool
	cmd := &cobra.Command{
		Use:   "exchange --auth-code <code>",
		Short: "Exchange an official TikTok Shop auth code for tokens",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := c.GetAccessToken(cmd.Context(), authCode)
			if err != nil {
				return classifyErr(err)
			}
			return printTokenSummary(cmd, flags, c.Config, raw, save)
		},
	}
	cmd.Flags().StringVar(&authCode, "auth-code", "", "Authorization code from TikTok Shop redirect URL")
	cmd.Flags().BoolVar(&save, "save", false, "Persist returned token bundle to config file with 0600 permissions")
	return cmd
}

func newAuthRefreshCmd(flags *rootFlags) *cobra.Command {
	var save bool
	cmd := &cobra.Command{
		Use:   "refresh",
		Short: "Refresh an access token using the official token refresh endpoint",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := c.RefreshToken(cmd.Context())
			if err != nil {
				return classifyErr(err)
			}
			return printTokenSummary(cmd, flags, c.Config, raw, save)
		},
	}
	cmd.Flags().BoolVar(&save, "save", false, "Persist returned token bundle to config file with 0600 permissions")
	return cmd
}

func printTokenSummary(cmd *cobra.Command, flags *rootFlags, cfg *config.Config, raw json.RawMessage, save bool) error {
	if flags.dryRun {
		return printRawJSON(cmd, raw)
	}
	var resp struct {
		Code      int    `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
		Data      struct {
			AccessToken          string   `json:"access_token"`
			AccessTokenExpireIn  int64    `json:"access_token_expire_in"`
			RefreshToken         string   `json:"refresh_token"`
			RefreshTokenExpireIn int64    `json:"refresh_token_expire_in"`
			OpenID               string   `json:"open_id"`
			SellerName           string   `json:"seller_name"`
			SellerBaseRegion     string   `json:"seller_base_region"`
			UserType             int      `json:"user_type"`
			GrantedScopes        []string `json:"granted_scopes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return err
	}
	if save && resp.Data.AccessToken != "" && resp.Data.RefreshToken != "" {
		if err := cfg.SaveTokens(resp.Data.AccessToken, resp.Data.RefreshToken, time.Unix(resp.Data.AccessTokenExpireIn, 0)); err != nil {
			return configErr(err)
		}
	}
	return printJSON(cmd, map[string]any{
		"code":                     resp.Code,
		"message":                  resp.Message,
		"request_id":               resp.RequestID,
		"access_token_received":    resp.Data.AccessToken != "",
		"refresh_token_received":   resp.Data.RefreshToken != "",
		"access_token_expires_at":  unixTimeOmit(resp.Data.AccessTokenExpireIn),
		"refresh_token_expires_at": unixTimeOmit(resp.Data.RefreshTokenExpireIn),
		"open_id":                  resp.Data.OpenID,
		"seller_name":              resp.Data.SellerName,
		"seller_base_region":       resp.Data.SellerBaseRegion,
		"user_type":                resp.Data.UserType,
		"granted_scopes":           resp.Data.GrantedScopes,
		"saved_to_config":          save,
		"tokens_redacted":          true,
		"doc_url":                  client.AuthorizationOverviewURL,
	})
}

func newShopsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "shops", Short: "TikTok Shop account and shop discovery"}
	cmd.AddCommand(newShopsInfoCmd(flags))
	return cmd
}

func newShopsInfoCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "List shops authorized for this app and token",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := c.AuthorizedShops(cmd.Context())
			if err != nil {
				return classifyErr(err)
			}
			return printRawJSON(cmd, raw)
		},
	}
}

func newProductsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "products", Short: "TikTok Shop product/listing operations"}
	cmd.AddCommand(newProductsListCmd(flags))
	cmd.AddCommand(newProductsGetCmd(flags))
	return cmd
}

func newProductsListCmd(flags *rootFlags) *cobra.Command {
	var pageSize int
	var pageToken, status, categoryVersion string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Search products with the confirmed 202309 Product API",
		RunE: func(cmd *cobra.Command, args []string) error {
			if pageSize < 1 || pageSize > 100 {
				return usageErr(fmt.Errorf("--limit must be between 1 and 100"))
			}
			q := url.Values{}
			q.Set("page_size", strconv.Itoa(pageSize))
			setIf(q, "page_token", pageToken)
			setIf(q, "category_version", categoryVersion)
			body := map[string]any{}
			if status != "" {
				body["status"] = status
			}
			return runOpenAPI(cmd, flags, "POST", "/product/202309/products/search", q, body)
		},
	}
	cmd.Flags().IntVar(&pageSize, "limit", 50, "Products per page, official range 1-100")
	cmd.Flags().StringVar(&pageToken, "page-token", "", "Opaque next page token")
	cmd.Flags().StringVar(&status, "status", "", "Optional product status filter, e.g. ALL, ACTIVATE, DRAFT")
	cmd.Flags().StringVar(&categoryVersion, "category-version", "", "Optional category version, e.g. v1 or v2")
	return cmd
}

func newProductsGetCmd(flags *rootFlags) *cobra.Command {
	var locale string
	returnDraft := false
	returnReview := false
	cmd := &cobra.Command{
		Use:   "get <product-id>",
		Short: "Get product details with the confirmed 202309 Product API",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			q := url.Values{}
			setIf(q, "locale", locale)
			if returnDraft {
				q.Set("return_draft_version", "true")
			}
			if returnReview {
				q.Set("return_under_review_version", "true")
			}
			return runOpenAPI(cmd, flags, "GET", "/product/202309/products/"+url.PathEscape(args[0]), q, nil)
		},
	}
	cmd.Flags().StringVar(&locale, "locale", "", "Optional locale/language")
	cmd.Flags().BoolVar(&returnDraft, "return-draft-version", false, "Return draft product version when supported")
	cmd.Flags().BoolVar(&returnReview, "return-under-review-version", false, "Return under-review product version when supported")
	return cmd
}

func newInventoryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "inventory", Short: "TikTok Shop inventory operations"}
	cmd.AddCommand(newInventoryListCmd(flags))
	cmd.AddCommand(newInventoryGetCmd(flags))
	cmd.AddCommand(newInventoryUpdateCmd(flags))
	return cmd
}

func newInventoryListCmd(flags *rootFlags) *cobra.Command {
	var productIDs, skuIDs []string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Search inventory by product IDs or SKU IDs",
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			if len(productIDs) > 0 {
				body["product_ids"] = productIDs
			}
			if len(skuIDs) > 0 {
				body["sku_ids"] = skuIDs
			}
			if len(body) == 0 {
				return usageErr(fmt.Errorf("provide --product-id or --sku-id; the official endpoint searches by explicit IDs"))
			}
			return runOpenAPI(cmd, flags, "POST", "/product/202309/inventory/search", nil, body)
		},
	}
	cmd.Flags().StringArrayVar(&productIDs, "product-id", nil, "Product ID to include; repeatable, max 100 per official docs")
	cmd.Flags().StringArrayVar(&skuIDs, "sku-id", nil, "SKU ID to include; repeatable, max 600 per official docs")
	return cmd
}

func newInventoryGetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get <sku-id>",
		Short: "Get inventory for one SKU ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOpenAPI(cmd, flags, "POST", "/product/202309/inventory/search", nil, map[string]any{"sku_ids": []string{args[0]}})
		},
	}
}

func newInventoryUpdateCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Not implemented: inventory mutation is confirmed but deferred for idempotency safety",
		RunE: func(cmd *cobra.Command, args []string) error {
			message := "inventory update is not implemented in safe v1; endpoint is confirmed but execution is deferred until idempotency, no-retry mutation behavior, and operator confirmation are designed"
			if flags.asJSON {
				return printJSON(cmd, map[string]any{
					"status":  "deferred_mutation",
					"command": "inventory update",
					"doc_url": client.InventoryUpdateDocsURL,
					"message": message,
				})
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), message)
			return err
		},
	}
}

func newFulfillmentCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "fulfillment", Short: "TikTok Shop logistics and fulfillment operations"}
	cmd.AddCommand(newFulfillmentListCmd(flags))
	cmd.AddCommand(newFulfillmentGetCmd(flags))
	cmd.AddCommand(newWarehousesListCmd(flags))
	return cmd
}

func newFulfillmentListCmd(flags *rootFlags) *cobra.Command {
	var pageSize int
	var pageToken, sortField, sortOrder, packageStatus string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Search packages with the confirmed 202309 Fulfillment API",
		RunE: func(cmd *cobra.Command, args []string) error {
			if pageSize < 1 || pageSize > 50 {
				return usageErr(fmt.Errorf("--limit must be between 1 and 50"))
			}
			q := url.Values{}
			q.Set("page_size", strconv.Itoa(pageSize))
			setIf(q, "page_token", pageToken)
			setIf(q, "sort_field", sortField)
			setIf(q, "sort_order", sortOrder)
			body := map[string]any{}
			if packageStatus != "" {
				body["package_status"] = packageStatus
			}
			return runOpenAPI(cmd, flags, "POST", "/fulfillment/202309/packages/search", q, body)
		},
	}
	cmd.Flags().IntVar(&pageSize, "limit", 20, "Packages per page, official range 1-50")
	cmd.Flags().StringVar(&pageToken, "page-token", "", "Opaque next page token")
	cmd.Flags().StringVar(&sortField, "sort-field", "", "Optional sort field: create_time, update_time, order_pay_time")
	cmd.Flags().StringVar(&sortOrder, "sort-order", "", "Optional sort order: ASC or DESC")
	cmd.Flags().StringVar(&packageStatus, "status", "", "Optional package status filter")
	return cmd
}

func newFulfillmentGetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get <package-id>",
		Short: "Get package detail with the confirmed 202309 Fulfillment API",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOpenAPI(cmd, flags, "GET", "/fulfillment/202309/packages/"+url.PathEscape(args[0]), nil, nil)
		},
	}
}

func newWarehousesListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "warehouses",
		Short: "List seller warehouses with the confirmed 202309 Logistics API",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOpenAPI(cmd, flags, "GET", "/logistics/202309/warehouses", nil, nil)
		},
	}
}

func newProfileCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "profile", Short: "Manage named CLI profiles"}
	cmd.AddCommand(pendingCommand(flags, "list", "List saved profiles after profile storage is wired", "profile list", "https://github.com/mvanhorn/printing-press-library/blob/main/AGENTS.md"))
	cmd.AddCommand(pendingCommand(flags, "save [name]", "Save profile after profile storage is wired", "profile save", "https://github.com/mvanhorn/printing-press-library/blob/main/AGENTS.md"))
	cmd.AddCommand(pendingCommand(flags, "show [name]", "Show profile after profile storage is wired", "profile show", "https://github.com/mvanhorn/printing-press-library/blob/main/AGENTS.md"))
	cmd.AddCommand(pendingCommand(flags, "delete [name]", "Delete profile after profile storage is wired", "profile delete", "https://github.com/mvanhorn/printing-press-library/blob/main/AGENTS.md"))
	return cmd
}

type whichEntry struct {
	Command     string `json:"command"`
	Description string `json:"description"`
	Status      string `json:"status"`
	DocURL      string `json:"doc_url,omitempty"`
}

var whichIndex = []whichEntry{
	{Command: "doctor", Description: "Check config, env, and token-validation readiness", Status: "implemented", DocURL: client.AuthorizationOverviewURL},
	{Command: "auth status", Description: "Show configured auth material without revealing secrets", Status: "implemented", DocURL: client.AuthorizationOverviewURL},
	{Command: "auth exchange", Description: "Exchange an auth code for tokens", Status: "implemented", DocURL: client.AuthorizationOverviewURL},
	{Command: "auth refresh", Description: "Refresh access token", Status: "implemented", DocURL: client.AuthorizationOverviewURL},
	{Command: "shops info", Description: "List shops authorized for the app and token", Status: "implemented", DocURL: client.AuthorizedShopsDocsURL},
	{Command: "orders list", Description: "List orders with filters", Status: "implemented", DocURL: client.OrderListDocsURL},
	{Command: "orders get", Description: "Get one order", Status: "implemented", DocURL: client.OrderDetailDocsURL},
	{Command: "products list", Description: "List products/listings", Status: "implemented", DocURL: client.ProductSearchDocsURL},
	{Command: "products get", Description: "Get one product/listing", Status: "implemented", DocURL: client.ProductDetailDocsURL},
	{Command: "inventory list", Description: "Search inventory by product or SKU IDs", Status: "implemented", DocURL: client.InventorySearchDocsURL},
	{Command: "inventory get", Description: "Get inventory for one SKU", Status: "implemented", DocURL: client.InventorySearchDocsURL},
	{Command: "inventory update", Description: "Update inventory after idempotency/retry safety is designed", Status: "deferred_mutation", DocURL: client.InventoryUpdateDocsURL},
	{Command: "fulfillment list", Description: "Search packages", Status: "implemented", DocURL: client.PackageSearchDocsURL},
	{Command: "fulfillment get", Description: "Get one package", Status: "implemented", DocURL: client.PackageDetailDocsURL},
	{Command: "fulfillment warehouses", Description: "List warehouses", Status: "implemented", DocURL: client.WarehouseListDocsURL},
}

func newWhichCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "which [query]",
		Short: "Find the command that implements a capability",
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.ToLower(strings.Join(args, " "))
			matches := []whichEntry{}
			for _, entry := range whichIndex {
				if query == "" || strings.Contains(strings.ToLower(entry.Command+" "+entry.Description), query) {
					matches = append(matches, entry)
				}
			}
			if len(matches) == 0 {
				return usageErr(fmt.Errorf("no match for %q", query))
			}
			return printValue(cmd, flags, matches)
		},
	}
}

func newAgentContextCmd(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   "agent-context",
		Short: "Describe CLI capabilities for agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			return printJSON(cmd, map[string]any{
				"binary":          root.Name(),
				"version":         version,
				"safe_v1":         true,
				"commands":        whichIndex,
				"official_docs":   config.OfficialDocs,
				"mutation_policy": "defer unless idempotency and retry behavior are confirmed",
			})
		},
	}
}

func runOpenAPI(cmd *cobra.Command, flags *rootFlags, method, path string, query url.Values, body any) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	raw, err := c.DoOpenAPI(cmd.Context(), method, path, query, body)
	if err != nil {
		return classifyErr(err)
	}
	return printRawJSON(cmd, raw)
}

func setIf(values url.Values, key, value string) {
	if value != "" {
		values.Set(key, value)
	}
}

func zeroTimeOmit(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}

func unixTimeOmit(ts int64) any {
	if ts <= 0 {
		return nil
	}
	return time.Unix(ts, 0).UTC().Format(time.RFC3339)
}
