// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// This file is the single seam where the Happenstance public-API response
// shape becomes the canonical client.Person consumed by every renderer in
// the CLI (coverage, hp people, prospect, warm-intro, dossier, JSON output,
// table output, MCP tool responses, etc.).
//
// Two surfaces, two schemas, one canonical shape:
//
//   - The cookie surface (internal/client/people_search.go) returns a RICH
//     schema: name, person_uuid, score, linkedin_url, twitter_url,
//     instagram_url, quotes, quotes_cited, current_title, current_company,
//     summary, referrers (with affinity scores and source chains).
//   - The bearer surface (this package) returns a THIN schema: name,
//     current_title, current_company, weighted_traits_score on /v1/search;
//     a deeper but still partial profile on /v1/research (employment,
//     education, projects, writings, hobbies, summary).
//
// The normalizers in this file project the thin/research bearer shapes
// into the canonical client.Person. Most fields end up at their zero
// value because the public API simply does not return them. That is by
// design: renderers must already tolerate empty fields (the cookie
// surface itself sometimes returns partial Persons too — e.g. when a
// match has no LinkedIn URL on file). Renderers do NOT branch on source.
// If a renderer ever starts crashing on an empty LinkedInURL or empty
// Quotes, the bug is in the renderer, not here.
//
// We intentionally do NOT add a Source field to client.Person. Knowing
// which surface produced a Person is a renderer concern (it shows up in
// CLI output as a "source: api" tag, plumbed via the call site), not a
// normalizer concern. Keeping the canonical shape source-agnostic means
// the seam stays a one-way projection: API -> Person, never the reverse.
package api

import "github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/client"

// ToClientPerson projects a /v1/search SearchResult row into the canonical
// client.Person. Only the four fields the public-API search endpoint
// returns are populated:
//
//   - Name          <- SearchResult.Name
//   - CurrentTitle  <- SearchResult.CurrentTitle
//   - CurrentCompany<- SearchResult.CurrentCompany
//   - Score         <- SearchResult.WeightedTraitsScore
//
// Every other client.Person field stays at its Go zero value (empty
// string for strings, nil for slices). Downstream renderers must tolerate
// those zero values; see the package-doc comment above for the
// invariant.
func ToClientPerson(api SearchResult) client.Person {
	return client.Person{
		Name:           api.Name,
		CurrentTitle:   api.CurrentTitle,
		CurrentCompany: api.CurrentCompany,
		Score:          api.WeightedTraitsScore,
	}
}

// ToClientPersonFromResearch projects a /v1/research ResearchProfile into
// the canonical client.Person. The research endpoint returns a deeper
// (but still partial) shape, so this normalizer hydrates more fields
// than ToClientPerson does:
//
//   - Name          <- displayName (the public-API research response
//                     does NOT echo back the subject's name; the caller
//                     knows it because the caller submitted the prose
//                     description that named the subject in the first
//                     place. Passing it in here keeps the normalizer
//                     pure — it never invents a name from prose).
//   - CurrentTitle  <- ResearchProfile.Employment[0].Title
//   - CurrentCompany<- ResearchProfile.Employment[0].Company
//   - Quotes        <- ResearchProfile.Summary (the canonical Person's
//                     Quotes field is freeform prose on the cookie
//                     surface; the research endpoint's Summary is the
//                     closest analog).
//
// Empty Employment is safe: the function leaves CurrentTitle and
// CurrentCompany at "" rather than panicking on a zero-length-slice
// index. ResearchProfile carries no LinkedIn / Twitter / Instagram URL
// fields today (the upstream OpenAPI spec does not expose them on the
// research surface), so those Person fields stay zero. If the upstream
// schema later adds them, hydrate them here and update the package-doc
// comment.
func ToClientPersonFromResearch(api ResearchProfile, displayName string) client.Person {
	p := client.Person{
		Name:   displayName,
		Quotes: api.Summary,
	}
	if len(api.Employment) > 0 {
		p.CurrentTitle = api.Employment[0].Title
		p.CurrentCompany = api.Employment[0].Company
	}
	return p
}
