package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"continente-pp-cli/internal/client"
	"github.com/spf13/cobra"
)

const (
	cartAddPath      = "/on/demandware.store/Sites-continente-Site/default/Cart-AddProduct"
	cartMiniPath     = "/on/demandware.store/Sites-continente-Site/default/Cart-MiniCart"
	cartMiniShowPath = "/on/demandware.store/Sites-continente-Site/default/Cart-MiniCartShow"
	cartUpdatePath   = "/on/demandware.store/Sites-continente-Site/default/Cart-UpdateQuantity"
	cartRemovePath   = "/on/demandware.store/Sites-continente-Site/default/Cart-RemoveProductLineItem"
	cartClearPath    = "/on/demandware.store/Sites-continente-Site/default/Cart-RemoveAllProductLineItems"
)

var (
	minicartQuantityRe = regexp.MustCompile(`(?s)<span class="minicart-quantity[^"]*">\s*([^<]+)\s*</span>`)
	minicartTotalRe    = regexp.MustCompile(`(?s)<span class="minicart-grandtotal">\s*([^<]+)\s*</span>`)
	minicartLinkRe     = regexp.MustCompile(`(?s)<a class="minicart-link" href="([^"]+)"`)
	minicartActionsRe  = regexp.MustCompile(`(?s)<span class="d-none js-minicart-actions"[^>]*data-add-to-cart="([^"]+)"[^>]*data-remove-action="([^"]+)"[^>]*data-action="([^"]+)"`)
	productQtyMapRe    = regexp.MustCompile(`(?s)data-productqtymap="([^"]+)"`)
)

type miniCartPayload struct {
	Quantity      int               `json:"quantity"`
	GrandTotal    string            `json:"grand_total,omitempty"`
	CartURL       string            `json:"cart_url,omitempty"`
	Actions       map[string]string `json:"actions,omitempty"`
	ProductQtyMap any               `json:"product_qty_map,omitempty"`
	Items         []cartLineItem    `json:"items,omitempty"`
}

type cartLineItem struct {
	ProductID  string `json:"product_id"`
	Name       string `json:"name,omitempty"`
	Brand      string `json:"brand,omitempty"`
	Quantity   int    `json:"quantity,omitempty"`
	UUID       string `json:"uuid,omitempty"`
	EntryUUID  string `json:"entry_uuid,omitempty"`
	Dimension  string `json:"dimension,omitempty"`
	ProductURL string `json:"product_url,omitempty"`
	Price      string `json:"price,omitempty"`
}

type cartMutationSummary struct {
	Action    string          `json:"action"`
	ProductID string          `json:"product_id,omitempty"`
	UUID      string          `json:"uuid,omitempty"`
	Quantity  int             `json:"quantity,omitempty"`
	Cart      miniCartPayload `json:"cart"`
}

func newCartCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cart",
		Short: "Guest cart workflows on continente.pt",
		Long:  "Manage the storefront guest cart using the same Demandware cart controllers as continente.pt.",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newCartAddCmd(flags))
	cmd.AddCommand(newCartMiniCmd(flags))
	cmd.AddCommand(newCartUpdateCmd(flags))
	cmd.AddCommand(newCartRemoveCmd(flags))
	cmd.AddCommand(newCartClearCmd(flags))
	return cmd
}

