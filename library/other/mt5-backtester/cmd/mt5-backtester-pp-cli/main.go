package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/mt5-backtester/internal/backtest"
	"github.com/mvanhorn/printing-press-library/library/other/mt5-backtester/internal/batch"
	"github.com/mvanhorn/printing-press-library/library/other/mt5-backtester/internal/compile"
	"github.com/mvanhorn/printing-press-library/library/other/mt5-backtester/internal/config"
	"github.com/mvanhorn/printing-press-library/library/other/mt5-backtester/internal/profiles"
	"github.com/mvanhorn/printing-press-library/library/other/mt5-backtester/internal/report"
	"github.com/mvanhorn/printing-press-library/library/other/mt5-backtester/internal/setfile"
)

var (
	version = "2.0.0"

	// Global flags
	terminalFlag string
	editorFlag   string
	workDir      string
	profileFlag  string
	verbose      bool
	outputFmt    string

	bold   = color.New(color.Bold).SprintFunc()
	green  = color.New(color.FgGreen).SprintFunc()
	red    = color.New(color.FgRed).SprintFunc()
	cyan   = color.New(color.FgCyan).SprintFunc()
	yellow = color.New(color.FgYellow).SprintFunc()
)

func main() {
	root := &cobra.Command{
		Use:   "mt5-backtester-pp-cli",
		Short: "MT5 backtester CLI — run, compile, report, batch, profiles",
		Long: `mt5-backtester-pp-cli — Printing Press CLI for MetaTrader 5 backtesting

Run Expert Advisor backtests, compile MQL5, parse reports,
batch test multiple EAs across symbols/timeframes, manage
multiple MT5 terminals via named profiles.

Quick start:
  mt5-backtester-pp-cli profile add --name broker1 --terminal "C:\MT5\terminal64.exe"
  mt5-backtester-pp-cli run --ea "MACD Sample" --symbol EURUSD --period H1
  mt5-backtester-pp-cli template && mt5-backtester-pp-cli batch --file batch.json
  mt5-backtester-pp-cli service install`,
		Version: version,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if workDir == "" {
				workDir = filepath.Join(os.TempDir(), "mt5-backtester-pp-cli")
			}
		},
	}

	root.PersistentFlags().StringVar(&terminalFlag, "terminal", "", "Path to terminal64.exe (overrides profile)")
	root.PersistentFlags().StringVar(&editorFlag, "editor", "", "Path to metaeditor64.exe")
	root.PersistentFlags().StringVar(&workDir, "workdir", "", "Working directory for ini/report files")
	root.PersistentFlags().StringVarP(&profileFlag, "profile", "p", "", "Named terminal profile to use")
	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	root.PersistentFlags().StringVarP(&outputFmt, "output", "o", "text", "Output: text, json, csv")

	root.AddCommand(
		runCmd(),
		compileCmd(),
		reportCmd(),
		batchCmd(),
		profileCmd(),
		serviceCmd(),
		configCmd(),
		templateCmd(),
		setfileCmd(),
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// resolveTerminal returns (terminalPath, portable, error) for a given profile name.
// Priority: --terminal flag > --profile flag > default profile > MT5_PATH env > auto-detect
func resolveTerminal(profileName string) (string, bool, error) {
	// 1. Explicit --terminal flag wins everything
	if terminalFlag != "" {
		return terminalFlag, false, nil
	}

	// 2. Profile lookup
	store, err := profiles.Load()
	if err != nil {
		return "", false, err
	}

	name := profileName
	if profileFlag != "" {
		name = profileFlag // global --profile flag overrides job-level profile
	}

	if name != "" || store.Default != "" {
		prof, err := store.Get(name)
		if err == nil {
			return prof.TerminalPath, prof.Portable, nil
		}
		if name != "" {
			return "", false, err // named profile not found — hard error
		}
		// default not set, fall through
	}

	// 3. MT5_PATH env var
	if env := os.Getenv("MT5_PATH"); env != "" {
		candidate := filepath.Join(env, "terminal64.exe")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, false, nil
		}
	}

	// 4. Auto-detect common paths
	if t := config.FindTerminal(); t != "" {
		return t, false, nil
	}

	return "", false, fmt.Errorf("no MT5 terminal found — run: mt5-backtester-pp-cli profile add")
}

// ── run ──────────────────────────────────────────────────────────────────────

