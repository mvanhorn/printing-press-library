// pp:data-source local

package cli

import (
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile"
)

type tbAccountRow struct {
	ID             string               `json:"id"`
	Name           string               `json:"name"`
	Type           string               `json:"type"`
	Hostname       string               `json:"hostname"`
	UserName       string               `json:"user_name"`
	Server         string               `json:"server"`
	Directory      string               `json:"directory"`
	IdentityCount  int                  `json:"identity_count"`
	IdentityEmails []string             `json:"identity_emails"`
	Identities     []tbprofile.Identity `json:"identities"`
	Source         string               `json:"source,omitempty"`
}

func tbAccountRowFromPrefs(a tbprofile.Account) tbAccountRow {
	emails := make([]string, 0, len(a.Identities))
	for _, id := range a.Identities {
		emails = append(emails, id.Email)
	}
	return tbAccountRow{
		ID: a.Key, Name: a.Name(), Type: a.Server.Type, Hostname: a.Server.Hostname, UserName: a.Server.UserName, Server: a.Server.Key,
		Directory: a.Server.Directory, IdentityCount: len(a.Identities), IdentityEmails: emails, Identities: a.Identities, Source: "prefs",
	}
}

// tbAccountMatches reports whether filter names the account by key or name.
func tbAccountMatches(filter, key, name string) bool {
	filter = strings.TrimSpace(filter)
	return filter == "" || strings.EqualFold(filter, key) || strings.EqualFold(filter, name)
}

func tbIdentityDocsFromPrefs(accounts []tbprofile.Account) []tbIdentityDoc {
	out := make([]tbIdentityDoc, 0)
	for _, a := range accounts {
		for _, id := range a.Identities {
			out = append(out, tbIdentityDoc{ID: id.Key, Account: a.Key, AccountName: a.Name(), Email: id.Email, FullName: id.FullName})
		}
	}
	return out
}

// tbOwnAddresses returns the lower-cased identity emails.
func tbOwnAddresses(ids []tbIdentityDoc) map[string]bool {
	own := map[string]bool{}
	for _, id := range ids {
		if id.Email != "" {
			own[strings.ToLower(id.Email)] = true
		}
	}
	return own
}
