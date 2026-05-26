// Copyright 2026 matysanchez. Licensed under Apache-2.0. See LICENSE.

// Package cli — contacts.go
// contacts command group: sync, list, get, search, create, update, delete.
// Reads use the local SQLite store (fast). Writes go through Contacts.app
// via JXA/AppleScript then update the local store.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newContactsCmd(f *rootFlags) *cobra.Command {
	contacts := &cobra.Command{
		Use:   "contacts",
		Short: "Query and manage your Contacts library",
		Long: `Read and write your macOS Contacts library locally — no network calls required.

Sync once to populate the local SQLite cache, then list/search/get run instantly.
Write operations (create, update, delete) go through Contacts.app and update the cache.`,
	}

	contacts.AddCommand(newContactsSyncCmd(f))
	contacts.AddCommand(newContactsListCmd(f))
	contacts.AddCommand(newContactsGetCmd(f))
	contacts.AddCommand(newContactsSearchCmd(f))
	contacts.AddCommand(newContactsCreateCmd(f))
	contacts.AddCommand(newContactsUpdateCmd(f))
	contacts.AddCommand(newContactsDeleteCmd(f))
	contacts.AddCommand(newContactsAnalyticsCmd(f))

	return contacts
}

// ── sync ──────────────────────────────────────────────────────────────────────

// jxaSyncScript exports all contacts via JavaScript for Automation (JXA).
// Single call — much faster than iterating in Go.
const jxaSyncScript = `
var app = Application("Contacts");
var people = app.people();
var result = people.map(function(p) {
  var phones = [];
  try { phones = p.phones().map(function(ph) { return {label: ph.label(), value: ph.value()}; }); } catch(e) {}
  var emails = [];
  try { emails = p.emails().map(function(e) { return {label: e.label(), value: e.value()}; }); } catch(e) {}
  var addresses = [];
  try { addresses = p.addresses().map(function(a) {
    return {label: a.label(), street: a.street(), city: a.city(), state: a.state(), zip: a.zip(), country: a.country()};
  }); } catch(e) {}
  var urls = [];
  try { urls = p.urls().map(function(u) { return {label: u.label(), value: u.value()}; }); } catch(e) {}
  return {
    id:           p.id(),
    firstName:    p.firstName(),
    lastName:     p.lastName(),
    middleName:   p.middleName(),
    organization: p.organization(),
    jobTitle:     p.jobTitle(),
    note:         p.note(),
    modifiedAt:   p.modificationDate() ? p.modificationDate().toISOString() : null,
    phones:       phones,
    emails:       emails,
    addresses:    addresses,
    urls:         urls
  };
});
JSON.stringify(result);
`

