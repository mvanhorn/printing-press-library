// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH(ticketmaster-presale-planner): safe explicit checkout handoff.

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/ticketmaster/internal/cliutil"
	"github.com/spf13/cobra"
)

type checkoutHandoff struct {
	EventID     string `json:"event_id"`
	Event       string `json:"event"`
	Presale     string `json:"presale,omitempty"`
	Status      string `json:"status"`
	CheckoutURL string `json:"checkout_url"`
	Launched    bool   `json:"launched"`
}

func newEventsCheckoutCmd(flags *rootFlags) *cobra.Command {
	var presaleName string
	var launch bool
	cmd := &cobra.Command{
		Use:   "checkout <event-id>",
		Short: "Print an official checkout URL; optionally launch it with --launch",
		Long: `Fetch an event from Ticketmaster Discovery and select a matching presale
URL when available, otherwise the event's official purchase page.

This is an explicit browser handoff, not unattended purchasing. It does not
reserve inventory, submit a presale code, bypass a queue, or charge a card.`,
		Example: `  ticketmaster-pp-cli events checkout G5diZ9YpV7fJb
  ticketmaster-pp-cli events checkout G5diZ9YpV7fJb --presale "Artist Presale" --launch`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := c.GetNoCache(cmd.Context(), "/events/"+url.PathEscape(args[0]), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var event ticketmasterEvent
			if err := json.Unmarshal(raw, &event); err != nil {
				return fmt.Errorf("decode Ticketmaster event: %w", err)
			}
			selected, status := selectCheckoutPresale(event.Sales.Presales, presaleName, time.Now().UTC())
			if strings.TrimSpace(presaleName) != "" && selected.URL == "" {
				return notFoundErr(fmt.Errorf("presale %q for event %s", presaleName, event.ID))
			}
			checkoutURL := selected.URL
			if checkoutURL == "" {
				checkoutURL = event.URL
				status = "public"
			}
			if err := validateCheckoutURL(checkoutURL); err != nil {
				return err
			}
			handoff := checkoutHandoff{
				EventID: event.ID, Event: event.Name, Presale: selected.Name,
				Status: status, CheckoutURL: checkoutURL,
			}
			if launch {
				if cliutil.IsVerifyEnv() {
					fmt.Fprintf(cmd.ErrOrStderr(), "would launch: %s\n", checkoutURL)
				} else if err := openSetupURL(checkoutURL); err != nil {
					return fmt.Errorf("launch checkout URL: %w", err)
				} else {
					handoff.Launched = true
				}
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(handoff)
			}
			fmt.Fprintln(cmd.OutOrStdout(), checkoutURL)
			if !launch {
				fmt.Fprintln(cmd.ErrOrStderr(), "Checkout not launched. Re-run with --launch to open this URL.")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&presaleName, "presale", "", "Select a presale by case-insensitive name")
	cmd.Flags().BoolVar(&launch, "launch", false, "Open the selected checkout URL in the default browser")
	return cmd
}

func selectCheckoutPresale(presales []ticketmasterPresale, wanted string, now time.Time) (ticketmasterPresale, string) {
	wanted = strings.TrimSpace(strings.ToLower(wanted))
	if wanted != "" {
		for _, presale := range presales {
			if strings.Contains(strings.ToLower(firstNonEmpty(presale.Name, presale.Description)), wanted) {
				start, startOK := parseTicketmasterTime(presale.StartDateTime)
				end, endOK := parseTicketmasterTime(presale.EndDateTime)
				return presale, presaleStatus(now, start, startOK, end, endOK)
			}
		}
		return ticketmasterPresale{}, ""
	}
	for _, status := range []string{"open", "upcoming"} {
		var candidates []ticketmasterPresale
		for _, presale := range presales {
			start, startOK := parseTicketmasterTime(presale.StartDateTime)
			end, endOK := parseTicketmasterTime(presale.EndDateTime)
			if presale.URL != "" && presaleStatus(now, start, startOK, end, endOK) == status {
				candidates = append(candidates, presale)
			}
		}
		if len(candidates) > 0 {
			return candidates[0], status
		}
	}
	return ticketmasterPresale{}, ""
}

func validateCheckoutURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil {
		return fmt.Errorf("Ticketmaster returned an unsafe or missing checkout URL")
	}
	host := strings.ToLower(parsed.Hostname())
	official := hostWithinAnyDomain(host, []string{
		"ticketmaster.com", "ticketmaster.ca", "ticketmaster.co.uk",
		"ticketmaster.ie", "ticketmaster.de", "ticketmaster.fr",
		"ticketmaster.es", "ticketmaster.nl", "ticketmaster.be",
		"ticketmaster.at", "ticketmaster.ch", "ticketmaster.cz",
		"ticketmaster.dk", "ticketmaster.fi", "ticketmaster.no",
		"ticketmaster.pl", "ticketmaster.se", "ticketmaster.it",
		"ticketmaster.com.au", "ticketmaster.co.nz", "ticketmaster.com.mx",
		"universe.com", "frontgatetickets.com",
	})
	if !official {
		return fmt.Errorf("Ticketmaster returned a checkout URL on an unrecognized host %q", host)
	}
	return nil
}

func hostWithinAnyDomain(host string, domains []string) bool {
	for _, domain := range domains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}
