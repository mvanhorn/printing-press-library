// Hand-authored DICE GraphQL query layer.
//
// The DICE Partners API exposes its data as paginated Connection fields under
// the root `viewer` object, using the Relay `edges { node }` shape — e.g.
// `viewer { events(first, after, where) { edges { node { ... } } pageInfo } }`.
// The generator's auto-GraphQL templates assume root-level `nodes` connections
// and cannot express the viewer wrapper, edges, nested selections, or the
// typed `where` inputs DICE requires, so the data layer is hand-authored here
// against the generated transport (client.Query). This file is NOT generated
// and survives `generate --force`.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/client"
)

// GraphQL field selections per entity, derived from the SpectaQL schema at
// partners-endpoint.dice.fm/graphql/docs. Money fields are integer cents.
const (
	eventSelection = `id eventIdLive name state announceDatetime onSaleDatetime offSaleDatetime startDatetime endDatetime description hidden totalTicketAllocationQty currency url updatedAt genreTypes genres promoters
		artists { name }
		images { type url }
		venues { id name type city state region country postalCode latitude longitude timezoneName }
		ticketTypes { id name description price faceValue doorSalesPrice totalTicketAllocationQty archived }
		ticketPools { id name allocation }
		socialLinks { campaign default url }`

	// dob intentionally NOT selected: it was collected + stored but never read
	// by any command/analytic/MCP tool (data minimization — GDPR Art. 5(1)(c)).
	// Existing stores keep stale dob in resources.data until re-sync ages it out.
	fanSelection = `id firstName lastName email phoneNumber optInPartners`

	ticketSelection = `id code fullPrice commission diceCommission total claimedAt
		holder { ` + fanSelection + ` }
		ticketType { id name price }
		seat { name }
		priceTier { id name price }
		fees { category promoter dice }`

	orderSelection = `id purchasedAt quantity salesChannel fullPrice commission diceCommission total ipCity ipCountry
		fan { ` + fanSelection + ` }
		event { id name }
		address { town county postCode countryCode }
		fees { category promoter dice }
		adjustments { feesChange { category dice promoter } processedAt reason ticket { id } }`

	// orderSelectionWithTickets extends orderSelection with per-order nested
	// ticket IDs. This is the enriched selection used when --order-tickets is
	// set on sync. Fetching nested tickets is opt-in: even the lean ID-only
	// payload adds per-order fan-out and makes full syncs slower. Only the
	// ticket ID is fetched here; type name and per-ticket total come from the
	// already-synced tickets table via a local join in computeRevenueByAxisScoped.
	//
	// NOTE: the nested tickets fetch below should be validated against the live
	// API for payload size and query complexity before relying on it in
	// production — DICE may apply per-order ticket limits or add latency.
	orderSelectionWithTickets = orderSelection + `
		tickets { id }`

	returnSelection = `id ticketId returnedAt reason
		ticket { id }
		order { id event { id name } }`

	transferSelection = `id transferredAt
		tickets { id }
		orders { id event { id name } }`

	extraSelection = `id code fullPrice commission diceCommission total hasSeparateAccessBarcode
		holder { ` + fanSelection + ` }
		product { id name }
		variant { id name size sku }
		ticket { id }`

	// The nested genres connection needs its own first/last arg — DICE rejects
	// any unbounded Relay connection. 100 generously covers the child-genre
	// vocabulary under a single genre type, which is a small controlled set.
	genreTypeSelection = `id name
		genres(first: 100) { edges { node { id name } } }`
)

// connectionSpec maps a CLI resource to its viewer connection field, GraphQL
// where-input type, and node field selection.
type connectionSpec struct {
	field     string // viewer connection field, e.g. "events"
	whereType string // GraphQL where-input type, e.g. "EventWhereInput" ("" = no where arg)
	selection string // node field selection set
}

