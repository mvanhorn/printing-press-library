// Copyright 2026 Matias Sanchez Moises and contributors. Licensed under Apache-2.0. See LICENSE.

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
	contacts.AddCommand(newContactsMergeCmd(f))
	contacts.AddCommand(newContactsDuplicatesCmd(f))
	contacts.AddCommand(newContactsAnalyticsCmd(f))

	return contacts
}

// ── sync ──────────────────────────────────────────────────────────────────────

// contactCreateScript creates a new contact in Contacts.app.
// Values are received via "on run argv" so they are never interpolated into the
// script body — this prevents AppleScript injection via special characters or
// quote sequences in user-supplied strings.
// Argv order: firstName, lastName, middleName, org, jobTitle, note, phone, email.
// Empty string args are accepted and skipped (no field is set).
const contactCreateScript = `
on run argv
	tell application "Contacts"
		set propsRec to {}
		if (count of argv) >= 1 and item 1 of argv is not "" then set propsRec to propsRec & {first name: item 1 of argv}
		if (count of argv) >= 2 and item 2 of argv is not "" then set propsRec to propsRec & {last name: item 2 of argv}
		if (count of argv) >= 3 and item 3 of argv is not "" then set propsRec to propsRec & {middle name: item 3 of argv}
		if (count of argv) >= 4 and item 4 of argv is not "" then set propsRec to propsRec & {organization: item 4 of argv}
		if (count of argv) >= 5 and item 5 of argv is not "" then set propsRec to propsRec & {job title: item 5 of argv}
		if (count of argv) >= 6 and item 6 of argv is not "" then set propsRec to propsRec & {note: item 6 of argv}
		set newPerson to make new person with properties propsRec
		if (count of argv) >= 7 and item 7 of argv is not "" then
			make new phone at end of phones of newPerson with properties {label: "mobile", value: item 7 of argv}
		end if
		if (count of argv) >= 8 and item 8 of argv is not "" then
			make new email at end of emails of newPerson with properties {label: "work", value: item 8 of argv}
		end if
		save
		return id of newPerson
	end tell
end run
`

// contactUpdateScript updates fields on an existing contact by ID.
// Argv order: id, firstName, lastName, middleName, org, jobTitle, note, addPhone, addEmail.
// Empty string args are skipped — only non-empty values overwrite existing fields.
const contactUpdateScript = `
on run argv
	tell application "Contacts"
		set p to person id (item 1 of argv)
		if (count of argv) >= 2 and item 2 of argv is not "" then set first name of p to item 2 of argv
		if (count of argv) >= 3 and item 3 of argv is not "" then set last name of p to item 3 of argv
		if (count of argv) >= 4 and item 4 of argv is not "" then set middle name of p to item 4 of argv
		if (count of argv) >= 5 and item 5 of argv is not "" then set organization of p to item 5 of argv
		if (count of argv) >= 6 and item 6 of argv is not "" then set job title of p to item 6 of argv
		if (count of argv) >= 7 and item 7 of argv is not "" then set note of p to item 7 of argv
		if (count of argv) >= 8 and item 8 of argv is not "" then
			make new phone at end of phones of p with properties {label: "mobile", value: item 8 of argv}
		end if
		if (count of argv) >= 9 and item 9 of argv is not "" then
			make new email at end of emails of p with properties {label: "work", value: item 9 of argv}
		end if
		save
		return "ok"
	end tell
end run
`

// contactDeleteScript deletes a contact by ID.
// Argv order: id.
const contactDeleteScript = `
on run argv
	tell application "Contacts"
		set p to person id (item 1 of argv)
		delete p
		save
		return "ok"
	end tell
end run
`

