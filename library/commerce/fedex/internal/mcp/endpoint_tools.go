// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

// endpointTool is one fixed FedEx REST operation exposed as its own MCP tool.
// The method and path are compile-time constants; callers supply only the
// JSON request body, so no tool can be pointed at a different endpoint.
type endpointTool struct {
	ID      string
	Method  string
	Path    string
	Summary string
}

// endpointTools covers every operation in the FedEx spec this CLI was printed
// from. Each one is registered as a typed tool named after its CLI command
// (for example shipments.create -> shipments_create).
var endpointTools = []endpointTool{
	{ID: "addresses.validate", Method: "POST", Path: "/address/v1/addresses/resolve", Summary: "Validate one or more addresses (resolved/standardized form, classification, optional resolved coordinates)"},
	{ID: "availability.services", Method: "POST", Path: "/availability/v1/packageandserviceoptions", Summary: "Get available services for an origin/destination pair"},
	{ID: "availability.special_services", Method: "POST", Path: "/availability/v1/specialserviceoptions", Summary: "Get available special-service options (alcohol, dangerous goods, signature requirements, etc.)"},
	{ID: "availability.transit_times", Method: "POST", Path: "/availability/v1/transittimes", Summary: "Get transit times for an origin/destination pair across services"},
	{ID: "consolidations.add_shipment", Method: "POST", Path: "/ship/v1/consolidations/shipments", Summary: "Add shipments to a consolidation"},
	{ID: "consolidations.confirm", Method: "POST", Path: "/ship/v1/consolidations/confirmations", Summary: "Confirm a consolidation (locks rates and triggers shipping)"},
	{ID: "consolidations.confirm_results", Method: "POST", Path: "/ship/v1/consolidations/confirmationresults", Summary: "Get results of a consolidation confirmation"},
	{ID: "consolidations.create", Method: "POST", Path: "/ship/v1/consolidations", Summary: "Create a consolidation"},
	{ID: "consolidations.delete", Method: "PUT", Path: "/ship/v1/consolidations/delete", Summary: "Delete a consolidation"},
	{ID: "consolidations.delete_shipment", Method: "PUT", Path: "/ship/v1/consolidations/shipments/delete", Summary: "Remove a shipment from a consolidation"},
	{ID: "consolidations.modify", Method: "PUT", Path: "/ship/v1/consolidations", Summary: "Modify a consolidation"},
	{ID: "consolidations.results", Method: "POST", Path: "/ship/v1/consolidations/results", Summary: "Get consolidation results"},
	{ID: "consolidations.retrieve", Method: "POST", Path: "/ship/v1/consolidations/retrieve", Summary: "Retrieve a consolidation"},
	{ID: "endofday.close", Method: "POST", Path: "/ship/v1/endofday/", Summary: "Submit Ground end-of-day close (manifest packages shipped today)"},
	{ID: "endofday.modify", Method: "PUT", Path: "/ship/v1/endofday/", Summary: "Re-submit or modify a previous end-of-day close"},
	{ID: "freight_pickups.availability", Method: "POST", Path: "/pickup/v1/freight/pickups/availabilities", Summary: "Check freight pickup availability"},
	{ID: "freight_pickups.cancel", Method: "PUT", Path: "/pickup/v1/freight/pickups/cancel", Summary: "Cancel a freight pickup"},
	{ID: "freight_pickups.create", Method: "POST", Path: "/pickup/v1/freight/pickups", Summary: "Schedule a freight (LTL) pickup"},
	{ID: "freight_shipments.create", Method: "POST", Path: "/ship/v1/freight/shipments", Summary: "Create a freight (LTL) shipment"},
	{ID: "globaltrade.regulatory", Method: "POST", Path: "/globaltrade/v1/shipments/regulatorydetails/retrieve", Summary: "Get regulatory details (HS codes, harmonized system, country restrictions)"},
	{ID: "locations.find", Method: "POST", Path: "/location/v1/locations", Summary: "Find FedEx locations near an address or postal code"},
	{ID: "openship.add_package", Method: "POST", Path: "/ship/v1/openshipments/packages", Summary: "Add a package to an open shipment"},
	{ID: "openship.create", Method: "POST", Path: "/ship/v1/openshipments", Summary: "Create an open (uncommitted) multi-piece shipment"},
	{ID: "openship.delete", Method: "PUT", Path: "/ship/v1/openshipments/delete", Summary: "Delete an open shipment"},
	{ID: "openship.delete_package", Method: "PUT", Path: "/ship/v1/openshipments/packages/delete", Summary: "Delete a package from an open shipment"},
	{ID: "openship.modify", Method: "PUT", Path: "/ship/v1/openshipments", Summary: "Modify an open shipment"},
	{ID: "openship.modify_package", Method: "PUT", Path: "/ship/v1/openshipments/packages", Summary: "Modify a package in an open shipment"},
	{ID: "openship.results", Method: "POST", Path: "/ship/v1/openshipments/results", Summary: "Get results of a confirmed open shipment"},
	{ID: "openship.retrieve", Method: "POST", Path: "/ship/v1/openshipments/retrieve", Summary: "Retrieve an open shipment by index"},
	{ID: "openship.retrieve_package", Method: "POST", Path: "/ship/v1/openshipments/packages/retrieve", Summary: "Retrieve a specific package from an open shipment"},
	{ID: "pickups.availability", Method: "POST", Path: "/pickup/v1/pickups/availabilities", Summary: "Check whether pickup is available for a postal code on a date"},
	{ID: "pickups.cancel", Method: "PUT", Path: "/pickup/v1/pickups/cancel", Summary: "Cancel a previously-scheduled Express/Ground pickup"},
	{ID: "pickups.create", Method: "POST", Path: "/pickup/v1/pickups", Summary: "Schedule an Express or Ground pickup at a specified address"},
	{ID: "postal.validate", Method: "POST", Path: "/country/v1/postal/validate", Summary: "Validate that FedEx services a postal code in a country"},
	{ID: "rates.quote", Method: "POST", Path: "/rate/v1/rates/quotes", Summary: "Quote rates for a shipment (Express/Ground/Home/Ground Economy)"},
	{ID: "rates.quote_freight", Method: "POST", Path: "/rate/v1/freight/rates/quotes", Summary: "Quote rates for freight (LTL) shipments"},
	{ID: "shipments.cancel", Method: "PUT", Path: "/ship/v1/shipments/cancel", Summary: "Cancel a shipment by tracking number"},
	{ID: "shipments.create", Method: "POST", Path: "/ship/v1/shipments", Summary: "Create a shipment and generate label(s)"},
	{ID: "shipments.results", Method: "POST", Path: "/ship/v1/shipments/results", Summary: "Retrieve results of an asynchronous shipment job"},
	{ID: "shipments.tag", Method: "POST", Path: "/ship/v1/shipments/tag", Summary: "Create a Ground Call Tag (return label) for an existing shipment"},
	{ID: "shipments.tag_cancel", Method: "PUT", Path: "/ship/v1/shipments/tag/cancel", Summary: "Cancel a Ground Call Tag"},
	{ID: "shipments.validate", Method: "POST", Path: "/ship/v1/shipments/packages/validate", Summary: "Validate a shipment package without creating a label (catches rejections before billing)"},
	{ID: "track.associated", Method: "POST", Path: "/track/v1/associatedshipments", Summary: "Track shipments associated with a master tracking number (multi-piece)"},
	{ID: "track.documents", Method: "POST", Path: "/track/v1/trackingdocuments", Summary: "Retrieve tracking documents (signature proof of delivery, etc.)"},
	{ID: "track.notifications", Method: "POST", Path: "/track/v1/notifications", Summary: "Configure tracking notifications (email/SMS) for a shipment"},
	{ID: "track.number", Method: "POST", Path: "/track/v1/trackingnumbers", Summary: "Track up to 30 tracking numbers in one call (returns full event timeline per shipment)"},
	{ID: "track.reference", Method: "POST", Path: "/track/v1/referencenumbers", Summary: "Track by reference number (PO/customer ref/RMA)"},
	{ID: "track.tcn", Method: "POST", Path: "/track/v1/tcn", Summary: "Track by Transportation Control Number (military/government use)"},
}
