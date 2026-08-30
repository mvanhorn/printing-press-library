// Copyright 2026 fuushyn and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type accountAlias struct {
	Alias     string   `json:"alias"`
	AccountID string   `json:"account_id"`
	Name      string   `json:"name,omitempty"`
	Type      string   `json:"type,omitempty"`
	Also      []string `json:"also_matches,omitempty"`
}

// providerAliases mirrors the provider vocabulary seeded into the learn loop so
// "gmail", "li", and "wa" resolve the same way on the command line.
var providerAliases = map[string][]string{
	"LINKEDIN":     {"linkedin", "li"},
	"WHATSAPP":     {"whatsapp", "wa"},
	"TELEGRAM":     {"telegram", "tg"},
	"INSTAGRAM":    {"instagram", "ig", "insta"},
	"MESSENGER":    {"messenger", "fb"},
	"TWITTER":      {"twitter", "x"},
	"GOOGLE":       {"google", "gmail"},
	"GOOGLE_OAUTH": {"google", "gmail"},
	"OUTLOOK":      {"outlook", "microsoft", "o365"},
	"ICLOUD":       {"icloud"},
	"IMAP":         {"imap"},
	"MAIL":         {"mail", "email"},
	"MOBILE":       {"mobile", "sms"},
}

func newAccountsAliasCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath   string
		doExport bool
	)
	cmd := &cobra.Command{
		Use:   "alias [name]",
		Short: "Resolve a provider or account name to its Unipile account id",
		Long: strings.Trim(`
Almost every Unipile route requires account_id, an opaque 22-character blob.
This resolves a human name to that id from the local mirror, so scripts can say
"linkedin" instead of pasting an id.

With no argument it lists every alias the mirror knows. With an argument it
prints just the matching account id, which is pipe-friendly:

  unipile-pp-cli chats list --account-id "$(unipile-pp-cli accounts alias linkedin)"

Use --export to emit a shell line that sets UNIPILE_ACCOUNT_ID for the session.
Note that the local mirror is scoped per account id, so keep the same value set
for both sync and query.`, "\n"),
		Example: strings.Trim(`
  unipile-pp-cli accounts alias
  unipile-pp-cli accounts alias linkedin
  eval "$(unipile-pp-cli accounts alias linkedin --export)"
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "name=linkedin",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "accounts alias")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			aliases := make([]accountAlias, 0)
			db, ok, err := novelStore(cmd, flags, dbPath, aliases)
			if err != nil || !ok {
				return err
			}
			defer db.Close()

			rows, err := db.QueryContext(ctx, `SELECT COALESCE(id,''), COALESCE(name,''), COALESCE(type,'') FROM accounts`)
			if err != nil {
				return fmt.Errorf("reading accounts: %w", err)
			}
			type acct struct{ id, name, typ string }
			accounts := make([]acct, 0)
			for rows.Next() {
				var id, name, typ sql.NullString
				if serr := rows.Scan(&id, &name, &typ); serr != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning account: %w", serr)
				}
				accounts = append(accounts, acct{id.String, name.String, typ.String})
			}
			if rerr := rows.Err(); rerr != nil {
				_ = rows.Close()
				return fmt.Errorf("iterating accounts: %w", rerr)
			}
			if cerr := rows.Close(); cerr != nil {
				return fmt.Errorf("closing account rows: %w", cerr)
			}

			for _, a := range accounts {
				primary := strings.ToLower(a.typ)
				also := make([]string, 0)
				for _, alt := range providerAliases[strings.ToUpper(a.typ)] {
					if alt != primary {
						also = append(also, alt)
					}
				}
				if a.name != "" {
					also = append(also, strings.ToLower(a.name))
				}
				aliases = append(aliases, accountAlias{Alias: primary, AccountID: a.id, Name: a.name, Type: a.typ, Also: also})
			}
			sort.Slice(aliases, func(i, j int) bool { return aliases[i].Alias < aliases[j].Alias })

			if len(args) == 0 {
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), aliases, flags)
				}
				if len(aliases) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No accounts in the local mirror. Run 'unipile-pp-cli sync --resources accounts' first.")
					return nil
				}
				tw := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintln(tw, "ALIAS\tACCOUNT ID\tNAME\tALSO MATCHES")
				for _, a := range aliases {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", a.Alias, a.AccountID, a.Name, strings.Join(a.Also, ", "))
				}
				return tw.Flush()
			}

			needle := strings.ToLower(strings.TrimSpace(args[0]))
			for _, a := range aliases {
				if a.Alias == needle || strings.EqualFold(a.AccountID, args[0]) {
					return emitAlias(cmd, flags, a, doExport)
				}
				for _, alt := range a.Also {
					if alt == needle || strings.Contains(alt, needle) {
						return emitAlias(cmd, flags, a, doExport)
					}
				}
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "no account matches %q; run 'unipile-pp-cli accounts alias' to see what is available\n", args[0])
			return notFoundErr(fmt.Errorf("no account alias %q", args[0]))
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory)")
	cmd.Flags().BoolVar(&doExport, "export", false, "print a shell export line for UNIPILE_ACCOUNT_ID instead of the bare id")
	return cmd
}

func emitAlias(cmd *cobra.Command, flags *rootFlags, a accountAlias, doExport bool) error {
	// Single-alias resolution is meant for command substitution, so a bare id is
	// the pipe-friendly answer. Only emit JSON when the caller asked for a
	// machine format explicitly - keying off wantsHumanTable would turn every
	// $(...) capture into a JSON blob.
	if flags != nil && (flags.asJSON || flags.agent || flags.csv) {
		return printJSONFiltered(cmd.OutOrStdout(), a, flags)
	}
	if doExport {
		fmt.Fprintf(cmd.OutOrStdout(), "export UNIPILE_ACCOUNT_ID=%s\n", a.AccountID)
		return nil
	}
	fmt.Fprintln(cmd.OutOrStdout(), a.AccountID)
	return nil
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		accountsCmd, _, err := root.Find([]string{"accounts"})
		if err == nil && accountsCmd != nil {
			addNovelCommandIfAbsent(accountsCmd, newAccountsAliasCmd(flags))
		}
	})
}
