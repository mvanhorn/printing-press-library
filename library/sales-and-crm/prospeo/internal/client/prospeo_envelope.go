// Hand-authored: Prospeo body envelope wrapper.
//
// Prospeo's /enrich-person and /enrich-company endpoints require matching
// fields (first_name, last_name, linkedin_url, company_website, ...) to be
// nested inside a top-level "data" object, while the option flags
// (only_verified_email, enrich_mobile, only_verified_mobile) remain at the
// top level.
//
// The generator emits a flat map[string]any body; this helper repartitions
// it into the correct envelope shape.

package client

// envelopeOptionFields lists the keys that must remain at the top level of
// the request body. Everything else gets nested under "data".
var envelopeOptionFields = map[string]struct{}{
	"only_verified_email":  {},
	"enrich_mobile":        {},
	"only_verified_mobile": {},
}

// wrapProspeoEnrichBody takes a flat body and repartitions it into the
// Prospeo enrich envelope shape:
//
//	input  {"linkedin_url": "...", "only_verified_email": true}
//	output {"data": {"linkedin_url": "..."}, "only_verified_email": true}
//
// If body is already in envelope shape (has a "data" object), it passes
// through unchanged so hand-authored callers can opt out of repartitioning.
func wrapProspeoEnrichBody(body any) any {
	flat, ok := body.(map[string]any)
	if !ok {
		return body
	}
	if _, alreadyEnveloped := flat["data"]; alreadyEnveloped {
		return body
	}
	data := make(map[string]any, len(flat))
	top := make(map[string]any, 4)
	for k, v := range flat {
		if _, isOption := envelopeOptionFields[k]; isOption {
			top[k] = v
			continue
		}
		data[k] = v
	}
	top["data"] = data
	return top
}
