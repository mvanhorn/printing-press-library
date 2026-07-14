package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/mt5/internal/bridge"
	"github.com/mvanhorn/printing-press-library/library/other/mt5/internal/config"
	"github.com/mvanhorn/printing-press-library/library/other/mt5/internal/safety"
	"github.com/mvanhorn/printing-press-library/library/other/mt5/internal/store"
)

// defaultInit returns an InitializeOptions seeded with MT5_PATH (if set) and
// the given MT5-side timeout in ms. Used by every command that just needs to
// attach to an already-running terminal.
func defaultInit(timeoutMs int) bridge.InitializeOptions {
	o := bridge.InitializeOptions{Timeout: timeoutMs}
	if p := terminalPathFromEnv(); p != "" {
		o.Path = p
	}
	return o
}

// profileInit returns InitializeOptions for the requested --profile, or
// defaultInit if no profile is set. The profile's password is read from the
// env var named in its password_env field — never stored on disk.
//
// Errors are returned as ExitConfig so the caller can surface them with the
// right exit code.
func profileInit(cmd *cobra.Command, timeoutMs int) (bridge.InitializeOptions, error) {
	name, _ := cmd.Flags().GetString("profile")
	if name == "" {
		return defaultInit(timeoutMs), nil
	}
	cfg, err := config.Load("")
	if err != nil {
		return bridge.InitializeOptions{}, &ExitErr{Code: ExitConfig, Err: fmt.Errorf("--profile %q: %w", name, err)}
	}
	p, ok := cfg.Profiles[name]
	if !ok {
		available := make([]string, 0, len(cfg.Profiles))
		for k := range cfg.Profiles {
			available = append(available, k)
		}
		sort.Strings(available)
		hint := "(no profiles defined — run `mt5-pp-cli config-init` to scaffold a config.toml)"
		if len(available) > 0 {
			hint = fmt.Sprintf("(available: %s)", strings.Join(available, ", "))
		}
		return bridge.InitializeOptions{}, &ExitErr{
			Code: ExitConfig,
			Err:  fmt.Errorf("--profile %q not found %s", name, hint),
		}
	}
	if p.PasswordEnv == "" {
		return bridge.InitializeOptions{}, &ExitErr{
			Code: ExitConfig,
			Err:  fmt.Errorf("--profile %q has no password_env — add one to %s", name, config.DefaultPath()),
		}
	}
	password := os.Getenv(p.PasswordEnv)
	if password == "" {
		return bridge.InitializeOptions{}, &ExitErr{
			Code: ExitConfig,
			Err:  fmt.Errorf("--profile %q: env var %q is empty (set it before invoking)", name, p.PasswordEnv),
		}
	}
	opts := defaultInit(timeoutMs)
	opts.Login = p.Account
	opts.Server = p.Server
	opts.Password = password
	return opts, nil
}

// resolveAccountLogin returns the account_login that mirror reads should
// scope to. Resolution order:
//
//  1. --account <N> flag (explicit override; user knows what they want)
//  2. Most recently synced row in the accounts table (last_synced DESC)
//
// If neither yields a value, returns a clean ExitConfig error pointing at
// 'mt5-pp-cli sync all' as the remediation. Read commands SHOULD scope to the
// resolved value because the mirror is multi-account-aware (the schema keys
// on account_login) — unscoped reads silently mix data across logins.
func resolveAccountLogin(ctx context.Context, db *sql.DB, cmd *cobra.Command) (int64, error) {
	if v, _ := cmd.Flags().GetInt64("account"); v != 0 {
		return v, nil
	}
	var login int64
	err := db.QueryRowContext(ctx,
		`SELECT login FROM accounts ORDER BY last_synced DESC LIMIT 1`).Scan(&login)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, &ExitErr{Code: ExitConfig, Err: fmt.Errorf(
				"no accounts in the local mirror — run `mt5-pp-cli sync all` first, or pass --account <login>")}
		}
		return 0, err
	}
	return login, nil
}

// initBridge resolves --profile, initializes the bridge, and maps bridge
// sentinels to their documented exit codes. Used by every command that takes
// the read-only "attach to the current terminal" path except doctor (which
// runs its own structured remediation) and connect login/logout (which manage
// their own auth flow).
func initBridge(cmd *cobra.Command, b *bridge.Bridge, timeoutMs int) error {
	opts, err := profileInit(cmd, timeoutMs)
	if err != nil {
		return err
	}
	if err := b.Initialize(opts); err != nil {
		return mapBridgeErr(err)
	}
	return nil
}

// terminalPathFromEnv resolves MT5_PATH to an absolute terminal64.exe path,
// accepting either a directory or a full .exe path. Returns "" if unset/missing.
func terminalPathFromEnv() string {
	env := os.Getenv("MT5_PATH")
	if env == "" {
		return ""
	}
	if fi, err := os.Stat(env); err == nil {
		if fi.IsDir() {
			cand := filepath.Join(env, "terminal64.exe")
			if _, err := os.Stat(cand); err == nil {
				return cand
			}
			return ""
		}
		return env
	}
	return ""
}