func newContactsSyncCmd(f *rootFlags) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync contacts from Contacts.app into local SQLite cache",
		Long: `Pulls all contacts from Contacts.app via JavaScript for Automation (JXA)
and stores them in a local SQLite database for instant querying.

Run sync once before using list, search, or analytics. Re-run after adding
or importing contacts to pick up changes.`,
		Example: `  icloud-pp-cli contacts sync
  icloud-pp-cli contacts sync --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			store, err := openContactStore()
			if err != nil {
				return fmt.Errorf("opening contacts store: %w", err)
			}
			defer store.Close()

			if !force {
				last := store.LastSyncedAt()
				if last != "" {
					fmt.Fprintf(out, "  %s Last synced: %s\n", yellow(f, out, "i"), last)
					fmt.Fprintln(out, "  Use --force to re-sync.")
					count, _ := store.Count()
					fmt.Fprintf(out, "  %s %s contacts in local store.\n", green(f, out, "✓"), formatInt(int64(count)))
					return nil
				}
			}

			fmt.Fprintln(out, bold(f, out, "Syncing contacts from Contacts.app..."))
			start := time.Now()

			// Run JXA — single call, returns full JSON array.
			fmt.Fprintln(out, "  → Fetching contacts via JXA...")
			raw, err := runOsascriptJS(jxaSyncScript)
			if err != nil {
				return fmt.Errorf("JXA sync failed: %w", err)
			}

			var contacts []jxaContact
			if err := json.Unmarshal([]byte(raw), &contacts); err != nil {
				return fmt.Errorf("parsing JXA output: %w", err)
			}
			fmt.Fprintf(out, "  → Fetched %s contacts\n", formatInt(int64(len(contacts))))

			fmt.Fprintln(out, "  → Resolving phone countries & indexing...")
			n, err := store.SyncAll(contacts)
			if err != nil {
				return fmt.Errorf("storing contacts: %w", err)
			}

			elapsed := time.Since(start).Round(time.Millisecond)
			fmt.Fprintln(out)
			fmt.Fprintf(out, "%s %s contacts synced in %s\n",
				green(f, out, "✓"), formatInt(int64(n)), elapsed)
			fmt.Fprintf(out, "    DB: %s\n", contactsDBPath())
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Re-sync even if already synced")
	return cmd
}

// ── list ──────────────────────────────────────────────────────────────────────

func newContactsListCmd(f *rootFlags) *cobra.Command {
	var limit, offset int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List contacts from local cache",
		Example: `  icloud-pp-cli contacts list
  icloud-pp-cli contacts list --limit 100
  icloud-pp-cli contacts list --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			store, err := openContactStore()
			if err != nil {
				return err
			}
			defer store.Close()

			cs, err := store.List(limit, offset)
			if err != nil {
				return err
			}
			if len(cs) == 0 {
				fmt.Fprintln(out, "No contacts in local store. Run: icloud-pp-cli contacts sync")
				return nil
			}

			if f.asJSON || !isTerminal(out) {
				return printJSON(out, cs)
			}
			printContactsTable(f, out, cs)
			total, _ := store.Count()
			if total > limit+offset {
				fmt.Fprintf(out, "\n  Showing %d of %s. Use --offset %d for next page.\n",
					len(cs), formatInt(int64(total)), offset+limit)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Max contacts to return")
	cmd.Flags().IntVar(&offset, "offset", 0, "Skip N contacts (for pagination)")
	return cmd
}

// ── get ───────────────────────────────────────────────────────────────────────

func newContactsGetCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get full details for a contact by UUID",
		Args:  cobra.ExactArgs(1),
		Example: `  icloud-pp-cli contacts get "7D7D265B-D6E9-4F41-9E37-2D97AE2C00FC:ABPerson"
  icloud-pp-cli contacts get "7D7D265B..." --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			store, err := openContactStore()
			if err != nil {
				return err
			}
			defer store.Close()

			c, err := store.Get(args[0])
			if err != nil {
				return err
			}
			if c == nil {
				return fmt.Errorf("contact not found: %s", args[0])
			}

			if f.asJSON || !isTerminal(out) {
				return printJSON(out, c)
			}
			printContactDetail(f, out, c)
			return nil
		},
	}
}

// ── search ────────────────────────────────────────────────────────────────────

func newContactsSearchCmd(f *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search across name, org, phone, and email",
		Args:  cobra.ExactArgs(1),
		Example: `  icloud-pp-cli contacts search "juan"
  icloud-pp-cli contacts search "gmail.com"
  icloud-pp-cli contacts search "+52"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			store, err := openContactStore()
			if err != nil {
				return err
			}
			defer store.Close()

			cs, err := store.Search(args[0], limit)
			if err != nil {
				return fmt.Errorf("search failed: %w\nTip: run 'contacts sync' first", err)
			}
			if len(cs) == 0 {
				fmt.Fprintf(out, "No results for %q\n", args[0])
				return nil
			}

			if f.asJSON || !isTerminal(out) {
				return printJSON(out, cs)
			}
			printContactsTable(f, out, cs)
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "Max results")
	return cmd
}

// ── create ────────────────────────────────────────────────────────────────────

