// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import "context"

// The source catalog and its export share the verified first-page link contract.
func sourceCatalogExtractionOptions(ctx context.Context, baseURL, path string, params map[string]string, limit int) htmlExtractionOptions {
	return htmlExtractionOptions{Context: ctx, Mode: "links", BaseURL: htmlExtractionRequestURL(baseURL, path, params), Limit: limit,
		LinkPrefixes: []string{"/park/rvpark", "/park/yypark", "/park/kurumatabipark", "/park/gourmet", "/park/minpark", "/park/train", "/park/campjrva", "/park/camp3000"}}
}