// ── doctor ───────────────────────────────────────────────────────────────────

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Verify Python, MetaTrader5 package, terminal, login, and network path",
		Long: `Run a full preflight check of the mt5-pp-cli install.

Each check prints PASS, FAIL, or SKIP with the exact remediation hint.
If anything red blocks live commands, doctor reports it before you discover
it inside a write command.`,
		RunE: runDoctor,
	}
}

type checkResult struct {
	Name      string `json:"name"`
	Status    string `json:"status"` // pass | fail | skip
	Detail    string `json:"detail,omitempty"`
	Remediate string `json:"remediate,omitempty"`
}

func runDoctor(cmd *cobra.Command, args []string) error {
	out := cmd.OutOrStdout()
	results := []checkResult{}

	// 1. Python on PATH
	py, err := bridge.FindPython()
	if err != nil {
		results = append(results, checkResult{
			Name: "python", Status: "fail", Detail: err.Error(),
			Remediate: "Install Python 3.10+ from https://python.org or via 'winget install Python.Python.3.13'",
		})
	} else {
		results = append(results, checkResult{Name: "python", Status: "pass", Detail: py})
	}

	// 2. MetaTrader5 package importable + (3) bridge spawn
	var b *bridge.Bridge
	if err == nil {
		// Drain bridge stderr into our own stderr so unhandled tracebacks surface.
		b, err = bridge.New(bridge.Options{Stderr: cmd.ErrOrStderr(), CallTimeout: 20 * time.Second})
		if err != nil {
			results = append(results, checkResult{
				Name: "bridge spawn", Status: "fail", Detail: err.Error(),
				Remediate: "Check that 'py -3' works and that mt5_bridge.py parses (run: py -3 -c \"import MetaTrader5\")",
			})
		}
	}
	if b != nil {
		defer b.Close()
	}

	// 3. OS check (informational)
	osStatus := "pass"
	osDetail := runtime.GOOS
	osRemediate := ""
	if runtime.GOOS != "windows" {
		osStatus = "skip"
		osDetail = runtime.GOOS + " (live commands disabled; replay/sql/stats/backtest still work against the local mirror)"
	}
	results = append(results, checkResult{Name: "host OS == windows", Status: osStatus, Detail: osDetail, Remediate: osRemediate})

	// 4. MetaTrader5 package + 5. terminal initialize.
	// Pass MT5_PATH if set so initialize() can spawn terminal64.exe if it isn't
	// already running. mt5 timeout is 5s so it fails fast; bridge wraps at 20s.
	if b != nil {
		err := b.Initialize(defaultInit(5000))
		switch {
		case err == nil:
			results = append(results, checkResult{Name: "MetaTrader5 package", Status: "pass"})
			results = append(results, checkResult{Name: "mt5.initialize()", Status: "pass"})
		case errors.Is(err, bridge.ErrMT5PkgMissing):
			results = append(results, checkResult{
				Name: "MetaTrader5 package", Status: "fail", Detail: err.Error(),
				Remediate: "py -3 -m pip install MetaTrader5  (Windows only — Mac/Linux can't use live commands)",
			})
			results = append(results, checkResult{Name: "mt5.initialize()", Status: "skip", Detail: "blocked by previous failure"})
		case errors.Is(err, bridge.ErrTerminalDown):
			results = append(results, checkResult{Name: "MetaTrader5 package", Status: "pass"})
			results = append(results, checkResult{
				Name: "mt5.initialize()", Status: "fail", Detail: err.Error(),
				Remediate: "Start the MetaTrader 5 terminal (terminal64.exe) and log into any account, then re-run doctor",
			})
		default:
			// A bridge timeout almost always means MT5 wasn't running and
			// auto-spawn either didn't try or didn't return. Translate.
			results = append(results, checkResult{Name: "MetaTrader5 package", Status: "pass"})
			results = append(results, checkResult{
				Name: "mt5.initialize()", Status: "fail", Detail: err.Error(),
				Remediate: "Start MetaTrader 5 (open the terminal and log in once), or set MT5_PATH to terminal64.exe so initialize() can auto-spawn it",
			})
		}
	}

	// 6. account_info() — proves login + broker reachable
	if b != nil {
		acc, err := b.AccountInfo()
		switch {
		case err == nil:
			results = append(results, checkResult{
				Name: "account_info()", Status: "pass",
				Detail: fmt.Sprintf("login=%d server=%s mode=%s currency=%s balance=%.2f",
					acc.Login, acc.Server, acc.TradeModeName(), acc.Currency, acc.Balance),
			})
		case errors.Is(err, bridge.ErrNotLoggedIn):
			results = append(results, checkResult{
				Name: "account_info()", Status: "fail", Detail: "no account logged in",
				Remediate: "Log into a broker in the MT5 terminal, or run: mt5-pp-cli connect login --account ... --server ... --password-env MT5_PASSWORD",
			})
		default:
			results = append(results, checkResult{Name: "account_info()", Status: "skip", Detail: err.Error()})
		}
	}

	// 7. store.db writable
	dbPath := store.DefaultPath()
	if db, err := store.Open(dbPath); err != nil {
		results = append(results, checkResult{
			Name: "store.db writable", Status: "fail", Detail: err.Error(),
			Remediate: "Ensure parent directory is writable: " + dbPath,
		})
	} else {
		_ = db.Ping()
		_ = db.Close()
		results = append(results, checkResult{Name: "store.db writable", Status: "pass", Detail: dbPath})
	}

	// 8. safety mode + config sanity. The mode string is informational; the
	// kill switch and config-load are real fails because they block every
	// write — surfacing them in doctor catches the case where the user
	// forgot a touched kill-switch file or has a malformed config.
	results = append(results, checkResult{
		Name: "safety mode", Status: "pass", Detail: safety.ModeDescription(),
	})
	cfg, cfgErr := config.Load("")
	switch {
	case cfgErr != nil:
		results = append(results, checkResult{
			Name: "config.toml", Status: "fail", Detail: cfgErr.Error(),
			Remediate: "Fix or remove the file at " + config.DefaultPath() + " — or run `mt5-pp-cli config-init` to regenerate it",
		})
	case cfg == nil:
		// No config file. Acceptable — guardrails default to off.
		results = append(results, checkResult{
			Name: "config.toml", Status: "skip", Detail: "not present (guardrails default to off)",
		})
	default:
		results = append(results, checkResult{
			Name: "config.toml", Status: "pass",
			Detail: fmt.Sprintf("max_volume=%g max_positions=%d max_daily_loss=%g kill_switch=%q",
				cfg.Guardrails.MaxVolumePerOrder, cfg.Guardrails.MaxOpenPositions,
				cfg.Guardrails.MaxDailyLoss, cfg.Guardrails.KillSwitchFile),
		})
	}
	if cfg != nil && cfg.Guardrails.KillSwitchFile != "" {
		if _, err := os.Stat(cfg.Guardrails.KillSwitchFile); err == nil {
			results = append(results, checkResult{
				Name: "kill switch", Status: "fail",
				Detail:    "file is present — every write will be refused",
				Remediate: "Delete " + cfg.Guardrails.KillSwitchFile + " to re-enable writes",
			})
		} else {
			results = append(results, checkResult{
				Name: "kill switch", Status: "pass",
				Detail: "not present (writes allowed past gate)",
			})
		}
	}

	// emit
	if jsonFlag(cmd) {
		enc := json.NewEncoder(out)
		if !compactFlag(cmd) {
			enc.SetIndent("", "  ")
		}
		_ = enc.Encode(results)
	} else {
		printChecks(out, results)
	}

	// Exit non-zero if anything blocks live commands; SKIPs are informational.
	for _, r := range results {
		if r.Status == "fail" {
			return &ExitErr{Code: ExitTerminalDown, Err: fmt.Errorf("doctor: %s — see remediation above", r.Name)}
		}
	}
	return nil
}

