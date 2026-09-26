// pp:data-source local

package cli

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newTBDraftsCmd(flags))
	})
}

type tbIdentityDoc struct {
	ID          string `json:"id"`
	Account     string `json:"account"`
	AccountName string `json:"account_name"`
	Email       string `json:"email"`
	FullName    string `json:"full_name"`
}

type tbComposeSpec struct {
	Identity       *tbIdentityDoc `json:"identity"`
	To             []string       `json:"to"`
	Cc             []string       `json:"cc"`
	Bcc            []string       `json:"bcc"`
	Subject        string         `json:"subject"`
	Body           string         `json:"body"`
	Attachments    []string       `json:"attachments"`
	InReplyTo      string         `json:"in_reply_to,omitempty"`
	References     []string       `json:"references,omitempty"`
	ReplyToID      string         `json:"reply_to_id,omitempty"`
	ComposeArg     string         `json:"compose_arg"`
	Executable     string         `json:"executable"`
	Argv           []string       `json:"argv"`
	CommandLine    string         `json:"command_line"`
	Opened         bool           `json:"opened"`
	EMLPath        string         `json:"eml_path,omitempty"`
	PID            int            `json:"pid,omitempty"`
	Note           string         `json:"note"`
	attachmentURLs []string
}

// tbFindThunderbird and tbLaunch are variables so tests never start a real client.
var tbFindThunderbird = func() (string, error) {
	for _, name := range []string{"thunderbird", "thunderbird.exe"} {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	candidates := []string{}
	for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)"} {
		if dir := os.Getenv(env); dir != "" {
			candidates = append(candidates, filepath.Join(dir, "Mozilla Thunderbird", "thunderbird.exe"))
		}
	}
	candidates = append(candidates, `C:\Program Files\Mozilla Thunderbird\thunderbird.exe`, "/usr/bin/thunderbird",
		"/Applications/Thunderbird.app/Contents/MacOS/thunderbird")
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c, nil
		}
	}
	return "", errors.New("thunderbird executable not found in PATH or the default install locations")
}

var tbLaunch = func(exe string, args []string) (int, error) {
	c := exec.Command(exe, args...) // #nosec G204 -- exe is the resolved Thunderbird binary; args go to it directly, no shell
	if err := c.Start(); err != nil {
		return 0, err
	}
	pid := c.Process.Pid
	return pid, c.Process.Release()
}

