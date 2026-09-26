package cli

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func tbBulkMsg(id, from, extra string, at time.Time) string {
	return fmt.Sprintf("\nFrom - Fri Jan 17 09:00:00 2025\nX-Mozilla-Status: 0000\nMessage-ID: <%s>\nDate: %s\nFrom: %s\nTo: bob@example.com\n%sSubject: Note %s\n\nHello.\n",
		id, at.Format(time.RFC1123Z), from, extra, id)
}

// tbSetupBulk adds fictional senders around the one-way threshold: digest
// (20 inbound, never written to), almost (19), peer (20 plus one reply from
// the sent folder), plus header-flagged, no-reply and quoted-name senders.
func tbSetupBulk(t *testing.T) tbSliceB {
	t.Helper()
	s := tbSetupB(t, false, false)
	imap := filepath.Join(s.profile, "ImapMail", "imap.example.com")
	base := time.Date(2025, 1, 17, 8, 0, 0, 0, time.UTC)
	var b strings.Builder
	for i := 0; i < 20; i++ {
		at := base.Add(time.Duration(i) * time.Minute)
		b.WriteString(tbBulkMsg(fmt.Sprintf("digest%d@example.com", i), "Stats Digest <digest@stats.example.com>", "", at))
		b.WriteString(tbBulkMsg(fmt.Sprintf("peer%d@example.com", i), "Pat Peer <peer@example.com>", "", at))
		b.WriteString(tbBulkMsg(fmt.Sprintf("mixa%d@example.com", i), "Mix A <mixa@stats.example.com>", map[bool]string{true: "Precedence: bulk\n"}[i == 0], at))
		b.WriteString(tbBulkMsg(fmt.Sprintf("mixb%d@example.com", i), "Mix B <mixb@stats.example.com>", map[bool]string{true: "Precedence: bulk\n"}[i == 19], at))
		if i < 19 {
			b.WriteString(tbBulkMsg(fmt.Sprintf("almost%d@example.com", i), "Al Most <almost@example.com>", "", at))
		}
	}
	at := base.Add(time.Hour)
	b.WriteString(tbBulkMsg("auto1@example.com", "Dana Example <dana@example.com>", "Auto-Submitted: auto-generated\n", at))
	b.WriteString(tbBulkMsg("human1@example.com", "Hal Example <hal@example.com>", "Auto-Submitted: no\n", at))
	b.WriteString(tbBulkMsg("dnr1@example.com", "Alerts <do-not-reply@alerts.example.com>", "", at))
	b.WriteString(tbBulkMsg("notif1@example.com", "Git <notifications@git.example.com>", "", at))
	b.WriteString(tbBulkMsg("quinn1@example.com", "'Quinn Example' <quinn@example.com>", "", at))
	tbAppendFile(t, filepath.Join(imap, "INBOX"), b.String())
	tbAppendFile(t, filepath.Join(imap, "Posta inviata"), "\nFrom - Fri Jan 17 12:00:00 2025\nX-Mozilla-Status: 0001\nMessage-ID: <peerreply@example.com>\n"+
		"In-Reply-To: <peer0@example.com>\nReferences: <peer0@example.com>\nDate: Fri, 17 Jan 2025 12:00:00 +0000\nFrom: Bob Example <bob@example.com>\n"+
		"To: Pat Peer <peer@example.com>\nSubject: Re: Note\n\nThanks.\n")
	orig := tbNow
	tbNow = func() time.Time { return tbSliceCNow }
	t.Cleanup(func() { tbNow = orig })
	if out, _, err := tbRun(t, s.home, "sync", "--json"); err != nil {
		t.Fatalf("sync: %v %s", err, out)
	}
	return s
}