func printChecks(w io.Writer, results []checkResult) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "CHECK\tSTATUS\tDETAIL")
	fmt.Fprintln(tw, "─────\t──────\t──────")
	for _, r := range results {
		mark := "?"
		switch r.Status {
		case "pass":
			mark = "✓ pass"
		case "fail":
			mark = "✗ FAIL"
		case "skip":
			mark = "~ skip"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Name, mark, r.Detail)
	}
	tw.Flush()

	hasFail := false
	for _, r := range results {
		if r.Status == "fail" && r.Remediate != "" {
			if !hasFail {
				fmt.Fprintln(w, "\nRemediation:")
				hasFail = true
			}
			fmt.Fprintf(w, "  %s → %s\n", r.Name, r.Remediate)
		}
	}
}

// ── connect ──────────────────────────────────────────────────────────────────

func newConnectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connect",
		Short: "Manage MT5 broker connections",
		Long:  "Manage broker login state. Credentials are read from env vars referenced by --password-env; never stored on disk by this CLI.",
	}
	cmd.AddCommand(newConnectLoginCmd())
	cmd.AddCommand(newConnectLogoutCmd())
	cmd.AddCommand(newConnectStatusCmd())
	return cmd
}

func newConnectLoginCmd() *cobra.Command {
	var (
		account     int64
		server      string
		passwordEnv string
	)
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in to a broker account via the Python bridge",
		Long: `Log in to a broker account. The password is read from the named env
var (never echoed, never stored). Example:

  $env:MT5_PASSWORD = "..."
  mt5-pp-cli connect login --account 12345678 --server "Broker-Live" --password-env MT5_PASSWORD

Note: the MT5 terminal itself remembers the active login between processes,
so subsequent mt5-pp-cli calls reconnect to that session without re-logging in.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if account == 0 {
				return &ExitErr{Code: ExitUsage, Err: fmt.Errorf("--account is required")}
			}
			if server == "" {
				return &ExitErr{Code: ExitUsage, Err: fmt.Errorf("--server is required")}
			}
			if passwordEnv == "" {
				return &ExitErr{Code: ExitUsage, Err: fmt.Errorf("--password-env is required (env var name, not the password)")}
			}
			password := os.Getenv(passwordEnv)
			if password == "" {
				return &ExitErr{Code: ExitConfig, Err: fmt.Errorf("env var %q is empty", passwordEnv)}
			}

			b, err := bridge.New(bridge.Options{Stderr: cmd.ErrOrStderr(), CallTimeout: 30 * time.Second})
			if err != nil {
				return err
			}
			defer b.Close()

			if err := b.Initialize(bridge.InitializeOptions{Login: account, Password: password, Server: server, Timeout: 20000}); err != nil {
				return mapBridgeErr(err)
			}
			acc, err := b.AccountInfo()
			if err != nil {
				return mapBridgeErr(err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "logged in: %d @ %s (%s, %s, balance=%.2f)\n",
				acc.Login, acc.Server, acc.TradeModeName(), acc.Currency, acc.Balance)
			return nil
		},
	}
	cmd.Flags().Int64Var(&account, "account", 0, "Broker account number (required)")
	cmd.Flags().StringVar(&server, "server", "", "Broker server name (required)")
	cmd.Flags().StringVar(&passwordEnv, "password-env", "MT5_PASSWORD", "Env var that holds the password")
	return cmd
}

func newConnectLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Disconnect this bridge from the terminal (terminal keeps its session)",
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := bridge.New(bridge.Options{Stderr: cmd.ErrOrStderr()})
			if err != nil {
				return err
			}
			defer b.Close()
			// logout intentionally uses defaultInit — it attaches to whatever
			// the terminal has and shuts the bridge down. --profile would be
			// confusing here (you don't 'log out of' a profile).
			if err := b.Initialize(defaultInit(5000)); err != nil {
				return mapBridgeErr(err)
			}
			if err := b.Shutdown(); err != nil {
				return mapBridgeErr(err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "bridge shutdown OK (terminal session retained)")
			return nil
		},
	}
}

func newConnectStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current login + connection state",
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := bridge.New(bridge.Options{Stderr: cmd.ErrOrStderr(), CallTimeout: 10 * time.Second})
			if err != nil {
				return err
			}
			defer b.Close()
			if err := initBridge(cmd, b, 5000); err != nil {
				return err
			}
			acc, err := b.AccountInfo()
			if err != nil {
				return mapBridgeErr(err)
			}
			term, _ := b.TerminalInfo()
			fmt.Fprintf(cmd.OutOrStdout(), "account: %d @ %s (%s, %s)\n", acc.Login, acc.Server, acc.TradeModeName(), acc.Currency)
			if term != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "terminal: build=%d connected=%v trade_allowed=%v ping=%d\n",
					term.Build, term.Connected, term.TradeAllowed, term.PingLast)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "safety: %s\n", safety.ModeDescription())
			return nil
		},
	}
}

// ── account / terminal ───────────────────────────────────────────────────────

func newAccountCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "account", Short: "Account-level info"}
	cmd.AddCommand(&cobra.Command{
		Use:   "info",
		Short: "Print account balance, equity, margin, leverage, currency, trade mode",
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := bridge.New(bridge.Options{Stderr: cmd.ErrOrStderr(), CallTimeout: 10 * time.Second})
			if err != nil {
				return err
			}
			defer b.Close()
			if err := initBridge(cmd, b, 10000); err != nil {
				return err
			}
			acc, err := b.AccountInfo()
			if err != nil {
				return mapBridgeErr(err)
			}
			return emit(cmd, acc, printAccount)
		},
	})
	return cmd
}

func newTerminalCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "terminal", Short: "MT5 terminal info"}
	cmd.AddCommand(&cobra.Command{
		Use:   "info",
		Short: "Print terminal build, data path, connected/trade-allowed flags",
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := bridge.New(bridge.Options{Stderr: cmd.ErrOrStderr(), CallTimeout: 10 * time.Second})
			if err != nil {
				return err
			}
			defer b.Close()
			if err := initBridge(cmd, b, 10000); err != nil {
				return err
			}
			term, err := b.TerminalInfo()
			if err != nil {
				return mapBridgeErr(err)
			}
			return emit(cmd, term, printTerminal)
		},
	})
	return cmd
}

func printAccount(w io.Writer, v any) {
	a := v.(*bridge.AccountInfo)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	rows := [][2]string{
		{"login", fmt.Sprintf("%d", a.Login)},
		{"name", a.Name},
		{"server", a.Server},
		{"company", a.Company},
		{"mode", a.TradeModeName()},
		{"currency", a.Currency},
		{"leverage", fmt.Sprintf("1:%d", a.Leverage)},
		{"balance", fmt.Sprintf("%.2f %s", a.Balance, a.Currency)},
		{"equity", fmt.Sprintf("%.2f %s", a.Equity, a.Currency)},
		{"profit", fmt.Sprintf("%+.2f %s", a.Profit, a.Currency)},
		{"margin", fmt.Sprintf("%.2f %s", a.Margin, a.Currency)},
		{"margin_free", fmt.Sprintf("%.2f %s", a.MarginFree, a.Currency)},
		{"margin_level", fmt.Sprintf("%.2f%%", a.MarginLevel)},
		{"trade_allowed", fmt.Sprintf("%v", a.TradeAllowed)},
		{"trade_expert", fmt.Sprintf("%v", a.TradeExpert)},
	}
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\n", r[0], r[1])
	}
	tw.Flush()
}

func printTerminal(w io.Writer, v any) {
	t := v.(*bridge.TerminalInfo)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	rows := [][2]string{
		{"name", t.Name},
		{"company", t.Company},
		{"build", fmt.Sprintf("%d", t.Build)},
		{"language", t.Language},
		{"connected", fmt.Sprintf("%v", t.Connected)},
		{"trade_allowed", fmt.Sprintf("%v", t.TradeAllowed)},
		{"dlls_allowed", fmt.Sprintf("%v", t.DLLsAllowed)},
		{"ping_last_us", fmt.Sprintf("%d", t.PingLast)},
		{"path", t.Path},
		{"data_path", t.DataPath},
		{"commondata_path", t.CommondataPath},
	}
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\n", r[0], r[1])
	}
	tw.Flush()
}

// ── sync ────────────────────────────────────────────────────────────────────

func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Mirror MT5 data into the local SQLite store",
		Long: `Pull symbols, bars, ticks, orders, deals, and positions into the local SQLite store.

The mirror is the source of truth for stats, replay, backtest, and sql. First
sync of a large account can take minutes; subsequent runs are incremental
(every sync uses upsert semantics).`,
	}
	cmd.AddCommand(newSyncAllCmd())
	cmd.AddCommand(newSyncSymbolsCmd())
	cmd.AddCommand(newSyncPositionsCmd())
	cmd.AddCommand(newSyncOrdersCmd())
	cmd.AddCommand(newSyncDealsCmd())
	cmd.AddCommand(newSyncHistoryOrdersCmd())
	cmd.AddCommand(newSyncBarsCmd())
	cmd.AddCommand(newSyncTicksCmd())
	return cmd
}

func newSyncAllCmd() *cobra.Command {
	var (
		since       string
		withBars    bool
		withTicks   bool
		barsTFs     []string
		onlySymbols []string
	)
	cmd := &cobra.Command{
		Use:   "all",
		Short: "symbols + positions + orders + history (deals+orders) since --since; optionally bars/ticks",
		RunE: func(cmd *cobra.Command, args []string) error {
			from, err := parseSince(since)
			if err != nil {
				return &ExitErr{Code: ExitUsage, Err: err}
			}
			b, db, acc, err := openBridgeAndStore(cmd)
			if err != nil {
				return err
			}
			defer b.Close()
			defer db.Close()

			verbose := func(format string, args ...any) {}
			if v, _ := cmd.Flags().GetBool("verbose"); v {
				verbose = func(format string, args ...any) {
					fmt.Fprintf(cmd.ErrOrStderr(), "  "+format+"\n", args...)
				}
			}
			counts, err := store.SyncAll(cmd.Context(), db, b, store.AllOptions{
				AccountLogin: acc.Login,
				Since:        from,
				OnlySymbols:  onlySymbols,
				IncludeBars:  withBars,
				IncludeTicks: withTicks,
				BarsTFs:      barsTFs,
				Verbose:      verbose,
			})
			if err != nil {
				return mapBridgeErr(err)
			}
			return emit(cmd, counts, printSyncCounts)
		},
	}
	cmd.Flags().StringVar(&since, "since", "30d", "ISO date (YYYY-MM-DD) or relative (30d, 2y)")
	cmd.Flags().BoolVar(&withBars, "bars", false, "Also sync bars (loops per --only-symbols × --bars-tf; slow without filters)")
	cmd.Flags().BoolVar(&withTicks, "ticks", false, "Also sync ticks (huge — use with --only-symbols)")
	cmd.Flags().StringSliceVar(&barsTFs, "bars-tf", []string{"M5", "H1", "D1"}, "Timeframes for --bars")
	cmd.Flags().StringSliceVar(&onlySymbols, "only-symbols", nil, "Restrict bars/ticks to these symbols")
	return cmd
}

func newSyncSymbolsCmd() *cobra.Command {
	var group string
	cmd := &cobra.Command{
		Use:   "symbols",
		Short: "Sync the symbols table only",
		RunE: func(cmd *cobra.Command, args []string) error {
			b, db, acc, err := openBridgeAndStore(cmd)
			if err != nil {
				return err
			}
			defer b.Close()
			defer db.Close()
			c, err := store.SyncSymbols(cmd.Context(), db, b, acc.Login, group)
			if err != nil {
				return mapBridgeErr(err)
			}
			return emit(cmd, map[string]store.Counts{"symbols": c}, printSyncCounts)
		},
	}
	cmd.Flags().StringVar(&group, "group", "", "MT5 group filter (e.g. \"EUR*\")")
	return cmd
}

func newSyncPositionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "positions",
		Short: "Snapshot current open positions",
		RunE: func(cmd *cobra.Command, args []string) error {
			b, db, acc, err := openBridgeAndStore(cmd)
			if err != nil {
				return err
			}
			defer b.Close()
			defer db.Close()
			c, err := store.SyncPositions(cmd.Context(), db, b, acc.Login)
			if err != nil {
				return mapBridgeErr(err)
			}
			return emit(cmd, map[string]store.Counts{"positions": c}, printSyncCounts)
		},
	}
}

func newSyncOrdersCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "orders",
		Short: "Snapshot active (pending) orders",
		RunE: func(cmd *cobra.Command, args []string) error {
			b, db, acc, err := openBridgeAndStore(cmd)
			if err != nil {
				return err
			}
			defer b.Close()
			defer db.Close()
			c, err := store.SyncOrders(cmd.Context(), db, b, acc.Login)
			if err != nil {
				return mapBridgeErr(err)
			}
			return emit(cmd, map[string]store.Counts{"orders": c}, printSyncCounts)
		},
	}
}

func newSyncDealsCmd() *cobra.Command {
	var from, to string
	cmd := &cobra.Command{
		Use:   "deals",
		Short: "Sync historical deals between --from and --to",
		RunE: func(cmd *cobra.Command, args []string) error {
			fromT, toT, err := parseRange(from, to)
			if err != nil {
				return &ExitErr{Code: ExitUsage, Err: err}
			}
			b, db, acc, err := openBridgeAndStore(cmd)
			if err != nil {
				return err
			}
			defer b.Close()
			defer db.Close()
			c, err := store.SyncDeals(cmd.Context(), db, b, acc.Login, fromT, toT)
			if err != nil {
				return mapBridgeErr(err)
			}
			return emit(cmd, map[string]store.Counts{"deals": c}, printSyncCounts)
		},
	}
	cmd.Flags().StringVar(&from, "from", "30d", "ISO date or relative")
	cmd.Flags().StringVar(&to, "to", "now", "ISO date or relative")
	return cmd
}

func newSyncHistoryOrdersCmd() *cobra.Command {
	var from, to string
	cmd := &cobra.Command{
		Use:   "history-orders",
		Short: "Sync historical orders between --from and --to",
		RunE: func(cmd *cobra.Command, args []string) error {
			fromT, toT, err := parseRange(from, to)
			if err != nil {
				return &ExitErr{Code: ExitUsage, Err: err}
			}
			b, db, acc, err := openBridgeAndStore(cmd)
			if err != nil {
				return err
			}
			defer b.Close()
			defer db.Close()
			c, err := store.SyncHistoryOrders(cmd.Context(), db, b, acc.Login, fromT, toT)
			if err != nil {
				return mapBridgeErr(err)
			}
			return emit(cmd, map[string]store.Counts{"history_orders": c}, printSyncCounts)
		},
	}
	cmd.Flags().StringVar(&from, "from", "30d", "ISO date or relative")
	cmd.Flags().StringVar(&to, "to", "now", "ISO date or relative")
	return cmd
}

func newSyncBarsCmd() *cobra.Command {
	var (
		symbol, tf, from, to string
	)
	cmd := &cobra.Command{
		Use:   "bars",
		Short: "Sync bars for one symbol+timeframe (--symbol --tf --from --to)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if symbol == "" {
				return &ExitErr{Code: ExitUsage, Err: fmt.Errorf("--symbol is required")}
			}
			fromT, toT, err := parseRange(from, to)
			if err != nil {
				return &ExitErr{Code: ExitUsage, Err: err}
			}
			b, db, acc, err := openBridgeAndStore(cmd)
			if err != nil {
				return err
			}
			defer b.Close()
			defer db.Close()
			c, err := store.SyncBars(cmd.Context(), db, b, acc.Login, symbol, tf, fromT, toT)
			if err != nil {
				return mapBridgeErr(err)
			}
			return emit(cmd, map[string]store.Counts{fmt.Sprintf("bars_%s_%s", symbol, tf): c}, printSyncCounts)
		},
	}
	cmd.Flags().StringVar(&symbol, "symbol", "", "Symbol (required)")
	cmd.Flags().StringVar(&tf, "tf", "H1", "Timeframe: M1 M5 M15 M30 H1 H4 D1 W1 MN1")
	cmd.Flags().StringVar(&from, "from", "30d", "ISO date or relative")
	cmd.Flags().StringVar(&to, "to", "now", "ISO date or relative")
	return cmd
}

func newSyncTicksCmd() *cobra.Command {
	var symbol, from, to string
	cmd := &cobra.Command{
		Use:   "ticks",
		Short: "Sync ticks for one symbol (--symbol --from --to)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if symbol == "" {
				return &ExitErr{Code: ExitUsage, Err: fmt.Errorf("--symbol is required")}
			}
			fromT, toT, err := parseRange(from, to)
			if err != nil {
				return &ExitErr{Code: ExitUsage, Err: err}
			}
			b, db, acc, err := openBridgeAndStore(cmd)
			if err != nil {
				return err
			}
			defer b.Close()
			defer db.Close()
			c, err := store.SyncTicks(cmd.Context(), db, b, acc.Login, symbol, fromT, toT)
			if err != nil {
				return mapBridgeErr(err)
			}
			return emit(cmd, map[string]store.Counts{"ticks_" + symbol: c}, printSyncCounts)
		},
	}
	cmd.Flags().StringVar(&symbol, "symbol", "", "Symbol (required)")
	cmd.Flags().StringVar(&from, "from", "1d", "ISO date/time or relative (ticks are huge — keep this narrow)")
	cmd.Flags().StringVar(&to, "to", "now", "ISO date/time or relative")
	return cmd
}

// ── sql ─────────────────────────────────────────────────────────────────────

func newSQLCmd() *cobra.Command {
	var allowWrite bool
	cmd := &cobra.Command{
		Use:   "sql \"<query>\"",
		Short: "Run an SQL query against the local mirror",
		Long: `Execute a read-only or write SQL statement against the local store.

Tables:
  schema_migrations, accounts, symbols, ticks,
  bars_M1, bars_M5, bars_M15, bars_M30, bars_H1, bars_H4,
  bars_D1, bars_W1, bars_MN1,
  orders, history_orders, positions, deals,
  calendar_events, features, backtests, audit.

Read-only by default — INSERT/UPDATE/DELETE/DROP/ALTER require --write.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]
			// looksLikeWrite is a fast UX guard so writers see a clear "use
			// --write" message instead of a cryptic SQLite error. The actual
			// gate is the read-only connection below: SQLite refuses every
			// write at the engine level, including writes smuggled inside a
			// CTE like `WITH x AS (SELECT 1) DELETE FROM audit`.
			if !allowWrite && looksLikeWrite(query) {
				return &ExitErr{Code: ExitUsage, Err: fmt.Errorf("query looks like a write — re-run with --write to allow it")}
			}
			var db *sql.DB
			var err error
			if allowWrite {
				db, err = store.OpenAndMigrate("")
			} else {
				db, err = store.OpenReadOnly("")
			}
			if err != nil {
				return &ExitErr{Code: ExitConfig, Err: err}
			}
			defer db.Close()
			return runSQL(cmd, db, query, allowWrite)
		},
	}
	cmd.Flags().BoolVar(&allowWrite, "write", false, "Allow INSERT/UPDATE/DELETE/DROP/ALTER statements")
	return cmd
}

