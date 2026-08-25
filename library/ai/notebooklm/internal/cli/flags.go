// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

type rootFlags struct {
	asJSON        bool
	asCSV         bool
	plain         bool
	quiet         bool
	compact       bool
	selectFields  string
	stdin         bool
	configPath    string
	dryRun        bool
	agent         bool
	noInput       bool
	yes           bool
	noColor       bool
	dataSource    string
	freshnessMeta any
}
