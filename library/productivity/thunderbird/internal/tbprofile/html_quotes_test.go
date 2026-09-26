package tbprofile

import "testing"

const (
	tbHTMLThunderbirdReply = `<html><head><style>p{margin:0}</style></head><body>
<p>Va bene, procedo.</p>
<div class="moz-signature">-- <br>Mario Esempio</div>
<div class="moz-cite-prefix">Il 09/01/2025 09:00, Anna Prova ha scritto:<br></div>
<blockquote type="cite" cite="mid:a1@example.com"><p>Mi confermi l'offerta?</p>
<blockquote type="cite">Prima versione</blockquote>
<p>Anna</p></blockquote>
</body></html>`
	tbHTMLGmailReply = `<div dir="ltr">Sounds good, see you then.</div><br><div class="gmail_quote"><div dir="ltr" class="gmail_attr">On Mon, Mar 3, 2025 at 9:00 AM Bob Sample &lt;<a href="mailto:bob@example.com">bob@example.com</a>&gt; wrote:<br></div><blockquote class="gmail_quote" style="margin:0px 0px 0px 0.8ex">Shall we meet?<br>Bob</blockquote></div>`
)

func TestHTMLToTextQuotes(t *testing.T) {
	tests := []struct {
		name, html, text, stripped string
	}{
		{"thunderbird reply", tbHTMLThunderbirdReply,
			"Va bene, procedo.\n\n--\nMario Esempio\n\nIl 09/01/2025 09:00, Anna Prova ha scritto:\n\n> Mi confermi l'offerta?\n\n> > Prima versione\n\n> Anna",
			"Va bene, procedo.\n\n--\nMario Esempio"},
		{"gmail reply", tbHTMLGmailReply,
			"Sounds good, see you then.\n\n> On Mon, Mar 3, 2025 at 9:00 AM Bob Sample <bob@example.com> wrote:\n\n> > Shall we meet?\n> > Bob",
			"Sounds good, see you then."},
		{"plain html unchanged", `<p>Ciao Anna,</p><div>il valore deve essere &gt; 10</div><blockquote-like>x</blockquote-like><p>A presto</p>`,
			"Ciao Anna,\nil valore deve essere > 10\nxA presto",
			"Ciao Anna,\nil valore deve essere > 10\nxA presto"},
		{"plain blockquote is new text", `<p>Come da contratto:</p><blockquote>Il pagamento avviene a 30 giorni.</blockquote><p>Mario</p>`,
			"Come da contratto:\nIl pagamento avviene a 30 giorni.\nMario",
			"Come da contratto:\nIl pagamento avviene a 30 giorni.\nMario"},
		{"unbalanced closing tags", `<p>Testo</p></div></blockquote><p>Fine</p>`, "Testo\n\nFine", "Testo\n\nFine"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HTMLToText(tt.html)
			if got != tt.text {
				t.Errorf("HTMLToText() = %q, want %q", got, tt.text)
			}
			if s := StripQuoted(got); s != tt.stripped {
				t.Errorf("StripQuoted(HTMLToText()) = %q, want %q", s, tt.stripped)
			}
		})
	}
}
