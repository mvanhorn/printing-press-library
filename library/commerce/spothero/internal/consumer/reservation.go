package consumer

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// reservationWire matches live GET /api/v1/reservations/ payloads (Sep 2026 smoke).
type reservationWire struct {
	RentalID         FlexInt `json:"rental_id"`
	DisplayID        string  `json:"display_id"`
	QRCodeUUID       string  `json:"qrcode_uuid"`
	IDLegacy         FlexID  `json:"id"`
	Price            FlexInt `json:"price"`
	PriceCents       FlexInt `json:"price_cents"`
	FacilityID       FlexInt `json:"facility_id"`
	FacilityTitle    string  `json:"facility_title"`
	Starts           string  `json:"starts"`
	Ends             string  `json:"ends"`
	Status           string  `json:"status"`
	ReservationState string  `json:"reservation_status"`
	StallName        string  `json:"stall_name"`
	// CancellationAllowed is who may cancel ("customer"), a string on the live API.
	CancellationOK   string        `json:"cancellation_allowed"`
	AccessKey        string        `json:"access_key"`
	IsCancellable    bool          `json:"is_cancellable"`
	Cancellable      bool          `json:"cancellable"`
	ConfirmationCode string        `json:"confirmation_code"`
	Barcode          string        `json:"barcode"`
	Facility         *facilityWire `json:"facility"`
}

type facilityWire struct {
	ID    FlexInt `json:"id"`
	Title string  `json:"title"`
	Name  string  `json:"name"`
}

// Reservation is the normalized CLI-facing reservation record.
type Reservation struct {
	ID               string `json:"id"`
	RentalID         int    `json:"rental_id"`
	DisplayID        string `json:"display_id,omitempty"`
	QRCodeUUID       string `json:"qrcode_uuid,omitempty"`
	Status           string `json:"status"`
	FacilityID       int    `json:"facility_id"`
	FacilityTitle    string `json:"facility_title"`
	Starts           string `json:"starts"`
	Ends             string `json:"ends"`
	ConfirmationCode string `json:"confirmation_code,omitempty"`
	PriceCents       int    `json:"price_cents"`
	Price            string `json:"price"`
	Cancellable      bool   `json:"cancellable"`
	// AccessKey is a per-reservation secret the refund call may need; keep it out
	// of output.
	AccessKey string `json:"-"`
}

func parseReservationsPayload(data json.RawMessage) ([]Reservation, error) {
	if len(data) == 0 || string(data) == "null" {
		return nil, nil
	}
	var wrapped struct {
		Results []json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal(data, &wrapped); err == nil && len(wrapped.Results) > 0 {
		return parseReservationItems(wrapped.Results)
	}
	if err := json.Unmarshal(data, &wrapped); err == nil && wrapped.Results != nil {
		return parseReservationItems(wrapped.Results)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(data, &items); err == nil {
		return parseReservationItems(items)
	}
	var one reservationWire
	if err := json.Unmarshal(data, &one); err == nil {
		return []Reservation{normalizeReservation(one)}, nil
	}
	return nil, fmt.Errorf("unrecognized reservations payload shape")
}

func parseReservationItems(items []json.RawMessage) ([]Reservation, error) {
	out := make([]Reservation, 0, len(items))
	for _, raw := range items {
		var wire reservationWire
		if err := json.Unmarshal(raw, &wire); err != nil {
			return nil, fmt.Errorf("reservation item: %w", err)
		}
		out = append(out, normalizeReservation(wire))
	}
	return out, nil
}

func normalizeReservation(w reservationWire) Reservation {
	priceCents := int(w.Price)
	if priceCents == 0 {
		priceCents = int(w.PriceCents)
	}
	title := w.FacilityTitle
	facilityID := int(w.FacilityID)
	if w.Facility != nil {
		if title == "" {
			title = w.Facility.Title
			if title == "" {
				title = w.Facility.Name
			}
		}
		if facilityID == 0 {
			facilityID = int(w.Facility.ID)
		}
	}
	cancellable := w.IsCancellable || w.Cancellable || w.CancellationOK == "customer"
	if title == "" {
		title = w.StallName
	}
	status := w.ReservationState
	if status == "" {
		status = w.Status
	}
	id := reservationPublicID(w)
	return Reservation{
		ID:               id,
		RentalID:         int(w.RentalID),
		DisplayID:        w.DisplayID,
		QRCodeUUID:       w.QRCodeUUID,
		Status:           status,
		AccessKey:        w.AccessKey,
		FacilityID:       facilityID,
		FacilityTitle:    title,
		Starts:           w.Starts,
		Ends:             w.Ends,
		ConfirmationCode: w.ConfirmationCode,
		PriceCents:       priceCents,
		Price:            formatUSD(priceCents),
		Cancellable:      cancellable,
	}
}

func reservationPublicID(w reservationWire) string {
	if w.RentalID != 0 {
		return strconv.Itoa(int(w.RentalID))
	}
	if w.DisplayID != "" {
		return w.DisplayID
	}
	if w.QRCodeUUID != "" {
		return w.QRCodeUUID
	}
	if s := w.IDLegacy.String(); s != "" {
		return s
	}
	return ""
}

func formatUSD(cents int) string {
	if cents == 0 {
		return ""
	}
	dollars := cents / 100
	rem := cents % 100
	if rem < 0 {
		rem = -rem
	}
	return fmt.Sprintf("$%d.%02d", dollars, rem)
}