func runCmd() *cobra.Command {
	var (
		ea           string
		symbol       string
		period       string
		from         string
		to           string
		model        int
		deposit      float64
		currency     string
		leverage     int
		optimization int
		forwardMode  int
		forwardDate  string
		reportName   string
		timeout      time.Duration
		inputs       []string
		setFlag      string
		portable     bool
		noShutdown   bool
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run a single backtest",
		Long: `Run an MT5 Expert Advisor backtest.

Tick models:
  0 = Every Tick      (most accurate, slowest — use for scalpers)
  1 = 1 Minute OHLC   (good balance, default)
  2 = Open Prices Only (fastest — use for daily/weekly strategies)
  4 = Real Ticks       (requires downloaded tick data from broker)

Examples:
  mt5-backtester-pp-cli run --ea "MACD Sample" --symbol EURUSD --period H1
  mt5-backtester-pp-cli run --ea MyEA --symbol GBPUSD --period M5 --model 0 --deposit 50000
  mt5-backtester-pp-cli run --ea MyEA --input StopLoss=50 --input TakeProfit=100
  mt5-backtester-pp-cli run --ea MyEA --profile broker2 --symbol XAUUSD`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if ea == "" {
				return fmt.Errorf("--ea is required (e.g. --ea \"MACD Sample\")")
			}

			termPath, isPortable, err := resolveTerminal("")
			if err != nil {
				return err
			}
			if portable {
				isPortable = true
			}

			inputMap := parseInputs(inputs)

			// Set default dates
			if from == "" {
				from = time.Now().AddDate(-1, 0, 0).Format("2006.01.02")
			}
			if to == "" {
				to = time.Now().Format("2006.01.02")
			}

			var setPath string
			if setFlag != "" {
				resolved, err := setfile.ResolvePath(setFlag, termPath, isPortable)
				if err != nil {
					return err
				}
				setPath = resolved
			}

			params := &config.BacktestParams{
				Expert:       config.ResolveExpertPath(ea),
				Symbol:       strings.ToUpper(symbol),
				Period:       strings.ToUpper(period),
				FromDate:     from,
				ToDate:       to,
				Model:        model,
				Deposit:      deposit,
				Currency:     strings.ToUpper(currency),
				Leverage:     leverage,
				Optimization: optimization,
				ForwardMode:  forwardMode,
				ForwardDate:  forwardDate,
				Report:       reportName,
				ShutdownMode: shutdownMode(noShutdown),
				Inputs:       inputMap,
				SetFile:      setPath,
			}

			fmt.Printf("\n%s\n", bold("MT5 Backtest"))
			fmt.Printf("Terminal: %s\n", cyan(termPath))
			fmt.Printf("EA:       %s\n", cyan(params.Expert))
			fmt.Printf("Symbol:   %s  %s\n", params.Symbol, params.Period)
			fmt.Printf("Range:    %s → %s\n", params.FromDate, params.ToDate)
			fmt.Printf("Model:    %s\n", params.ModelName())
			fmt.Printf("Deposit:  %.2f %s\n", params.Deposit, params.Currency)
			if params.SetFile != "" {
				fmt.Printf("Set file: %s\n", cyan(params.SetFile))
				if len(params.Inputs) > 0 {
					fmt.Printf("Overrides: %d input(s) overriding set file\n", len(params.Inputs))
				}
			}
			fmt.Println()

			iniPath, err := config.WriteINI(params, workDir)
			if err != nil {
				return fmt.Errorf("write ini: %w", err)
			}
			if verbose {
				fmt.Printf("INI: %s\n", iniPath)
			}

			fmt.Println("→ Launching terminal64.exe...")

			result := backtest.Run(backtest.RunOptions{
				TerminalPath: termPath,
				INIPath:      iniPath,
				ReportDir:    workDir,
				Timeout:      timeout,
				Portable:     isPortable,
				KeepOpen:     noShutdown,
				Verbose:      verbose,
			})

			if result.Error != nil {
				fmt.Printf("%s %v\n", red("✗"), result.Error)
				return result.Error
			}

			fmt.Printf("%s Completed in %s\n", green("✓"), result.Duration.Round(time.Second))

			if result.ReportPath != "" {
				fmt.Printf("Report: %s\n", result.ReportPath)
				stats, err := report.ParseFile(result.ReportPath)
				if err != nil {
					fmt.Printf("Warning: could not parse report: %v\n", err)
				} else {
					printStats(stats)
				}
			} else {
				fmt.Printf("%s No report file found.\n", yellow("~"))
				fmt.Println("Check that MT5 is logged into a broker account and has history for this symbol.")
				fmt.Printf("Reports land in: %%APPDATA%%\\MetaQuotes\\Terminal\\<hash>\\tester\\\n")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&ea, "ea", "", "EA name or path (required)")
	cmd.Flags().StringVar(&symbol, "symbol", "EURUSD", "Trading symbol")
	cmd.Flags().StringVar(&period, "period", "H1", "Timeframe: M1 M5 M15 M30 H1 H4 D1 W1")
	cmd.Flags().StringVar(&from, "from", "", "Start date YYYY.MM.DD (default: 1 year ago)")
	cmd.Flags().StringVar(&to, "to", "", "End date YYYY.MM.DD (default: today)")
	cmd.Flags().IntVar(&model, "model", 1, "Tick model: 0=EveryTick 1=1MinOHLC 2=OpenOnly 4=RealTicks")
	cmd.Flags().Float64Var(&deposit, "deposit", 10000, "Initial deposit")
	cmd.Flags().StringVar(&currency, "currency", "USD", "Deposit currency")
	cmd.Flags().IntVar(&leverage, "leverage", 100, "Account leverage")
	cmd.Flags().IntVar(&optimization, "optimization", 0, "0=off 1=slow 2=genetic")
	cmd.Flags().IntVar(&forwardMode, "forward", 0, "Forward mode: 0=none 1=1/2 2=1/3 3=1/4 4=custom")
	cmd.Flags().StringVar(&forwardDate, "forward-date", "", "Custom forward date")
	cmd.Flags().StringVar(&reportName, "report", "", "Report filename (auto if empty)")
	cmd.Flags().DurationVar(&timeout, "timeout", 4*time.Hour, "Max wait time")
	cmd.Flags().StringArrayVar(&inputs, "input", nil, "EA input: --input StopLoss=50 (overrides set file values)")
	cmd.Flags().StringVar(&setFlag, "set", "", "Path or name of .set file (resolves under MQL5\\Profiles\\Tester\\ if not absolute)")
	cmd.Flags().BoolVar(&portable, "portable", false, "Pass /portable to terminal")
	cmd.Flags().BoolVar(&noShutdown, "no-shutdown", false, "Keep terminal open after test")
	return cmd
}

// ── compile ──────────────────────────────────────────────────────────────────

func compileCmd() *cobra.Command {
	var includePaths []string

	cmd := &cobra.Command{
		Use:   "compile <file.mq5>",
		Short: "Compile an MQL5 source file to .ex5",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			edPath := editorFlag
			if edPath == "" {
				termPath, _, err := resolveTerminal("")
				if err == nil {
					edPath = config.FindMetaEditor(termPath)
				}
			}
			if edPath == "" {
				return fmt.Errorf("metaeditor64.exe not found — use --editor")
			}

			srcFile, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			fmt.Printf("Compiling: %s\n", srcFile)

			result, err := compile.Run(compile.Options{
				MetaEditorPath: edPath,
				SourceFile:     srcFile,
				IncludePaths:   includePaths,
				Verbose:        verbose,
			})
			if err != nil {
				return err
			}
			compile.PrintResult(result, verbose)
			if !result.Success {
				return fmt.Errorf("compilation failed")
			}
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&includePaths, "inc", nil, "Additional include directories")
	return cmd
}

// ── report ───────────────────────────────────────────────────────────────────

func reportCmd() *cobra.Command {
	var reportDir string

	cmd := &cobra.Command{
		Use:   "report [file.htm]",
		Short: "Parse and display a backtest report",
		RunE: func(cmd *cobra.Command, args []string) error {
			var reportPath string
			if len(args) > 0 {
				reportPath = args[0]
			} else if reportDir != "" {
				path, err := findMostRecentReport(reportDir)
				if err != nil {
					return err
				}
				reportPath = path
				fmt.Printf("Found: %s\n", reportPath)
			} else {
				return fmt.Errorf("provide a report file or --dir")
			}

			stats, err := report.ParseFile(reportPath)
			if err != nil {
				return fmt.Errorf("parse report: %w", err)
			}
			printStats(stats)
			return nil
		},
	}
	cmd.Flags().StringVar(&reportDir, "dir", "", "Directory to search for most recent report")
	return cmd
}

// ── batch ────────────────────────────────────────────────────────────────────

func batchCmd() *cobra.Command {
	var (
		batchFile string
		timeout   time.Duration
	)

	cmd := &cobra.Command{
		Use:   "batch",
		Short: "Run multiple EAs/symbols/timeframes from a JSON file",
		Long: `Run a batch of backtests from a JSON file.

No need to change any .ini file — each job generates its own ini automatically.
Set defaults once, list your EAs/symbols/periods, and mt5-backtester-pp-cli runs them all.

Jobs run sequentially (MT5 limitation: one backtest at a time per terminal).
Different jobs can target different MT5 terminals via "profile" field.

Examples:
  mt5-backtester-pp-cli template                        # generate batch.json
  mt5-backtester-pp-cli batch --file batch.json         # run all jobs
  mt5-backtester-pp-cli batch --file batch.json -o csv > results.csv
  mt5-backtester-pp-cli batch --file batch.json -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if batchFile == "" {
				return fmt.Errorf("--file is required (generate one with: mt5-backtester-pp-cli template)")
			}

			bf, err := batch.LoadBatchFile(batchFile)
			if err != nil {
				return err
			}

			fmt.Printf("\n%s\n", bold("Batch Backtest"))
			fmt.Printf("File:    %s\n", batchFile)
			fmt.Printf("Workdir: %s\n", workDir)

			results := batch.RunBatch(
				bf.Jobs,
				bf.Defaults,
				workDir,
				timeout,
				resolveTerminal, // each job calls this — supports mixed profiles
				verbose,
			)

			switch outputFmt {
			case "json":
				batch.PrintSummaryJSON(results)
			case "csv":
				batch.PrintSummaryCSV(results)
			default:
				batch.PrintSummaryTable(results)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&batchFile, "file", "", "Path to batch JSON file")
	cmd.Flags().DurationVar(&timeout, "timeout", 4*time.Hour, "Timeout per job")
	return cmd
}

// ── profile ───────────────────────────────────────────────────────────────────

func profileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage named MT5 terminal profiles",
		Long: `Manage named MT5 terminal profiles.

Each profile points to a specific terminal64.exe installation.
Useful when you have multiple brokers or multiple MT5 copies.

Commands:
  profile list                  — show all profiles
  profile add  --name <n> ...   — add or update a profile
  profile use  <name>           — set default profile
  profile remove <name>         — delete a profile`,
	}

	// list
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := profiles.Load()
			if err != nil {
				return err
			}
			fmt.Printf("\nProfiles file: %s\n\n", profiles.DefaultStorePath())
			store.Print()
			if store.Default != "" {
				fmt.Printf("\nDefault: %s\n", green(store.Default))
			}
			return nil
		},
	})

	// add
	var (
		addName     string
		addTerminal string
		addEditor   string
		addPortable bool
		addLogin    string
		addServer   string
		addDesc     string
		setDefault  bool
	)
	addCmd := &cobra.Command{
		Use:   "add",
		Short: "Add or update a profile",
		Long: `Add a named MT5 terminal profile.

Examples:
  mt5-backtester-pp-cli profile add --name broker1 --terminal "C:\MT5-Broker1\terminal64.exe"
  mt5-backtester-pp-cli profile add --name broker2 --terminal "C:\MT5-Broker2\terminal64.exe" --portable --default
  mt5-backtester-pp-cli profile add --name gold-desk --terminal "C:\MT5-XAUUSD\terminal64.exe" --server "Broker-Live"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if addName == "" {
				return fmt.Errorf("--name is required")
			}
			if addTerminal == "" {
				return fmt.Errorf("--terminal is required")
			}
			if _, err := os.Stat(addTerminal); err != nil {
				fmt.Printf("%s terminal64.exe not found at %s — saving anyway\n", yellow("~"), addTerminal)
			}

			edPath := addEditor
			if edPath == "" {
				edPath = profiles.DeriveEditorPath(addTerminal)
			}

			store, err := profiles.Load()
			if err != nil {
				return err
			}

			prof := &profiles.Profile{
				Name:         addName,
				TerminalPath: addTerminal,
				EditorPath:   edPath,
				Portable:     addPortable,
				Login:        addLogin,
				Server:       addServer,
				Description:  addDesc,
			}
			store.Add(prof)
			if setDefault {
				store.Default = addName
			}

			if err := store.Save(); err != nil {
				return err
			}
			fmt.Printf("%s Profile %q saved\n", green("✓"), addName)
			if store.Default == addName {
				fmt.Printf("  Default: yes\n")
			}
			fmt.Printf("  Terminal: %s\n", addTerminal)
			if edPath != "" {
				fmt.Printf("  Editor:   %s\n", edPath)
			}
			fmt.Printf("\nRun a backtest with this profile:\n")
			fmt.Printf("  mt5-backtester-pp-cli run --profile %s --ea \"MACD Sample\" --symbol EURUSD\n", addName)
			return nil
		},
	}
	addCmd.Flags().StringVar(&addName, "name", "", "Profile name (required)")
	addCmd.Flags().StringVar(&addTerminal, "terminal", "", "Path to terminal64.exe (required)")
	addCmd.Flags().StringVar(&addEditor, "editor", "", "Path to metaeditor64.exe (auto-derived if empty)")
	addCmd.Flags().BoolVar(&addPortable, "portable", false, "Use /portable flag with this terminal")
	addCmd.Flags().StringVar(&addLogin, "login", "", "Broker account number (for documentation)")
	addCmd.Flags().StringVar(&addServer, "server", "", "Broker server name (for documentation)")
	addCmd.Flags().StringVar(&addDesc, "desc", "", "Human description (e.g. 'IC Markets Demo')")
	addCmd.Flags().BoolVar(&setDefault, "default", false, "Set as default profile")
	cmd.AddCommand(addCmd)

	// use (set default)
	cmd.AddCommand(&cobra.Command{
		Use:   "use <name>",
		Short: "Set the default profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := profiles.Load()
			if err != nil {
				return err
			}
			if err := store.SetDefault(args[0]); err != nil {
				return err
			}
			if err := store.Save(); err != nil {
				return err
			}
			fmt.Printf("%s Default profile set to %q\n", green("✓"), args[0])
			return nil
		},
	})

	// remove
	cmd.AddCommand(&cobra.Command{
		Use:   "remove <name>",
		Short: "Delete a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := profiles.Load()
			if err != nil {
				return err
			}
			if err := store.Remove(args[0]); err != nil {
				return err
			}
			if err := store.Save(); err != nil {
				return err
			}
			fmt.Printf("%s Profile %q removed\n", green("✓"), args[0])
			return nil
		},
	})

	return cmd
}

