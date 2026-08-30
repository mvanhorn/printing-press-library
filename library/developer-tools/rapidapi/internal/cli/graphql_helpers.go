// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: GraphQL document registry + friendly-flag executor for the RapidAPI hub gateway.

package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// gqlDocs maps operation name -> the exact validated GraphQL query document
// extracted from the RapidAPI hub web app bundle (chunk 6944) and live-validated
// against POST https://rapidapi.com/gateway/graphql.
var gqlDocs = map[string]string{
	"searchApis": `query searchApis($searchApiWhereInput: SearchApiWhereInput!, $paginationInput: PaginationInput, $searchApiOrderByInput: SearchApiOrderByInput) { products: searchApis(where: $searchApiWhereInput, pagination: $paginationInput, orderBy: $searchApiOrderByInput) { nodes { id thumbnail name description slugifiedName pricing updatedAt categoryName isSavedApi title visibility apiCategory { name color } score { popularityScore avgLatency avgServiceLevel avgSuccessRate } version { tags { id status tagdefinition type value } } user: User { id username slugifiedName: username name type parents { id name slugifiedName type thumbnail } } } facets { category { key count } } pageInfo { endCursor hasNextPage hasPreviousPage startCursor } total queryID replicaIndex } }`,

	"getCategoriesByCtx": `query getCategoriesByCtx($limit: Int) { categoriesByCtx(limit: $limit) { id name weight thumbnail shortDescription slugifiedName color } }`,

	"GetCollectionsCollapsed": `query GetCollectionsCollapsed($page: Int, $limit: Int) { collections: collapsedCollections(orderByField: "weight", orderDirection: asc, page: $page, limit: $limit) { id title slugifiedKey weight thumbnail shortDescription items: detailedApisAndSpotlights(limit: $limit) { ...CollectionSpotlightInfo ...ApiInfo } } } fragment ApiInfo on Api { __typename id name title description visibility slugifiedName pricing updatedAt category thumbnail isSavedApi categoryId apiCategory { name color } score { popularityScore avgLatency avgServiceLevel avgSuccessRate } } fragment CollectionSpotlightInfo on CollectionSpotlight { __typename id title description spotlightUrl thumbnailUrl }`,

	"getCollectionBySlug": `query getCollectionBySlug($slug: String!) { collection: collectionBySlugifiedKeyV3(slugifiedKey: $slug) { id title thumbnail shortDescription longDescription slugifiedKey collectionType: collection_type post { id link title thumbnail image } apis(apisSkip: 0) { __typename ... on Api { id name slugifiedName description isFavorite pricing thumbnail } ... on CollectionSpotlight { id title description spotlightUrl thumbnailUrl } } } }`,

	"getApiBySlugAndOwner": `query getApiBySlugAndOwner($apiOwnerSlug: String, $apiSlug: String, $withEndpoints: Boolean! = false) { apiBySlugifiedNameAndOwnerName(slugifiedName: $apiSlug, ownerName: $apiOwnerSlug) { id name title slugifiedName description thumbnail gatewayIds createdAt status longDescription apiType allowedContext isCtxSubscriber quality { score } owner { id name slugifiedName type thumbnail username parents { id name slugifiedName type thumbnail } } versions { id name current createdAt versionStatus } version { endpoints(pagingArgs: { limit: 50 }) @include(if: $withEndpoints) { id isGraphQL route method name description apiversion group index } } billingPlans { id name recommended visibility } billingItems { id name title displayName type allEndpoints } billingFeatures { id name description type } rating { rating votes bestRating } activeUserRating { rating } documentation { readme { text } } termsOfService { id text name } subscriptionsCount websiteUrl spotlights { apiId thumbnailURL title type weight description id published spotlightURL status slugifiedName } } }`,

	"getUserProfile": `query getUserProfile($username: String!) { userProfile: userByUsername(username: $username) { __typename id thumbnail githubUsername facebookUsername company position location name username emailPublic githubUrl githubUrlPublic createdAt bio followedApis { api { id name slugifiedName thumbnail description pricing } } publishedApisList { id name slugifiedName thumbnail description pricing updatedAt } attributes { rows { attributeName attributeValue } } } }`,

	"getHubMetrics": `query getHubMetrics($where: MetricsInput!) { publicMetrics(where: $where) { publicApis(where: $where) { totalValue } users(where: $where) { totalValue currentPeriodValue previousPeriodValue } activeApiConsumers(where: $where) { totalValue currentPeriodValue previousPeriodValue } totalApiTraffic(where: $where) { totalValue currentPeriodValue previousPeriodValue } } }`,

	"activeUser": `query activeUser { activeUser { name id mashapeId email username entity thumbnail billingType paymentProvider isUserCreatedBySSO verified organizations { id name slugifiedName teams { id name slugifiedName isTeamMember } } } tenant { id } }`,

	"getUserSavedApis": `query getUserSavedApis { userSavedApis: getUserSavedApis { api { id name title slugifiedName thumbnail description pricing updatedAt } } }`,

	"getApiSubscriptions": `query getApiSubscriptions($input: GetSubscriptionInput) { getApiSubscriptions(input: $input) { count rows { id status userId parentId billingPlanVersionId apiId apiVersionId stripeId hasRelatedActiveSubscriptions createdAt canceledAt mashapeId billingPlanVersion { id name current status period price pricing localePrice { price symbol } billinglimits { item period amount limitType unlimited billingitem { name displayName } } } } } }`,

	"getNotifications": `query getNotifications($userId: Int, $limit: Int, $offset: Int) { newNotificationsByUserId(userId: $userId, limit: $limit, offset: $offset) { id type createdAt isRead title body callToAction image any __typename } }`,

	"getWorkspaceData": `query getWorkspaceData($fromDate: DateTime, $toDate: DateTime) { workspaceData { ownedApis { apis { id name slugifiedName thumbnail description } metrics(fromDate: $fromDate, toDate: $toDate) { averageErrorRate averageLatency subscribers totalApis totalRequests totalSales } } subscribedApis { apis { id name slugifiedName thumbnail description } metrics(fromDate: $fromDate, toDate: $toDate) { lastPayment { totalAmount createdAt } totalRequests } subscriptions(statuses: ["ACTIVE", "BLOCKED"]) { status } } invitedApis { id name slugifiedName thumbnail description } } }`,

	"apiTrafficAnalytics": `query apiTrafficAnalytics($where: AnalyticsStatsInput!) { apiTrafficAnalytics(where: $where) { date requests latency errors } }`,

	"GetGateways": `query GetGateways { getGateways { id dns type status version isDefault lastActive serviceStatus } }`,

	"getTagsList": `query getTagsList { getTagsList { id name values type } }`,

	"organizations": `query organizations($where: OrganizationWhereInput) { organizations(where: $where) { id name slugifiedName } }`,

	"consumerAnalytics": `query consumerAnalytics($where: MetricsInput!) { metrics(where: $where) { apiCallVolumeConsumer { currentPeriodValue previousPeriodValue } totalApiCostConsumer { currentPeriodValue previousPeriodValue } totalActiveApiConsumer { currentPeriodValue previousPeriodValue } apiUsageDaysConsumer { currentPeriodValue previousPeriodValue } apiSuccessRateConsumer { currentPeriodValue previousPeriodValue } } }`,

	"providerAnalyticsMetrics": `query providerAnalyticsMetrics($where: MetricsInput!) { metrics(where: $where) { apiCallVolumeProvider { currentPeriodValue previousPeriodValue } totalApiRevenue { currentPeriodValue previousPeriodValue } totalApiEstimatedRevenueByProvider { currentPeriodValue previousPeriodValue } apiErrorRateProvider { currentPeriodValue previousPeriodValue } pricePerThousandCallsByProvider { currentPeriodValue previousPeriodValue } totalApiCreatedByProvider { currentPeriodValue previousPeriodValue } } }`,
	"teams":                    `query teams($orgId: Int, $slugifiedName: String) { teams(where: { orgId: $orgId, slugifiedName: $slugifiedName }) { id name status usersCount description slugifiedName } }`,

	"apiBillingPlans": `query apiBillingPlans($apiId: ID!, $entityId: ID!) { api(id: $apiId) { id slugifiedName websiteUrl billingItems { id title name displayName description type allEndpoints } owner { id slugifiedName } billingPlans: nacBillingPlans(pagingArgs: { limit: -1 }) { id name recommended visibility } } entityById(id: $entityId) { id type } }`,

	"getInvoice": `query getInvoice($input: GetInvoiceInput!) { getInvoice(input: $input) { id total status subTotal amountDue } }`,

	"getConsumerApiTransactions": `query getConsumerApiTransactions($input: GetTransactionsListInput!) { getConsumerApiTransactions(input: $input) { count rows { transactionId totalAmount taxAmount createdAt paymentStatus currency } } }`,

	"getIssuesByApiIdV2": `query getIssuesByApiIdV2($apiId: String!, $pagingArgs: PagingArgs, $providerId: String) { getIssuesByApiIdV2(apiId: $apiId, pagingArgs: $pagingArgs, providerId: $providerId) { data { createdAt updatedAt title status rating closed body commentsCount user { username thumbnail } } total } }`,

	"messageThreadsPaginated": `query messageThreadsPaginated($where: MessageThreadsWhereInput, $pagination: PaginationInput) { messageThreadsPaginated(where: $where, pagination: $pagination) { nodes { id apiId ownerDisplayName entityDisplayName apiDisplayName updatedAt lastMessage { id body createdAt } } pageInfo { endCursor hasNextPage } } }`,

	"tutorials": `query tutorials($id: ID!, $versionId: ID!) { tutorials(where: { apiId: $id, apiVersion: $versionId }) { nodes { id slugifiedName published title content thumbnailURL publishedDate readTime } } }`,

	"getCertificates": `query getCertificates($providersId: [ID!]) { certs: apiCertificates(where: { ownerId: $providersId }) { nodes { id alias effectiveDate expiry serialNumber createdAt } } }`,

	"announcements": `query announcements($where: ApiWhereInput, $pagingArgs: PagingArgs) { apis(where: $where) { nodes { id name announcements(pagingArgs: $pagingArgs) { id body status createdAt } } } }`,

	"getUserInviteByToken": `query getUserInviteByToken($token: String!) { getUserInviteByToken(token: $token) { id email organization { id name } } }`,

	"roles": `query roles($where: RoleWhereInput!) { roles(where: $where) { nodes { id name description } } }`,

	"getSeoData": `query getSeoData($where: SEOWhereInput) { getSeoData(where: $where) { id url tags { tag innerBody attributes { key value } } } }`,

	"getUsagesForSubscription": `query getUsagesForSubscription($apiId: String, $subscriptionId: String, $billingItemIds: [String], $resolution: String, $periods: [String], $parentId: Int) { getUsagesAndParentUsageForSubscriptionByBuckets(apiId: $apiId, subscriptionId: $subscriptionId, billingItemIds: $billingItemIds, resolution: $resolution, periods: $periods, parentId: $parentId) { usage period parentUsage } }`,

	"getAuditByOwnerId": `query getAuditByOwnerId($where: AuditsWhereInput, $orderBy: AuditsOrderByInput, $pagination: PaginationInput) { audits(where: $where, orderBy: $orderBy, pagination: $pagination) { nodes { id action eventName user { id name } } totalCount } }`,

	"getWorkflowsByRequestee": `query getWorkflowsByRequestee($options: GetWorkflowsOptions, $pagingArgs: PagingArgs) { getWorkflowsByRequestee(options: $options, pagingArgs: $pagingArgs) { data { id topic status workflowStatus createdAt updatedAt requesteeEmail } total } }`,

	"getEntityProfile": `query getEntityProfile($where: EntityWhereInput, $publishedApisWhere: PublishedApisWhereInput) { entity(where: $where) { thumbnail id name username email slugifiedName type description bio publishedApisList(where: $publishedApisWhere) { id name slugifiedName thumbnail } } }`,

	"getEntityProjects": `query getEntityProjects($entityID: ID) { entityById(id: $entityID) { id name slugifiedName type } }`,
}