// contactMergeScript copies all phones, emails, URLs, and addresses from
// contact 2 into contact 1, appends contact 2's note, then deletes contact 2.
// Argv order: id1 (primary), id2 (to absorb).
const contactMergeScript = `
on run argv
	tell application "Contacts"
		set p1 to person id (item 1 of argv)
		set p2 to person id (item 2 of argv)
		repeat with ph in (get phones of p2)
			set v to value of ph
			set isDup to false
			repeat with existing in (phones of p1)
				if value of existing is v then
					set isDup to true
					exit repeat
				end if
			end repeat
			if not isDup then
				make new phone at end of phones of p1 with properties {label: label of ph, value: v}
			end if
		end repeat
		repeat with em in (get emails of p2)
			set v to value of em
			set isDup to false
			repeat with existing in (emails of p1)
				if value of existing is v then
					set isDup to true
					exit repeat
				end if
			end repeat
			if not isDup then
				make new email at end of emails of p1 with properties {label: label of em, value: v}
			end if
		end repeat
		repeat with u in (get urls of p2)
			set v to value of u
			set isDup to false
			repeat with existing in (urls of p1)
				if value of existing is v then
					set isDup to true
					exit repeat
				end if
			end repeat
			if not isDup then
				make new url at end of urls of p1 with properties {label: label of u, value: v}
			end if
		end repeat
		repeat with addr in (get addresses of p2)
			set s to street of addr
			set c to city of addr
			set z to zip of addr
			set isDup to false
			repeat with existing in (addresses of p1)
				if (street of existing is s) and (city of existing is c) and (zip of existing is z) then
					set isDup to true
					exit repeat
				end if
			end repeat
			if not isDup then
				make new address at end of addresses of p1 with properties {label: label of addr, street: s, city: c, state: state of addr, zip: z, country: country of addr}
			end if
		end repeat
		set n2 to note of p2
		if n2 is not missing value and n2 is not "" then
			set n1 to note of p1
			if n1 is missing value or n1 is "" then
				set note of p1 to n2
			else
				set note of p1 to (n1 & return & "---" & return & n2)
			end if
		end if
		delete p2
		save
		return id of p1
	end tell
end run
`

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

			c, err := store.GetByAny(args[0])
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

			// Pass values as out-of-band argv — no string interpolation into the
			// script body, so quotes and backslashes in user data cannot escape
			// the AppleScript string context.
			appleID, err := runOsascriptWithArgs(contactCreateScript,
				firstName, lastName, middle, org, jobTitle, note, phone, email)
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

			// Resolve id (accepts full Apple ID, bare UUID, or UUID prefix)
			store, err := openContactStore()
			if err != nil {
				return err
			}
			resolved, err := store.GetByAny(args[0])
			store.Close()
			if err != nil {
				return err
			}
			if resolved == nil {
				return fmt.Errorf("contact not found: %s", args[0])
			}
			id := resolved.ID // use full Apple ID for AppleScript

			// Pass ID and field values as out-of-band argv — no string interpolation.
			if _, err := runOsascriptWithArgs(contactUpdateScript,
				id, firstName, lastName, middle, org, jobTitle, note, addPhone, addEmail); err != nil {
				return fmt.Errorf("updating contact: %w", err)
			}

			// Refresh local store entry.
			store2, err := openContactStore()
			if err != nil {
				return err
			}
			defer store2.Close()
			store = store2

			existing, err := store.Get(id)
			if err == nil && existing != nil {
				if firstName != "" {
					existing.FirstName = firstName
				}
				if lastName != "" {
					existing.LastName = lastName
				}
				if middle != "" {
					existing.MiddleName = middle
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

			// Resolve id (accepts full Apple ID, bare UUID, or UUID prefix)
			store, err := openContactStore()
			if err != nil {
				return err
			}
			c, err := store.GetByAny(args[0])
			store.Close()
			if err != nil {
				return err
			}
			if c == nil {
				return fmt.Errorf("contact not found: %s", args[0])
			}
			id := c.ID // use full Apple ID for AppleScript

			// Pass id as out-of-band argv — prevents injection if the ID ever
			// contains characters that AppleScript would misparse in a string literal.
			if _, err := runOsascriptWithArgs(contactDeleteScript, id); err != nil {
				return fmt.Errorf("deleting contact: %w", err)
			}

			store2, err := openContactStore()
			if err != nil {
				return err
			}
			defer store2.Close()
			if err := store2.Delete(id); err != nil {
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

// ── merge ─────────────────────────────────────────────────────────────────────

func newContactsMergeCmd(f *rootFlags) *cobra.Command {
	var confirm bool
	cmd := &cobra.Command{
		Use:   "merge <id1> <id2>",
		Short: "Merge contact 2 into contact 1, combining all data",
		Args:  cobra.ExactArgs(2),
		Example: `  icloud-pp-cli contacts merge 7D7D265B 3E9B2104
  icloud-pp-cli contacts merge 7D7D265B 3E9B2104 --confirm`,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			store, err := openContactStore()
			if err != nil {
				return err
			}
			defer store.Close()

			primary, err := store.GetByAny(args[0])
			if err != nil {
				return fmt.Errorf("contact 1: %w", err)
			}
			if primary == nil {
				return fmt.Errorf("contact not found: %s", args[0])
			}
			absorbed, err := store.GetByAny(args[1])
			if err != nil {
				return fmt.Errorf("contact 2: %w", err)
			}
			if absorbed == nil {
				return fmt.Errorf("contact not found: %s", args[1])
			}
			// Self-merge would have AppleScript copy values onto the same
			// scripting object and then `delete p2` — which deletes the
			// shared contact outright. Refuse before any side effects.
			if primary.ID == absorbed.ID {
				return fmt.Errorf("contact 1 and contact 2 resolve to the same contact (%s) — nothing to merge", primary.ID)
			}

			// Compute what would be added
			phoneSet := map[string]bool{}
			for _, ph := range primary.Phones {
				phoneSet[normalizePhone(ph.Value)] = true
			}
			var newPhones []ContactPhone
			for _, ph := range absorbed.Phones {
				if !phoneSet[normalizePhone(ph.Value)] {
					newPhones = append(newPhones, ph)
				}
			}
			emailSet := map[string]bool{}
			for _, em := range primary.Emails {
				emailSet[strings.ToLower(em.Value)] = true
			}
			var newEmails []ContactEmail
			for _, em := range absorbed.Emails {
				if !emailSet[strings.ToLower(em.Value)] {
					newEmails = append(newEmails, em)
				}
			}

			// Preview
			fmt.Fprintf(out, "\n%s  %s\n", bold(f, out, "Primary:"), primary.DisplayName())
			fmt.Fprintf(out, "    UUID: %s\n", primary.UUID)
			fmt.Fprintf(out, "%s  %s\n", bold(f, out, "Absorb: "), absorbed.DisplayName())
			fmt.Fprintf(out, "    UUID: %s\n", absorbed.UUID)
			fmt.Fprintln(out)

			if len(newPhones) > 0 {
				fmt.Fprintln(out, "  Phones to add:")
				for _, ph := range newPhones {
					fmt.Fprintf(out, "    %-10s %s\n", ph.Label, ph.Value)
				}
			} else {
				fmt.Fprintln(out, "  Phones to add: (none new)")
			}
			if len(newEmails) > 0 {
				fmt.Fprintln(out, "  Emails to add:")
				for _, em := range newEmails {
					fmt.Fprintf(out, "    %-10s %s\n", em.Label, em.Value)
				}
			} else {
				fmt.Fprintln(out, "  Emails to add: (none new)")
			}
			if len(absorbed.Addresses) > 0 {
				fmt.Fprintf(out, "  Addresses to add: %d\n", len(absorbed.Addresses))
			}
			if absorbed.Note != "" {
				fmt.Fprintln(out, "  Note: will be appended")
			}
			fmt.Fprintln(out)

			if !confirm {
				fmt.Fprintln(out, yellow(f, out, "  Run with --confirm to execute the merge."))
				return nil
			}

			if _, err := runOsascriptWithArgs(contactMergeScript, primary.ID, absorbed.ID); err != nil {
				return fmt.Errorf("merge in Contacts.app: %w", err)
			}
			if _, err := store.MergeIntoStore(primary.ID, absorbed.ID); err != nil {
				fmt.Fprintf(os.Stderr, "warning: local store merge failed: %v\n", err)
			}

			if f.asJSON || !isTerminal(out) {
				return printJSON(out, map[string]string{
					"status":     "merged",
					"primary_id": primary.ID,
					"removed_id": absorbed.ID,
				})
			}
			fmt.Fprintf(out, "%s Merged %s into %s\n",
				green(f, out, "✓"), absorbed.DisplayName(), primary.DisplayName())
			fmt.Fprintf(out, "   Primary UUID: %s\n", primary.UUID)
			return nil
		},
	}
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Required: confirm the merge")
	return cmd
}

// ── duplicates ────────────────────────────────────────────────────────────────

func newContactsDuplicatesCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "duplicates",
		Short: "Find potential duplicate contacts (same name, phone, or email)",
		Example: `  icloud-pp-cli contacts duplicates
  icloud-pp-cli contacts duplicates --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			store, err := openContactStore()
			if err != nil {
				return err
			}
			defer store.Close()

			pairs, err := store.FindDuplicates()
			if err != nil {
				return fmt.Errorf("finding duplicates: %w", err)
			}
			if len(pairs) == 0 {
				fmt.Fprintln(out, green(f, out, "✓")+" No duplicate contacts found.")
				return nil
			}

			if f.asJSON || !isTerminal(out) {
				return printJSON(out, pairs)
			}

			reasonLabel := map[string]string{
				"exact_name":   "Exact name match",
				"similar_name": "Similar name (possible suffix)",
			}

			fmt.Fprintf(out, "\n%s\n\n", bold(f, out, fmt.Sprintf("Found %d potential duplicate pair(s)", len(pairs))))
			for i, p := range pairs {
				label := p.Reason
				if l, ok := reasonLabel[p.Reason]; ok {
					label = l
				} else if strings.HasPrefix(p.Reason, "same_phone:") {
					label = "Same phone: " + strings.TrimPrefix(p.Reason, "same_phone:")
				} else if strings.HasPrefix(p.Reason, "same_email:") {
					label = "Same email: " + strings.TrimPrefix(p.Reason, "same_email:")
				}
				fmt.Fprintf(out, "  %d. %s\n", i+1, bold(f, out, label))
				shortA := p.A.UUID
				if len(shortA) > 8 {
					shortA = shortA[:8]
				}
				shortB := p.B.UUID
				if len(shortB) > 8 {
					shortB = shortB[:8]
				}
				phA := ""
				if len(p.A.Phones) > 0 {
					phA = "  " + p.A.Phones[0].Value
				}
				phB := ""
				if len(p.B.Phones) > 0 {
					phB = "  " + p.B.Phones[0].Value
				}
				fmt.Fprintf(out, "     [%s]  %-30s%s\n", shortA, truncate(p.A.DisplayName(), 30), phA)
				fmt.Fprintf(out, "     [%s]  %-30s%s\n", shortB, truncate(p.B.DisplayName(), 30), phB)
				fmt.Fprintf(out, "     → contacts merge %s %s --confirm\n\n", shortA, shortB)
			}
			return nil
		},
	}
}

// ── display helpers ───────────────────────────────────────────────────────────

func printContactsTable(f *rootFlags, out io.Writer, cs []Contact) {
	tw := newTabWriter(out)
	fmt.Fprintln(tw, bold(f, out, "UUID")+"\t"+bold(f, out, "Name")+"\t"+bold(f, out, "Phone")+"\t"+bold(f, out, "Email")+"\t"+bold(f, out, "Company"))
	fmt.Fprintln(tw, strings.Repeat("─", 8)+"\t"+strings.Repeat("─", 20)+"\t"+strings.Repeat("─", 18)+"\t"+strings.Repeat("─", 22)+"\t"+strings.Repeat("─", 16))
	for _, c := range cs {
		short := c.UUID
		if len(short) > 8 {
			short = short[:8]
		}
		ph := ""
		if len(c.Phones) > 0 {
			ph = c.Phones[0].Value
		}
		em := ""
		if len(c.Emails) > 0 {
			em = c.Emails[0].Value
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			short,
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
	if c.UUID != "" {
		fmt.Fprintf(out, "  UUID: %s\n", c.UUID)
	}
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

// runOsascriptWithArgs runs an "on run argv" AppleScript, passing values as
// separate process arguments after "--" rather than interpolating them into the
// script body. This is the safe way to pass user-supplied strings to AppleScript:
// the values are never parsed by the AppleScript lexer, so quotes, backslashes,
// and other special characters cannot escape the string context or inject code.
func runOsascriptWithArgs(script string, args ...string) (string, error) {
	out, err := osascriptRawWithArgs(script, args...)
	if err == nil {
		return out, nil
	}
	// Contacts.app may not be running (-600). Launch it silently and retry.
	if strings.Contains(err.Error(), "-600") || strings.Contains(err.Error(), "isn't running") {
		if launchErr := exec.Command("open", "-a", "Contacts").Run(); launchErr == nil {
			for i := 0; i < 6; i++ {
				time.Sleep(500 * time.Millisecond)
				if out2, err2 := osascriptRawWithArgs(script, args...); err2 == nil {
					return out2, nil
				}
			}
		}
	}
	return "", err
}

func osascriptRawWithArgs(script string, args ...string) (string, error) {
	cmdArgs := append([]string{"-e", script, "--"}, args...)
	cmd := exec.Command("osascript", cmdArgs...)
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