// ── service ───────────────────────────────────────────────────────────────────

func serviceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Manage MT5 Windows services (requires NSSM)",
		Long: `Install and manage MT5 terminal instances as Windows Services.

Each profile becomes a service that starts automatically on boot,
runs in the background without anyone logged in, and restarts on crash.

Requires NSSM (Non-Sucking Service Manager):
  choco install nssm
  or download from https://nssm.cc/download

Commands:
  service install    — generate and run the install PowerShell script
  service list       — show status of all MT5 services
  service start <n>  — start a service
  service stop <n>   — stop a service
  service remove <n> — uninstall a service`,
	}

	// install
	cmd.AddCommand(&cobra.Command{
		Use:   "install",
		Short: "Generate PowerShell script to install all profiles as Windows services",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := profiles.Load()
			if err != nil {
				return err
			}
			if len(store.Profiles) == 0 {
				return fmt.Errorf("no profiles configured — add one with: mt5-backtester-pp-cli profile add")
			}

			scriptPath := filepath.Join(workDir, "install-services.ps1")
			if err := os.MkdirAll(workDir, 0755); err != nil {
				return err
			}
			if err := profiles.WriteServiceScript(store, scriptPath); err != nil {
				return err
			}

			fmt.Printf("%s Service install script written: %s\n\n", green("✓"), scriptPath)
			fmt.Printf("Services to be installed:\n")
			for name := range store.Profiles {
				fmt.Printf("  MT5-%s\n", name)
			}
			fmt.Printf("\nRun as Administrator:\n")
			fmt.Printf("  powershell -ExecutionPolicy Bypass -File %q\n\n", scriptPath)
			fmt.Printf("Or step by step:\n")
			fmt.Printf("  1. Install NSSM:  choco install nssm\n")
			fmt.Printf("  2. Run the script above as Administrator\n")
			fmt.Printf("  3. Check status:  mt5-backtester-pp-cli service list\n")
			return nil
		},
	})

	// list
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "Show status of all MT5 services",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("\nRun this in PowerShell to see MT5 service status:")
			fmt.Println(`  Get-Service | Where-Object { $_.Name -like "MT5-*" } | Format-Table Name, Status, StartType`)
			fmt.Println()
			fmt.Println("Or with NSSM:")
			fmt.Println(`  nssm status MT5-broker1`)
		},
	})

	// start / stop / remove shortcuts
	for _, action := range []struct{ use, short, nssm string }{
		{"start <name>", "Start an MT5 service", "start"},
		{"stop <name>", "Stop an MT5 service", "stop"},
		{"remove <name>", "Uninstall an MT5 service", "remove"},
	} {
		a := action
		cmd.AddCommand(&cobra.Command{
			Use:   a.use,
			Short: a.short,
			Args:  cobra.ExactArgs(1),
			Run: func(cmd *cobra.Command, args []string) {
				svcName := "MT5-" + args[0]
				fmt.Printf("Run as Administrator:\n")
				fmt.Printf("  nssm %s %s\n", a.nssm, svcName)
				fmt.Printf("\nOr via PowerShell:\n")
				switch a.nssm {
				case "start":
					fmt.Printf("  Start-Service %s\n", svcName)
				case "stop":
					fmt.Printf("  Stop-Service %s\n", svcName)
				case "remove":
					fmt.Printf("  nssm remove %s confirm\n", svcName)
				}
			},
		})
	}

	return cmd
}

