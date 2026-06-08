package cli

import (
	"context"
	"encoding/json"
	"errors"
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
	checkoutDeliveryPath = "/checkout/entrega/"
	checkoutStoresPath   = "/on/demandware.store/Sites-continente-Site/default/Stores-GetDelivery"
	checkoutBookSlotPath = "/on/demandware.store/Sites-continente-Site/default/CheckoutServices-BookDeliverySlot"
)

var (
	checkoutPageDataLayerRe = regexp.MustCompile(`data-page-data-layer="([^"]+)"`)
	checkoutShipmentUUIDRe  = regexp.MustCompile(`data-shipment-uuid="([^"]+)"`)
	checkoutShipmentIDRe    = regexp.MustCompile(`data-shipment-id="([^"]+)"`)
	checkoutShipmentIndexRe = regexp.MustCompile(`data-shipment-index="([^"]+)"`)
	checkoutStoreIDRe       = regexp.MustCompile(`data-current-store-id="([^"]+)"`)
	checkoutStoreKeyRe      = regexp.MustCompile(`data-current-shipment-store="([^"]+)"`)
	checkoutStorePostalRe   = regexp.MustCompile(`data-current-shipment-store-postal-code="([^"]+)"`)
	checkoutPrevSlotsRe     = regexp.MustCompile(`data-previously-booked-delivery-slots="([^"]*)"`)
	checkoutSlotsJSONRe     = regexp.MustCompile(`data-shipment-delivery-slots="([^"]+)"`)
	checkoutStoreBlockRe    = regexp.MustCompile(`(?s)<div id="store-[^"]+" class="[^"]*js-store-details[^"]*"[^>]*data-storedetails="([^"]+)"[^>]*data-index="([^"]+)"[^>]*>`)
)

type checkoutStore struct {
	ID         string `json:"id,omitempty"`
	Key        string `json:"key,omitempty"`
	Name       string `json:"name,omitempty"`
	PostalCode string `json:"postal_code,omitempty"`
}

type checkoutSlot struct {
	SlotRef      string `json:"slot_ref,omitempty"`
	DateLabel    string `json:"date_label,omitempty"`
	Weekday      string `json:"weekday,omitempty"`
	WeekdayShort string `json:"weekday_short,omitempty"`
	Day          string `json:"day,omitempty"`
	Month        string `json:"month,omitempty"`
	Start        string `json:"start,omitempty"`
	End          string `json:"end,omitempty"`
	Cost         string `json:"cost,omitempty"`
	SchedulerID  string `json:"scheduler_id,omitempty"`
	ListID       string `json:"list_id,omitempty"`
	OperatorID   int    `json:"operator_id,omitempty"`
	Selected     bool   `json:"selected,omitempty"`
}

type checkoutStoreOption struct {
	ID         string  `json:"id,omitempty"`
	Name       string  `json:"name,omitempty"`
	Address1   string  `json:"address1,omitempty"`
	City       string  `json:"city,omitempty"`
	PostalCode string  `json:"postal_code,omitempty"`
	Latitude   float64 `json:"latitude,omitempty"`
	Longitude  float64 `json:"longitude,omitempty"`
	StoreHours string  `json:"store_hours,omitempty"`
	Selected   bool    `json:"selected,omitempty"`
	Rank       int     `json:"rank,omitempty"`
}

type checkoutStatusPayload struct {
	Stage          string         `json:"stage,omitempty"`
	DeliveryMethod string         `json:"delivery_method,omitempty"`
	CurrentStore   checkoutStore  `json:"current_store,omitempty"`
	ShipmentID     string         `json:"shipment_id,omitempty"`
	ShipmentUUID   string         `json:"shipment_uuid,omitempty"`
	SlotsCount     int            `json:"slots_count,omitempty"`
	SelectedSlot   *checkoutSlot  `json:"selected_slot,omitempty"`
	PaymentURL     string         `json:"payment_url,omitempty"`
	PreferredStore *checkoutStore `json:"preferred_store,omitempty"`
}

