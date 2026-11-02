// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"net/url"

	"github.com/mvanhorn/printing-press-library/library/monitoring/sentry/internal/cliutil"
)

// appendArrayQueryParam sends a repeatable flag's values as repeated query keys.
// The single-value params map can't carry them, so array params go on the path.
func appendArrayQueryParam(path, name string, flagValues []string) string {
	return cliutil.AppendArrayQuery(path, name, flagValues)
}

// appendQueryParam sends one scalar query param on PUT, POST, PATCH and DELETE
// commands, whose requests have no params map.
func appendQueryParam(path, name, value string) string {
	return cliutil.AppendQueryValues(path, url.Values{name: {value}})
}