// ── config ───────────────────────────────────────────────────────────────────

func configCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Show detected configuration and paths",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("\n%s\n\n", bold("mt5-backtester-pp-cli Configuration"))

			store, _ := profiles.Load()

			// Profiles
			if store != nil && len(store.Profiles) > 0 {
				fmt.Printf("Profiles (%s):\n", profiles.DefaultStorePath())
				store.Print()
				fmt.Println()
			} else {
				fmt.Printf("Profiles: none — run: mt5-backtester-pp-cli profile add\n\n")
			}

			// Auto-detect
			term := config.FindTerminal()
			if term == "" {
				term = red("not found")
			} else {
				term = green(term)
			}
			fmt.Printf("Auto-detected terminal: %s\n", term)
			fmt.Printf("MT5_PATH env:           %s\n", os.Getenv("MT5_PATH"))
			fmt.Printf("Work dir:               %s\n\n", workDir)

			fmt.Println("Set MT5_PATH to skip --terminal flag:")
			fmt.Println(`  $env:MT5_PATH = "C:\Program Files\MetaTrader 5"   (PowerShell)`)
			fmt.Println(`  set MT5_PATH=C:\Program Files\MetaTrader 5          (CMD)`)
		},
	}
}

// ── template ─────────────────────────────────────────────────────────────────

func templateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "template",
		Short: "Generate a batch.json template",
		Long: `Generate a batch.json template showing all features:
- defaults section (set symbol/period/dates once)
- multiple EAs
- multiple symbols
- multiple timeframes
- per-job inputs
- per-job profile (broker)`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := os.MkdirAll(workDir, 0755); err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			path := batch.ExampleBatchFilePath(workDir)
			if err := batch.WriteBatchTemplate(path); err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("%s Template written: %s\n\n", green("✓"), path)
			fmt.Printf("Edit it, then run:\n")
			fmt.Printf("  mt5-backtester-pp-cli batch --file %s\n", path)
		},
	}
}

// ── setfile ───────────────────────────────────────────────────────────────────

func setfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setfile",
		Short: "Inspect, generate, and list MT5 .set files",
		Long: `Work with MetaTrader 5 strategy tester .set files.

Set files live under <MT5 data dir>\MQL5\Profiles\Tester\. They store EA
input parameters plus optimization ranges in the pipe-delimited format
MT5 uses internally:

  StopLoss=50||50||1||200||N
  TakeProfit=100||100||1||500||Y
  MagicNumber=99001
  UseFilter=true

The trailing Y/N marks whether the parameter is swept during optimization.
Lines without || are simple values with no optimization range.

Commands:
  setfile show <file.set>       — pretty-print params and optimization ranges
  setfile export --output FILE  — generate a .set file from --input flags
  setfile list                  — list .set files in MQL5\Profiles\Tester\`,
	}

	cmd.AddCommand(setfileShowCmd())
	cmd.AddCommand(setfileExportCmd())
	cmd.AddCommand(setfileListCmd())
	return cmd
}

func setfileShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <file.set>",
		Short: "Pretty-print all params and optimization ranges in a .set file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			termPath, isPortable, _ := resolveTerminal("")
			path, err := setfile.ResolvePath(args[0], termPath, isPortable)
			if err != nil {
				return err
			}
			sf, err := setfile.Parse(path)
			if err != nil {
				return err
			}

			fmt.Printf("\n%s\n", bold("Set file"))
			fmt.Printf("Path:  %s\n", cyan(path))
			fmt.Printf("Count: %d parameter(s)\n\n", len(sf.Params))

			fixed, ranged := 0, 0
			for _, p := range sf.Params {
				if p.HasRange {
					ranged++
				} else {
					fixed++
				}
			}
			fmt.Printf("Fixed values: %d   Optimization-aware: %d\n\n", fixed, ranged)

			for _, p := range sf.Params {
				if !p.HasRange {
					fmt.Printf("  %-30s = %s\n", p.Name, p.Value)
					continue
				}
				flag := red("OFF")
				if p.OptOn {
					flag = green("ON ")
				}
				fmt.Printf("  %-30s = %-12s  range[%s → %s step %s]  opt=%s\n",
					p.Name, p.Value, p.OptStart, p.OptStop, p.OptStep, flag)
			}
			fmt.Println()
			return nil
		},
	}
}

func setfileExportCmd() *cobra.Command {
	var (
		outPath string
		inputs  []string
	)
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Generate a .set file from --input key=value flags",
		Long: `Generate a .set file from simple key=value inputs.