// ── shared helpers for sync/sql ─────────────────────────────────────────────

// openBridgeAndStore is the boilerplate every sync subcommand needs.
// Returns a connected bridge, an opened+migrated DB, and the current account
// (so we know which account_login to tag rows with).
func openBridgeAndStore(cmd *cobra.Command) (*bridge.Bridge, *sql.DB, *bridge.AccountInfo, error) {
	b, err := bridge.New(bridge.Options{Stderr: cmd.ErrOrStderr(), CallTimeout: 60 * time.Second})
	if err != nil {
		return nil, nil, nil, err
	}
	if err := initBridge(cmd, b, 10000); err != nil {
		_ = b.Close()
		return nil, nil, nil, err
	}
	acc, err := b.AccountInfo()
	if err != nil {
		_ = b.Close()
		return nil, nil, nil, mapBridgeErr(err)
	}
	db, err := store.OpenAndMigrate("")
	if err != nil {
		_ = b.Close()
		return nil, nil, nil, &ExitErr{Code: ExitConfig, Err: err}
	}
	// Also stamp the accounts table so SQL queries can join freely.
	_, _ = db.ExecContext(cmd.Context(), `INSERT INTO accounts(
		login, server, name, company, currency, leverage, trade_mode, last_synced
	) VALUES (?,?,?,?,?,?,?,?)
	ON CONFLICT(login) DO UPDATE SET
		server=excluded.server, name=excluded.name, company=excluded.company,
		currency=excluded.currency, leverage=excluded.leverage,
		trade_mode=excluded.trade_mode, last_synced=excluded.last_synced`,
		acc.Login, acc.Server, acc.Name, acc.Company, acc.Currency, acc.Leverage,
		acc.TradeModeName(), time.Now().UnixMilli())
	return b, db, acc, nil
}