var diceConnections = map[string]connectionSpec{
	"events":    {field: "events", whereType: "EventWhereInput", selection: eventSelection},
	"tickets":   {field: "tickets", whereType: "TicketWhereInput", selection: ticketSelection},
	"orders":    {field: "orders", whereType: "OrderWhereInput", selection: orderSelection},
	"returns":   {field: "returns", whereType: "ReturnWhereInput", selection: returnSelection},
	"transfers": {field: "ticketTransfers", whereType: "TicketTransferWhereInput", selection: transferSelection},
	"extras":    {field: "extras", whereType: "ExtraWhereInput", selection: extraSelection},
	"genres":    {field: "genreTypes", whereType: "", selection: genreTypeSelection},
}

// effectiveConnectionSpec returns the connectionSpec for resource, substituting
// the enriched order selection (including nested tickets) when enrichOrders is
// true and the resource is "orders". All other resources are unaffected.
func effectiveConnectionSpec(resource string, enrichOrders bool) (connectionSpec, bool) {
	cs, ok := diceConnections[resource]
	if !ok {
		return connectionSpec{}, false
	}
	if enrichOrders && resource == "orders" {
		cs.selection = orderSelectionWithTickets
	}
	return cs, true
}

// buildConnectionQuery renders a paginated viewer-connection query. When latest
// is true it pages backward from the end (last/before) so the newest records
// come first: DICE connections are ordered oldest-first, so a forward page-1
// fetch returns the OLDEST rows, not the newest.
func buildConnectionQuery(cs connectionSpec, latest bool) string {
	whereDecl, whereArg := "", ""
	if cs.whereType != "" {
		whereDecl = fmt.Sprintf(", $where: %s", cs.whereType)
		whereArg = ", where: $where"
	}
	if latest {
		// No pageInfo selection: viewerConnectionPage only parses the forward
		// hasNextPage/endCursor fields, and --latest-only caps at a single
		// backward page, so a backward pageInfo { hasPreviousPage startCursor }
		// was dead — selected but never read. Drop it.
		return fmt.Sprintf(`query($last: Int!, $before: String%s) {
  viewer {
    %s(last: $last, before: $before%s) {
      edges { node { %s } }
    }
  }
}`, whereDecl, cs.field, whereArg, cs.selection)
	}
	return fmt.Sprintf(`query($first: Int!, $after: String%s) {
  viewer {
    %s(first: $first, after: $after%s) {
      edges { node { %s } }
      pageInfo { hasNextPage endCursor }
    }
  }
}`, whereDecl, cs.field, whereArg, cs.selection)
}

// viewerConnectionPage is the parsed shape of viewer.<field>.
type viewerConnectionPage struct {
	Edges []struct {
		Node json.RawMessage `json:"node"`
	} `json:"edges"`
	PageInfo struct {
		HasNextPage bool   `json:"hasNextPage"`
		EndCursor   string `json:"endCursor"`
	} `json:"pageInfo"`
}

// diceQuery POSTs a GraphQL query through the read-only transport path
// (PostQueryWithParams → doRead), which—unlike the generated client.Query
// (graphql.go), which rides the gated Post—is NOT short-circuited under
// PRINTING_PRESS_VERIFY=1. That keeps these read commands behaving like the
// generated endpoint commands under verify/dogfood. It parses the standard
// GraphQL envelope and surfaces errors (including access denial).
func diceQuery(ctx context.Context, c *client.Client, query string, variables map[string]any) (json.RawMessage, error) {
	req := client.GraphQLRequest{Query: query, Variables: variables}
	raw, _, err := c.PostQueryWithParams(ctx, "/graphql", nil, req)
	if err != nil {
		return nil, err
	}
	// Verify-mode synthetic envelope: surface as empty so read commands stay
	// green under PRINTING_PRESS_VERIFY without a live token.
	if isVerifySynthetic(raw) {
		return json.RawMessage(`{}`), nil
	}
	var resp client.GraphQLResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("decoding graphql response: %w", err)
	}
	if len(resp.Errors) > 0 {
		msgs := make([]string, len(resp.Errors))
		codes := make([]string, 0, len(resp.Errors))
		allDenial := true
		for i, e := range resp.Errors {
			msgs[i] = e.Message
			if e.Extensions.Code != "" {
				codes = append(codes, e.Extensions.Code)
			}
			if !isGraphQLAccessDenialCode(e.Extensions.Code) {
				allDenial = false
			}
		}
		// When every error is an access-denial code, surface the typed error so
		// sync's isSyncAccessWarning can warn-and-skip the resource instead of
		// hard-failing the whole run (matches the generated client.Query path).
		if allDenial && len(codes) > 0 {
			return nil, &client.GraphQLAccessDeniedError{Codes: codes, Messages: msgs}
		}
		return nil, fmt.Errorf("graphql: %s", strings.Join(msgs, "; "))
	}
	return resp.Data, nil
}

