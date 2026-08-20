// Namecheap registrar integration: `domain-goat-pp-cli namecheap check/pricing/doctor`.
//
// Auth: NAMECHEAP_USERNAME (or legacy NAMECHEAP_API_USER), NAMECHEAP_API_KEY, NAMECHEAP_CLIENT_IP.
// Sandbox: set NAMECHEAP_SANDBOX=1 to hit the api.sandbox.namecheap.com host.
package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/domain-goat/internal/source/namecheap"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/domain-goat/internal/store"
)

func newNamecheapCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "namecheap",
		Short: "Namecheap registrar adapter: live availability + pricing sync (requires NAMECHEAP_USERNAME + NAMECHEAP_API_KEY + whitelisted IP).",
	}
	cmd.AddCommand(newNamecheapCheckCmd(flags))
	cmd.AddCommand(newNamecheapPricingSyncCmd(flags))
	cmd.AddCommand(newNamecheapDoctorCmd(flags))
	return cmd
}

func newNamecheapCheckCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check <domain...>",
		Short: "Check availability via the Namecheap API (returns premium flag + premium prices when applicable).",
		Long: `Sends one bulk DomainCheck call to Namecheap for up to 50 domains.
Returns the registry-authoritative availability + premium flag + premium
registration/renewal prices when the name is a marketplace listing.

Auth: NAMECHEAP_USERNAME (or NAMECHEAP_API_USER), NAMECHEAP_API_KEY, and
NAMECHEAP_CLIENT_IP (optional — auto-detected). Your client IP must be
whitelisted in the Namecheap dashboard at
https://ap.www.namecheap.com/settings/tools/apiaccess/whitelisted-ips`,
		Example: `  domain-goat-pp-cli namecheap check kindred.io
  domain-goat-pp-cli namecheap check kindred.io kindred.ai kindred.studio --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			fqdns, err := normalizeAll(args)
			if err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "fqdns": fqdns, "registrar": "namecheap"})
			}
			creds := namecheap.FromEnv(os.Getenv)
			if err := creds.Validate(); err != nil {
				return apiErr(fmt.Errorf("HTTP 401: %w", err))
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			results, err := namecheap.CheckAvailability(ctx, creds, fqdns)
			if err != nil {
				return apiErr(err)
			}
			// Persist domain rows so other commands see Namecheap-flagged premiums.
			s, _ := openStore(cmd.Context())
			if s != nil {
				defer s.Close()
				for _, r := range results {
					status := "registered"
					if r.Available {
						status = "available"
					}
					_ = s.UpsertDomain(cmd.Context(), store.DomainRow{
						FQDN: r.FQDN, ASCII: r.FQDN, Label: labelOf(r.FQDN), TLD: tldOf(r.FQDN),
						Length: len(labelOf(r.FQDN)),
						Status: status, Source: "namecheap", Premium: r.Premium,
						LastCheckedAt: time.Now().UTC().Format(time.RFC3339),
					})
				}
			}
			if wantJSON(cmd, flags) {
				return emitJSON(cmd, flags, results)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "DOMAIN\tAVAILABLE\tPREMIUM\tPREMIUM_REG_PRICE\tPREMIUM_RENEW_PRICE")
			for _, r := range results {
				fmt.Fprintf(tw, "%s\t%v\t%v\t$%.2f\t$%.2f\n", r.FQDN, r.Available, r.Premium, r.PremiumRegPrice, r.PremiumRenewPrice)
			}
			return tw.Flush()
		},
	}
	return cmd
}

func newNamecheapPricingSyncCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pricing-sync",
		Short: "Sync Namecheap TLD pricing (REGISTER + RENEW + TRANSFER) into the local store under registrar=namecheap.",
		Long: `Calls namecheap.users.getPricing three times (one per ActionName) and
