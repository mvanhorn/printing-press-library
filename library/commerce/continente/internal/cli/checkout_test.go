package cli

import (
	"errors"
	"testing"
)

func TestParseCheckoutDeliveryPage(t *testing.T) {
	t.Parallel()

	raw := []byte(`
<span class="page-data-layer" data-page-data-layer="{&quot;page_data&quot;:{&quot;checkout_step_name&quot;:&quot;Entrega&quot;},&quot;user_data&quot;:{&quot;store&quot;:&quot;Continente Mafra&quot;,&quot;delivery_method&quot;:&quot;Click &amp; Go&quot;}}"></span>
<span data-previously-booked-delivery-slots="slot-2"></span>
<div class="js-shipment-step" data-shipment-uuid="shipment-uuid-1">
  <div class="step-type--address"
    data-shipment-id="430698332_001"
    data-current-store-id="439"
    data-current-shipment-store="col-439-store"
    data-current-shipment-store-postal-code="2640-453">
  </div>
</div>
<div class="shipping-delivery-slot" data-shipment-id="430698332_001" data-shipment-delivery-slots="{&quot;shipmentDeliverySlots&quot;:{&quot;shipmentId&quot;:&quot;430698332_001&quot;,&quot;groupedDeliverySlots&quot;:[{&quot;slotDateDayName&quot;:&quot;S&aacute;bado&quot;,&quot;slotDateDayNameShort&quot;:&quot;S&aacute;b&quot;,&quot;slotDateDay&quot;:&quot;6&quot;,&quot;slotDateMonth&quot;:&quot;Jun&quot;,&quot;slotDetails&quot;:[{&quot;formattedStart&quot;:&quot;10:00&quot;,&quot;formattedEnd&quot;:&quot;12:00&quot;,&quot;formattedShippingCost&quot;:&quot;0,00&euro;&quot;,&quot;shippingSlotSchedulerId&quot;:&quot;slot-1&quot;,&quot;shippingListId&quot;:&quot;list-1&quot;,&quot;shippingOperatorId&quot;:1268},{&quot;formattedStart&quot;:&quot;12:00&quot;,&quot;formattedEnd&quot;:&quot;14:00&quot;,&quot;formattedShippingCost&quot;:&quot;0,00&euro;&quot;,&quot;shippingSlotSchedulerId&quot;:&quot;slot-2&quot;,&quot;shippingListId&quot;:&quot;list-2&quot;,&quot;shippingOperatorId&quot;:1268}]}]},&quot;summaryCheckoutPaymentURL&quot;:&quot;https://www.continente.pt/checkout/pagamento/&quot;}"></div>
`)

	got, err := parseCheckoutDeliveryPage(raw)
	if err != nil {
		t.Fatalf("parseCheckoutDeliveryPage: %v", err)
	}
	if got.Stage != "Entrega" {
		t.Fatalf("Stage = %q, want Entrega", got.Stage)
	}
	if got.DeliveryMethod != "Click & Go" {
		t.Fatalf("DeliveryMethod = %q, want Click & Go", got.DeliveryMethod)
	}
	if got.CurrentStore.Name != "Continente Mafra" {
		t.Fatalf("CurrentStore.Name = %q", got.CurrentStore.Name)
	}
	if got.CurrentStore.ID != "439" || got.CurrentStore.Key != "col-439-store" {
		t.Fatalf("unexpected store identifiers: %#v", got.CurrentStore)
	}
	if got.ShipmentID != "430698332_001" || got.ShipmentUUID != "shipment-uuid-1" {
		t.Fatalf("unexpected shipment identifiers: %#v", got)
	}
	if got.PaymentURL != "https://www.continente.pt/checkout/pagamento/" {
		t.Fatalf("PaymentURL = %q", got.PaymentURL)
	}
	if len(got.Slots) != 2 {
		t.Fatalf("len(Slots) = %d, want 2", len(got.Slots))
	}
	if got.Slots[0].SlotRef != "1.1" || got.Slots[1].SlotRef != "1.2" {
		t.Fatalf("unexpected slot refs: %#v", got.Slots)
	}
	if !got.Slots[1].Selected {
		t.Fatalf("expected second slot to be selected: %#v", got.Slots[1])
	}
	if got.SelectedSlot == nil || got.SelectedSlot.SchedulerID != "slot-2" {
		t.Fatalf("unexpected selected slot: %#v", got.SelectedSlot)
	}
}

func TestResolveCheckoutSlot(t *testing.T) {
	t.Parallel()

	page := checkoutDeliveryPage{
		ShipmentID: "430698332_001",
		Slots: []checkoutSlot{
			{SlotRef: "1.1", SchedulerID: "slot-1", ListID: "list-1", OperatorID: 1268},
			{SlotRef: "1.2", SchedulerID: "slot-2", ListID: "list-2", OperatorID: 1268},
		},
	}

	got, err := resolveCheckoutSlot(page, "", "1.2", "")
	if err != nil {
		t.Fatalf("resolveCheckoutSlot by slot ref: %v", err)
	}
	if got.SchedulerID != "slot-2" {
		t.Fatalf("SchedulerID = %q, want slot-2", got.SchedulerID)
	}

	got, err = resolveCheckoutSlot(page, "430698332_001", "", "slot-1")
	if err != nil {
		t.Fatalf("resolveCheckoutSlot by scheduler id: %v", err)
	}
	if got.ListID != "list-1" {
		t.Fatalf("ListID = %q, want list-1", got.ListID)
	}
}