func newContactsCreateCmd(f *rootFlags) *cobra.Command {
	var (
		firstName, lastName, middle string
		org, jobTitle, note         string
		phone, email                string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new contact in Contacts.app",
		Example: `  icloud-pp-cli contacts create --first "Ana" --last "García" --phone "+52 984 100 0000"
  icloud-pp-cli contacts create --first "Acme" --org "Acme Corp" --email "info@acme.com"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			if firstName == "" && org == "" {
				return usageErr(fmt.Errorf("--first or --org is required"))
			}

			// Build AppleScript
			var sb strings.Builder
			sb.WriteString("tell application \"Contacts\"\n")
			sb.WriteString("  set propsRec to {}\n")
			if firstName != "" {
				sb.WriteString(fmt.Sprintf("  set propsRec to propsRec & {first name:%q}\n", firstName))
			}
			if lastName != "" {
				sb.WriteString(fmt.Sprintf("  set propsRec to propsRec & {last name:%q}\n", lastName))
			}
			if middle != "" {
				sb.WriteString(fmt.Sprintf("  set propsRec to propsRec & {middle name:%q}\n", middle))
			}
			if org != "" {
				sb.WriteString(fmt.Sprintf("  set propsRec to propsRec & {organization:%q}\n", org))
			}
			if jobTitle != "" {
				sb.WriteString(fmt.Sprintf("  set propsRec to propsRec & {job title:%q}\n", jobTitle))
			}
			if note != "" {
				sb.WriteString(fmt.Sprintf("  set propsRec to propsRec & {note:%q}\n", note))
			}
			sb.WriteString("  set newPerson to make new person with properties propsRec\n")
			if phone != "" {
				sb.WriteString(fmt.Sprintf("  make new phone at end of phones of newPerson with properties {label:\"mobile\", value:%q}\n", phone))
			}
			if email != "" {
				sb.WriteString(fmt.Sprintf("  make new email at end of emails of newPerson with properties {label:\"work\", value:%q}\n", email))
			}
			sb.WriteString("  save\n")
			sb.WriteString("  return id of newPerson\n")
			sb.WriteString("end tell\n")

			appleID, err := runOsascript(sb.String())
			if err != nil {
				return fmt.Errorf("creating contact: %w", err)
			}
			appleID = strings.TrimSpace(appleID)

			// Upsert into local store
			store, err := openContactStore()
			if err != nil {
				return err
			}
			defer store.Close()

			c := &Contact{
				ID:           appleID,
				FirstName:    firstName,
				LastName:     lastName,
				MiddleName:   middle,
				Organization: org,
				JobTitle:     jobTitle,
				Note:         note,
			}
			if phone != "" {
				c.Phones = []ContactPhone{{Label: "mobile", Value: phone}}
			}
			if email != "" {
				c.Emails = []ContactEmail{{Label: "work", Value: email}}
			}
			if err := store.UpsertOne(c); err != nil {
				fmt.Fprintf(os.Stderr, "warning: local store update failed: %v\n", err)
			}

			if f.asJSON || !isTerminal(out) {
				return printJSON(out, map[string]string{"id": appleID, "status": "created"})
			}
			fmt.Fprintf(out, "%s Created: %s\n", green(f, out, "✓"), c.DisplayName())
			fmt.Fprintf(out, "   ID: %s\n", appleID)
			return nil
		},
	}
	cmd.Flags().StringVar(&firstName, "first", "", "First name")
	cmd.Flags().StringVar(&lastName, "last", "", "Last name")
	cmd.Flags().StringVar(&middle, "middle", "", "Middle name")
	cmd.Flags().StringVar(&org, "org", "", "Organization / company")
	cmd.Flags().StringVar(&jobTitle, "job-title", "", "Job title")
	cmd.Flags().StringVar(&note, "note", "", "Note")
	cmd.Flags().StringVar(&phone, "phone", "", "Phone number (e.g. +52 984 100 0000)")
	cmd.Flags().StringVar(&email, "email", "", "Email address")
	return cmd
}

// ── update ────────────────────────────────────────────────────────────────────

func newContactsUpdateCmd(f *rootFlags) *cobra.Command {
	var (
		firstName, lastName, middle string
		org, jobTitle, note         string
		addPhone, addEmail          string
	)
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update fields on an existing contact",
		Args:  cobra.ExactArgs(1),
		Example: `  icloud-pp-cli contacts update "UUID:ABPerson" --org "New Corp"
  icloud-pp-cli contacts update "UUID:ABPerson" --add-phone "+1 555 000 0001"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			id := args[0]

			var sb strings.Builder
			sb.WriteString("tell application \"Contacts\"\n")
			sb.WriteString(fmt.Sprintf("  set p to person id %q\n", id))
			if firstName != "" {
				sb.WriteString(fmt.Sprintf("  set first name of p to %q\n", firstName))
			}
			if lastName != "" {
				sb.WriteString(fmt.Sprintf("  set last name of p to %q\n", lastName))
			}
			if middle != "" {
				sb.WriteString(fmt.Sprintf("  set middle name of p to %q\n", middle))
			}
			if org != "" {
				sb.WriteString(fmt.Sprintf("  set organization of p to %q\n", org))
			}
			if jobTitle != "" {
				sb.WriteString(fmt.Sprintf("  set job title of p to %q\n", jobTitle))
			}
			if note != "" {
				sb.WriteString(fmt.Sprintf("  set note of p to %q\n", note))
			}
			if addPhone != "" {
				sb.WriteString(fmt.Sprintf("  make new phone at end of phones of p with properties {label:\"mobile\", value:%q}\n", addPhone))
			}
			if addEmail != "" {
				sb.WriteString(fmt.Sprintf("  make new email at end of emails of p with properties {label:\"work\", value:%q}\n", addEmail))
			}
			sb.WriteString("  save\n")
			sb.WriteString("  return \"ok\"\n")
			sb.WriteString("end tell\n")

			if _, err := runOsascript(sb.String()); err != nil {
				return fmt.Errorf("updating contact: %w", err)
			}

			// Refresh local store entry.
			store, err := openContactStore()
			if err != nil {
				return err
			}
			defer store.Close()

			existing, err := store.Get(id)
			if err == nil && existing != nil {
				if firstName != "" {
					existing.FirstName = firstName
				}
				if lastName != "" {
					existing.LastName = lastName
				}
				if org != "" {
					existing.Organization = org
				}
				if jobTitle != "" {
					existing.JobTitle = jobTitle
				}
				if note != "" {
					existing.Note = note
				}
				if addPhone != "" {
					existing.Phones = append(existing.Phones, ContactPhone{Label: "mobile", Value: addPhone})
				}
				if addEmail != "" {
					existing.Emails = append(existing.Emails, ContactEmail{Label: "work", Value: addEmail})
				}
				if err := store.UpsertOne(existing); err != nil {
					fmt.Fprintf(os.Stderr, "warning: local store update failed: %v\n", err)
				}
			}

			if f.asJSON || !isTerminal(out) {
				return printJSON(out, map[string]string{"id": id, "status": "updated"})
			}
			fmt.Fprintf(out, "%s Updated contact %s\n", green(f, out, "✓"), id)
			return nil
		},
	}
	cmd.Flags().StringVar(&firstName, "first", "", "New first name")
	cmd.Flags().StringVar(&lastName, "last", "", "New last name")
	cmd.Flags().StringVar(&middle, "middle", "", "New middle name")
	cmd.Flags().StringVar(&org, "org", "", "New organization")
	cmd.Flags().StringVar(&jobTitle, "job-title", "", "New job title")
	cmd.Flags().StringVar(&note, "note", "", "New note")
	cmd.Flags().StringVar(&addPhone, "add-phone", "", "Add a phone number")
	cmd.Flags().StringVar(&addEmail, "add-email", "", "Add an email address")
	return cmd
}

