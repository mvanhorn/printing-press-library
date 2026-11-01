// Google Voice extensions use the generated registries and survive regeneration.
// pp:data-source live
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/google-voice/internal/client"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/google-voice/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/google-voice/internal/voice"
	"github.com/spf13/cobra"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func init() {
	localDataWriterCommands["mirror sync"] = true
	registerClientHook(func(c *client.Client) error {
		c.NoCache = true
		c.HTTPClient.Transport = &voice.Gateway{Transport: c.HTTPClient.Transport}
		c.HTTPClient.Jar = nil
		return nil
	})
	registerNovelCommand(func(root *cobra.Command, f *rootFlags) {
		replaceVoiceCommand(root, newVoiceAuth(f))
		replaceVoiceCommand(root, newVoiceThreads(f))
		replaceVoiceCommand(root, newVoiceDoctor(f))
		if local, _, _ := root.Find([]string{"local-data"}); local != nil {
			local.Short = "Preview or erase framework cache and learning data"
			local.Long = "Framework store, cache and journal cleanup. Voice profile message mirrors and credentials are retained."
			if purge, _, _ := local.Find([]string{"purge"}); purge != nil {
				purge.Long += "\nVoice profile message mirrors are retained; this command purges only framework-owned store/cache/journal data."
			}
		}
		account := &cobra.Command{Use: "account", Short: "Verify the imported Voice account", Example: "  google-voice-pp-cli account get --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "computed", "pp:parent-group": "true"}, RunE: parentNoSubcommandRunE(f)}
		get := voiceCommand(&cobra.Command{Use: "get", Short: "Read the account bound to the imported private session"}, "live", f, func(cmd *cobra.Command, _ []string) error {
			r, e := voiceRemote(f)
			if e != nil {
				return e
			}
			ctx, cancel := boundCtx(cmd.Context(), f)
			defer cancel()
			a, e := r.Account(ctx)
			if e != nil {
				return voiceError(e)
			}
			return f.printJSON(cmd, a)
		})
		get.Example = "  google-voice-pp-cli account get --agent"
		account.AddCommand(get)
		replaceVoiceCommand(root, account)
		mirror, _, _ := root.Find([]string{"mirror"})
		mirror.AddCommand(newVoiceSync(f), newVoiceExport(f))
		root.AddCommand(newVoiceSearch(f), newVoiceMCP(f))
		applyVoiceFixtures(root)
	})
}
func replaceVoiceCommand(root, newCmd *cobra.Command) {
	for _, c := range root.Commands() {
		if c.Name() == newCmd.Name() {
			root.RemoveCommand(c)
		}
	}
	root.AddCommand(newCmd)
}
func voiceError(err error) error {
	var h *voice.HTTPError
	if errors.As(err, &h) {
		if h.Status == 401 || h.Status == 403 {
			return authErr(err)
		}
		if h.Status == 404 {
			return notFoundErr(err)
		}
	}
	var limit *cliutil.RateLimitError
	if errors.As(err, &limit) {
		return rateLimitErr(err)
	}
	return apiErr(err)
}
func voiceRemote(f *rootFlags) (*voice.Remote, error) {
	s, e := voice.LoadSession()
	if e != nil {
		return nil, authErr(e)
	}
	return voiceRemoteSession(f, &s)
}

// Use generated TLS/config resolution without sending logical object bodies or credentials.
func voiceRemoteSession(f *rootFlags, s *voice.Session) (*voice.Remote, error) {
	c, e := f.newClient()
	if e != nil {
		return nil, e
	}
	if strings.TrimRight(c.BaseURL, "/") != voice.Base {
		return nil, configErr(fmt.Errorf("Google Voice uses a fixed verified API origin; remove the base_url override"))
	}
	r := voice.NewRemote(s, f.rateLimit)
	r.HTTP.Timeout = f.timeout
	if gateway, ok := c.HTTPClient.Transport.(*voice.Gateway); ok {
		r.HTTP.Transport = gateway.Transport
	} else {
		return nil, configErr(fmt.Errorf("Voice transport adapter is not configured"))
	}
	return r, nil
}
func voiceCommand(cmd *cobra.Command, source string, f *rootFlags, run func(*cobra.Command, []string) error) *cobra.Command {
	cmd.Annotations = map[string]string{"mcp:read-only": "true", "pp:data-source": source}
	cmd.RunE = func(c *cobra.Command, a []string) error {
		if dryRunOK(f) {
			return writeDryRun(c.OutOrStdout(), f, c.CommandPath())
		}
		if source == "local" && f.dataSource == "live" {
			return usageErr(fmt.Errorf("%s has no live equivalent; use --data-source local", c.CommandPath()))
		}
		if source == "live" && f.dataSource == "local" {
			return usageErr(fmt.Errorf("%s has no local equivalent; use --data-source live", c.CommandPath()))
		}
		if len(a) > 0 {
			return usageErr(fmt.Errorf("unexpected positional arguments; run %s --help", c.CommandPath()))
		}
		if source != "computed" {
			ctx, cancel := boundCtx(c.Context(), f)
			defer cancel()
			c.SetContext(ctx)
		}
		return run(c, a)
	}
	return cmd
}
func newVoiceAuth(f *rootFlags) *cobra.Command {
	group := &cobra.Command{Use: "auth", Short: "Import a private Voice session or inspect its health", RunE: parentNoSubcommandRunE(f)}
	var file string
	imp := voiceCommand(&cobra.Command{Use: "import-har", Short: "Verify and privately import one successful signed-in Voice read capture"}, "live", f, func(c *cobra.Command, _ []string) error {
		if file == "" {
			return usageErr(fmt.Errorf("--file is required; use an authenticated local HAR outside Dropbox with mode 0600"))
		}
		b, e := voice.ReadPrivate(file)
		if e != nil {
			return configErr(e)
		}
		s, e := voice.ImportHAR(b)
		if e != nil {
			return usageErr(e)
		}
		r, e := voiceRemoteSession(f, &s)
		if e != nil {
			return e
		}
		ctx, cancel := boundCtx(c.Context(), f)
		defer cancel()
		a, e := r.Account(ctx)
		if e != nil {
			return voiceError(e)
		}
		if old, e := voice.LoadSession(); e == nil && old.AccountID != a.ID {
			return configErr(fmt.Errorf("existing Voice profile belongs to another account; choose a separate GOOGLE_VOICE_PROFILE"))
		}
		_, dbPath, e := voice.Paths()
		if e != nil {
			return configErr(e)
		}
		if _, e = os.Stat(dbPath); e == nil {
			db, e := voice.OpenMirrorContext(ctx, dbPath, a.ID, false)
			if e != nil {
				return configErr(e)
			}
			if e = db.Close(); e != nil {
				return e
			}
		}
		s.AccountID = a.ID
		if e = voice.SaveSession(s); e != nil {
			return configErr(e)
		}
		profile, _ := voice.Profile()
		return f.printJSON(c, map[string]any{"profile": profile, "account_id": a.ID, "verified": true, "imported_at": s.ImportedAt})
	})
	imp.Annotations["mcp:read-only"] = "false"
	imp.Example = "  google-voice-pp-cli auth import-har --file /private/tmp/voice.har --agent"
	imp.Flags().StringVar(&file, "file", "", "Private local HAR file containing authenticated Voice read headers")
	var live bool
	status := voiceCommand(&cobra.Command{Use: "status", Short: "Inspect local session presence and optionally verify it against Voice"}, "auto", f, func(c *cobra.Command, _ []string) error {
		s, e := voice.LoadSession()
		if e != nil {
			return authErr(e)
		}
		profile, _ := voice.Profile()
		out := map[string]any{"profile": profile, "account_id": s.AccountID, "configured": true, "imported_at": s.ImportedAt, "live_verified": false}
		f.agentSource = "local"
		if f.dataSource == "live" {
			live = true
		}
		if live && f.dataSource == "local" {
			return usageErr(fmt.Errorf("auth status --live conflicts with --data-source local"))
		}
		if live {
			ctx, cancel := boundCtx(c.Context(), f)
			defer cancel()
			r, re := voiceRemoteSession(f, &s)
			if re != nil {
				return re
			}
			_, e = r.Account(ctx)
			if e != nil {
				return voiceError(e)
			}
			out["live_verified"] = true
			f.agentSource = "live"
		}
		return f.printJSON(c, out)
	})
	status.Example = "  google-voice-pp-cli auth status --live --agent"
	status.Flags().BoolVar(&live, "live", false, "Verify the imported session with a read-only account request")
	group.AddCommand(imp, status)
	return group
}
func newVoiceThreads(f *rootFlags) *cobra.Command {
	group := &cobra.Command{Use: "threads", Short: "Read bounded conversation and message pages without changing read state", Example: "  google-voice-pp-cli threads list --folder texts --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "computed", "pp:parent-group": "true"}, RunE: parentNoSubcommandRunE(f)}
	var folder, cursor string
	var unread bool
	var limit int
	list := voiceCommand(&cobra.Command{Use: "list", Short: "Read a conversation inventory page and its continuation cursor"}, "live", f, func(c *cobra.Command, _ []string) error {
		if limit < 1 || limit > 20 {
			return usageErr(fmt.Errorf("--limit must be 1 through 20"))
		}
		if folder != "texts" && folder != "archived" {
			return usageErr(fmt.Errorf("--folder must be texts or archived"))
		}
		r, e := voiceRemote(f)
		if e != nil {
			return e
		}
		ctx, cancel := boundCtx(c.Context(), f)
		defer cancel()
		p, e := r.List(ctx, folder, cursor)
		if e != nil {
			return voiceError(e)
		}
		ts := []voice.Thread{}
		for _, t := range p.Threads {
			if !unread || (t.Read != nil && !*t.Read) {
				ts = append(ts, t)
			}
		}
		out := map[string]any{"threads": ts, "next_cursor": p.NextCursor, "version": p.Version, "folder": folder, "clipped": false}
		if len(ts) > limit {
			out["threads"] = ts[:limit]
			out["clipped"] = true
			out["resume_note"] = "The source cursor advances past the entire page. Re-read this page with --limit 20 to recover locally clipped rows."
		}
		return f.printJSON(c, out)
	})
	list.Example = "  google-voice-pp-cli threads list --folder texts --limit 20 --agent"
	list.Flags().StringVar(&folder, "folder", "texts", "Conversation folder to inspect: texts or archived")
	list.Flags().StringVar(&cursor, "cursor", "", "Opaque inventory continuation cursor returned by Voice")
	list.Flags().BoolVar(&unread, "unread", false, "Include only threads currently reported unread by Voice")
	list.Flags().IntVar(&limit, "limit", 20, "Maximum conversation rows emitted from this source page")
	var id, older string
	var count int
	var unreadOnly bool
	get := voiceCommand(&cobra.Command{Use: "get", Short: "Read a bounded message page and its older-history continuation cursor"}, "live", f, func(c *cobra.Command, _ []string) error {
		if id == "" || count < 1 || count > 100 {
			return usageErr(fmt.Errorf("--thread-id is required; --count must be 1 through 100"))
		}
		r, e := voiceRemote(f)
		if e != nil {
			return e
		}
		ctx, cancel := boundCtx(c.Context(), f)
		defer cancel()
		p, e := r.Get(ctx, id, older, count)
		if e != nil {
			return voiceError(e)
		}
		if unreadOnly && (p.Threads[0].Read == nil || *p.Threads[0].Read) {
			p.UnknownReadState = p.Threads[0].Read == nil
			p.Threads = []voice.Thread{}
		}
		return f.printJSON(c, p)
	})
	get.Example = "  google-voice-pp-cli threads get --thread-id sms-example --count 50 --agent"
	get.Annotations["pp:happy-args"] = "--thread-id=sms-example"
	if fixture := os.Getenv("GOOGLE_VOICE_FIXTURE_THREAD_ID"); fixture != "" {
		get.Annotations["pp:happy-args"] = "--thread-id=" + fixture
	}
	get.Flags().StringVar(&id, "thread-id", "", "Stable conversation ID returned by threads list")
	get.Flags().StringVar(&older, "cursor", "", "Opaque older-history cursor returned for this exact thread")
	get.Flags().IntVar(&count, "count", 100, "Maximum messages requested in this page, from 1 through 100")
	get.Flags().BoolVar(&unreadOnly, "unread", false, "Return messages only when the thread is currently unread")
	group.AddCommand(list, get)
	return group
}
func newVoiceDoctor(f *rootFlags) *cobra.Command {
	if registeredPlatformSource != nil {
		return newDoctorCmd(f)
	}
	var failOn string
	cmd := voiceCommand(&cobra.Command{Use: "doctor", Short: "Report local private-session and mirror paths; remote checks use auth status --live"}, "local", f, func(c *cobra.Command, _ []string) error {
		if failOn != "none" && failOn != "warn" && failOn != "error" {
			return usageErr(fmt.Errorf("--fail-on must be none, warn or error"))
		}
		s, sessionErr := voice.LoadSession()
		profile, e := voice.Profile()
		if e != nil {
			return configErr(e)
		}
		sessionPath, dbPath, e := voice.Paths()
		if e != nil {
			return configErr(e)
		}
		warnings := []string{}
		critical := false
		if sessionErr != nil {
			warnings = append(warnings, "Private Voice session is absent or unusable; import a successful private read HAR.")
			critical = !errors.Is(sessionErr, os.ErrNotExist)
		}
		for _, kind := range []cliutil.PathKind{cliutil.PathKindConfig, cliutil.PathKindData, cliutil.PathKindState, cliutil.PathKindCache} {
			resolution, err := cliutil.ResolveKindDir(kind)
			if err != nil {
				return configErr(err)
			}
			if len(resolution.IgnoredOverrides) > 0 {
				warnings = append(warnings, "A relative or invalid path override was ignored for "+resolution.KindName)
			}
		}
		out := map[string]any{"profile": profile, "session_configured": sessionErr == nil, "session_file": sessionPath, "database": dbPath, "runtime_browser_required": false, "live_auth": "not checked; run auth status --live", "warnings": warnings}
		if sessionErr == nil {
			out["account_id"] = s.AccountID
			out["session_imported_at"] = s.ImportedAt
		}
		if e := f.printJSON(c, out); e != nil {
			return e
		}
		if (failOn == "warn" && len(warnings) > 0) || (failOn == "error" && critical) {
			return configErr(fmt.Errorf("doctor found local setup warnings; inspect the structured warnings and import/repair the private session"))
		}
		return nil
	})
	cmd.Example = "  google-voice-pp-cli doctor --json\n  google-voice-pp-cli doctor --fail-on warn --json"
	cmd.Flags().StringVar(&failOn, "fail-on", "none", "Return nonzero for local setup warnings or errors: none, warn, error")
	return cmd
}
func newVoiceSync(f *rootFlags) *cobra.Command {
	var folders, dbPath string
	var pages int
	c := voiceCommand(&cobra.Command{Use: "sync", Short: "Collect bounded Voice pages with atomic checkpoints and source-ID deduplication"}, "live", f, func(c *cobra.Command, _ []string) error {
		if pages < 1 || pages > 1000 {
			return usageErr(fmt.Errorf("--max-pages must be 1 through 1000"))
		}
		fs := strings.Split(folders, ",")
		for _, v := range fs {
			if v != "texts" && v != "archived" {
				return usageErr(fmt.Errorf("--folders accepts texts,archived"))
			}
		}
		r, e := voiceRemote(f)
		if e != nil {
			return e
		}
		ctx, cancel := boundCtx(c.Context(), f)
		defer cancel()
		a, e := r.Account(ctx)
		if e != nil {
			return voiceError(e)
		}
		if dbPath == "" {
			_, dbPath, e = voice.Paths()
			if e != nil {
				return configErr(e)
			}
		}
		db, e := voice.OpenMirrorContext(ctx, dbPath, a.ID, true)
		if e != nil {
			return configErr(e)
		}
		defer db.Close()
		budget := pages
		if cliutil.IsDogfoodEnv() && budget > 1 {
			budget = 1
		}
		result, e := db.Sync(ctx, r, fs, budget)
		if e != nil {
			return voiceError(e)
		}
		return f.printJSON(c, result)
	})
	c.Annotations["mcp:read-only"] = "false"
	c.Example = "  google-voice-pp-cli mirror sync --folders texts --max-pages 5 --agent"
	c.Flags().StringVar(&folders, "folders", "texts", "Comma-separated verified folders to discover: texts,archived")
	c.Flags().IntVar(&pages, "max-pages", 5, "Maximum inventory or message pages committed during this run")
	c.Flags().StringVar(&dbPath, "db", "", "Private local SQLite mirror path; defaults to the active Voice profile")
	return c
}
func openVoiceMirror(c *cobra.Command, f *rootFlags, path string) (*voice.Mirror, error) {
	if path == "" {
		_, p, e := voice.Paths()
		if e != nil {
			return nil, configErr(e)
		}
		path = p
	}
	if _, e := os.Stat(path); errors.Is(e, os.ErrNotExist) {
		fmt.Fprintln(c.ErrOrStderr(), "No local Voice mirror yet. Run: google-voice-pp-cli mirror sync --max-pages 5")
		return nil, nil
	}
	account := ""
	if s, e := voice.LoadSession(); e == nil {
		account = s.AccountID
	}
	ctx, cancel := boundCtx(c.Context(), f)
	defer cancel()
	m, e := voice.OpenMirrorContext(ctx, path, account, false)
	if e != nil {
		return nil, configErr(e)
	}
	if !hintIfUnsynced(c, m, "messages") {
		hintIfStale(c, m, "messages", f.maxAge)
	}
	return m, nil
}
func hintIfUnsynced(c *cobra.Command, m *voice.Mirror, _ string) bool {
	var count int
	if e := m.DB.QueryRowContext(c.Context(), "SELECT count(*) FROM voice_runs WHERE status IN ('bounded','observed_exhausted')").Scan(&count); e == nil && count == 0 {
		fmt.Fprintln(c.ErrOrStderr(), "No completed local collection run; inspect mirror coverage and run mirror sync.")
		return true
	}
	return false
}
func hintIfStale(c *cobra.Command, m *voice.Mirror, _ string, maxAge time.Duration) {
	if maxAge <= 0 {
		return
	}
	var timestamp string
	if e := m.DB.QueryRowContext(c.Context(), "SELECT COALESCE(max(finished_at),'') FROM voice_runs WHERE status IN ('bounded','observed_exhausted')").Scan(&timestamp); e == nil {
		if t, e := time.Parse(time.RFC3339Nano, timestamp); e == nil && time.Since(t) > maxAge {
			fmt.Fprintln(c.ErrOrStderr(), "Local Voice observations may be stale; inspect mirror coverage or run mirror sync.")
		}
	}
}

func newVoiceSearch(f *rootFlags) *cobra.Command {
	var limit int
	var dbPath, typ string
	c := voiceCommand(&cobra.Command{Use: "search [query]", Short: "Search literal text in the private local Voice message index"}, "local", f, func(c *cobra.Command, _ []string) error { return nil })
	// One optional positional is validated after the dry-run guard, without Cobra hard gates.
	c.RunE = func(c *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(c.OutOrStdout(), f, "search")
		}
		ctx, cancel := boundCtx(c.Context(), f)
		defer cancel()
		c.SetContext(ctx)
		if f.dataSource == "live" {
			return usageErr(fmt.Errorf("search has no live equivalent"))
		}
		if len(args) != 1 || args[0] == "" {
			return usageErr(fmt.Errorf("search requires one literal text argument"))
		}
		if typ != "messages" {
			return usageErr(fmt.Errorf("--type must be messages"))
		}
		if limit < 1 || limit > 10000 {
			return usageErr(fmt.Errorf("--limit must be 1 through 10000"))
		}
		m, e := openVoiceMirror(c, f, dbPath)
		if e != nil {
			return e
		}
		out := []voice.Record{}
		if m != nil {
			defer m.Close()
			ctx, cancel := boundCtx(c.Context(), f)
			defer cancel()
			out, e = m.Messages(ctx, voice.Query{Text: args[0], Limit: limit, FTS: true})
			if e != nil {
				return usageErr(e)
			}
		}
		return f.printJSON(c, map[string]any{"messages": out})
	}
	c.Example = "  google-voice-pp-cli search appointment --limit 20 --agent"
	c.Annotations["pp:happy-args"] = "<query>=appointment"
	c.Flags().IntVar(&limit, "limit", 20, "Maximum matching locally mirrored messages to return")
	c.Flags().StringVar(&typ, "type", "messages", "Local resource to search; this reader indexes messages")
	c.Flags().StringVar(&dbPath, "db", "", "Private existing Voice SQLite mirror path")
	return c
}
func newVoiceExport(f *rootFlags) *cobra.Command {
	var limit int
	var dbPath, format string
	c := voiceCommand(&cobra.Command{Use: "export", Short: "Export bounded local messages as JSON or newline-delimited JSON"}, "local", f, func(c *cobra.Command, _ []string) error {
		if limit < 1 || limit > 10000 {
			return usageErr(fmt.Errorf("--limit must be 1 through 10000"))
		}
		if format != "json" && format != "jsonl" {
			return usageErr(fmt.Errorf("--format must be json or jsonl"))
		}
		if format == "jsonl" && (f.asJSON || f.selectFields != "" || f.csv || f.plain || f.quiet || f.compact || c.Flags().Changed("agent") || c.InheritedFlags().Changed("agent")) {
			return usageErr(fmt.Errorf("--format jsonl cannot be combined with other output flags; use --format json for agent, JSON, CSV, plain, compact or field selection"))
		}
		m, e := openVoiceMirror(c, f, dbPath)
		if e != nil {
			return e
		}
		rows := []voice.Record{}
		var coverage any
		if m != nil {
			defer m.Close()
			ctx, cancel := boundCtx(c.Context(), f)
			defer cancel()
			rows, e = m.Messages(ctx, voice.Query{Limit: limit})
			if e != nil {
				return e
			}
			coverage, e = m.Coverage(ctx, "", 1000)
			if e != nil {
				return e
			}
		}
		if format == "jsonl" {
			enc := json.NewEncoder(c.OutOrStdout())
			for _, v := range rows {
				if e = enc.Encode(v); e != nil {
					return e
				}
			}
			return nil
		}
		return f.printJSON(c, map[string]any{"messages": rows, "coverage": coverage, "limit": limit, "potentially_clipped": len(rows) == limit})
	})
	c.Example = "  google-voice-pp-cli mirror export --limit 100 --agent"
	c.Flags().IntVar(&limit, "limit", 100, "Maximum locally mirrored messages emitted to stdout")
	c.Flags().StringVar(&format, "format", "json", "Output framing: json with coverage, or jsonl records")
	c.Flags().StringVar(&dbPath, "db", "", "Private existing Voice SQLite mirror path")
	return c
}
func newVoiceMCP(f *rootFlags) *cobra.Command {
	var serve bool
	c := voiceCommand(&cobra.Command{Use: "mcp", Short: "Print MCP connection configuration or serve through the companion binary"}, "computed", f, func(c *cobra.Command, _ []string) error {
		exe, e := os.Executable()
		if e != nil {
			return e
		}
		server := filepath.Join(filepath.Dir(exe), "google-voice-pp-mcp")
		if !serve {
			return f.printJSON(c, map[string]any{"transport": "stdio", "command": server, "args": []string{}, "note": "Launch the companion binary through your agent's MCP configuration, or use mcp --serve."})
		}
		if cliutil.IsAnyHarness() {
			return writeHarnessRefusal(c.OutOrStdout(), f, "start long-running MCP server")
		}
		child := exec.CommandContext(context.Background(), server) // #nosec G204 -- Execute only the fixed companion basename beside this executable, without a shell or user-controlled arguments.
		child.Stdin = c.InOrStdin()
		child.Stdout = c.OutOrStdout()
		child.Stderr = c.ErrOrStderr()
		return child.Run()
	})
	c.Annotations["mcp:hidden"] = "true"
	c.Example = "  google-voice-pp-cli mcp --agent"
	c.Flags().BoolVar(&serve, "serve", false, "Start the companion stdio MCP server for an agent session")
	return c
}

// Fixtures are supplied only by the operator at test time; no account IDs,
// credential bytes or private filesystem paths are embedded in generated code.
func applyVoiceFixtures(root *cobra.Command) {
	home := os.Getenv("GOOGLE_VOICE_FIXTURE_HOME")
	if home == "" || !filepath.IsAbs(home) || strings.ContainsAny(home, "; \t\r\n") {
		return
	}
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		if c.Annotations["pp:data-source"] != "" && c.RunE != nil {
			args := c.Annotations["pp:happy-args"]
			if c.CommandPath() == root.Name()+" auth import-har" {
				if har := os.Getenv("GOOGLE_VOICE_FIXTURE_HAR"); har != "" && !strings.ContainsAny(har, "; \t\r\n") {
					args = "--file=" + har
				}
			}
			c.Annotations["pp:happy-args"] = strings.Trim(args+";--home="+home+";--no-learn=true", "; ")
		}
		for _, child := range c.Commands() {
			walk(child)
		}
	}
	walk(root)
}