func parseSince(s string) (time.Time, error) {
	t, _, err := parseRangeOne(s, time.Now())
	return t, err
}

func parseRange(from, to string) (time.Time, time.Time, error) {
	now := time.Now()
	toT, _, err := parseRangeOne(to, now)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("--to: %w", err)
	}
	fromT, _, err := parseRangeOne(from, toT)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("--from: %w", err)
	}
	return fromT, toT, nil
}

// parseRangeOne accepts:
//   - "now" / "today" → now / now (truncated to day)
//   - ISO date "YYYY-MM-DD"
//   - ISO datetime "YYYY-MM-DD HH:MM:SS"
//   - relative "<int><unit>" with unit ∈ {m,h,d,w,y} (m = minutes, not months)
//
// The second return value is a "specified day" flag (true if YYYY-MM-DD form).
func parseRangeOne(s string, anchor time.Time) (time.Time, bool, error) {
	s = strings.TrimSpace(s)
	switch s {
	case "now":
		return anchor, false, nil
	case "today":
		return time.Date(anchor.Year(), anchor.Month(), anchor.Day(), 0, 0, 0, 0, anchor.Location()), true, nil
	}
	for _, fmt2 := range []string{"2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.Parse(fmt2, s); err == nil {
			return t, fmt2 == "2006-01-02", nil
		}
	}
	if d, err := parseRelativeDuration(s); err == nil {
		return anchor.Add(-d), false, nil
	}
	return time.Time{}, false, fmt.Errorf("unparseable date %q (try 2024-01-01, 30d, 2y)", s)
}

