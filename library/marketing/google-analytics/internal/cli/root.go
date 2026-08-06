// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-analytics/internal/ga4"
	"github.com/spf13/cobra"
)

var version = "2026.7.1"

type rootFlags struct {
	asJSON      bool
	compact     bool
	noInput     bool
	yes         bool
	agent       bool
	propertyID  string
	credentials string
	timeout     time.Duration
	clients     map[string]*ga4.Client
	key         ga4.Credentials
}

func RootCmd() *cobra.Command { var f rootFlags; return newRootCmd(&f) }
func Execute() error          { var f rootFlags; return newRootCmd(&f).Execute() }

func newRootCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "google-analytics-pp-cli", Short: "Agent-first Google Analytics 4 CLI", Long: `Google Analytics 4 Printing Press CLI.

Raw wrappers: report, pivot, batch, realtime, metadata, compatibility, properties, property, streams.
Novel commands: health/doctor, channels, sources, top-pages, events, conversions, funnel, compare, whats-changed, revenue, audience, cohort.
Admin writes: key-events, custom-dimensions, custom-metrics, data-streams, plus the admin escape hatch.

Auth: accepts either a Google service-account JSON key or an ADC authorized_user JSON
(client_id + client_secret + refresh_token). Resolution: --credentials, then GOOGLE_ANALYTICS_ADC,
then GOOGLE_APPLICATION_CREDENTIALS. Reads request analytics.readonly; writes request analytics.edit
and additionally need Editor or Administrator on the property.

Destructive writes (every delete, and custom-dimension/custom-metric archive) require a typed --yes.
--agent implies --yes for prompts but deliberately does not authorize destruction on its own.

Property resolution for data commands: --property, then GA4_PROPERTY_ID. The CLI never hard-codes a brand property for reads.`, SilenceUsage: true, Version: version}
	cmd.SetVersionTemplate("google-analytics-pp-cli {{ .Version }}\n")
	cmd.PersistentFlags().BoolVar(&flags.asJSON, "json", false, "Output JSON")
	cmd.PersistentFlags().BoolVar(&flags.compact, "compact", false, "Prefer token-compact fields where supported")
	cmd.PersistentFlags().BoolVar(&flags.noInput, "no-input", false, "Disable prompts (agent/CI safe)")
	cmd.PersistentFlags().BoolVar(&flags.yes, "yes", false, "Assume yes for safe non-mutating confirmations")
	cmd.PersistentFlags().BoolVar(&flags.agent, "agent", false, "Agent mode: --json --compact --no-input --yes")
	cmd.PersistentFlags().StringVar(&flags.propertyID, "property", "", "GA4 numeric property ID (defaults to GA4_PROPERTY_ID)")
	cmd.PersistentFlags().StringVar(&flags.credentials, "credentials", "", "Service-account JSON key path (defaults to GOOGLE_APPLICATION_CREDENTIALS)")
	cmd.PersistentFlags().DurationVar(&flags.timeout, "timeout", 30*time.Second, "HTTP request timeout")
	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if flags.agent {
			flags.asJSON, flags.compact, flags.noInput, flags.yes = true, true, true, true
		}
		return nil
	}
	cmd.AddCommand(newAgentContextCmd())
	cmd.AddCommand(newHealthCmd(flags), newDoctorCmd(flags))
	cmd.AddCommand(newReportCmd(flags), newPivotCmd(flags), newBatchCmd(flags), newRealtimeCmd(flags), newMetadataCmd(flags), newCompatibilityCmd(flags))
	cmd.AddCommand(newPropertiesCmd(flags), newPropertyCmd(flags), newStreamsCmd(flags))
	cmd.AddCommand(newKeyEventsCmd(flags), newCustomDimensionsCmd(flags), newCustomMetricsCmd(flags), newDataStreamsCmd(flags), newAdminCmd(flags))
	cmd.AddCommand(newChannelsCmd(flags), newSourcesCmd(flags), newTopPagesCmd(flags), newEventsCmd(flags), newConversionsCmd(flags))
	cmd.AddCommand(newFunnelCmd(flags), newCompareCmd(flags), newWhatsChangedCmd(flags), newRevenueCmd(flags), newAudienceCmd(flags), newCohortCmd(flags))
	return cmd
}

// mint loads credentials and returns a per-scope cached client. Read and write
// commands in one process therefore keep distinct clients so they never clobber
// each other. For authorized_user both scope keys resolve to the same token,
// which is correct: the grant, not the request, decides the scope.
func (f *rootFlags) mint(scope string) (*ga4.Client, ga4.Credentials, error) {
	if f.clients == nil {
		f.clients = map[string]*ga4.Client{}
	}
	if c, ok := f.clients[scope]; ok {
		return c, f.key, nil
	}
	var key ga4.Credentials
	path := credentialPath(f)
	if path == "" {
		return nil, key, fmt.Errorf("missing credentials: set GOOGLE_APPLICATION_CREDENTIALS (or GOOGLE_ANALYTICS_ADC) or pass --credentials")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, key, err
	}
	if err := jsonDecode(b, &key); err != nil {
		return nil, key, err
	}
	tok, err := ga4.MintToken(key, scope)
	if err != nil {
		return nil, key, err
	}
	f.clients[scope] = ga4.NewClient(tok, f.timeout)
	f.key = key
	return f.clients[scope], key, nil
}

func (f *rootFlags) newClient() (*ga4.Client, ga4.ServiceAccountKey, error) {
	c, k, err := f.mint(ga4.AnalyticsReadonlyScope)
	return c, k, err
}
func (f *rootFlags) newWriteClient() (*ga4.Client, ga4.Credentials, error) {
	return f.mint(ga4.AnalyticsEditScope)
}

func output(cmd *cobra.Command, f *rootFlags, v any, human string) error {
	if f.asJSON || f.agent {
		return printJSON(cmd.OutOrStdout(), v)
	}
	if human != "" {
		_, err := io.Copy(cmd.OutOrStdout(), bytes.NewBufferString(human))
		return err
	}
	return printJSON(cmd.OutOrStdout(), v)
}

func configuredProperty(f *rootFlags) string {
	if f.propertyID != "" {
		return cleanProperty(f.propertyID)
	}
	return cleanProperty(os.Getenv("GA4_PROPERTY_ID"))
}
func requireProperty(f *rootFlags) (string, error) {
	p := configuredProperty(f)
	if p == "" {
		return "", fmt.Errorf("missing GA4 property: pass --property or set GA4_PROPERTY_ID")
	}
	return p, nil
}
func cleanProperty(p string) string { return strings.TrimSpace(strings.TrimPrefix(p, "properties/")) }
func credentialPath(f *rootFlags) string {
	var p string
	if f.credentials != "" {
		p = f.credentials
	} else if p = os.Getenv("GOOGLE_ANALYTICS_ADC"); p == "" {
		p = os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	}
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, strings.TrimPrefix(p, "~/"))
		}
	}
	return p
}