type checkoutSlotsPayload struct {
	Stage          string         `json:"stage,omitempty"`
	DeliveryMethod string         `json:"delivery_method,omitempty"`
	CurrentStore   checkoutStore  `json:"current_store,omitempty"`
	ShipmentID     string         `json:"shipment_id,omitempty"`
	Slots          []checkoutSlot `json:"slots"`
}

type checkoutStoresPayload struct {
	Stage          string                `json:"stage,omitempty"`
	DeliveryMethod string                `json:"delivery_method,omitempty"`
	CurrentStore   checkoutStore         `json:"current_store,omitempty"`
	ShipmentID     string                `json:"shipment_id,omitempty"`
	ShipmentIndex  string                `json:"shipment_index,omitempty"`
	Stores         []checkoutStoreOption `json:"stores"`
}

type checkoutBookSlotPayload struct {
	Booked       bool                  `json:"booked"`
	SelectedSlot checkoutSlot          `json:"selected_slot"`
	Status       checkoutStatusPayload `json:"status"`
}

type checkoutCompactStatusPayload struct {
	Stage          string               `json:"stage,omitempty"`
	DeliveryMethod string               `json:"delivery_method,omitempty"`
	CurrentStore   checkoutStore        `json:"current_store,omitempty"`
	ShipmentID     string               `json:"shipment_id,omitempty"`
	SlotsCount     int                  `json:"slots_count,omitempty"`
	SelectedSlot   *checkoutCompactSlot `json:"selected_slot,omitempty"`
	PaymentURL     string               `json:"payment_url,omitempty"`
	PreferredStore *checkoutStore       `json:"preferred_store,omitempty"`
}

type checkoutCompactSlotsPayload struct {
	CurrentStore checkoutStore         `json:"current_store,omitempty"`
	ShipmentID   string                `json:"shipment_id,omitempty"`
	Slots        []checkoutCompactSlot `json:"slots"`
}

type checkoutCompactStoresPayload struct {
	CurrentStore checkoutStore                `json:"current_store,omitempty"`
	ShipmentID   string                       `json:"shipment_id,omitempty"`
	Stores       []checkoutCompactStoreOption `json:"stores"`
}

type checkoutCompactBookSlotPayload struct {
	Booked       bool                         `json:"booked"`
	SelectedSlot checkoutCompactSlot          `json:"selected_slot"`
	Status       checkoutCompactStatusPayload `json:"status"`
}

type checkoutCompactSlot struct {
	SlotRef   string `json:"slot_ref,omitempty"`
	DateLabel string `json:"date_label,omitempty"`
	Start     string `json:"start,omitempty"`
	End       string `json:"end,omitempty"`
	Cost      string `json:"cost,omitempty"`
	Selected  bool   `json:"selected,omitempty"`
}

type checkoutCompactStoreOption struct {
	ID         string `json:"id,omitempty"`
	Name       string `json:"name,omitempty"`
	City       string `json:"city,omitempty"`
	PostalCode string `json:"postal_code,omitempty"`
	Selected   bool   `json:"selected,omitempty"`
	Rank       int    `json:"rank,omitempty"`
}

type checkoutDeliveryPage struct {
	Stage          string
	DeliveryMethod string
	CurrentStore   checkoutStore
	ShipmentID     string
	ShipmentUUID   string
	ShipmentIndex  string
	Slots          []checkoutSlot
	SelectedSlot   *checkoutSlot
	PaymentURL     string
}

type checkoutPageDataEnvelope struct {
	PageData struct {
		CheckoutStepName string `json:"checkout_step_name"`
	} `json:"page_data"`
	UserData struct {
		Store          string `json:"store"`
		DeliveryMethod string `json:"delivery_method"`
	} `json:"user_data"`
}