func parseRelativeDuration(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("too short")
	}
	unit := s[len(s)-1]
	n, err := strconv.Atoi(s[:len(s)-1])
	if err != nil {
		return 0, err
	}
	switch unit {
	case 'm':
		return time.Duration(n) * time.Minute, nil
	case 'h':
		return time.Duration(n) * time.Hour, nil
	case 'd':
		return time.Duration(n) * 24 * time.Hour, nil
	case 'w':
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	case 'y':
		return time.Duration(n) * 365 * 24 * time.Hour, nil
	}
	return 0, fmt.Errorf("unknown unit %q", unit)
}

// looksLikeWrite is a fast UX guard. The real gate against writes on a
// read-only connection is store.OpenReadOnly (mode=ro + query_only=1), which
// rejects writes at the SQLite engine level. This heuristic just lets us
// emit a clearer "use --write" message before the engine error reaches the
// user. Note: PRAGMA is intentionally NOT in the list because read-only
// PRAGMAs (table_info, database_list, schema_version, ...) are legitimate
// SELECTs in disguise; write-PRAGMAs (journal_mode = wal, foreign_keys =
// 0) fail at the engine on an RO connection.
func looksLikeWrite(q string) bool {
	first := strings.ToUpper(strings.TrimSpace(q))
	for _, kw := range []string{"INSERT", "UPDATE", "DELETE", "DROP", "ALTER", "CREATE", "REPLACE", "TRUNCATE"} {
		if strings.HasPrefix(first, kw) {
			return true
		}
	}
	return false
}

