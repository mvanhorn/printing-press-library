// Copyright 2026 salmonumbrella. Licensed under Apache-2.0. See LICENSE.

package cli

func newReservationCreateBody(classSessionID, paymentOptionID, reservationType string) map[string]any {
	attrs := map[string]any{}
	if classSessionID != "" {
		attrs["class_session_id"] = classSessionID
	}
	if paymentOptionID != "" {
		attrs["payment_option_id"] = paymentOptionID
	}
	if reservationType != "" {
		attrs["reservation_type"] = reservationType
	}
	return newReservationCreateBodyFromAttributes(attrs)
}

// PATCH(greptile #487): Mariana Tek accepts /me/reservations as JSONAPI
// attributes. The generated command built nested class_session/payment_option
// objects; normalize those IDs to the same wire shape proven by watch/book-regular.
func newReservationCreateBodyFromGeneratedBody(generated map[string]any) map[string]any {
	attrs := make(map[string]any, len(generated))
	for key, value := range generated {
		switch key {
		case "class_session":
			copyNestedID(attrs, value, "class_session_id")
		case "payment_option":
			copyNestedID(attrs, value, "payment_option_id")
		default:
			attrs[key] = value
		}
	}
	return newReservationCreateBodyFromAttributes(attrs)
}

func newReservationCreateBodyFromAttributes(attrs map[string]any) map[string]any {
	return map[string]any{
		"data": map[string]any{
			"type":       "reservation",
			"attributes": attrs,
		},
	}
}

func copyNestedID(attrs map[string]any, value any, targetKey string) {
	nested, ok := value.(map[string]any)
	if !ok {
		return
	}
	id, ok := nested["id"].(string)
	if !ok || id == "" {
		return
	}
	attrs[targetKey] = id
}