// isGraphQLAccessDenialCode reports whether a GraphQL extension code denotes
// access denial (mirrors the unexported helper in the generated client).
func isGraphQLAccessDenialCode(code string) bool {
	switch strings.ToUpper(code) {
	case "FORBIDDEN", "UNAUTHENTICATED", "UNAUTHORIZED", "PERMISSION_DENIED":
		return true
	}
	return false
}

// isVerifySynthetic reports whether raw is the verify-mode synthetic noop
// envelope (carries the __pp_verify_synthetic__ sentinel).
func isVerifySynthetic(raw json.RawMessage) bool {
	var probe struct {
		Synthetic bool `json:"__pp_verify_synthetic__"`
	}
	_ = json.Unmarshal(raw, &probe)
	return probe.Synthetic
}

// fetchConnectionStream paginates a viewer connection, calling onPage for each
// page as it is fetched. where is the GraphQL where-input value (nil for none).
// perPage caps the page size; max caps total nodes (0 = unbounded). startCursor
// resumes pagination. When latest is true it fetches a single newest page via
// backward pagination (DICE connections are oldest-first), ignoring max and
// startCursor. enrichOrders selects the heavier per-ticket nested selection for
// the orders connection; all other resources are unaffected.
//
// onPage receives the page's nodes, the cursor after this page, and the running
// total-fetched count so far. If onPage returns an error the loop stops and
// that error is returned. truncated is true when max was hit while more records
// remained.
func fetchConnectionStream(
	ctx context.Context,
	c *client.Client,
	resource string,
	where map[string]any,
	perPage, max int,
	startCursor string,
	latest bool,
	enrichOrders bool,
	onPage func(pageNodes []json.RawMessage, endCursor string, totalFetched int) error,
) (truncated bool, err error) {
	cs, ok := effectiveConnectionSpec(resource, enrichOrders)
	if !ok {
		return false, fmt.Errorf("unknown DICE connection %q", resource)
	}
	query := buildConnectionQuery(cs, latest)
	if perPage <= 0 {
		perPage = 50
	}

	// latest-only: a single newest page via backward (last) pagination.
	if latest {
		vars := map[string]any{"last": perPage}
		if cs.whereType != "" && len(where) > 0 {
			vars["where"] = where
		}
		data, err := diceQuery(ctx, c, query, vars)
		if err != nil {
			return false, err
		}
		nodes, err := parseConnectionNodes(data, cs.field)
		if err != nil {
			return false, err
		}
		return false, onPage(nodes, "", len(nodes))
	}

	cursor := startCursor
	totalFetched := 0
	for {
		vars := map[string]any{"first": perPage}
		if cursor != "" {
			vars["after"] = cursor
		}
		if cs.whereType != "" && len(where) > 0 {
			vars["where"] = where
		}
		data, err := diceQuery(ctx, c, query, vars)
		if err != nil {
			return false, err
		}
		nodes, hasNext, endCursor, parseErr := parseConnectionPage(data, cs.field)
		if parseErr != nil {
			return false, parseErr
		}

		// Apply the max cap per page: deliver a trimmed slice and stop.
		if max > 0 && totalFetched+len(nodes) >= max {
			keep := max - totalFetched
			truncated = keep < len(nodes) || hasNext
			pageNodes := nodes[:keep]
			totalFetched += len(pageNodes)
			if cbErr := onPage(pageNodes, endCursor, totalFetched); cbErr != nil {
				return truncated, cbErr
			}
			return truncated, nil
		}

		totalFetched += len(nodes)
		if cbErr := onPage(nodes, endCursor, totalFetched); cbErr != nil {
			return false, cbErr
		}
		if !hasNext || endCursor == "" {
			return false, nil
		}
		cursor = endCursor
	}
}

