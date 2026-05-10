package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

func newServeCmd(flags *rootFlags) *cobra.Command {
	var port int
	var host string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start a local web UI — no terminal knowledge required",
		Long: `Starts a local web server with a chat-style interface.
Anyone on the local machine can open the URL in a browser and ask
workplace questions in plain German or English — no CLI knowledge needed.

This is the easiest way to share the tool with employees or BR members
who are not comfortable with the terminal.

By default the server binds to 127.0.0.1 (localhost only). Use
--host 0.0.0.0 to expose it on the local network (e.g. to share with
colleagues on the same WiFi).`,
		Example: strings.Trim(`
  betriebsrat serve
  betriebsrat serve --port 9000
  betriebsrat serve --host 0.0.0.0 --port 8080`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			addr := fmt.Sprintf("%s:%d", host, port)
			fmt.Fprintf(cmd.OutOrStdout(), "Betriebsrat advisor running at http://localhost:%d\n", port)
			fmt.Fprintf(cmd.OutOrStdout(), "Press Ctrl+C to stop.\n")

			mux := http.NewServeMux()
			mux.HandleFunc("/", serveUI)
			mux.HandleFunc("/ask", serveAsk)
			srv := &http.Server{Addr: addr, Handler: mux}
			go func() {
				<-cmd.Context().Done()
				srv.Shutdown(context.Background()) //nolint:errcheck
			}()
			if err := srv.ListenAndServe(); err != http.ErrServerClosed {
				return err
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&port, "port", 7890, "Port to listen on")
	cmd.Flags().StringVar(&host, "host", "127.0.0.1", "Host to bind (use 0.0.0.0 to expose on LAN)")
	return cmd
}

func serveUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, serveHTML)
}

// askLimiter tracks the last request time per IP for simple rate limiting.
var (
	askLimiter   sync.Map
	askMinGap    = 2 * time.Second
	askMaxBodySz = int64(4 * 1024) // 4 KB
)

