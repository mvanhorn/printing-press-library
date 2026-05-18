package instacart

import "encoding/json"

// Request variables used by the CLI's outgoing GraphQL operations. These are
// modeled after the live variables captured during the sniff; unused fields
// are omitted with omitempty so the server can apply defaults.

type Coordinates struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type ShopCollectionScopedVars struct {
	RetailerSlug           string       `json:"retailerSlug"`
	PostalCode             string       `json:"postalCode"`
	Coordinates            *Coordinates `json:"coordinates,omitempty"`
	AddressID              string       `json:"addressId,omitempty"`
	AllowCanonicalFallback bool         `json:"allowCanonicalFallback"`
}

// PATCH (fix-shop-collection-coordinates):
// NewShopCollectionScopedVars builds the variable bundle and omits
// `coordinates` entirely when neither latitude nor longitude is set, instead
// of sending the zero-valued {0,0} pair. Instacart's `UsersCoordinatesInput`
// rejects {0,0} as invalid (it's a point in the Atlantic Ocean) which surfaces
// as a server-side "no shops" / "variable was provided invalid value" failure
// at every search/add/cart-show bootstrap when the user has only configured
// postal_code. The local op declaration types coordinates as optional, so
// omitting it is the correct shape when the caller has none to provide.
// Reported in mvanhorn/printing-press-library#501.
func NewShopCollectionScopedVars(retailerSlug, postalCode, addressID string, latitude, longitude float64) ShopCollectionScopedVars {
	v := ShopCollectionScopedVars{
		RetailerSlug: retailerSlug,
		PostalCode:   postalCode,
		AddressID:    addressID,
	}
	if latitude != 0 || longitude != 0 {
		v.Coordinates = &Coordinates{Latitude: latitude, Longitude: longitude}
	}
	return v
}

type AutosuggestionsVars struct {
	RetailerInventorySessionToken string `json:"retailerInventorySessionToken,omitempty"`
	Query                         string `json:"query"`
	AutosuggestionSessionID       string `json:"autosuggestionSessionId,omitempty"`
}

type ActiveCartIDVars struct {
	AddressID string `json:"addressId"`
	ShopID    string `json:"shopId"`
}

type CartItemCountVars struct {
	ID string `json:"id"`
}

type ShopBasketsVars struct {
	ShopID    string `json:"shopId"`
	AddressID string `json:"addressId"`
}

type ItemsVars struct {
	IDs        []string `json:"ids"`
	ShopID     string   `json:"shopId,omitempty"`
	ZoneID     string   `json:"zoneId,omitempty"`
	PostalCode string   `json:"postalCode,omitempty"`
}

// UpdateCartItemsVars is the mutation payload for add/remove/update cart items.
// We keep `trackingParams` as an opaque json.RawMessage so callers can pass an
// empty object `{}` (the server accepts it) or a richer object if captured.
type UpdateCartItemsVars struct {
	CartItemUpdates  []CartItemUpdate `json:"cartItemUpdates"`
	RequestTimestamp int64            `json:"requestTimestamp,omitempty"`
	CartType         string           `json:"cartType,omitempty"`
	CartID           string           `json:"cartId,omitempty"`
}

type CartItemUpdate struct {
	ItemID         string          `json:"itemId"`
	Quantity       float64         `json:"quantity"`
	QuantityType   string          `json:"quantityType"`
	TrackingParams json.RawMessage `json:"trackingParams,omitempty"`
}

// Parsed response shapes. These cover what our commands read; anything else
// stays in the raw json.RawMessage.

type CurrentUser struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

type CurrentUserFieldsResponse struct {
	CurrentUser *CurrentUser `json:"currentUser"`
}

type PersonalActiveCart struct {
	ID        string       `json:"id"`
	ItemCount int          `json:"itemCount"`
	Retailer  CartRetailer `json:"retailer"`
}

type CartRetailer struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type PersonalActiveCartsResponse struct {
	UserCarts struct {
		ID    string               `json:"id"`
		Carts []PersonalActiveCart `json:"carts"`
	} `json:"userCarts"`
}

type ActiveCartIDResponse struct {
	ActiveCartID struct {
		ID string `json:"id"`
	} `json:"activeCartId"`
}

type CartItemCountResponse struct {
	Cart struct {
		ID        string `json:"id"`
		ItemCount int    `json:"itemCount"`
	} `json:"cart"`
}

type UpdateCartItemsResponse struct {
	UpdateCartItems struct {
		Typename         string                `json:"__typename"`
		Cart             *UpdateCartResultCart `json:"cart,omitempty"`
		UpdatedItemIDs   []UpdatedItemID       `json:"updatedItemIds,omitempty"`
		RequestTimestamp float64               `json:"requestTimestamp,omitempty"`
		ErrorType        string                `json:"errorType,omitempty"`
	} `json:"updateCartItems"`
}

type UpdateCartResultCart struct {
	ID         string `json:"id"`
	CartType   string `json:"cartType"`
	UpdatedAt  string `json:"updatedAt"`
	ItemCount  int    `json:"itemCount"`
	RetailerID string `json:"retailerId"`
}

type UpdatedItemID struct {
	ItemID string `json:"itemId"`
}