// ── delete ────────────────────────────────────────────────────────────────────

func newContactsDeleteCmd(f *rootFlags) *cobra.Command {
	var confirm bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a contact from Contacts.app (permanent)",
		Args:  cobra.ExactArgs(1),
		Example: `  icloud-pp-cli contacts delete "UUID:ABPerson" --confirm`,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			if !confirm {
				return usageErr(fmt.Errorf("pass --confirm to delete the contact permanently"))
			}
			id := args[0]

			script := fmt.Sprintf(`
tell application "Contacts"
  set p to person id %q
  delete p
  save
  return "ok"
end tell`, id)

			if _, err := runOsascript(script); err != nil {
				return fmt.Errorf("deleting contact: %w", err)
			}

			store, err := openContactStore()
			if err != nil {
				return err
			}
			defer store.Close()
			if err := store.Delete(id); err != nil {
				fmt.Fprintf(os.Stderr, "warning: local store delete failed: %v\n", err)
			}

			if f.asJSON || !isTerminal(out) {
				return printJSON(out, map[string]string{"id": id, "status": "deleted"})
			}
			fmt.Fprintf(out, "%s Deleted contact %s\n", red(f, out, "✗"), id)
			return nil
		},
	}
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Required: confirm permanent deletion")
	return cmd
}

