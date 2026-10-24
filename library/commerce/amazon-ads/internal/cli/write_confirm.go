// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// Amazon Ads rides many reads on POST (v3 list endpoints, recommendations,
// forecasts, report and snapshot requests). These patterns keep those
// read-style POSTs out of the write-confirmation gate.
var (
	readStylePostPath = regexp.MustCompile(`(?i)recommendation|/eligibility(/|$)|/(list|search|localize|preview|count|forecasts?|usage|history|metadata|notifications|statuses|state|reports?|snapshot|premoderation)/?$`)
	readStylePostName = regexp.MustCompile(`^(get|bulk-get|check|list|search|preview|post-creative-preview)-`)
)

// isRemoteWriteCommand reports whether cmd is a generated endpoint command
// that changes Amazon Ads state: every PUT/PATCH/DELETE, plus POSTs that are
// not read-style.
func isRemoteWriteCommand(cmd *cobra.Command) bool {
	switch strings.ToUpper(cmd.Annotations["pp:method"]) {
	case "PUT", "PATCH", "DELETE":
		return true
	case "POST":
		if cmd.Annotations["mcp:read-only"] == "true" {
			return false
		}
		endpoint := cmd.Annotations["pp:endpoint"]
		name := endpoint[strings.LastIndex(endpoint, ".")+1:]
		return !readStylePostName.MatchString(name) && !readStylePostPath.MatchString(cmd.Annotations["pp:path"])
	}
	return false
}

// requireRemoteWriteConfirmation blocks remote writes unless the caller passed
// --yes explicitly or asked for --dry-run. --agent never satisfies it.
func requireRemoteWriteConfirmation(cmd *cobra.Command, flags *rootFlags) error {
	if flags.yes || flags.dryRun || !isRemoteWriteCommand(cmd) {
		return nil
	}
	return usageErr(fmt.Errorf("%s sends %s %s to Amazon Ads; pass --yes to confirm or --dry-run to preview",
		cmd.CommandPath(), strings.ToUpper(cmd.Annotations["pp:method"]), cmd.Annotations["pp:path"]))
}
