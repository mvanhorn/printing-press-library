// Hand-authored. Survives `cli-printing-press generate --force` because it
// is not generator-emitted. The Load() call into PopulateZohoOrgHeader is in
// the templated config.go and must be re-applied after a regen.
package config

// PopulateZohoOrgHeader copies cfg.ZohoExpenseOrganizationId (sourced from
// env var ZOHO_EXPENSE_ORGANIZATION_ID or the [config] file) into
// cfg.Headers under the canonical Zoho key. The generated HTTP client
// (client.go) iterates cfg.Headers and sets each on every outbound request,
// so a single Load-time mapping fans the header out across every command.
//
// Why not RequiredHeaders in the spec: that field accepts only static
// values, not config-bound ones. The Printing Press doesn't yet support a
// {config.field} substitution syntax in RequiredHeaders.
func PopulateZohoOrgHeader(cfg *Config) {
	if cfg == nil || cfg.ZohoExpenseOrganizationId == "" {
		return
	}
	if cfg.Headers == nil {
		cfg.Headers = make(map[string]string)
	}
	cfg.Headers["X-com-zoho-expense-organizationid"] = cfg.ZohoExpenseOrganizationId
}