func TestTBAutomatedHeaderStored(t *testing.T) {
	s := tbSetupBulk(t)
	db, err := tbOpenStoreForTest(t, s.home)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	docs, err := tbQueryMessages(db, "", nil, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, d := range docs {
		got[d.MessageID] = d.Automated
	}
	for id, want := range map[string]bool{"auto1@example.com": true, "m6@example.com": true, "human1@example.com": false, "digest0@example.com": false, "dnr1@example.com": false} {
		if got[id] != want {
			t.Errorf("%s automated = %v, want %v", id, got[id], want)
		}
	}
}

func TestTBBulkAwaitingReply(t *testing.T) {
	s := tbSetupBulk(t)
	count := func(args ...string) map[string]int {
		t.Helper()
		out, _, err := tbRun(t, s.home, append([]string{"awaiting-reply", "--json", "--days", "30", "--limit", "0"}, args...)...)
		if err != nil {
			t.Fatalf("%v: %v %s", args, err, out)
		}
		n := map[string]int{}
		for _, r := range tbDecode[[]tbAwaitingRow](t, out) {
			n[r.Counterpart]++
		}
		return n
	}
	def := count()
	for addr, want := range map[string]int{
		"digest@stats.example.com": 0, "dana@example.com": 0, "do-not-reply@alerts.example.com": 0, "notifications@git.example.com": 0,
		"almost@example.com": 19, "peer@example.com": 19, "hal@example.com": 1, "quinn@example.com": 1,
	} {
		if def[addr] != want {
			t.Errorf("default %s = %d, want %d", addr, def[addr], want)
		}
	}
	bulk := count("--include-bulk")
	for addr, want := range map[string]int{
		"digest@stats.example.com": 20, "dana@example.com": 1, "do-not-reply@alerts.example.com": 1, "notifications@git.example.com": 1,
	} {
		if bulk[addr] != want {
			t.Errorf("--include-bulk %s = %d, want %d", addr, bulk[addr], want)
		}
	}
}

func TestTBBulkNewsletterKinds(t *testing.T) {
	s := tbSetupBulk(t)
	out, _, err := tbRun(t, s.home, "newsletters", "--json", "--since", "30d", "--min", "1", "--limit", "0")
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
	kinds := map[string]string{}
	msgs := map[string]int{}
	for _, r := range tbDecode[[]tbNewsletterRow](t, out) {
		kinds[r.Key], msgs[r.Key] = r.Kind, r.Messages
	}
	for key, want := range map[string]string{
		"news.example.com": "list", "digest@stats.example.com": "one_way", "dana@example.com": "automated",
		"do-not-reply@alerts.example.com": "automated", "notifications@git.example.com": "automated",
		"mixa@stats.example.com": "automated", "mixb@stats.example.com": "automated",
		"almost@example.com": "", "peer@example.com": "", "hal@example.com": "",
	} {
		if kinds[key] != want {
			t.Errorf("%s kind = %q, want %q", key, kinds[key], want)
		}
	}
	if msgs["digest@stats.example.com"] != 20 {
		t.Errorf("one_way messages = %d", msgs["digest@stats.example.com"])
	}
}

func TestTBBulkContactsTop(t *testing.T) {
	s := tbSetupBulk(t)
	run := func(args ...string) map[string]tbCorrespondentRow {
		t.Helper()
		out, _, err := tbRun(t, s.home, append([]string{"contacts", "top", "--json", "--since", "30d", "--limit", "0"}, args...)...)
		if err != nil {
			t.Fatalf("%v: %v %s", args, err, out)
		}
		m := map[string]tbCorrespondentRow{}
		for _, r := range tbDecode[[]tbCorrespondentRow](t, out) {
			m[r.Address] = r
		}
		return m
	}
	def := run()
	for _, gone := range []string{"digest@stats.example.com", "dana@example.com", "do-not-reply@alerts.example.com", "notifications@git.example.com"} {
		if _, ok := def[gone]; ok {
			t.Errorf("default keeps bulk sender %s", gone)
		}
	}
	if r := def["almost@example.com"]; r.Received != 19 {
		t.Errorf("almost = %+v", r)
	}
	if r := def["peer@example.com"]; r.Received != 20 || r.SentTo != 1 {
		t.Errorf("peer = %+v", r)
	}
	if r := def["quinn@example.com"]; r.Name != "Quinn Example" {
		t.Errorf("quoted display name not cleaned: %q", r.Name)
	}
	bulk := run("--include-bulk")
	if bulk["digest@stats.example.com"].Received != 20 || bulk["dana@example.com"].Received != 1 {
		t.Errorf("--include-bulk = %+v / %+v", bulk["digest@stats.example.com"], bulk["dana@example.com"])
	}
}

func TestTBHelpTextNoHTTPBoilerplate(t *testing.T) {
	out, err := tbRunBare(t, "--help")
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"--rate-limit", "--no-cache", "--client-profile", "verify connectivity"} {
		if strings.Contains(out, bad) {
			t.Errorf("root help contains %q", bad)
		}
	}
	for args, want := range map[string]string{"newsletters --help": "'contacts top'", "search --help": "--type messages", "export --help": "export messages"} {
		out, err := tbRunBare(t, strings.Fields(args)...)
		if err != nil || !strings.Contains(out, want) || strings.Contains(out, "API data") || strings.Contains(out, "search endpoint") || strings.Contains(out, "group-by") || strings.Contains(out, "<resource> --format") {
			t.Errorf("%s: err=%v, want %q and no API boilerplate", args, err, want)
		}
	}
}
