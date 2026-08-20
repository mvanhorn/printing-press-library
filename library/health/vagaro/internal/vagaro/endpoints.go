// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package vagaro

import "context"

// Services fetches and flattens a business's service menu.
func (c *Client) Services(ctx context.Context, businessID string) ([]ServiceRow, error) {
	d, err := c.PostWebsiteAPI(ctx, MethodServices, map[string]any{
		"businessID":  businessID,
		"loginUserID": "",
		"bookText":    "Book",
	})
	if err != nil {
		return nil, err
	}
	return ParseServices(d), nil
}

// Staff fetches a business's service providers.
func (c *Client) Staff(ctx context.Context, businessID string) ([]Provider, error) {
	d, err := c.PostWebsiteAPI(ctx, MethodStaff, map[string]any{
		"businessID":  businessID,
		"loginUserID": "",
		"bookText":    "Book",
	})
	if err != nil {
		return nil, err
	}
	return ParseStaff(d), nil
}

// Reviews fetches a page of reviews. providerID filters to one provider when
// non-empty; pageSize defaults to 20 when <= 0.
func (c *Client) Reviews(ctx context.Context, businessID, providerID string, pageSize int) ([]Review, error) {
	if pageSize <= 0 {
		pageSize = 20
	}
	d, err := c.PostWebsiteAPI(ctx, MethodReviews, map[string]any{
		"currentPageIndex":  1,
		"pageIndex":         1,
		"PageSize":          pageSize,
		"businessID":        businessID,
		"SortType":          "1",
		"FilterType":        "1",
		"ReviewGUID":        "",
		"ServiceProviderId": providerID,
		"ReviewID":          0,
	})
	if err != nil {
		return nil, err
	}
	return ParseReviews(d), nil
}

// Availability fetches available appointment slots for the week starting at
// appDate (in "Ddd Mon-DD-YYYY" format). csvSPID may be empty for any provider.
func (c *Client) Availability(ctx context.Context, businessID, csvServiceID, csvSPID, appDate string) ([]SlotGroup, error) {
	d, err := c.PostWebsiteAPI(ctx, MethodAvailability, map[string]any{
		"lAppointmentID":          "",
		"businessID":              businessID,
		"csvServiceID":            csvServiceID,
		"csvSPID":                 csvSPID,
		"AppDate":                 appDate,
		"StyleID":                 nil,
		"isPublic":                true,
		"isOutcallAppointment":    false,
		"strCurrencySymbol":       "$",
		"IsFromWidgetPage":        "false",
		"isFromShopAdmin":         false,
		"isMoveBack":              false,
		"BusinessPackageID":       0,
		"PromotionID":             "",
		"TIME_ZONE":               -8,
		"CUSTOM_DAY_LIGHT_SAVING": true,
		"DAY_LIGHT_SAVING":        "Y",
		"CountryID":               1,
		"CustomerTimezone":        -8,
		"Customerzoneid":          "",
		"CustomerCulture":         "1",
		"CustIsDayLightSaving":    true,
	})
	if err != nil {
		return nil, err
	}
	return ExtractSlots(d, appDate, csvSPID), nil
}