joins the 1-year prices into the pricing_snapshots table under registrar
'namecheap'. Subsequent commands can compare against the Porkbun snapshot:

  domain-goat-pp-cli pricing show com --registrar namecheap
  domain-goat-pp-cli pricing show com --registrar porkbun`,
		Example:     `  domain-goat-pp-cli namecheap pricing-sync`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "registrar": "namecheap"})
			}
			creds := namecheap.FromEnv(os.Getenv)
			if err := creds.Validate(); err != nil {
				return apiErr(fmt.Errorf("HTTP 401: %w", err))
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 90*time.Second)
			defer cancel()
			entries, err := namecheap.FetchAllPricing(ctx, creds)
			if err != nil {
				return apiErr(err)
			}
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			count := 0
			for _, e := range entries {
				if err := s.UpsertPricing(cmd.Context(), store.PricingRow{
					TLD: e.TLD, Registrar: "namecheap",
					Registration: e.Registration, Renewal: e.Renewal, Transfer: e.Transfer,
				}); err == nil {
					count++
				}
			}
			out := map[string]any{
				"synced":    count,
				"registrar": "namecheap",
			}
			if wantJSON(cmd, flags) {
				return emitJSON(cmd, flags, out)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Synced %d Namecheap TLD prices\n", count)
			return nil
		},
	}
	return cmd
}

func newNamecheapDoctorCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Verify Namecheap auth: env vars present + a probe call returns OK.",
		Example: `  domain-goat-pp-cli namecheap doctor
  domain-goat-pp-cli namecheap doctor --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true})
			}
			creds := namecheap.FromEnv(os.Getenv)
			type DocResult struct {
				EnvAPIUser  string `json:"env_user"`
				EnvAPIKey   string `json:"env_key"`
				EnvClientIP string `json:"env_client_ip"`
				Probe       string `json:"probe"`
				ProbeError  string `json:"probe_error,omitempty"`
			}
			r := DocResult{}
			r.EnvAPIUser = redactPresent(creds.APIUser)
			r.EnvAPIKey = redactPresent(creds.APIKey)
			if creds.ClientIP == "" {
				r.EnvClientIP = "(auto-detect)"
			} else {
				r.EnvClientIP = creds.ClientIP
			}
			if err := creds.Validate(); err != nil {
				r.Probe = "skip"
				r.ProbeError = err.Error()
				if wantJSON(cmd, flags) {
					return emitJSON(cmd, flags, r)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "USER\t%s\nKEY\t%s\nIP\t%s\nPROBE\tskip — %s\n", r.EnvAPIUser, r.EnvAPIKey, r.EnvClientIP, r.ProbeError)
				return apiErr(fmt.Errorf("HTTP 401: %w", err))
			}
			// probe with example.com
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			_, err := namecheap.CheckAvailability(ctx, creds, []string{"example.com"})
			if err != nil {
				r.Probe = "fail"
				r.ProbeError = err.Error()
				if wantJSON(cmd, flags) {
					return emitJSON(cmd, flags, r)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "USER\t%s\nKEY\t%s\nIP\t%s\nPROBE\tFAIL — %s\n", r.EnvAPIUser, r.EnvAPIKey, r.EnvClientIP, r.ProbeError)
				return apiErr(err)
			}
			r.Probe = "ok"
			if wantJSON(cmd, flags) {
				return emitJSON(cmd, flags, r)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "USER\t%s\nKEY\t%s\nIP\t%s\nPROBE\tOK\n", r.EnvAPIUser, r.EnvAPIKey, r.EnvClientIP)
			return nil
		},
	}
	return cmd
}

func redactPresent(v string) string {
	if v == "" {
		return "(missing)"
	}
	if len(v) <= 4 {
		return "***"
	}
	return v[:2] + "***" + v[len(v)-2:]
}

func labelOf(fqdn string) string {
	for i, c := range fqdn {
		if c == '.' {
			return fqdn[:i]
		}
	}
	return fqdn
}

func tldOf(fqdn string) string {
	for i, c := range fqdn {
		if c == '.' {
			return fqdn[i+1:]
		}
	}
	return ""
}
