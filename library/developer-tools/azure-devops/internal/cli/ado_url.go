// pp:data-source live
package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/azure-devops/internal/config"
)

// adoAPIPath builds the correct API path for an Azure DevOps REST call.
// Supports both URL formats:
//   - dev.azure.com/{org}/{project}/_apis/...
//   - {org}.visualstudio.com/{project}/_apis/...
//
// The visualstudio.com format encodes the org in the subdomain, so the path
// only needs the project prefix.
func adoAPIPath(flags *rootFlags, project, resource string) string {
	cfg, _ := config.Load(flags.configPath)
	base := strings.TrimRight(cfg.BaseURL, "/")
	resource = strings.TrimPrefix(resource, "/")

	if strings.Contains(base, ".visualstudio.com") {
		// {org}.visualstudio.com — org is in subdomain, project goes first in path
		if project != "" {
			return fmt.Sprintf("/%s/_apis/%s", project, resource)
		}
		return fmt.Sprintf("/_apis/%s", resource)
	}

	// dev.azure.com — org and project both in path
	org := flags.org
	if org == "" {
		// Try extracting from config base URL just in case
		// e.g. "https://dev.azure.com" → org must come from flag
		return fmt.Sprintf("/_apis/%s", resource)
	}
	if project != "" {
		return fmt.Sprintf("/%s/%s/_apis/%s", org, project, resource)
	}
	return fmt.Sprintf("/%s/_apis/%s", org, resource)
}

// adoOrgAPIPath builds an org-scoped (no project) API path.
func adoOrgAPIPath(flags *rootFlags, resource string) string {
	cfg, _ := config.Load(flags.configPath)
	base := strings.TrimRight(cfg.BaseURL, "/")
	resource = strings.TrimPrefix(resource, "/")

	if strings.Contains(base, ".visualstudio.com") {
		return fmt.Sprintf("/_apis/%s", resource)
	}
	org := flags.org
	if org == "" {
		return fmt.Sprintf("/_apis/%s", resource)
	}
	return fmt.Sprintf("/%s/_apis/%s", org, resource)
}