type checkoutSlotsEnvelope struct {
	ShipmentDeliverySlots struct {
		ShipmentID           string `json:"shipmentId"`
		GroupedDeliverySlots []struct {
			SlotDateDayName      string `json:"slotDateDayName"`
			SlotDateDayNameShort string `json:"slotDateDayNameShort"`
			SlotDateDay          string `json:"slotDateDay"`
			SlotDateMonth        string `json:"slotDateMonth"`
			SlotDetails          []struct {
				FormattedStart          string `json:"formattedStart"`
				FormattedEnd            string `json:"formattedEnd"`
				FormattedShippingCost   string `json:"formattedShippingCost"`
				ShippingSlotSchedulerID string `json:"shippingSlotSchedulerId"`
				ShippingListID          string `json:"shippingListId"`
				ShippingOperatorID      int    `json:"shippingOperatorId"`
			} `json:"slotDetails"`
		} `json:"groupedDeliverySlots"`
	} `json:"shipmentDeliverySlots"`
	SummaryCheckoutPaymentURL string `json:"summaryCheckoutPaymentURL"`
}

func newCheckoutCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "checkout",
		Short: "Inspect and manage the authenticated delivery-step checkout",
		Long:  "Inspect the live authenticated delivery step on continente.pt and select a pickup slot for the current shipment.",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newCheckoutStatusCmd(flags))
	cmd.AddCommand(newCheckoutStoresCmd(flags))
	cmd.AddCommand(newCheckoutSlotsCmd(flags))
	cmd.AddCommand(newCheckoutSelectSlotCmd(flags))
	return cmd
}

func newCheckoutStatusCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "status",
		Short:       "Show the current authenticated delivery-step checkout state",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			page, err := fetchCheckoutDeliveryPage(cmd.Context(), c)
			if err != nil {
				return err
			}
			payload := checkoutStatusPayload{
				Stage:          page.Stage,
				DeliveryMethod: page.DeliveryMethod,
				CurrentStore:   page.CurrentStore,
				ShipmentID:     page.ShipmentID,
				ShipmentUUID:   page.ShipmentUUID,
				SlotsCount:     len(page.Slots),
				SelectedSlot:   page.SelectedSlot,
				PaymentURL:     page.PaymentURL,
			}
			if preferred, err := loadPreferredStore(flags); err == nil && preferred != nil {
				payload.PreferredStore = &checkoutStore{
					ID:         preferred.ID,
					Name:       preferred.Name,
					PostalCode: preferred.PostalCode,
				}
			}
			return emitStructuredOutputWithCompact(cmd, flags, payload, compactCheckoutStatusPayload(payload), DataProvenance{Source: "live"}, 1, []map[string]any{{
				"stage":           payload.Stage,
				"delivery_method": payload.DeliveryMethod,
				"store":           payload.CurrentStore.Name,
				"postal_code":     payload.CurrentStore.PostalCode,
				"shipment_id":     payload.ShipmentID,
				"slots_count":     payload.SlotsCount,
			}})
		},
	}
	return cmd
}

func newCheckoutSlotsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "slots",
		Short:       "List the currently available pickup slots for the delivery step",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			page, err := fetchCheckoutDeliveryPage(cmd.Context(), c)
			if err != nil {
				return err
			}
			payload := checkoutSlotsPayload{
				Stage:          page.Stage,
				DeliveryMethod: page.DeliveryMethod,
				CurrentStore:   page.CurrentStore,
				ShipmentID:     page.ShipmentID,
				Slots:          page.Slots,
			}
			rows := make([]map[string]any, 0, len(page.Slots))
			for _, slot := range page.Slots {
				rows = append(rows, map[string]any{
					"slot_ref":     slot.SlotRef,
					"date":         slot.DateLabel,
					"time":         slot.Start + "-" + slot.End,
					"cost":         slot.Cost,
					"selected":     slot.Selected,
					"scheduler_id": slot.SchedulerID,
					"operator_id":  slot.OperatorID,
				})
			}
			return emitStructuredOutputWithCompact(cmd, flags, payload, compactCheckoutSlotsPayload(payload), DataProvenance{Source: "live"}, len(page.Slots), rows)
		},
	}
	return cmd
}

func newCheckoutStoresCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "stores",
		Short:       "List pickup-store choices for the current checkout shipment",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			page, err := fetchCheckoutDeliveryPage(cmd.Context(), c)
			if err != nil {
				return err
			}
			stores, err := fetchCheckoutStores(cmd.Context(), c, page)
			if err != nil {
				return err
			}
			payload := checkoutStoresPayload{
				Stage:          page.Stage,
				DeliveryMethod: page.DeliveryMethod,
				CurrentStore:   page.CurrentStore,
				ShipmentID:     page.ShipmentID,
				ShipmentIndex:  page.ShipmentIndex,
				Stores:         stores,
			}
			rows := make([]map[string]any, 0, len(stores))
			for _, store := range stores {
				rows = append(rows, map[string]any{
					"id":          store.ID,
					"name":        store.Name,
					"city":        store.City,
					"postal_code": store.PostalCode,
					"selected":    store.Selected,
					"rank":        store.Rank,
				})
			}
			return emitStructuredOutputWithCompact(cmd, flags, payload, compactCheckoutStoresPayload(payload), DataProvenance{Source: "live"}, len(stores), rows)
		},
	}
	return cmd
}

func newCheckoutSelectSlotCmd(flags *rootFlags) *cobra.Command {
	var slotRef string
	var schedulerID string
	var shipmentID string

	cmd := &cobra.Command{
		Use:   "select-slot",
		Short: "Book a pickup slot for the current authenticated shipment",
		Long:  "Books the selected pickup slot using the live CheckoutServices-BookDeliverySlot endpoint observed on continente.pt.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(slotRef) == "" && strings.TrimSpace(schedulerID) == "" {
				return usageErr(fmt.Errorf("pass --slot-ref or --scheduler-id"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			page, err := fetchCheckoutDeliveryPage(cmd.Context(), c)
			if err != nil {
				return err
			}
			target, err := resolveCheckoutSlot(page, shipmentID, slotRef, schedulerID)
			if err != nil {
				return err
			}
			form := url.Values{
				"shipmentID":              {page.ShipmentID},
				"shippingSlotSchedulerId": {target.SchedulerID},
				"shippingSlotOperatorId":  {strconv.Itoa(target.OperatorID)},
				"shippingListId":          {target.ListID},
			}
			data, _, err := c.PostFormWithHeaders(cmd.Context(), checkoutBookSlotPath, form, cartAJAXHeaders(c.RequestBaseURL()+checkoutDeliveryPath))
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var resp struct {
				Error   bool   `json:"error"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(data, &resp); err == nil && resp.Error {
				if resp.Message == "" {
					resp.Message = "slot booking failed"
				}
				return apiErr(errors.New(resp.Message))
			}
			refreshed, err := fetchCheckoutDeliveryPage(cmd.Context(), c)
			if err != nil {
				return err
			}
			payload := checkoutBookSlotPayload{
				Booked:       true,
				SelectedSlot: target,
				Status: checkoutStatusPayload{
					Stage:          refreshed.Stage,
					DeliveryMethod: refreshed.DeliveryMethod,
					CurrentStore:   refreshed.CurrentStore,
					ShipmentID:     refreshed.ShipmentID,
					ShipmentUUID:   refreshed.ShipmentUUID,
					SlotsCount:     len(refreshed.Slots),
					SelectedSlot:   refreshed.SelectedSlot,
					PaymentURL:     refreshed.PaymentURL,
				},
			}
			return emitStructuredOutputWithCompact(cmd, flags, payload, compactCheckoutBookSlotPayload(payload), DataProvenance{Source: "live"}, 1, []map[string]any{{
				"booked":      payload.Booked,
				"slot_ref":    payload.SelectedSlot.SlotRef,
				"date":        payload.SelectedSlot.DateLabel,
				"time":        payload.SelectedSlot.Start + "-" + payload.SelectedSlot.End,
				"store":       payload.Status.CurrentStore.Name,
				"payment_url": payload.Status.PaymentURL,
			}})
		},
	}
	cmd.Flags().StringVar(&slotRef, "slot-ref", "", "Friendly slot reference from `checkout slots` (for example 1.2)")
	cmd.Flags().StringVar(&schedulerID, "scheduler-id", "", "Raw shipping slot scheduler ID")
	cmd.Flags().StringVar(&shipmentID, "shipment-id", "", "Shipment ID override for future multi-shipment carts")
	return cmd
}

func fetchCheckoutDeliveryPage(ctx context.Context, c *client.Client) (checkoutDeliveryPage, error) {
	data, err := c.GetWithHeadersNoCache(ctx, checkoutDeliveryPath, nil, cartAJAXHeaders(c.RequestBaseURL()+authLoginCheckPath))
	if err != nil {
		return checkoutDeliveryPage{}, classifyAPIError(err, nil)
	}
	return parseCheckoutDeliveryPage(data)
}

func parseCheckoutDeliveryPage(data []byte) (checkoutDeliveryPage, error) {
	raw := string(data)
	if strings.Contains(raw, "/login/?redirectUrl=") || strings.Contains(raw, "Account-Login") {
		return checkoutDeliveryPage{}, authErr(fmt.Errorf("authenticated storefront session required; run 'continente-pp-cli auth status' and re-import cookies if needed"))
	}
	page := checkoutDeliveryPage{}

	if matches := checkoutPageDataLayerRe.FindStringSubmatch(raw); len(matches) == 2 {
		var payload checkoutPageDataEnvelope
		if err := json.Unmarshal([]byte(html.UnescapeString(matches[1])), &payload); err == nil {
			page.Stage = payload.PageData.CheckoutStepName
			page.DeliveryMethod = html.UnescapeString(payload.UserData.DeliveryMethod)
			page.CurrentStore.Name = html.UnescapeString(payload.UserData.Store)
		}
	}
	if matches := checkoutShipmentUUIDRe.FindStringSubmatch(raw); len(matches) == 2 {
		page.ShipmentUUID = matches[1]
	}
	if matches := checkoutShipmentIDRe.FindStringSubmatch(raw); len(matches) == 2 {
		page.ShipmentID = matches[1]
	}
	if matches := checkoutShipmentIndexRe.FindStringSubmatch(raw); len(matches) == 2 {
		page.ShipmentIndex = matches[1]
	}
	if matches := checkoutStoreIDRe.FindStringSubmatch(raw); len(matches) == 2 {
		page.CurrentStore.ID = matches[1]
	}
	if matches := checkoutStoreKeyRe.FindStringSubmatch(raw); len(matches) == 2 {
		page.CurrentStore.Key = matches[1]
	}
	if matches := checkoutStorePostalRe.FindStringSubmatch(raw); len(matches) == 2 {
		page.CurrentStore.PostalCode = matches[1]
	}

	var selectedIDs []string
	if matches := checkoutPrevSlotsRe.FindStringSubmatch(raw); len(matches) == 2 {
		value := strings.TrimSpace(html.UnescapeString(matches[1]))
		if value != "" && value != "null" {
			for _, candidate := range strings.Split(value, ",") {
				candidate = strings.TrimSpace(candidate)
				if candidate != "" && candidate != "null" {
					selectedIDs = append(selectedIDs, candidate)
				}
			}
		}
	}

	if matches := checkoutSlotsJSONRe.FindStringSubmatch(raw); len(matches) == 2 {
		var envelope checkoutSlotsEnvelope
		if err := json.Unmarshal([]byte(html.UnescapeString(matches[1])), &envelope); err == nil {
			if envelope.ShipmentDeliverySlots.ShipmentID != "" {
				page.ShipmentID = envelope.ShipmentDeliverySlots.ShipmentID
			}
			page.PaymentURL = envelope.SummaryCheckoutPaymentURL
			page.Slots = flattenCheckoutSlots(envelope.ShipmentDeliverySlots.GroupedDeliverySlots, selectedIDs)
			for i := range page.Slots {
				if page.Slots[i].Selected {
					slot := page.Slots[i]
					page.SelectedSlot = &slot
					break
				}
			}
		}
	}

	if page.ShipmentID == "" {
		return checkoutDeliveryPage{}, fmt.Errorf("missing shipment id in checkout delivery page")
	}
	return page, nil
}

func fetchCheckoutStores(ctx context.Context, c *client.Client, page checkoutDeliveryPage) ([]checkoutStoreOption, error) {
	shipmentNumber := strings.TrimSpace(page.ShipmentIndex)
	if shipmentNumber == "" {
		shipmentNumber = "1"
	}
	data, err := c.GetWithHeadersNoCache(ctx, checkoutStoresPath, map[string]string{
		"location":       "shipping-new-store",
		"shipmentNumber": shipmentNumber,
	}, cartAJAXHeaders(c.RequestBaseURL()+checkoutDeliveryPath))
	if err != nil {
		return nil, classifyAPIError(err, nil)
	}
	return parseCheckoutStoreOptions(data)
}

func parseCheckoutStoreOptions(data []byte) ([]checkoutStoreOption, error) {
	raw := string(data)
	matches := checkoutStoreBlockRe.FindAllStringSubmatchIndex(raw, -1)
	if len(matches) == 0 {
		return nil, notFoundErr(fmt.Errorf("no pickup stores found in checkout store picker"))
	}

	type storeDetails struct {
		ID         string  `json:"ID"`
		Name       string  `json:"name"`
		Address1   string  `json:"address1"`
		City       string  `json:"city"`
		PostalCode string  `json:"postalCode"`
		Latitude   float64 `json:"latitude"`
		Longitude  float64 `json:"longitude"`
		StoreHours string  `json:"storeHours"`
	}

	stores := make([]checkoutStoreOption, 0, len(matches))
	for i, match := range matches {
		blockStart, blockEnd := match[0], len(raw)
		if i+1 < len(matches) {
			blockEnd = matches[i+1][0]
		}
		block := raw[blockStart:blockEnd]
		storedetailsRaw := html.UnescapeString(raw[match[2]:match[3]])
		rankText := raw[match[4]:match[5]]
		var details storeDetails
		if err := json.Unmarshal([]byte(storedetailsRaw), &details); err != nil {
			return nil, fmt.Errorf("parse checkout store option: %w", err)
		}
		rank, _ := strconv.Atoi(strings.TrimSpace(rankText))
		stores = append(stores, checkoutStoreOption{
			ID:         details.ID,
			Name:       html.UnescapeString(details.Name),
			Address1:   html.UnescapeString(details.Address1),
			City:       html.UnescapeString(details.City),
			PostalCode: details.PostalCode,
			Latitude:   details.Latitude,
			Longitude:  details.Longitude,
			StoreHours: html.UnescapeString(details.StoreHours),
			Selected:   strings.Contains(block, "checked"),
			Rank:       rank,
		})
	}
	return stores, nil
}

func flattenCheckoutSlots(groups []struct {
	SlotDateDayName      string `json:"slotDateDayName"`
	SlotDateDayNameShort string `json:"slotDateDayNameShort"`
	SlotDateDay          string `json:"slotDateDay"`
	SlotDateMonth        string `json:"slotDateMonth"`
	SlotDetails          []struct {
		FormattedStart          string `json:"formattedStart"`
		FormattedEnd            string `json:"formattedEnd"`
		FormattedShippingCost   string `json:"formattedShippingCost"`
		ShippingSlotSchedulerID string `json:"shippingSlotSchedulerId"`
		ShippingListID          string `json:"shippingListId"`
		ShippingOperatorID      int    `json:"shippingOperatorId"`
	} `json:"slotDetails"`
}, selectedIDs []string) []checkoutSlot {
	selected := map[string]bool{}
	for _, id := range selectedIDs {
		selected[id] = true
	}
	slots := make([]checkoutSlot, 0)
	for groupIndex, group := range groups {
		dateLabel := strings.TrimSpace(fmt.Sprintf("%s %s %s", html.UnescapeString(group.SlotDateDayName), group.SlotDateDay, group.SlotDateMonth))
		for slotIndex, detail := range group.SlotDetails {
			slots = append(slots, checkoutSlot{
				SlotRef:      fmt.Sprintf("%d.%d", groupIndex+1, slotIndex+1),
				DateLabel:    dateLabel,
				Weekday:      html.UnescapeString(group.SlotDateDayName),
				WeekdayShort: html.UnescapeString(group.SlotDateDayNameShort),
				Day:          group.SlotDateDay,
				Month:        group.SlotDateMonth,
				Start:        detail.FormattedStart,
				End:          detail.FormattedEnd,
				Cost:         html.UnescapeString(detail.FormattedShippingCost),
				SchedulerID:  detail.ShippingSlotSchedulerID,
				ListID:       detail.ShippingListID,
				OperatorID:   detail.ShippingOperatorID,
				Selected:     selected[detail.ShippingSlotSchedulerID],
			})
		}
	}
	return slots
}

func resolveCheckoutSlot(page checkoutDeliveryPage, shipmentID, slotRef, schedulerID string) (checkoutSlot, error) {
	if shipmentID != "" && shipmentID != page.ShipmentID {
		return checkoutSlot{}, usageErr(fmt.Errorf("shipment %q is not present in the current checkout page", shipmentID))
	}
	for _, slot := range page.Slots {
		if slotRef != "" && slot.SlotRef == slotRef {
			return slot, nil
		}
		if schedulerID != "" && slot.SchedulerID == schedulerID {
			return slot, nil
		}
	}
	if slotRef != "" {
		return checkoutSlot{}, notFoundErr(fmt.Errorf("slot-ref %q not found", slotRef))
	}
	return checkoutSlot{}, notFoundErr(fmt.Errorf("scheduler-id %q not found", schedulerID))
}

func compactCheckoutStatusPayload(payload checkoutStatusPayload) checkoutCompactStatusPayload {
	compact := checkoutCompactStatusPayload{
		Stage:          payload.Stage,
		DeliveryMethod: payload.DeliveryMethod,
		CurrentStore:   payload.CurrentStore,
		ShipmentID:     payload.ShipmentID,
		SlotsCount:     payload.SlotsCount,
		PaymentURL:     payload.PaymentURL,
		PreferredStore: payload.PreferredStore,
	}
	if payload.SelectedSlot != nil {
		slot := compactCheckoutSlot(*payload.SelectedSlot)
		compact.SelectedSlot = &slot
	}
	return compact
}

func compactCheckoutSlotsPayload(payload checkoutSlotsPayload) checkoutCompactSlotsPayload {
	out := checkoutCompactSlotsPayload{
		CurrentStore: payload.CurrentStore,
		ShipmentID:   payload.ShipmentID,
		Slots:        make([]checkoutCompactSlot, 0, len(payload.Slots)),
	}
	for _, slot := range payload.Slots {
		out.Slots = append(out.Slots, compactCheckoutSlot(slot))
	}
	return out
}

func compactCheckoutStoresPayload(payload checkoutStoresPayload) checkoutCompactStoresPayload {
	out := checkoutCompactStoresPayload{
		CurrentStore: payload.CurrentStore,
		ShipmentID:   payload.ShipmentID,
		Stores:       make([]checkoutCompactStoreOption, 0, len(payload.Stores)),
	}
	for _, store := range payload.Stores {
		out.Stores = append(out.Stores, checkoutCompactStoreOption{
			ID:         store.ID,
			Name:       store.Name,
			City:       store.City,
			PostalCode: store.PostalCode,
			Selected:   store.Selected,
			Rank:       store.Rank,
		})
	}
	return out
}

func compactCheckoutBookSlotPayload(payload checkoutBookSlotPayload) checkoutCompactBookSlotPayload {
	return checkoutCompactBookSlotPayload{
		Booked:       payload.Booked,
		SelectedSlot: compactCheckoutSlot(payload.SelectedSlot),
		Status:       compactCheckoutStatusPayload(payload.Status),
	}
}

func compactCheckoutSlot(slot checkoutSlot) checkoutCompactSlot {
	return checkoutCompactSlot{
		SlotRef:   slot.SlotRef,
		DateLabel: slot.DateLabel,
		Start:     slot.Start,
		End:       slot.End,
		Cost:      slot.Cost,
		Selected:  slot.Selected,
	}
}