// tbComposeValue quotes one -compose value. Thunderbird's GetArgs takes a
// single-quoted value literally and ends it at a quote followed by a comma,
// so values containing "'," are sent unquoted and percent-encoded instead
// (unquoted values are decodeURIComponent-ed).
func tbComposeValue(v string) string {
	if !strings.Contains(v, "',") {
		return "'" + v + "'"
	}
	var b strings.Builder
	for i := 0; i < len(v); i++ {
		c := v[i]
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || strings.IndexByte("-_.~", c) >= 0 {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

func tbComposeArg(s *tbComposeSpec) string {
	var parts []string
	add := func(k, v string) {
		if v != "" {
			parts = append(parts, k+"="+tbComposeValue(v))
		}
	}
	add("to", strings.Join(s.To, ", "))
	add("cc", strings.Join(s.Cc, ", "))
	add("bcc", strings.Join(s.Bcc, ", "))
	add("subject", s.Subject)
	add("body", s.Body)
	add("attachment", strings.Join(s.attachmentURLs, ","))
	if s.Identity != nil {
		add("preselectid", s.Identity.ID)
	}
	add("format", "text")
	return strings.Join(parts, ",")
}

// tbFileURL builds a file:/// URL; commas are encoded because Thunderbird
// splits the attachment list on them.
func tbFileURL(abs string) string {
	p := filepath.ToSlash(abs)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return strings.ReplaceAll((&url.URL{Scheme: "file", Path: p}).String(), ",", "%2C")
}

// tbWinQuote is the Windows command-line quoting of one argument.
func tbWinQuote(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\n\v\"") {
		return s
	}
	var b strings.Builder
	b.WriteByte('"')
	slashes := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			slashes++
		case '"':
			b.WriteString(strings.Repeat(`\`, slashes*2+1))
			slashes = 0
			b.WriteByte('"')
			continue
		default:
			b.WriteString(strings.Repeat(`\`, slashes))
			slashes = 0
		}
		if s[i] != '\\' {
			b.WriteByte(s[i])
		}
	}
	b.WriteString(strings.Repeat(`\`, slashes*2))
	b.WriteByte('"')
	return b.String()
}

func tbFinishSpec(s *tbComposeSpec) {
	s.ComposeArg = tbComposeArg(s)
	exe, err := tbFindThunderbird()
	if err != nil {
		exe = "thunderbird"
	}
	s.Executable = exe
	s.Argv = []string{exe, "-compose", s.ComposeArg}
	quoted := make([]string, len(s.Argv))
	for i, a := range s.Argv {
		quoted[i] = tbWinQuote(a)
	}
	s.CommandLine = strings.Join(quoted, " ")
	for _, p := range []*[]string{&s.To, &s.Cc, &s.Bcc, &s.Attachments} {
		if *p == nil {
			*p = []string{}
		}
	}
	s.Note = "Thunderbird opens a compose window; nothing is sent until you press Send."
}

// tbFormatAddress renders "Name <addr>" quoting names with specials.
func tbFormatAddress(name, addr string) string {
	name = strings.TrimSpace(name)
	if name == "" || strings.EqualFold(name, addr) {
		return addr
	}
	if strings.ContainsAny(name, `,;<>"@()[]:\.`) {
		name = `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(name) + `"`
	}
	return name + " <" + addr + ">"
}

func tbParseRecipients(values []string) ([]string, error) {
	out := make([]string, 0)
	for _, v := range values {
		if strings.TrimSpace(v) == "" {
			continue
		}
		list, err := mail.ParseAddressList(v)
		if err != nil {
			return nil, fmt.Errorf("invalid address %q: %v", v, err)
		}
		for _, a := range list {
			out = append(out, tbFormatAddress(a.Name, a.Address))
		}
	}
	return out, nil
}

// tbLoadIdentities reads identities from the store, falling back to prefs.js.
func tbLoadIdentities(cmd *cobra.Command, flags *rootFlags) ([]tbIdentityDoc, error) {
	db, err := tbOpenStoreQuiet(cmd)
	if err != nil {
		return nil, err
	}
	if db != nil {
		ids, err := tbLoadDocs[tbIdentityDoc](db, "identities")
		_ = db.Close()
		if err != nil {
			return nil, err
		}
		if len(ids) > 0 {
			return ids, nil
		}
	}
	profileDir, err := resolveTBProfile(flags)
	if errors.Is(err, tbprofile.ErrNoProfile) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	accs, err := tbprofile.LoadAccounts(profileDir)
	if err != nil {
		return nil, fmt.Errorf("reading prefs.js: %w", err)
	}
	return tbIdentityDocsFromPrefs(accs), nil
}

func tbFindIdentity(ids []tbIdentityDoc, ref string) *tbIdentityDoc {
	for i := range ids {
		if ids[i].ID == ref || strings.EqualFold(ids[i].Email, ref) {
			return &ids[i]
		}
	}
	return nil
}

var tbReplyPrefixRE = regexp.MustCompile(`(?i)^\s*re\s*:`)

func tbReplySubject(s string) string {
	if tbReplyPrefixRE.MatchString(s) {
		return strings.TrimSpace(s)
	}
	return "Re: " + strings.TrimSpace(s)
}

func tbQuoteBody(when time.Time, who, body string) string {
	var b strings.Builder
	if when.IsZero() {
		fmt.Fprintf(&b, "%s wrote:\n", who)
	} else {
		fmt.Fprintf(&b, "On %s, %s wrote:\n", when.Format("Mon, 2 Jan 2006 15:04"), who)
	}
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		if line == "" {
			b.WriteString(">\n")
		} else {
			b.WriteString("> " + line + "\n")
		}
	}
	return b.String()
}

// tbBuildReply fills recipients, subject, quoted body and identity for a reply.
func tbBuildReply(d *tbMessageDoc, raw []byte, ids []tbIdentityDoc, body string, all bool) *tbComposeSpec {
	m := tbprofile.ParseMessage(raw)
	own := tbOwnAddresses(ids)
	toAddrs := tbprofile.ParseAddresses(m.Header.Get("To"))
	ccAddrs := tbprofile.ParseAddresses(m.Header.Get("Cc"))
	s := &tbComposeSpec{ReplyToID: d.ID, InReplyTo: d.MessageID}
	seen := map[string]bool{}
	addTo := func(dst *[]string, a *mail.Address) {
		k := strings.ToLower(a.Address)
		if k == "" || seen[k] {
			return
		}
		seen[k] = true
		*dst = append(*dst, tbFormatAddress(a.Name, a.Address))
	}
	outgoing := own[strings.ToLower(m.FromAddr)]
	switch {
	case outgoing:
		for _, a := range toAddrs {
			addTo(&s.To, a)
		}
	default:
		primary := tbprofile.ParseAddresses(m.Header.Get("Reply-To"))
		if len(primary) == 0 {
			primary = []*mail.Address{{Name: m.FromName, Address: m.FromAddr}}
		}
		for _, a := range primary {
			addTo(&s.To, a)
		}
	}
	if all {
		for _, a := range append(append([]*mail.Address{}, toAddrs...), ccAddrs...) {
			if !own[strings.ToLower(a.Address)] {
				addTo(&s.Cc, a)
			}
		}
	}
	var pick *tbIdentityDoc
	if outgoing {
		pick = tbFindIdentity(ids, m.FromAddr)
	}
	for _, a := range append(append([]*mail.Address{}, toAddrs...), ccAddrs...) {
		if pick == nil {
			pick = tbFindIdentity(ids, a.Address)
		}
	}
	for i := range ids {
		if pick == nil && ids[i].Account == d.Account {
			pick = &ids[i]
		}
	}
	if pick == nil && len(ids) > 0 {
		pick = &ids[0]
	}
	s.Identity = pick
	s.Subject = tbReplySubject(m.Subject)
	quoted := tbQuoteBody(m.Date, tbSender(m.FromName, m.FromAddr), m.BodyText)
	if strings.TrimSpace(body) != "" {
		s.Body = strings.TrimRight(body, "\n") + "\n\n" + quoted
	} else {
		s.Body = "\n\n" + quoted
	}
	s.References = append(append([]string{}, m.References...), d.MessageID)
	if d.MessageID == "" {
		s.References = m.References
	}
	return s
}

// tbBuildDraftEML renders the draft as an RFC822 message (text plus attachments).
func tbBuildDraftEML(s *tbComposeSpec, now time.Time) ([]byte, error) {
	var buf bytes.Buffer
	hdr := func(k, v string) {
		if v != "" {
			fmt.Fprintf(&buf, "%s: %s\r\n", k, v)
		}
	}
	encodeList := func(list []string) string {
		out := make([]string, 0, len(list))
		for _, v := range list {
			if a, err := mail.ParseAddress(v); err == nil {
				out = append(out, a.String())
			} else {
				out = append(out, v)
			}
		}
		return strings.Join(out, ", ")
	}
	hdr("Date", now.Format(time.RFC1123Z))
	if s.Identity != nil {
		hdr("From", (&mail.Address{Name: s.Identity.FullName, Address: s.Identity.Email}).String())
	}
	hdr("To", encodeList(s.To))
	hdr("Cc", encodeList(s.Cc))
	hdr("Bcc", encodeList(s.Bcc))
	hdr("Subject", mime.QEncoding.Encode("utf-8", s.Subject))
	if s.InReplyTo != "" {
		hdr("In-Reply-To", "<"+s.InReplyTo+">")
	}
	if len(s.References) > 0 {
		hdr("References", "<"+strings.Join(s.References, "> <")+">")
	}
	hdr("X-Unsent", "1")
	hdr("MIME-Version", "1.0")
	text := func(w *bytes.Buffer) error {
		qp := quotedprintable.NewWriter(w)
		if _, err := qp.Write([]byte(strings.ReplaceAll(strings.ReplaceAll(s.Body, "\r\n", "\n"), "\n", "\r\n"))); err != nil {
			return err
		}
		return qp.Close()
	}
	if len(s.Attachments) == 0 {
		hdr("Content-Type", "text/plain; charset=utf-8")
		hdr("Content-Transfer-Encoding", "quoted-printable")
		buf.WriteString("\r\n")
		if err := text(&buf); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	hdr("Content-Type", "multipart/mixed; boundary=\""+mw.Boundary()+"\"")
	buf.WriteString("\r\n")
	tp, err := mw.CreatePart(textproto.MIMEHeader{"Content-Type": {"text/plain; charset=utf-8"}, "Content-Transfer-Encoding": {"quoted-printable"}})
	if err != nil {
		return nil, err
	}
	var tb bytes.Buffer
	if err := text(&tb); err != nil {
		return nil, err
	}
	if _, err := tp.Write(tb.Bytes()); err != nil {
		return nil, err
	}
	for _, p := range s.Attachments {
		data, err := os.ReadFile(filepath.Clean(p))
		if err != nil {
			return nil, err
		}
		name := filepath.Base(p)
		ctype := mime.TypeByExtension(filepath.Ext(name))
		if ctype == "" {
			ctype = "application/octet-stream"
		}
		ap, err := mw.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {mime.FormatMediaType(ctype, map[string]string{"name": name})},
			"Content-Disposition":       {mime.FormatMediaType("attachment", map[string]string{"filename": name})},
			"Content-Transfer-Encoding": {"base64"},
		})
		if err != nil {
			return nil, err
		}
		enc := base64.StdEncoding.EncodeToString(data)
		for len(enc) > 76 {
			fmt.Fprintf(ap, "%s\r\n", enc[:76])
			enc = enc[76:]
		}
		fmt.Fprintf(ap, "%s\r\n", enc)
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}
	buf.Write(body.Bytes())
	return buf.Bytes(), nil
}

// tbEmitCompose prints the spec, or with --open writes the .eml copy under
// the CLI data dir and launches Thunderbird.
func tbEmitCompose(cmd *cobra.Command, flags *rootFlags, s *tbComposeSpec, open bool, action string) error {
	tbFinishSpec(s)
	if open {
		if cliutil.IsAnyHarness() {
			return writeHarnessRefusal(cmd.OutOrStdout(), flags, action)
		}
		exe, err := tbFindThunderbird()
		if err != nil {
			return err
		}
		dataDir, err := cliutil.DataDir()
		if err != nil {
			return err
		}
		eml, err := tbBuildDraftEML(s, time.Now())
		if err != nil {
			return err
		}
		suffix := make([]byte, 3)
		_, _ = rand.Read(suffix)
		dir := filepath.Join(dataDir, "drafts")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
		s.EMLPath = filepath.Join(dir, time.Now().Format("20060102-150405")+"-"+hex.EncodeToString(suffix)+".eml")
		if err := os.WriteFile(s.EMLPath, eml, 0o600); err != nil {
			return err
		}
		pid, err := tbLaunch(exe, s.Argv[1:])
		if err != nil {
			return fmt.Errorf("launching %s: %w", exe, err)
		}
		s.Opened, s.PID = true, pid
	}
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printJSONFiltered(cmd.OutOrStdout(), s, flags)
	}
	w := tbHumanOut(cmd)
	from := "(Thunderbird default identity)"
	if s.Identity != nil {
		from = tbFormatAddress(s.Identity.FullName, s.Identity.Email) + " [" + s.Identity.ID + "]"
	}
	fmt.Fprintf(w, "From:    %s\nTo:      %s\n", from, strings.Join(s.To, ", "))
	if len(s.Cc) > 0 {
		fmt.Fprintf(w, "Cc:      %s\n", strings.Join(s.Cc, ", "))
	}
	if len(s.Bcc) > 0 {
		fmt.Fprintf(w, "Bcc:     %s\n", strings.Join(s.Bcc, ", "))
	}
	fmt.Fprintf(w, "Subject: %s\n", s.Subject)
	for _, a := range s.Attachments {
		fmt.Fprintf(w, "Attach:  %s\n", a)
	}
	fmt.Fprintf(w, "\n%s\n\n", s.Body)
	if s.Opened {
		fmt.Fprintf(w, "opened Thunderbird compose window (pid %d, %s); draft copy: %s\n", s.PID, s.Executable, s.EMLPath)
		return nil
	}
	fmt.Fprintf(w, "not opened (add --open to launch the compose window):\n  %s\n", s.CommandLine)
	return nil
}

func newTBDraftsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "drafts",
		Short:       "Prepare new messages and replies in a Thunderbird compose window (never sends)",
		Annotations: map[string]string{"pp:parent-group": "true", "pp:typed-exit-codes": "0,2", "pp:data-source": "local"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newTBDraftsNewCmd(flags), newTBDraftsReplyCmd(flags))
	return cmd
}

func newTBDraftsNewCmd(flags *rootFlags) *cobra.Command {
	var to, cc, bcc, attach []string
	var subject, body, bodyFile, fromIdentity string
	var open bool
	cmd := &cobra.Command{
		Use:   "new",
		Short: "Prepare a new message for Thunderbird's compose window",
		Long: `Prepare a new message. By default this only prints the compose fields and the
exact thunderbird -compose command line. --open writes a .eml copy under this
CLI's data directory and opens the compose window; nothing is ever sent.
--from-identity takes an identity email or key (idN) from accounts.
--body-file <path> reads the body from a file and --attach <path> (repeatable)
attaches a local file; both are CLI-only and not offered to MCP agents.`,
		Example: strings.Trim(`
  thunderbird-pp-cli drafts new --to alice@example.com --subject "Budget review" --body "Hi Alice,"
  thunderbird-pp-cli drafts new --to alice@example.com --cc carol@example.com --from-identity id1 --json
  thunderbird-pp-cli drafts new --to alice@example.com --subject "Agenda" --body-file ./agenda.txt --open`, "\n"),
		Annotations: map[string]string{"pp:data-source": "local", "pp:typed-exit-codes": "0,2,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "drafts new")
			}
			if len(args) > 0 {
				return usageErr(fmt.Errorf("unexpected argument %q; use --to/--subject/--body", args[0]))
			}
			if body != "" && bodyFile != "" {
				return usageErr(errors.New("use either --body or --body-file, not both"))
			}
			s := &tbComposeSpec{Subject: subject, Body: body}
			var err error
			if s.To, err = tbParseRecipients(to); err != nil {
				return usageErr(err)
			}
			if s.Cc, err = tbParseRecipients(cc); err != nil {
				return usageErr(err)
			}
			if s.Bcc, err = tbParseRecipients(bcc); err != nil {
				return usageErr(err)
			}
			if bodyFile != "" {
				b, err := os.ReadFile(filepath.Clean(bodyFile))
				if err != nil {
					return usageErr(fmt.Errorf("--body-file: %w", err))
				}
				s.Body = string(b)
			}
			for _, a := range attach {
				abs, err := filepath.Abs(a)
				if err != nil {
					return usageErr(err)
				}
				if info, err := os.Stat(abs); err != nil || info.IsDir() {
					return usageErr(fmt.Errorf("--attach %q: not a readable file", a))
				}
				s.Attachments = append(s.Attachments, abs)
				s.attachmentURLs = append(s.attachmentURLs, tbFileURL(abs))
			}
			if fromIdentity != "" {
				ids, err := tbLoadIdentities(cmd, flags)
				if err != nil {
					return err
				}
				if s.Identity = tbFindIdentity(ids, fromIdentity); s.Identity == nil {
					return notFoundErr(fmt.Errorf("no identity %q (see: %s accounts)", fromIdentity, tbCLIName))
				}
			}
			return tbEmitCompose(cmd, flags, s, open, "drafts new --open")
		},
	}
	cmd.Flags().StringArrayVar(&to, "to", nil, "Recipient (repeatable; \"Name <addr>\" or a comma-separated list)")
	cmd.Flags().StringArrayVar(&cc, "cc", nil, "Cc recipient (repeatable)")
	cmd.Flags().StringArrayVar(&bcc, "bcc", nil, "Bcc recipient (repeatable)")
	cmd.Flags().StringVar(&subject, "subject", "", "Subject")
	cmd.Flags().StringVar(&body, "body", "", "Plain-text body")
	cmd.Flags().StringVar(&bodyFile, "body-file", "", "Read the plain-text body from this file")
	cmd.Flags().StringArrayVar(&attach, "attach", nil, "File to attach (repeatable)")
	// Hidden flags are dropped from the MCP tool schema, so agents cannot read local files through them.
	_ = cmd.Flags().MarkHidden("body-file")
	_ = cmd.Flags().MarkHidden("attach")
	cmd.Flags().StringVar(&fromIdentity, "from-identity", "", "Sending identity: email or key like id1")
	cmd.Flags().BoolVar(&open, "open", false, "Open the Thunderbird compose window (otherwise only print the command)")
	return cmd
}

