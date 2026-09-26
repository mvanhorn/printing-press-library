// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		if parent := tbFindChild(root, "contacts"); parent != nil {
			addNovelCommandIfAbsent(parent, newTBContactsSearchCmd(flags))
			addNovelCommandIfAbsent(parent, newTBContactsShowCmd(flags))
		}
	})
}

// tbContactDoc is the stored JSON shape of a contact.
type tbContactDoc struct {
	ID           string   `json:"id"`
	Book         string   `json:"book"`
	Collected    bool     `json:"collected"`
	DisplayName  string   `json:"display_name"`
	FirstName    string   `json:"first_name"`
	LastName     string   `json:"last_name"`
	NickName     string   `json:"nickname"`
	Company      string   `json:"company"`
	Emails       []string `json:"emails"`
	PrimaryEmail string   `json:"primary_email"`
}

func tbContactMatches(c tbContactDoc, q string) bool {
	q = strings.ToLower(strings.TrimSpace(q))
	fields := append([]string{c.DisplayName, c.FirstName, c.LastName, c.NickName, strings.TrimSpace(c.FirstName + " " + c.LastName)}, c.Emails...)
	for _, f := range fields {
		if f != "" && strings.Contains(strings.ToLower(f), q) {
			return true
		}
	}
	return false
}

func tbSortContacts(rows []tbContactDoc) {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Collected != rows[j].Collected {
			return !rows[i].Collected
		}
		a, b := strings.ToLower(tbContactLabel(rows[i])), strings.ToLower(tbContactLabel(rows[j]))
		if a != b {
			return a < b
		}
		return rows[i].ID < rows[j].ID
	})
}

func tbContactName(c tbContactDoc) string {
	if n := strings.TrimSpace(c.DisplayName); n != "" {
		return n
	}
	return strings.TrimSpace(c.FirstName + " " + c.LastName)
}

func tbContactLabel(c tbContactDoc) string {
	if n := tbContactName(c); n != "" {
		return n
	}
	return c.PrimaryEmail
}

func tbLoadContacts(cmd *cobra.Command, flags *rootFlags) ([]tbContactDoc, bool, error) {
	db, err := tbStoreFor(cmd, flags, "contacts")
	if err != nil || db == nil {
		return nil, false, err
	}
	defer db.Close()
	docs, err := tbLoadDocs[tbContactDoc](db, "contacts")
	for i := range docs {
		if docs[i].Emails == nil {
			docs[i].Emails = []string{}
		}
	}
	return docs, true, err
}

func tbPrintContacts(cmd *cobra.Command, flags *rootFlags, rows []tbContactDoc) error {
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
	}
	tw := newTabWriter(tbHumanOut(cmd))
	fmt.Fprintln(tw, "ID\tNAME\tEMAILS\tBOOK")
	for _, c := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", c.ID, tbContactLabel(c), strings.Join(c.Emails, ", "), c.Book)
	}
	return tw.Flush()
}

func newTBContactsSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Find contacts whose name or email contains the query (case-insensitive)",
		Long: `Search every address book and the Collected Addresses book for contacts whose
display name, first/last name, nickname or any email contains the query,
ignoring case. Address-book contacts come before collected ones.`,
		Example: strings.Trim(`
  thunderbird-pp-cli contacts search alice
  thunderbird-pp-cli contacts search example.com --json --select id,display_name,emails`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "query=a"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "contacts search")
			}
			q := strings.TrimSpace(strings.Join(args, " "))
			if q == "" {
				return usageErr(fmt.Errorf("a search query is required\nUsage: %s <query>", cmd.CommandPath()))
			}
			docs, ok, err := tbLoadContacts(cmd, flags)
			if err != nil || !ok {
				return err
			}
			rows := make([]tbContactDoc, 0)
			for _, c := range docs {
				if tbContactMatches(c, q) {
					rows = append(rows, c)
				}
			}
			tbSortContacts(rows)
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			return tbPrintContacts(cmd, flags, rows)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum contacts to return (0 = all)")
	return cmd
}

func newTBContactsShowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <id|email>",
		Short: "Show one contact by card id or email address",
		Example: strings.Trim(`
  thunderbird-pp-cli contacts show alice@example.com
  thunderbird-pp-cli contacts show a1b2c3d4-0000-4000-8000-000000000001 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:typed-exit-codes": "0,2,3", "pp:happy-args": "id=nobody@example.com"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "contacts show")
			}
			if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
				return usageErr(fmt.Errorf("expected exactly one contact id or email\nUsage: %s <id|email>", cmd.CommandPath()))
			}
			docs, ok, err := tbLoadContacts(cmd, flags)
			if err != nil || !ok {
				return err
			}
			ref := strings.TrimSpace(args[0])
			matches := make([]tbContactDoc, 0)
			for _, c := range docs {
				hit := c.ID == ref
				for _, e := range c.Emails {
					hit = hit || strings.EqualFold(e, ref)
				}
				if hit {
					matches = append(matches, c)
				}
			}
			if len(matches) == 0 {
				return notFoundErr(fmt.Errorf("no contact with id or email %q", ref))
			}
			tbSortContacts(matches)
			c := matches[0]
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), c, flags)
			}
			w := tbHumanOut(cmd)
			fmt.Fprintf(w, "ID:       %s\nName:     %s\nEmails:   %s\n", c.ID, tbContactLabel(c), strings.Join(c.Emails, ", "))
			if c.NickName != "" {
				fmt.Fprintf(w, "Nickname: %s\n", c.NickName)
			}
			if c.Company != "" {
				fmt.Fprintf(w, "Company:  %s\n", c.Company)
			}
			fmt.Fprintf(w, "Book:     %s\n", c.Book)
			return nil
		},
	}
	return cmd
}