// gqlResponsePaths maps operation name -> JSON path to the payload rows/object
// (relative to the GraphQL "data" wrapper), matching the spec's response_path.
var gqlResponsePaths = map[string]string{
	"searchApis": "data.products.nodes",
	// searchApisPage is a distinct entry from "searchApis" above: it keeps
	// the pageInfo envelope (endCursor/hasNextPage) intact for sync.go's
	// cursor-pagination loop, whereas "searchApis" strips down to the bare
	// nodes array for search.go/promoted_marketplace.go's output rendering.
	// Do not merge these two entries — they serve different response shapes.
	"searchApisPage":          "data.products",
	"getCategoriesByCtx":      "data.categoriesByCtx",
	"GetCollectionsCollapsed": "data.collections",
	"getCollectionBySlug":     "data.collection",
	"getApiBySlugAndOwner":    "data.apiBySlugifiedNameAndOwnerName",
	"getUserProfile":          "data.userProfile",
	"getHubMetrics":           "data.publicMetrics",
	"activeUser":              "data.activeUser",
	"getUserSavedApis":        "data.userSavedApis",
	"getApiSubscriptions":     "data.getApiSubscriptions.rows",
	"getNotifications":        "data.newNotificationsByUserId",
	"getWorkspaceData":        "data.workspaceData",
}

