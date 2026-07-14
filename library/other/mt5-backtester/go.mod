module github.com/mvanhorn/printing-press-library/library/other/mt5-backtester

// Internal packages:
//   internal/config   - INI generation, path detection
//   internal/compile  - metaeditor64.exe wrapper
//   internal/backtest - terminal64.exe runner
//   internal/report   - HTML report parser
//   internal/batch    - multi-job runner with defaults/expand
//   internal/profiles - named terminal profiles + service scripts

go 1.22

require (
	github.com/fatih/color v1.16.0
	github.com/spf13/cobra v1.8.0
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	golang.org/x/sys v0.18.0 // indirect
)
