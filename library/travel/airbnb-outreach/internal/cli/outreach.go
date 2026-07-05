// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/airbnb-outreach/internal/airbnb"
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb-outreach/internal/store"
	"github.com/spf13/cobra"
)

// contactRecord is one logged outreach contact, for the CRM.
type contactRecord struct {
	ListingID   string    `json:"listing_id"`
	ListingName string    `json:"listing_name,omitempty"`
	Message     string    `json:"message"`
	ContactedAt time.Time `json:"contacted_at"`
	ThreadRaw   string    `json:"thread_ref,omitempty"`
}

func newOutreachCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "outreach",
		Short: "Bulk-contact hosts from a search and track replies (CRM)",
		Long: `Turn a search into outreach: contact many hosts/property owners at once with
a templated message, and keep a local CRM of who you contacted and who replied.
This is the outreach workflow the website can't do.`,
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newOutreachRunCmd(flags))
	cmd.AddCommand(newOutreachCRMCmd(flags))
	return cmd
}

func newOutreachRunCmd(flags *rootFlags) *cobra.Command {
	var p airbnb.SearchParams
	var message string
	var limit int
	var confirm bool
	cmd := &cobra.Command{
		Use:   "run [location]",
		Short: "Search a location and contact the top hosts with a templated message",
		Long: `Search a location, then send an initial message to the top N hosts. The
message template supports {name} (listing name) and {id} (listing id). Guarded:
without --confirm it previews the recipient list and does nothing.`,
		Example: strings.Trim(`
  airbnb-outreach-pp-cli outreach run "Berlin, Germany" --message "Hi {name} team, I'm interested in a monthly stay — is that possible?" --limit 5
  airbnb-outreach-pp-cli outreach run "Lisbon" --price-max 120 --message "Hello, is {name} available long-term?" --limit 10 --confirm`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return cmd.Help()
			}
			p.Location = args[0]
			if message == "" {
				return usageErr(fmt.Errorf("--message is required"))
			}
			c := newAirbnbClient(flags)
			results, _, err := c.Search(p)
			if err != nil {
				return classifyAirbnb(err, flags)
			}
			if len(results) > limit {
				results = results[:limit]
			}

			type plan struct {
				ListingID string `json:"listing_id"`
				Name      string `json:"name"`
				Message   string `json:"message"`
			}
			var planned []plan
			for _, r := range results {
				planned = append(planned, plan{ListingID: r.ID, Name: r.Name, Message: renderTemplate(message, r)})
			}

			guarded := !(confirm || flags.yes) || flags.dryRun
			if guarded {
				preview := map[string]any{"action": "outreach.run", "status": "preview", "recipients": len(planned), "plan": planned, "hint": "add --confirm to actually send"}
				if flags.asJSON {
					return flags.printJSON(cmd, preview)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s Would contact %d hosts in %q:\n", yellow("[preview]"), len(planned), p.Location)
				for _, pl := range planned {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s (%s)\n", truncate(pl.Name, 40), pl.ListingID)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "  (add --confirm to actually send)")
				return nil
			}

			db, err := store.Open(defaultDBPath("airbnb-outreach-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()

			type outcome struct {
				ListingID string `json:"listing_id"`
				Name      string `json:"name"`
				Status    string `json:"status"`
				Error     string `json:"error,omitempty"`
			}
			var outcomes []outcome
			for _, pl := range planned {
				res, err := c.Contact(airbnb.ContactParams{ListingID: pl.ListingID, Message: pl.Message, Checkin: p.Checkin, Checkout: p.Checkout, Adults: p.Adults})
				oc := outcome{ListingID: pl.ListingID, Name: pl.Name, Status: "contacted"}
				if err != nil {
					oc.Status = "failed"
					oc.Error = err.Error()
				} else {
					rec := contactRecord{ListingID: pl.ListingID, ListingName: pl.Name, Message: pl.Message, ContactedAt: time.Now(), ThreadRaw: truncate(string(res), 200)}
					data, _ := json.Marshal(rec)
					_ = db.Upsert("contact", pl.ListingID, data)
				}
				outcomes = append(outcomes, oc)
				time.Sleep(1500 * time.Millisecond) // be polite between contacts
			}
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{"status": "done", "outcomes": outcomes})
			}
			for _, oc := range outcomes {
				mark := green("✓")
				if oc.Status != "contacted" {
					mark = red("✗")
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s (%s) %s\n", mark, truncate(oc.Name, 40), oc.ListingID, oc.Error)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&message, "message", "", "Message template ({name} and {id} are substituted)")
	cmd.Flags().IntVar(&limit, "limit", 5, "Maximum number of hosts to contact")
	cmd.Flags().StringVar(&p.Checkin, "checkin", "", "Check-in date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&p.Checkout, "checkout", "", "Check-out date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&p.Adults, "adults", 0, "Number of adults")
	cmd.Flags().IntVar(&p.PriceMax, "price-max", 0, "Maximum nightly price")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Actually contact the hosts (without this, only a preview is shown)")
	return cmd
}

func newOutreachCRMCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "crm",
		Short:   "Show hosts you've contacted (and reply status if the inbox is reachable)",
		Example: "  airbnb-outreach-pp-cli outreach crm --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := store.Open(defaultDBPath("airbnb-outreach-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := db.List("contact", 500)
			if err != nil {
				return err
			}
			recs := make([]contactRecord, 0, len(rows))
			for _, r := range rows {
				var rec contactRecord
				if json.Unmarshal(r, &rec) == nil {
					recs = append(recs, rec)
				}
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, recs)
			}
			if len(recs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No contacts logged yet. Run 'airbnb-outreach-pp-cli outreach run ... --confirm'.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, bold("LISTING\tNAME\tCONTACTED"))
			for _, rec := range recs {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", rec.ListingID, truncate(rec.ListingName, 40), rec.ContactedAt.Format("2006-01-02"))
			}
			return tw.Flush()
		},
	}
	return cmd
}

// renderTemplate substitutes {name} and {id} placeholders in an outreach message.
func renderTemplate(tmpl string, r airbnb.SearchResult) string {
	name := r.Name
	if name == "" {
		name = r.Title
	}
	out := strings.ReplaceAll(tmpl, "{name}", name)
	out = strings.ReplaceAll(out, "{id}", r.ID)
	return out
}