func TestParseCheckoutStoreOptions(t *testing.T) {
	t.Parallel()

	raw := []byte(`
<div class="delivery-nearest" data-url="https://www.continente.pt/on/demandware.store/Sites-continente-Site/default/Stores-GetStoresChunk?location=shipping-new-store&amp;shipmentNumber=1">
  <div id="store-col-439-store" class="row col-12 js-store-details store-details m-0 d-flex align-items-baseline"
    data-storedetails="{&quot;ID&quot;:&quot;col-439-store&quot;,&quot;name&quot;:&quot;Continente Mafra&quot;,&quot;address1&quot;:&quot;Rua Professor Armindo Ayres de Carvalho 11&quot;,&quot;city&quot;:&quot;Mafra&quot;,&quot;postalCode&quot;:&quot;2640-453&quot;,&quot;latitude&quot;:38.945322,&quot;longitude&quot;:-9.355379,&quot;storeHours&quot;:&quot;Seg a Dom: 08h00 &agrave;s 22h00&quot;}"
    data-index="1">
    <input type="radio" title id="store-postalcode-2640-453" name="delivery" checked />
  </div>
  <div id="store-col-234-store" class="row col-12 js-store-details store-details m-0 d-flex align-items-baseline"
    data-storedetails="{&quot;ID&quot;:&quot;col-234-store&quot;,&quot;name&quot;:&quot;Continente Modelo Mafra&quot;,&quot;address1&quot;:&quot;R. da Escola&quot;,&quot;city&quot;:&quot;Mafra&quot;,&quot;postalCode&quot;:&quot;2640-577&quot;,&quot;latitude&quot;:38.953978,&quot;longitude&quot;:-9.334232,&quot;storeHours&quot;:&quot;Seg a Dom: 09h00 &agrave;s 21h00&quot;}"
    data-index="2">
    <input type="radio" title id="store-postalcode-2640-577" name="delivery" />
  </div>
</div>
`)

	got, err := parseCheckoutStoreOptions(raw)
	if err != nil {
		t.Fatalf("parseCheckoutStoreOptions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].ID != "col-439-store" || got[0].Name != "Continente Mafra" {
		t.Fatalf("unexpected first store: %#v", got[0])
	}
	if !got[0].Selected || got[0].Rank != 1 {
		t.Fatalf("expected first store selected rank 1: %#v", got[0])
	}
	if got[1].PostalCode != "2640-577" || got[1].Selected {
		t.Fatalf("unexpected second store: %#v", got[1])
	}
}

func TestParseCheckoutDeliveryPageRequiresAuth(t *testing.T) {
	t.Parallel()

	_, err := parseCheckoutDeliveryPage([]byte(`<a href="/login/?redirectUrl=https%3A%2F%2Fwww.continente.pt%2Fcheckout%2Fentrega%2F">login</a>`))
	if err == nil {
		t.Fatal("expected auth error")
	}
	var cliErr *cliError
	if !errors.As(err, &cliErr) || cliErr.code != 4 {
		t.Fatalf("expected auth cliError, got %#v", err)
	}
}

func TestCompactCheckoutStatusPayload(t *testing.T) {
	t.Parallel()

	payload := checkoutStatusPayload{
		Stage:          "Entrega",
		DeliveryMethod: "Click & Go",
		CurrentStore:   checkoutStore{ID: "439", Key: "col-439-store", Name: "Continente Mafra", PostalCode: "2640-453"},
		ShipmentID:     "430698332_001",
		ShipmentUUID:   "shipment-uuid-1",
		SlotsCount:     2,
		SelectedSlot: &checkoutSlot{
			SlotRef:     "1.2",
			DateLabel:   "Sabado 6 Jun",
			Start:       "12:00",
			End:         "14:00",
			Cost:        "0,00€",
			SchedulerID: "slot-2",
			ListID:      "list-2",
			OperatorID:  1268,
			Selected:    true,
		},
		PaymentURL: "https://www.continente.pt/checkout/pagamento/",
	}

	compact := compactCheckoutStatusPayload(payload)
	if compact.SelectedSlot == nil || compact.SelectedSlot.SlotRef != "1.2" {
		t.Fatalf("unexpected compact selected slot: %#v", compact.SelectedSlot)
	}
	if compact.SelectedSlot.Start != "12:00" {
		t.Fatalf("unexpected compact selected slot start: %#v", compact.SelectedSlot)
	}
}

func TestCompactCheckoutSlotsPayloadDropsBookingIDs(t *testing.T) {
	t.Parallel()

	payload := checkoutSlotsPayload{
		CurrentStore: checkoutStore{Name: "Continente Mafra"},
		ShipmentID:   "430698332_001",
		Slots: []checkoutSlot{{
			SlotRef:     "1.2",
			DateLabel:   "Sabado 6 Jun",
			Start:       "12:00",
			End:         "14:00",
			Cost:        "0,00€",
			SchedulerID: "slot-2",
			ListID:      "list-2",
			OperatorID:  1268,
			Selected:    true,
		}},
	}

	compact := compactCheckoutSlotsPayload(payload)
	if len(compact.Slots) != 1 {
		t.Fatalf("len(compact.Slots) = %d, want 1", len(compact.Slots))
	}
	if compact.Slots[0].SlotRef != "1.2" {
		t.Fatalf("unexpected compact slot: %#v", compact.Slots[0])
	}
}