func serveAsk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	ip := r.RemoteAddr
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ip = strings.SplitN(xff, ",", 2)[0]
	}
	now := time.Now()
	if last, ok := askLimiter.Load(ip); ok && now.Sub(last.(time.Time)) < askMinGap {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	askLimiter.Store(ip, now)

	r.Body = http.MaxBytesReader(w, r.Body, askMaxBodySz)
	var body struct {
		Question string `json:"question"`
		Lang     string `json:"lang"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	lang := body.Lang
	if lang == "" {
		lang = "de"
	}
	if lang == "de" && looksEnglish(body.Question) {
		lang = "en"
	}
	result := buildAskResult(lang, body.Question)
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(result)
}

const serveHTML = `<!DOCTYPE html>
<html lang="de">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Betriebsrat Advisor</title>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { font-family: system-ui, sans-serif; background: #0f1117; color: #e0e0e0; min-height: 100vh; }
  header { background: #1a1f2e; border-bottom: 1px solid #2d3450; padding: 16px 24px; display: flex; align-items: center; gap: 16px; }
  header h1 { font-size: 1.1rem; color: #fff; }
  header small { color: #8892a4; font-size: 0.8rem; }
  #lang-toggle { margin-left: auto; background: #2d3450; border: 1px solid #3d4560; color: #ccc; padding: 6px 14px; border-radius: 6px; cursor: pointer; font-size: 0.85rem; }
  #lang-toggle:hover { background: #3d4560; }
  #chat { max-width: 800px; margin: 0 auto; padding: 24px 16px 120px; display: flex; flex-direction: column; gap: 16px; }
  .bubble { padding: 14px 18px; border-radius: 12px; max-width: 90%; line-height: 1.6; }
  .bubble.user { background: #1e3a5f; align-self: flex-end; color: #cce4ff; }
  .bubble.assistant { background: #1a1f2e; border: 1px solid #2d3450; align-self: flex-start; }
  .bubble.assistant h3 { font-size: 0.75rem; text-transform: uppercase; color: #5a6a8a; margin-bottom: 8px; letter-spacing: 0.08em; }
  .bubble.assistant .classification { color: #7eb8f0; font-size: 0.9rem; margin-bottom: 10px; }
  .bubble.assistant .role-badge { display: inline-block; background: #2d3450; border-radius: 4px; padding: 2px 8px; font-size: 0.75rem; color: #8892a4; margin-bottom: 10px; }
  .bubble.assistant .answer { color: #d0d8e8; margin-bottom: 12px; }
  .bubble.assistant .actions { border-top: 1px solid #2d3450; padding-top: 10px; margin-top: 8px; }
  .bubble.assistant .actions h4 { font-size: 0.8rem; color: #5a6a8a; margin-bottom: 8px; }
  .bubble.assistant .actions ol { padding-left: 20px; }
  .bubble.assistant .actions li { font-size: 0.88rem; color: #a0aec0; margin-bottom: 5px; }
  .bubble.assistant .paras { display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 10px; }
  .bubble.assistant .para-tag { background: #1e3450; border: 1px solid #2d4a70; color: #7eb8f0; border-radius: 4px; padding: 2px 8px; font-size: 0.78rem; }
  .bubble.assistant .deadline { background: #2a1f10; border: 1px solid #5a3a00; color: #f0b060; border-radius: 6px; padding: 8px 12px; font-size: 0.85rem; margin-bottom: 10px; }
  .bubble.assistant .sozialplan { background: #102a1a; border: 1px solid #1a5a30; color: #60c090; border-radius: 6px; padding: 8px 12px; font-size: 0.85rem; margin-bottom: 10px; }
  .bubble.assistant .disclaimer { color: #5a6a8a; font-size: 0.78rem; margin-top: 10px; border-top: 1px solid #1d2235; padding-top: 8px; }
  .bubble.assistant .topic-url { font-size: 0.8rem; margin-top: 8px; }
  .bubble.assistant .topic-url a { color: #5a8fd0; }
  .bubble.error { background: #2a1010; border: 1px solid #5a2020; color: #f08080; align-self: flex-start; }
  #input-bar { position: fixed; bottom: 0; left: 0; right: 0; background: #1a1f2e; border-top: 1px solid #2d3450; padding: 16px; }
  #input-bar form { max-width: 800px; margin: 0 auto; display: flex; gap: 10px; }
  #q { flex: 1; background: #0f1117; border: 1px solid #3d4560; color: #e0e0e0; padding: 12px 16px; border-radius: 8px; font-size: 1rem; resize: none; height: 52px; }
  #q:focus { outline: none; border-color: #5a8fd0; }
  #submit { background: #1e4080; border: none; color: #fff; padding: 12px 20px; border-radius: 8px; cursor: pointer; font-size: 0.9rem; white-space: nowrap; }
  #submit:hover { background: #2a50a0; }
  #submit:disabled { background: #2d3450; color: #5a6a8a; cursor: not-allowed; }
  .loading { color: #5a6a8a; font-style: italic; }
  #suggestions { max-width: 800px; margin: 0 auto; display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 16px; }
  .suggestion { background: #1a1f2e; border: 1px solid #2d3450; color: #8892a4; padding: 8px 14px; border-radius: 20px; cursor: pointer; font-size: 0.84rem; }
  .suggestion:hover { border-color: #5a8fd0; color: #cce4ff; }
</style>
</head>
<body>
<header>
  <div>
    <h1>⚖️ Betriebsrat Advisor</h1>
    <small id="header-sub">Fragen zu Betriebsrat-Rechten und Arbeitnehmerrechten — auf Deutsch oder Englisch</small>
  </div>
  <button id="lang-toggle" onclick="toggleLang()">Switch to English</button>
</header>
<div id="chat">
  <div id="suggestions">
    <span class="suggestion" onclick="fillSuggestion(this.textContent)">Wurde der BR vor meiner Kündigung angehört?</span>
    <span class="suggestion" onclick="fillSuggestion(this.textContent)">Habe ich Anspruch auf Sozialplan? 8 Jahre, 4500 Euro</span>
    <span class="suggestion" onclick="fillSuggestion(this.textContent)">Arbeitgeber führt KI-System ein — haben wir Mitbestimmung?</span>
    <span class="suggestion" onclick="fillSuggestion(this.textContent)">Employer dismissed me without consulting the works council</span>
    <span class="suggestion" onclick="fillSuggestion(this.textContent)">Was passiert wenn wir die Frist für die Anhörung verpassen?</span>
  </div>
</div>
<div id="input-bar">
  <form onsubmit="sendQuestion(event)">
    <textarea id="q" placeholder="Stellen Sie Ihre Frage auf Deutsch oder English..." onkeydown="handleKey(event)"></textarea>
    <button id="submit" type="submit">Fragen</button>
  </form>
</div>
<script>
  var uiLang = 'de';

  function toggleLang() {
    uiLang = uiLang === 'de' ? 'en' : 'de';
    document.getElementById('lang-toggle').textContent = uiLang === 'de' ? 'Switch to English' : 'Zu Deutsch wechseln';
    document.getElementById('header-sub').textContent = uiLang === 'de'
      ? 'Fragen zu Betriebsrat-Rechten und Arbeitnehmerrechten — auf Deutsch oder Englisch'
      : 'Questions about works council rights and employee rights — in German or English';
    document.getElementById('q').placeholder = uiLang === 'de'
      ? 'Stellen Sie Ihre Frage auf Deutsch oder English...'
      : 'Ask your question in German or English...';
    document.getElementById('submit').textContent = uiLang === 'de' ? 'Fragen' : 'Ask';
    var sug = document.querySelector('.suggestion');
    if (sug && uiLang === 'en') {
      var englishSuggestions = [
        'Was the works council consulted before my dismissal?',
        'Am I entitled to a Sozialplan? 8 years service, 4500 EUR salary',
        'Employer introducing AI system — do we have co-determination rights?',
        'What happens if we miss the Anhörung deadline?',
        'Can employer restructure without works council consent?'
      ];
      var nodes = document.querySelectorAll('.suggestion');
      nodes.forEach(function(n, i) { if (englishSuggestions[i]) n.textContent = englishSuggestions[i]; });
    }
  }

  function fillSuggestion(text) {
    document.getElementById('q').value = text;
    document.getElementById('q').focus();
  }

  function handleKey(e) {
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); sendQuestion(e); }
  }

  function sendQuestion(e) {
    e.preventDefault();
    var q = document.getElementById('q').value.trim();
    if (!q) return;

    var chat = document.getElementById('chat');
    document.getElementById('suggestions').style.display = 'none';

    // User bubble
    var userDiv = document.createElement('div');
    userDiv.className = 'bubble user';
    userDiv.textContent = q;
    chat.appendChild(userDiv);

    // Loading indicator
    var loadDiv = document.createElement('div');
    loadDiv.className = 'bubble assistant loading';
    loadDiv.textContent = uiLang === 'de' ? 'Analysiere...' : 'Analysing...';
    chat.appendChild(loadDiv);
    chat.scrollTop = chat.scrollHeight;

    document.getElementById('q').value = '';
    document.getElementById('submit').disabled = true;

    fetch('/ask', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ question: q, lang: uiLang })
    })
    .then(function(r) { return r.json(); })
    .then(function(data) {
      chat.removeChild(loadDiv);
      chat.appendChild(renderResult(data));
      chat.scrollTop = chat.scrollHeight;
    })
    .catch(function(err) {
      loadDiv.className = 'bubble error';
      loadDiv.textContent = 'Error: ' + err.message;
    })
    .finally(function() {
      document.getElementById('submit').disabled = false;
    });
  }

  function renderResult(d) {
    var div = document.createElement('div');
    div.className = 'bubble assistant';

    var html = '';

    var roleLabel = d.user_role === 'employee'
      ? (d.lang === 'en' ? 'Employee' : 'Arbeitnehmer')
      : (d.lang === 'en' ? 'BR member' : 'Betriebsratsmitglied');
    html += '<span class="role-badge">' + esc(roleLabel) + '</span><br>';

    html += '<div class="classification">' + esc(d.classification) + '</div>';

    if (d.right_type) {
      html += '<span class="para-tag">' + esc(d.right_type) + '</span><br><br>';
    }

    if (d.paragraphs && d.paragraphs.length > 0) {
      html += '<div class="paras">';
      d.paragraphs.slice(0, 5).forEach(function(p) {
        html += '<span class="para-tag">§ ' + p.paragraph + ' ' + esc(p.title) + '</span>';
      });
      html += '</div>';
    }

    html += '<div class="answer">' + esc(d.answer).replace(/\n/g, '<br>') + '</div>';

    if (d.deadline) {
      html += '<div class="deadline">⏰ ' + esc(d.deadline) + '</div>';
    }

    if (d.sozialplan_hint) {
      html += '<div class="sozialplan">💶 ' + esc(d.sozialplan_hint) + '</div>';
    }

    if (d.recommended_actions && d.recommended_actions.length > 0) {
      var actLabel = d.lang === 'en' ? 'Recommended steps' : 'Empfohlene Schritte';
      html += '<div class="actions"><h4>' + actLabel + '</h4><ol>';
      d.recommended_actions.forEach(function(a) {
        html += '<li>' + esc(a) + '</li>';
      });
      html += '</ol></div>';
    }

    if (d.topic_url) {
      var moreLabel = d.lang === 'en' ? 'More info' : 'Mehr Infos';
      html += '<div class="topic-url">' + moreLabel + ': <a href="' + esc(d.topic_url) + '" target="_blank">' + esc(d.topic_url) + '</a></div>';
    }

    html += '<div class="disclaimer">⚠️ ' + esc(d.disclaimer) + '</div>';

    div.innerHTML = html;
    return div;
  }

  function esc(s) {
    if (!s) return '';
    return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
  }
</script>
</body>
</html>`
