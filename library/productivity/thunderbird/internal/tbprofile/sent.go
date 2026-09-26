package tbprofile

import (
	"net/url"
	"sort"
	"strings"
)

var sentFolderNames = map[string]bool{
	"sent": true, "sent items": true, "sent messages": true, "sent mail": true,
	"posta inviata": true, "inviata": true, "inviati": true, "elementi inviati": true,
	"gesendet": true, "gesendete elemente": true, "envoyés": true, "éléments envoyés": true,
	"enviados": true, "elementos enviados": true,
}

var trashJunkFolderNames = map[string]bool{
	"trash": true, "deleted items": true, "deleted messages": true, "bin": true, "cestino": true,
	"elementi eliminati": true, "papierkorb": true, "corbeille": true, "papelera": true,
	"junk": true, "junk e-mail": true, "junk email": true, "spam": true, "bulk mail": true,
	"posta indesiderata": true, "indesiderata": true,
}

func leafName(path string) string {
	path = strings.TrimRight(path, "/")
	if i := strings.LastIndex(path, "/"); i >= 0 {
		path = path[i+1:]
	}
	return strings.ToLower(strings.TrimSpace(path))
}

// IsSentFolderName reports whether a folder name or path has a well-known
// sent-mail leaf name (case-insensitive, several languages).
func IsSentFolderName(path string) bool { return sentFolderNames[leafName(path)] }

// IsTrashOrJunkFolderName reports whether a folder name or path is a
// well-known trash, junk or spam folder.
func IsTrashOrJunkFolderName(path string) bool { return trashJunkFolderNames[leafName(path)] }

// FolderURI is a decoded Thunderbird folder URI such as
// imap://user%40host@imap.example.com/Sent or mailbox://nobody@Local%20Folders/Trash.
type FolderURI struct {
	Scheme string
	User   string
	Host   string
	Path   string
}

// ParseFolderURI decodes a folder URI; ok is false for anything else.
func ParseFolderURI(uri string) (FolderURI, bool) {
	scheme, rest, found := strings.Cut(strings.TrimSpace(uri), "://")
	if !found || scheme == "" || rest == "" {
		return FolderURI{}, false
	}
	authority, path, _ := strings.Cut(rest, "/")
	user, host := "", authority
	if i := strings.LastIndex(authority, "@"); i >= 0 {
		user, host = authority[:i], authority[i+1:]
	}
	dec := func(s string) string {
		if v, err := url.PathUnescape(s); err == nil {
			return v
		}
		return s
	}
	f := FolderURI{Scheme: strings.ToLower(scheme), User: dec(user), Host: dec(host), Path: strings.Trim(dec(path), "/")}
	if f.Host == "" || f.Path == "" {
		return FolderURI{}, false
	}
	return f, true
}

// MatchFolderURI maps a folder URI to the account owning it and the folder
// path inside that account (the same path DiscoverFolders reports).
func MatchFolderURI(uri string, accounts []Account) (account, path string, ok bool) {
	f, ok := ParseFolderURI(uri)
	if !ok {
		return "", "", false
	}
	for _, a := range accounts {
		if !strings.EqualFold(a.Server.Hostname, f.Host) {
			continue
		}
		if f.User != "" && a.Server.UserName != "" && !strings.EqualFold(a.Server.UserName, f.User) {
			continue
		}
		return a.Key, f.Path, true
	}
	return "", "", false
}

// SentFolderURIs returns the distinct fcc_folder URIs of every identity
// that saves a copy of sent mail (mail.identity.idN.fcc not false).
func (p Prefs) SentFolderURIs(accounts []Account) []string {
	seen := map[string]bool{}
	out := make([]string, 0)
	for _, a := range accounts {
		for _, id := range a.Identities {
			base := "mail.identity." + id.Key + "."
			if p[base+"fcc"] == "false" {
				continue
			}
			if uri := strings.TrimSpace(p[base+"fcc_folder"]); uri != "" && !seen[uri] {
				seen[uri] = true
				out = append(out, uri)
			}
		}
	}
	sort.Strings(out)
	return out
}