// gqlExec executes a GraphQL operation against the hub gateway with a
// pre-baked document, composing the body from friendly flags.
func gqlExec(cmd *cobra.Command, flags *rootFlags, operation string, variables any, responsePath string) (json.RawMessage, error) {
	doc, ok := gqlDocs[operation]
	if !ok {
		return nil, fmt.Errorf("unknown GraphQL operation %q", operation)
	}
	// Allow the raw --query / --variables escape hatch to override the baked doc.
	bodyMap := map[string]any{}
	if cmd.Flags().Changed("query") {
		q, _ := cmd.Flags().GetString("query")
		bodyMap["query"] = q
	} else {
		bodyMap["query"] = doc
	}
	bodyMap["operationName"] = operation
	if cmd.Flags().Changed("variables") {
		raw, _ := cmd.Flags().GetString("variables")
		var overrides map[string]any
		if err := json.Unmarshal([]byte(raw), &overrides); err == nil && overrides != nil {
			merged := map[string]any{}
			b, _ := json.Marshal(variables)
			_ = json.Unmarshal(b, &merged)
			for k, v := range overrides {
				merged[k] = v
			}
			variables = merged
		}
	}
	bodyMap["variables"] = variables

	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	data, _, err := c.PostQueryWithParams(cmd.Context(), "/gateway/graphql", map[string]string{}, bodyMap)
	if err != nil {
		return nil, classifyAPIError(cmd.OutOrStdout(), err, flags)
	}
	if flags.dryRun {
		return data, nil
	}
	if responsePath != "" {
		data = applyResponsePath(data, responsePath)
	}
	return data, nil
}