// runSQL executes a SELECT (and PRAGMA, and EXPLAIN) through QueryContext, or
// a DML/DDL statement through ExecContext. Pass writable=true only when the
// caller opened the DB read-write; on an RO connection, always use the Query
// path because Exec would silently swallow rows from PRAGMA-style reads.
func runSQL(cmd *cobra.Command, db *sql.DB, query string, writable bool) error {
	if writable && looksLikeWrite(query) {
		res, err := db.ExecContext(cmd.Context(), query)
		if err != nil {
			return &ExitErr{Code: ExitConfig, Err: err}
		}
		n, _ := res.RowsAffected()
		fmt.Fprintf(cmd.OutOrStdout(), "ok: %d row(s) affected\n", n)
		return nil
	}
	rows, err := db.QueryContext(cmd.Context(), query)
	if err != nil {
		return &ExitErr{Code: ExitConfig, Err: err}
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return err
	}

	var all []map[string]any
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		r := map[string]any{}
		for i, c := range cols {
			r[c] = normalizeSQLValue(vals[i])
		}
		all = append(all, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if jsonFlag(cmd) {
		return emit(cmd, all, nil)
	}
	printSQLTable(cmd.OutOrStdout(), cols, all)
	return nil
}

func normalizeSQLValue(v any) any {
	switch x := v.(type) {
	case []byte:
		return string(x)
	default:
		return x
	}
}

func printSQLTable(w io.Writer, cols []string, rows []map[string]any) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(cols, "\t"))
	sep := make([]string, len(cols))
	for i := range sep {
		sep[i] = strings.Repeat("─", len(cols[i]))
	}
	fmt.Fprintln(tw, strings.Join(sep, "\t"))
	for _, r := range rows {
		parts := make([]string, len(cols))
		for i, c := range cols {
			parts[i] = fmt.Sprintf("%v", r[c])
		}
		fmt.Fprintln(tw, strings.Join(parts, "\t"))
	}
	tw.Flush()
	fmt.Fprintf(w, "\n(%d row%s)\n", len(rows), pluralS(len(rows)))
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func printSyncCounts(w io.Writer, v any) {
	counts := v.(map[string]store.Counts)
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "RESOURCE\tROWS")
	fmt.Fprintln(tw, "────────\t────")
	total := 0
	for _, k := range keys {
		c := counts[k]
		total += c.Inserted
		fmt.Fprintf(tw, "%s\t%d\n", k, c.Inserted)
	}
	tw.Flush()
	fmt.Fprintf(w, "\nTotal rows synced: %d\n", total)
}