func newTBDraftsReplyCmd(flags *rootFlags) *cobra.Command {
	var body string
	var all, open bool
	cmd := &cobra.Command{
		Use:   "reply <message-id>",
		Short: "Prepare a reply (or reply-all) with the original quoted, from the identity that received it",
		Long: `Prepare a reply to a stored message: To is the sender (or Reply-To), --all
adds the other recipients as Cc without your own identities, the subject gets
one "Re: " prefix, the original body is quoted under "On <date>, <name>
wrote:", and the identity that received the message is preselected. By
default only prints the compose fields and command line; --open launches the
compose window. Nothing is ever sent.`,
		Example: strings.Trim(`
  thunderbird-pp-cli drafts reply 3f9a1c2b7d4e --body "Thanks, see you Monday."
  thunderbird-pp-cli drafts reply 3f9a1c2b7d4e --all --json
  thunderbird-pp-cli drafts reply 3f9a1c2b7d4e --body "Agreed." --open`, "\n"),
		Annotations: map[string]string{"pp:data-source": "local", "pp:typed-exit-codes": "0,2,3", "pp:happy-args": "message-id=0123456789ab"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "drafts reply")
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("expected exactly one message id\nUsage: %s <message-id>", cmd.CommandPath()))
			}
			db, err := tbStoreFor(cmd, flags, "messages")
			if err != nil || db == nil {
				return err
			}
			d, err := tbGetMessage(db, args[0])
			_ = db.Close()
			if err != nil {
				return err
			}
			raw, err := tbReadRaw(d)
			if err != nil {
				return err
			}
			ids, err := tbLoadIdentities(cmd, flags)
			if err != nil {
				return err
			}
			return tbEmitCompose(cmd, flags, tbBuildReply(d, raw, ids, body, all), open, "drafts reply --open")
		},
	}
	cmd.Flags().StringVar(&body, "body", "", "Reply text placed above the quoted original")
	cmd.Flags().BoolVar(&all, "all", false, "Reply to all: other recipients go to Cc (your identities excluded)")
	cmd.Flags().BoolVar(&open, "open", false, "Open the Thunderbird compose window (otherwise only print the command)")
	return cmd
}
