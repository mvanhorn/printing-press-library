package tbprofile

import (
	"strings"
	"testing"
)

func TestStripQuoted(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"plain unchanged", "Ciao Anna,\n\nconfermo la riunione.\n\n\nA domani\n=====", "Ciao Anna,\n\nconfermo la riunione.\n\n\nA domani\n====="},
		{"gt inside text untouched", "Il valore deve essere > 10\ne a -> b", "Il valore deve essere > 10\ne a -> b"},
		{"italian thunderbird top-post keeps signature",
			"Va bene, procedo.\n\n-- \nMario Esempio\n\nIl 12/03/2025 10:15, Anna Prova ha scritto:\n> Puoi procedere?\n>\n> Anna",
			"Va bene, procedo.\n\n-- \nMario Esempio"},
		{"english wrapped attribution",
			"Sounds good.\n\nOn Mon, Mar 3, 2025 at 9:00 AM Bob Sample <\nbob@example.com> wrote:\n> Shall we meet?\n",
			"Sounds good."},
		{"french and german attribution",
			"Oui.\n\nLe 3 mars 2025, Alice a écrit :\n> Question ?\n\nJa.\n\nAm 3. März 2025 schrieb Carl:\n> Frage?",
			"Oui.\n\nJa."},
		{"bottom-posted reply keeps answers",
			"Il 12/03/2025, Anna Prova ha scritto:\n> Prima domanda?\n\nPrima risposta.\n\n> Seconda domanda?\n\nSeconda risposta.\n\n-- \nMario",
			"Prima risposta.\n\nSeconda risposta.\n\n-- \nMario"},
		{"attribution without gt cuts to end",
			"Perfetto.\n\nOn Tue, 4 Mar 2025, Dana Demo wrote:\nThe quoted html text lost its markers.",
			"Perfetto."},
		{"outlook italian header block",
			"Ricevuto, grazie.\n\nMario\n\n________________________________\nDa: Anna Prova <anna@example.com>\nInviato: lunedì 3 marzo 2025 09:00\nA: Mario Esempio <mario@example.com>\nOggetto: Offerta\n\nTesto originale.",
			"Ricevuto, grazie.\n\nMario"},
		{"outlook english bold header block",
			"Noted.\n\n*From:* Bob Sample <bob@example.com>\n*Sent:* Monday, March 3, 2025 9:00 AM\n*To:* Eve Test\n*Subject:* Plan\n\nOld text.",
			"Noted."},
		{"original message separator", "Ok.\n\n-----Original Message-----\nFrom: x@example.com\nold", "Ok."},
		{"messaggio originale separator", "Ok.\n-----Messaggio originale-----\nvecchio", "Ok."},
		{"da line without header block untouched", "Da: domani cambia tutto.\nSaluti", "Da: domani cambia tutto.\nSaluti"},
		{"forwarded message kept", "Ti giro questa.\n\n-------- Messaggio inoltrato --------\nOggetto: Offerta\nTesto inoltrato.", "Ti giro questa.\n\n-------- Messaggio inoltrato --------\nOggetto: Offerta\nTesto inoltrato."},
		{"crlf input", "Si.\r\n\r\nOn Mon, 3 Mar 2025, Bob wrote:\r\n> q", "Si."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripQuoted(tt.in); got != tt.want {
				t.Errorf("StripQuoted() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLikelyInline(t *testing.T) {
	tests := []struct {
		filename, contentType string
		want                  bool
	}{
		{"image001.png", "image/png", true},
		{"image12.JPG", "image/jpeg", true},
		{"logo-example.gif", "image/gif", true},
		{"Firma_Mario.jpg", "image/jpeg", true},
		{"LinkedIn.png", "image/png", true},
		{"holiday-photo.jpg", "image/jpeg", false},
		{"image001.png.pdf", "application/pdf", false},
		{"signature.pdf", "application/pdf", false},
		{"myimage001.png.zip", "image/png", false},
	}
	for _, tt := range tests {
		if got := LikelyInline(tt.filename, tt.contentType); got != tt.want {
			t.Errorf("LikelyInline(%q, %q) = %v, want %v", tt.filename, tt.contentType, got, tt.want)
		}
	}
}

func TestParseMessageInline(t *testing.T) {
	raw := strings.ReplaceAll(`From: a@example.com
Subject: Inline test
Content-Type: multipart/mixed; boundary="O"

--O
Content-Type: multipart/related; boundary="R"

--R
Content-Type: text/html

<p>Hi</p><img src="cid:Logo1@example.com">
--R
Content-Type: application/octet-stream; name="banner.bin"
Content-ID: <logo1@example.com>

AAAA
--R--
--O
Content-Type: image/png; name="sig.png"
Content-Disposition: inline; filename="sig.png"

QUJD
--O
Content-Type: image/png; name="icon.png"
Content-ID: <unref@example.com>

QUJD
--O
Content-Type: application/pdf; name="report.pdf"
Content-Disposition: inline; filename="report.pdf"

QUJD
--O
Content-Type: image/jpeg; name="photo.jpg"
Content-Disposition: attachment; filename="photo.jpg"
Content-ID: <photo@example.com>

QUJD
--O--
`, "\n", "\r\n")
	m := ParseMessage([]byte(raw))
	want := map[string]bool{"banner.bin": true, "sig.png": true, "icon.png": true, "report.pdf": false, "photo.jpg": false}
	if len(m.Attachments) != len(want) {
		t.Fatalf("attachments = %+v", m.Attachments)
	}
	for i, a := range m.Attachments {
		if a.Index != i || a.Inline != want[a.Filename] {
			t.Errorf("attachment %d %s inline=%v, want %v", a.Index, a.Filename, a.Inline, want[a.Filename])
		}
	}
}