// ── display helpers ───────────────────────────────────────────────────────────

func printContactsTable(f *rootFlags, out io.Writer, cs []Contact) {
	tw := newTabWriter(out)
	fmt.Fprintln(tw, bold(f, out, "Name")+"\t"+bold(f, out, "Phone")+"\t"+bold(f, out, "Email")+"\t"+bold(f, out, "Company"))
	fmt.Fprintln(tw, strings.Repeat("─", 20)+"\t"+strings.Repeat("─", 18)+"\t"+strings.Repeat("─", 22)+"\t"+strings.Repeat("─", 16))
	for _, c := range cs {
		ph := ""
		if len(c.Phones) > 0 {
			ph = c.Phones[0].Value
		}
		em := ""
		if len(c.Emails) > 0 {
			em = c.Emails[0].Value
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			truncate(c.DisplayName(), 28),
			truncate(ph, 20),
			truncate(em, 24),
			truncate(c.Organization, 18),
		)
	}
	tw.Flush()
}

func printContactDetail(f *rootFlags, out io.Writer, c *Contact) {
	fmt.Fprintf(out, "\n%s\n", bold(f, out, c.DisplayName()))
	fmt.Fprintf(out, "  ID:  %s\n", c.ID)
	if c.Organization != "" {
		fmt.Fprintf(out, "  Org: %s\n", c.Organization)
	}
	if c.JobTitle != "" {
		fmt.Fprintf(out, "  Job: %s\n", c.JobTitle)
	}
	if c.Birthday != "" {
		fmt.Fprintf(out, "  Born: %s\n", c.Birthday)
	}
	if len(c.Phones) > 0 {
		fmt.Fprintln(out, "  Phones:")
		for _, ph := range c.Phones {
			flag := countryFlag(ph.CountryISO)
			fmt.Fprintf(out, "    %-10s %s  %s %s\n", ph.Label, ph.Value, flag, ph.Country)
		}
	}
	if len(c.Emails) > 0 {
		fmt.Fprintln(out, "  Emails:")
		for _, em := range c.Emails {
			fmt.Fprintf(out, "    %-10s %s\n", em.Label, em.Value)
		}
	}
	if len(c.Addresses) > 0 {
		fmt.Fprintln(out, "  Addresses:")
		for _, a := range c.Addresses {
			parts := filterEmpty(a.Street, a.City, a.State, a.Zip, a.Country)
			fmt.Fprintf(out, "    %-10s %s\n", a.Label, strings.Join(parts, ", "))
		}
	}
	if len(c.URLs) > 0 {
		fmt.Fprintln(out, "  URLs:")
		for _, u := range c.URLs {
			fmt.Fprintf(out, "    %-10s %s\n", u.Label, u.Value)
		}
	}
	if c.Note != "" {
		fmt.Fprintf(out, "  Note: %s\n", truncate(c.Note, 80))
	}
	if c.ModifiedAt != "" {
		fmt.Fprintf(out, "  Modified: %s\n", c.ModifiedAt)
	}
	fmt.Fprintln(out)
}

// countryFlag converts ISO2 code to emoji flag (works on macOS Terminal).
func countryFlag(iso string) string {
	if len(iso) != 2 {
		return "  "
	}
	r1 := rune(iso[0]-'A') + 0x1F1E6
	r2 := rune(iso[1]-'A') + 0x1F1E6
	return string([]rune{r1, r2})
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}

// ── osascript helpers ─────────────────────────────────────────────────────────

func runOsascript(script string) (string, error) {
	cmd := exec.Command("osascript", "-e", script)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(ee.Stderr)))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func runOsascriptJS(script string) (string, error) {
	cmd := exec.Command("osascript", "-l", "JavaScript", "-e", script)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(ee.Stderr)))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
