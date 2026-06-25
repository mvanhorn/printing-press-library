// Copyright 2026 Chirantan Rajhans and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: Substack publication-scoped endpoints must route through the
// publication host so {publication} can be substituted from --subdomain or env.

package cli

const substackPublicationAPIBase = "https://{publication}.substack.com/api/v1"

func publicationAPIPath(path string) string {
	return substackPublicationAPIBase + path
}