// gqlExecWithContext is like gqlExec but allows injecting a context (used by
// tests and by commands that need cancellation).
func gqlExecWithContext(ctx context.Context, cmd *cobra.Command, flags *rootFlags, operation string, variables any, responsePath string) (json.RawMessage, error) {
	doc, ok := gqlDocs[operation]
	if !ok {
		return nil, fmt.Errorf("unknown GraphQL operation %q", operation)
	}
	bodyMap := map[string]any{}
	bodyMap["query"] = doc
	bodyMap["operationName"] = operation
	bodyMap["variables"] = variables
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	data, _, err := c.PostQueryWithParams(ctx, "/gateway/graphql", map[string]string{}, bodyMap)
	if err != nil {
		return nil, classifyAPIError(cmd.OutOrStdout(), err, flags)
	}
	if flags.dryRun {
		return data, nil
	}
	if responsePath != "" {
		data = applyResponsePath(data, responsePath)
	}
	return data, nil
}

// gqlOutput renders the GraphQL result through the standard output pipeline.
func gqlOutput(cmd *cobra.Command, flags *rootFlags, data json.RawMessage, compactFields map[string]bool) error {
	if isDryRunResponse(flags.dryRun, data) {
		if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
			return printOutputWithFlagsMeta(cmd.OutOrStdout(), data, flags, map[string]any{"source": "dry-run"}, compactFields)
		}
		return nil
	}
	outputData := data
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		var items []map[string]any
		if json.Unmarshal(outputData, &items) == nil && len(items) > 0 {
			if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
				return err
			}
			return nil
		}
	}
	return printOutputWithFlagsMeta(cmd.OutOrStdout(), outputData, flags, map[string]any{"source": "live"}, compactFields)
}