func newCartAddCmd(flags *rootFlags) *cobra.Command {
	var pid string
	var quantity int

	cmd := &cobra.Command{
		Use:         "add",
		Short:       "Add a product to the guest cart",
		Annotations: map[string]string{"pp:endpoint": "cart.add", "pp:method": "POST", "pp:path": cartAddPath},
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(pid) == "" {
				return usageErr(fmt.Errorf("--pid is required"))
			}
			if quantity < 1 {
				return usageErr(fmt.Errorf("--quantity must be >= 1"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			form := url.Values{
				"pid":      {pid},
				"quantity": {strconv.Itoa(quantity)},
			}
			data, _, err := c.PostForm(cmd.Context(), cartAddPath, form)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return emitCartMutationJSON(cmd, flags, c, data, cartMutationSummary{
				Action:    "add",
				ProductID: pid,
				Quantity:  quantity,
			})
		},
	}
	cmd.Flags().StringVar(&pid, "pid", "", "Storefront product ID")
	cmd.Flags().IntVar(&quantity, "quantity", 1, "Quantity to add")
	return cmd
}

func newCartMiniCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "mini",
		Aliases:     []string{"status"},
		Short:       "Fetch the current mini-cart state",
		Annotations: map[string]string{"pp:endpoint": "cart.mini", "pp:method": "GET", "pp:path": cartMiniShowPath, "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			payload, err := fetchMiniCart(cmd.Context(), c)
			if err != nil {
				return err
			}
			return emitStructuredOutputWithCompact(cmd, flags, payload, compactMiniCartPayload(payload), DataProvenance{Source: "live"}, 1, nil)
		},
	}
	return cmd
}

func newCartUpdateCmd(flags *rootFlags) *cobra.Command {
	var pid string
	var uuid string
	var quantity int
	var dimension string

	cmd := &cobra.Command{
		Use:         "update",
		Short:       "Update the quantity of an existing cart line item",
		Annotations: map[string]string{"pp:endpoint": "cart.update", "pp:method": "GET", "pp:path": cartUpdatePath},
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(pid) == "" {
				return usageErr(fmt.Errorf("--pid is required"))
			}
			if strings.TrimSpace(uuid) == "" {
				return usageErr(fmt.Errorf("--uuid is required"))
			}
			if quantity < 1 {
				return usageErr(fmt.Errorf("--quantity must be >= 1"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{
				"pid":                   pid,
				"quantity":              strconv.Itoa(quantity),
				"step":                  "1",
				"uuid":                  uuid,
				"dimension":             dimension,
				"isCart":                "false",
				"gtmList":               "CLI",
				"gtmIndex":              "1",
				"promotionData":         "null",
				"taggstarPromotionData": "",
			}
			data, err := c.GetWithHeadersNoCache(cmd.Context(), cartUpdatePath, params, cartAJAXHeaders(c.RequestBaseURL()+authLoginCheckPath))
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return emitCartMutationJSON(cmd, flags, c, data, cartMutationSummary{
				Action:    "update",
				ProductID: pid,
				UUID:      uuid,
				Quantity:  quantity,
			})
		},
	}
	cmd.Flags().StringVar(&pid, "pid", "", "Storefront product ID")
	cmd.Flags().StringVar(&uuid, "uuid", "", "Cart line-item UUID")
	cmd.Flags().IntVar(&quantity, "quantity", 1, "Updated quantity")
	cmd.Flags().StringVar(&dimension, "dimension", "un", "Selected dimension")
	return cmd
}

func newCartRemoveCmd(flags *rootFlags) *cobra.Command {
	var pid string
	var uuid string

	cmd := &cobra.Command{
		Use:         "remove",
		Short:       "Remove a line item from the current cart",
		Annotations: map[string]string{"pp:endpoint": "cart.remove", "pp:method": "GET", "pp:path": cartRemovePath},
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(pid) == "" {
				return usageErr(fmt.Errorf("--pid is required"))
			}
			if strings.TrimSpace(uuid) == "" {
				return usageErr(fmt.Errorf("--uuid is required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{
				"isMiniCart": "true",
				"pid":        pid,
				"uuid":       uuid,
				"gtmIndex":   "1",
				"gtmList":    "CLI",
			}
			data, err := c.GetWithHeadersNoCache(cmd.Context(), cartRemovePath, params, cartAJAXHeaders(c.RequestBaseURL()+authLoginCheckPath))
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return emitCartMutationJSON(cmd, flags, c, data, cartMutationSummary{
				Action:    "remove",
				ProductID: pid,
				UUID:      uuid,
			})
		},
	}
	cmd.Flags().StringVar(&pid, "pid", "", "Storefront product ID")
	cmd.Flags().StringVar(&uuid, "uuid", "", "Cart line-item UUID")
	return cmd
}

func newCartClearCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "clear",
		Short:       "Clear the current guest cart",
		Annotations: map[string]string{"pp:endpoint": "cart.clear", "pp:method": "POST", "pp:path": cartClearPath},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, _, err := c.PostForm(cmd.Context(), cartClearPath, url.Values{})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return emitCartMutationJSON(cmd, flags, c, data, cartMutationSummary{
				Action: "clear",
			})
		},
	}
	return cmd
}