// fetchConnection paginates a viewer connection and returns the node payloads.
// where is the GraphQL where-input value (nil for none). perPage caps the page
// size; max caps total nodes (0 = unbounded). startCursor resumes pagination.
// When latest is true it fetches a single newest page via backward pagination
// (DICE connections are oldest-first), ignoring max and startCursor.
//
// It returns the collected nodes, the final endCursor (for resumable sync), and
// truncated — true when max was hit while more records remained, so callers can
// warn instead of silently dropping data.
//
// Implemented on top of fetchConnectionStream so read commands (runList, door)
// keep working unchanged while the streaming core is shared with sync.
func fetchConnection(ctx context.Context, c *client.Client, resource string, where map[string]any, perPage, max int, startCursor string, latest bool) ([]json.RawMessage, string, bool, error) {
	var all []json.RawMessage
	var finalCursor string
	truncated, err := fetchConnectionStream(ctx, c, resource, where, perPage, max, startCursor, latest, false,
		func(pageNodes []json.RawMessage, endCursor string, _ int) error {
			all = append(all, pageNodes...)
			finalCursor = endCursor
			return nil
		},
	)
	return all, finalCursor, truncated, err
}

// parseConnectionPage extracts the node payloads, hasNextPage, and endCursor from
// a single viewer.<field> connection page. A missing field (e.g. verify-mode
// synthetic envelope) returns no nodes and a stop signal rather than an error.
func parseConnectionPage(data json.RawMessage, field string) (nodes []json.RawMessage, hasNext bool, endCursor string, err error) {
	var root struct {
		Viewer map[string]json.RawMessage `json:"viewer"`
	}
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, false, "", fmt.Errorf("parsing viewer response: %w", err)
	}
	connRaw, ok := root.Viewer[field]
	if !ok {
		return nil, false, "", nil
	}
	var page viewerConnectionPage
	if err := json.Unmarshal(connRaw, &page); err != nil {
		return nil, false, "", fmt.Errorf("parsing connection %q: %w", field, err)
	}
	out := make([]json.RawMessage, 0, len(page.Edges))
	for _, e := range page.Edges {
		out = append(out, e.Node)
	}
	return out, page.PageInfo.HasNextPage, page.PageInfo.EndCursor, nil
}

// parseConnectionNodes extracts just the node payloads from a viewer.<field>
// response (used by the latest-only single-page path).
func parseConnectionNodes(data json.RawMessage, field string) ([]json.RawMessage, error) {
	nodes, _, _, err := parseConnectionPage(data, field)
	return nodes, err
}

// eqWhere builds a single-field equality where-input value: {field: {eq: val}}.
// DICE operator inputs (OperatorsIdInput, OperatorsEventStateInput, etc.) all
// accept an `eq` member; EqStringInput/EqBooleanInput take `eq` directly too.
func eqWhere(field string, value any) map[string]any {
	return map[string]any{field: map[string]any{"eq": value}}
}

// mergeWhere combines where clauses; later keys win.
func mergeWhere(clauses ...map[string]any) map[string]any {
	out := map[string]any{}
	for _, c := range clauses {
		for k, v := range c {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// extractID pulls the "id" field from a node payload (best-effort).
func extractID(node json.RawMessage) string {
	var probe struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(node, &probe)
	return probe.ID
}

// nodeQuery fetches a single object by global ID via the root `node` query,
// using an inline fragment for the given GraphQL type.
func fetchNodeByID(ctx context.Context, c *client.Client, gqlType, selection, id string) (json.RawMessage, error) {
	query := fmt.Sprintf(`query($id: ID!) {
  node(id: $id) {
    ... on %s { %s }
  }
}`, gqlType, selection)
	data, err := diceQuery(ctx, c, query, map[string]any{"id": id})
	if err != nil {
		return nil, err
	}
	var root struct {
		Node json.RawMessage `json:"node"`
	}
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parsing node response: %w", err)
	}
	if len(root.Node) == 0 || strings.TrimSpace(string(root.Node)) == "null" {
		return nil, fmt.Errorf("no %s found with id %q", strings.ToLower(gqlType), id)
	}
	return root.Node, nil
}