The output contains one line per input with no optimization range. Use
'setfile show' to verify, or edit the resulting file by hand to add
optimization ranges in the value||start||step||stop||Y/N form.

Example:
  mt5-backtester-pp-cli setfile export --output MyEA.set \
    --input StopLoss=50 --input TakeProfit=100 --input MagicNumber=99001`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if outPath == "" {
				return fmt.Errorf("--output is required")
			}
			if len(inputs) == 0 {
				return fmt.Errorf("at least one --input is required")
			}
			sf := setfile.FromInputs(parseInputs(inputs))
			if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
				return err
			}
			if err := sf.Write(outPath); err != nil {
				return err
			}
			fmt.Printf("%s Wrote %d param(s) to %s\n", green("✓"), len(sf.Params), outPath)
			return nil
		},
	}
	cmd.Flags().StringVarP(&outPath, "output", "O", "", "Output .set file path (required)")
	cmd.Flags().StringArrayVar(&inputs, "input", nil, "Input: --input StopLoss=50 (repeatable)")
	return cmd
}

func setfileListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all .set files under MQL5\\Profiles\\Tester\\",
		RunE: func(cmd *cobra.Command, args []string) error {
			termPath, isPortable, _ := resolveTerminal("")
			files := setfile.ListSetFiles(termPath, isPortable)
			dirs := setfile.TesterDirs(termPath, isPortable)

			fmt.Printf("\n%s\n", bold("Set files"))
			if len(dirs) == 0 {
				fmt.Println("No MQL5\\Profiles\\Tester\\ directories found.")
				return nil
			}
			fmt.Println("Searched:")
			for _, d := range dirs {
				fmt.Printf("  %s\n", d)
			}
			fmt.Println()

			if len(files) == 0 {
				fmt.Println("(no .set files found)")
				return nil
			}
			for _, f := range files {
				fmt.Printf("  %s\n", f)
			}
			fmt.Printf("\n%d file(s)\n", len(files))
			return nil
		},
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func parseInputs(inputs []string) map[string]string {
	m := make(map[string]string)
	for _, kv := range inputs {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			m[parts[0]] = parts[1]
		}
	}
	return m
}

func shutdownMode(noShutdown bool) int {
	if noShutdown {
		return 0
	}
	return 1
}

func findMostRecentReport(dir string) (string, error) {
	var newest string
	var newestTime time.Time
	for _, pattern := range []string{"*.htm", "*.html"} {
		matches, _ := filepath.Glob(filepath.Join(dir, pattern))
		for _, m := range matches {
			fi, err := os.Stat(m)
			if err != nil {
				continue
			}
			if fi.ModTime().After(newestTime) {
				newestTime = fi.ModTime()
				newest = m
			}
		}
	}
	if newest == "" {
		return "", fmt.Errorf("no .htm report files found in %s", dir)
	}
	return newest, nil
}

func printStats(stats *report.Stats) {
	switch outputFmt {
	case "json":
		report.PrintJSON(stats)
	case "csv":
		report.PrintCSVHeader()
		report.PrintCSVRow(stats)
	default:
		report.Print(stats, verbose)
	}
}