func emitCartJSON(cmd *cobra.Command, flags *rootFlags, data json.RawMessage, count int) error {
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
	}
	return emitStructuredOutput(cmd, flags, payload, DataProvenance{Source: "live"}, count, nil)
}

func emitCartMutationJSON(cmd *cobra.Command, flags *rootFlags, c *client.Client, data json.RawMessage, summary cartMutationSummary) error {
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
	}
	if flags.compact && flags.selectFields == "" {
		// GET-based cart mutations still refresh the mini-cart summary; clear
		// any cached mini-cart snapshot first so the compact payload reflects
		// the post-mutation state rather than a stale pre-mutation entry.
		c.InvalidateCache()
		if mini, err := fetchMiniCart(cmd.Context(), c); err == nil {
			summary.Cart = compactMiniCartPayload(mini)
		}
	}
	return emitStructuredOutputWithCompact(cmd, flags, payload, summary, DataProvenance{Source: "live"}, 1, nil)
}

func compactMiniCartPayload(payload miniCartPayload) miniCartPayload {
	compact := miniCartPayload{
		Quantity:   payload.Quantity,
		GrandTotal: payload.GrandTotal,
		CartURL:    payload.CartURL,
		Items:      make([]cartLineItem, 0, len(payload.Items)),
	}
	for _, item := range payload.Items {
		compact.Items = append(compact.Items, cartLineItem{
			ProductID: item.ProductID,
			Name:      item.Name,
			Brand:     item.Brand,
			Quantity:  item.Quantity,
			UUID:      item.UUID,
			Dimension: item.Dimension,
			Price:     item.Price,
		})
	}
	return compact
}

func fetchMiniCart(ctx context.Context, c *client.Client) (miniCartPayload, error) {
	data, err := c.Get(ctx, cartMiniShowPath, nil)
	if err == nil {
		if payload, parseErr := parseMiniCartShowJSON(data); parseErr == nil {
			return payload, nil
		}
	}
	data, err = c.Get(ctx, cartMiniPath, nil)
	if err != nil {
		return miniCartPayload{}, err
	}
	return parseMiniCartHTML(data)
}

func parseMiniCartHTML(data []byte) (miniCartPayload, error) {
	raw := string(data)
	payload := miniCartPayload{
		Actions: map[string]string{},
	}

	if matches := minicartQuantityRe.FindStringSubmatch(raw); len(matches) == 2 {
		qty, err := strconv.Atoi(strings.TrimSpace(html.UnescapeString(matches[1])))
		if err == nil {
			payload.Quantity = qty
		}
	}
	if matches := minicartTotalRe.FindStringSubmatch(raw); len(matches) == 2 {
		payload.GrandTotal = strings.TrimSpace(html.UnescapeString(matches[1]))
	}
	if matches := minicartLinkRe.FindStringSubmatch(raw); len(matches) == 2 {
		payload.CartURL = html.UnescapeString(matches[1])
	}
	if matches := minicartActionsRe.FindStringSubmatch(raw); len(matches) == 4 {
		payload.Actions["add"] = html.UnescapeString(matches[1])
		payload.Actions["remove"] = html.UnescapeString(matches[2])
		payload.Actions["update_quantity"] = html.UnescapeString(matches[3])
	}
	if matches := productQtyMapRe.FindStringSubmatch(raw); len(matches) == 2 {
		unescaped := html.UnescapeString(matches[1])
		var productQtyMap any
		if err := json.Unmarshal([]byte(unescaped), &productQtyMap); err == nil {
			payload.ProductQtyMap = productQtyMap
		}
	}

	return payload, nil
}

