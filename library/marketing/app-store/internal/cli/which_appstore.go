// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// The generated whichIndex (which.go) lists only the novel commands, so
// `which "app store reviews for"` routed to a history command. These entries
// add every other shipped command; which.go itself stays as generated.

func init() {
	whichIndex = append(whichIndex, appStoreWhichEntries...)
}

const (
	whichGroupLive    = "Live App Store reads"
	whichGroupLocal   = "Local history readers"
	whichGroupCatalog = "Categories"
)

var appStoreWhichEntries = []whichEntry{
	{Command: "app", Description: "One app's listing in one storefront: Lookup fields plus the product page's subtitle, star histogram, chart badge and Featured In; --full adds description, versions, privacy, IAP prices and events.", Group: whichGroupLive, WhyItMatters: "Use it for an app's details, rating, version, size or listing page."},
	{Command: "top", Description: "One top chart (top-free, top-paid, top-grossing) for a storefront, platform and genre, with exact rating counts; records the chart for movers and rank-history.", Group: whichGroupLive, WhyItMatters: "Use it for the top free, paid or grossing apps or games in a country or category."},
	{Command: "reviews", Description: "Customer reviews for one app from Apple's reviews feed (recent and helpful, up to 10 pages per sort per storefront), stored for review-digest and search.", Group: whichGroupLive, WhyItMatters: "Use it to read what users say about an app in one or more countries."},
	{Command: "ratings", Description: "An app's exact rating count and average per storefront; --histogram adds the 5-to-1 star split from the product page.", Group: whichGroupLive, WhyItMatters: "Use it to compare an app's ratings across countries and to capture rating history for traction."},
	{Command: "compare", Description: "Two or more apps side by side per storefront: developer, genre, price, ratings, version, size and release dates.", Group: whichGroupLive, WhyItMatters: "Use it to line up competitors field by field."},
	{Command: "prices", Description: "Price and availability of one or more apps in each storefront (one Lookup per storefront).", Group: whichGroupLive, WhyItMatters: "Use it to check what an app costs, or whether it is sold, in different countries."},
	{Command: "iap", Description: "The in-app purchases an app's product page lists (Apple shows the top 10), with prices per storefront.", Group: whichGroupLive, WhyItMatters: "Use it for in-app purchase prices (IAP prices) and how they differ by country."},
	{Command: "privacy", Description: "An app's App Privacy labels from its product page: data types collected and what they are used for.", Group: whichGroupLive, WhyItMatters: "Use it for privacy labels or the data an app collects."},
	{Command: "versions", Description: "An app's version history from its product page, newest first, with release notes.", Group: whichGroupLive, WhyItMatters: "Use it for release history, update frequency or what changed in a version."},
	{Command: "events", Description: "The in-app events on an app's product page: title, kind, dates and badge.", Group: whichGroupLive, WhyItMatters: "Use it to see an app's current and upcoming in-app events."},
	{Command: "similar", Description: "The apps an app's product page lists under You Might Also Like, in page order.", Group: whichGroupLive, WhyItMatters: "Use it to find competitors or related apps."},
	{Command: "developer", Description: "A developer's apps (up to 200 with exact counts) plus the shelves on the developer's page.", Group: whichGroupLive, WhyItMatters: "Use it to list everything a publisher has on the store."},
	{Command: "featured", Description: "The App Store Today tab for a storefront: the stories and the apps they feature.", Group: whichGroupLive, WhyItMatters: "Use it to see what Apple is featuring today."},
	{Command: "search-store", Description: "A live App Store search in one storefront, returning matching apps with ratings and prices.", Group: whichGroupLive, WhyItMatters: "Use it to find apps by keyword or to look up an app's id."},
	{Command: "suggest", Description: "The App Store's search suggestions (autocomplete hints) for a partial term, from Apple's undocumented search-hints endpoint.", Group: whichGroupLive, WhyItMatters: "Use it for keyword ideas when the user asks for them."},
	{Command: "keyword-rank", Description: "One live search for a term, reporting where a target app ranks and storing the result list for keyword-history and keyword-share.", Group: whichGroupLive, WhyItMatters: "Use it to check an app's keyword ranking for a search term."},
	{Command: "categories list", Description: "The App Store app categories and Kids age bands with the genre ids the chart commands take; --live reads them from the chart page.", Group: whichGroupCatalog, WhyItMatters: "Use it to find a genre id for top, movers or breakouts."},
	{Command: "categories games", Description: "The game subcategories (genre ids 7001-7019) with chart links; --live reads them from the games chart page.", Group: whichGroupCatalog, WhyItMatters: "Use it to find a game genre id for the chart commands."},
	{Command: "movers", Description: "Apps that climbed, dropped, entered or left one chart between its two most recent local captures.", Group: whichGroupLocal, WhyItMatters: "Use it after two or more top captures to see chart movement."},
	{Command: "rank-history", Description: "An app's rank over time in one chart, from local chart captures.", Group: whichGroupLocal, WhyItMatters: "Use it to track an app's chart position over days."},
	{Command: "keyword-history", Description: "One search term's stored captures over time: where an app ranked and who held the top results.", Group: whichGroupLocal, WhyItMatters: "Use it to track a keyword ranking over time after keyword-rank captures."},
	{Command: "watch-listing", Description: "A field-by-field diff of an app's two most recent stored listing snapshots (title, subtitle, price, version, rating).", Group: whichGroupLocal, WhyItMatters: "Use it to see what changed on one app's listing."},
	{Command: "review-digest", Description: "A mechanical digest of an app's stored reviews: star histogram, volume over time, common words and recent low ratings.", Group: whichGroupLocal, WhyItMatters: "Use it to summarize reviews you already captured, without an LLM."},
	{Command: "search", Description: "Offline search of the local store: stored reviews by title and text, and stored app listings.", Group: whichGroupLocal, WhyItMatters: "Use it to find something in what you already captured, without network calls."},
}