// ── shared helpers ───────────────────────────────────────────────────────────

// mapBridgeErr converts a bridge sentinel into an ExitErr with the right code.
func mapBridgeErr(err error) error {
	switch {
	case errors.Is(err, bridge.ErrNotLoggedIn):
		return &ExitErr{Code: ExitAuth, Err: err}
	case errors.Is(err, bridge.ErrTerminalDown):
		return &ExitErr{Code: ExitTerminalDown, Err: err}
	case errors.Is(err, bridge.ErrBrokerRejected):
		return &ExitErr{Code: ExitBrokerRejected, Err: err}
	case errors.Is(err, bridge.ErrRateLimited):
		return &ExitErr{Code: ExitRateLimited, Err: err}
	case errors.Is(err, bridge.ErrMT5PkgMissing):
		return &ExitErr{Code: ExitConfig, Err: err}
	default:
		return err
	}
}

// emit prints the value as JSON if --json/--agent is set, otherwise calls human.
func emit(cmd *cobra.Command, v any, human func(io.Writer, any)) error {
	w := cmd.OutOrStdout()
	if jsonFlag(cmd) {
		enc := json.NewEncoder(w)
		if !compactFlag(cmd) {
			enc.SetIndent("", "  ")
		}
		return enc.Encode(v)
	}
	human(w, v)
	return nil
}

func jsonFlag(cmd *cobra.Command) bool {
	// Explicit human-friendly always wins.
	if b, _ := cmd.Flags().GetBool("human-friendly"); b {
		return false
	}
	if b, _ := cmd.Flags().GetBool("json"); b {
		return true
	}
	if b, _ := cmd.Flags().GetBool("agent"); b {
		return true
	}
	// auto: emit JSON when stdout isn't a TTY
	if !isTerminal(cmd.OutOrStdout()) {
		return true
	}
	return false
}

func compactFlag(cmd *cobra.Command) bool {
	if b, _ := cmd.Flags().GetBool("compact"); b {
		return true
	}
	if b, _ := cmd.Flags().GetBool("agent"); b {
		return true
	}
	return false
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