func parseMiniCartShowJSON(data []byte) (miniCartPayload, error) {
	var envelope struct {
		QuantityTotal int `json:"quantityTotal"`
		Cart          *struct {
			ItemsSortedByBrand []struct {
				Items []struct {
					ID                string `json:"id"`
					ProductName       string `json:"productName"`
					Brand             string `json:"brand"`
					UUID              string `json:"UUID"`
					EntryUUID         string `json:"uuid"`
					SelectedDimension string `json:"selectedDimension"`
					SecondaryQuantity int    `json:"secondaryQuantity"`
					ProductURL        string `json:"productURL"`
					Price             struct {
						Sales struct {
							Formatted string `json:"formatted"`
						} `json:"sales"`
					} `json:"price"`
				} `json:"items"`
			} `json:"itemsSortedByBrand"`
		} `json:"cart"`
		Basket *struct {
			ItemsSortedByBrand []struct {
				Items []struct {
					ID                string `json:"id"`
					ProductName       string `json:"productName"`
					Brand             string `json:"brand"`
					UUID              string `json:"UUID"`
					EntryUUID         string `json:"uuid"`
					SelectedDimension string `json:"selectedDimension"`
					SecondaryQuantity int    `json:"secondaryQuantity"`
					ProductURL        string `json:"productURL"`
					Price             struct {
						Sales struct {
							Formatted string `json:"formatted"`
						} `json:"sales"`
					} `json:"price"`
				} `json:"items"`
			} `json:"itemsSortedByBrand"`
		} `json:"basket"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return miniCartPayload{}, err
	}

	var brands []struct {
		Items []struct {
			ID                string `json:"id"`
			ProductName       string `json:"productName"`
			Brand             string `json:"brand"`
			UUID              string `json:"UUID"`
			EntryUUID         string `json:"uuid"`
			SelectedDimension string `json:"selectedDimension"`
			SecondaryQuantity int    `json:"secondaryQuantity"`
			ProductURL        string `json:"productURL"`
			Price             struct {
				Sales struct {
					Formatted string `json:"formatted"`
				} `json:"sales"`
			} `json:"price"`
		} `json:"items"`
	}
	switch {
	case envelope.Basket != nil:
		brands = envelope.Basket.ItemsSortedByBrand
	case envelope.Cart != nil:
		brands = envelope.Cart.ItemsSortedByBrand
	default:
		return miniCartPayload{}, fmt.Errorf("missing cart payload")
	}

	payload := miniCartPayload{Actions: map[string]string{}}
	for _, brand := range brands {
		for _, item := range brand.Items {
			qty := item.SecondaryQuantity
			if qty < 1 {
				qty = 1
			}
			payload.Items = append(payload.Items, cartLineItem{
				ProductID:  item.ID,
				Name:       item.ProductName,
				Brand:      item.Brand,
				Quantity:   qty,
				UUID:       item.UUID,
				EntryUUID:  item.EntryUUID,
				Dimension:  item.SelectedDimension,
				ProductURL: item.ProductURL,
				Price:      item.Price.Sales.Formatted,
			})
			payload.Quantity += qty
		}
	}
	if envelope.QuantityTotal > 0 {
		payload.Quantity = envelope.QuantityTotal
	}
	return payload, nil
}

func cartAJAXHeaders(referer string) map[string]string {
	return map[string]string{
		"Accept":           "application/json, text/html;q=0.9, */*;q=0.8",
		"Referer":          referer,
		"X-Requested-With": "XMLHttpRequest",
	}
}
