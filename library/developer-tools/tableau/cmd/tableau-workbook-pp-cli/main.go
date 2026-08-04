package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/tableau/internal/cmdutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/tableau/internal/twb"
	"github.com/spf13/cobra"
)

const version = "0.3.0"

func main() {
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitCodeFor(err))
	}
}

type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }

func exitCodeFor(err error) int {
	if err == nil {
		return cmdutil.ExitOK
	}
	if ee, ok := err.(*exitError); ok {
		return ee.code
	}
	// Usage / flag errors from cobra.
	return cmdutil.ExitUsage
}

func fail(code int, format string, args ...any) error {
	return &exitError{code: code, msg: fmt.Sprintf(format, args...)}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "tableau-workbook-pp-cli",
		Short:         "Parse, lint, and safely mutate Tableau .twb / .twbx workbooks",
		Long:          "Agent-safe Tableau workbook compiler: parse → structured ops → lint → write. Prevents illegal .twb XML (e.g. invented enums like bold).",
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.SetVersionTemplate("{{.Version}}\n")

	root.AddCommand(newWorkbookCmd())
	root.AddCommand(newCalcCmd())
	root.AddCommand(newSheetCmd())
	root.AddCommand(newStyleCmd())
	root.AddCommand(newDashboardCmd())
	root.AddCommand(newOpenCheckCmd())
	return root
}

func openWB(path string) (*twb.Workbook, error) {
	wb, err := twb.Open(path)
	if err != nil {
		return nil, fail(cmdutil.ExitIO, "%v", err)
	}
	return wb, nil
}

func writeMutation(wb *twb.Workbook, input, output string, inPlace, dryRun bool) error {
	out, err := cmdutil.ResolveOutputPath(input, output, inPlace)
	if err != nil {
		return fail(cmdutil.ExitUsage, "%v", err)
	}
	// Gate: never write illegal structure (Ann fix — validate before Desktop).
	if issues := wb.Lint(); twb.HasErrors(issues) {
		for _, iss := range issues {
			loc := iss.Path
			if loc != "" {
				loc = " @ " + loc
			}
			fmt.Fprintf(os.Stderr, "%s [%s]%s: %s\n", iss.Severity, iss.Code, loc, iss.Message)
		}
		return fail(cmdutil.ExitValidation, "refusing to write: lint failed with %d issue(s)", len(issues))
	}
	if dryRun {
		fmt.Fprintf(os.Stderr, "dry-run: would write %s\n", out)
		return nil
	}
	if err := wb.Write(out); err != nil {
		return fail(cmdutil.ExitIO, "%v", err)
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", out)
	return nil
}

// --- workbook ---

func newWorkbookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workbook",
		Short: "Inspect, lint, and validate workbooks",
	}
	cmd.AddCommand(newWorkbookInspectCmd())
	cmd.AddCommand(newWorkbookLintCmd())
	validate := newWorkbookLintCmd()
	validate.Use = "validate"
	validate.Short = "Alias for workbook lint"
	validate.Aliases = nil
	cmd.AddCommand(validate)
	return cmd
}

func newWorkbookInspectCmd() *cobra.Command {
	var asJSON, compact bool
	cmd := &cobra.Command{
		Use:   "inspect <path>",
		Short: "Summarize workbook structure (sheets, dashboards, zones, calcs)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			wb, err := openWB(args[0])
			if err != nil {
				return err
			}
			sum := wb.Inspect()
			if asJSON || compact {
				enc := json.NewEncoder(os.Stdout)
				if !compact {
					enc.SetIndent("", "  ")
				}
				return enc.Encode(sum)
			}
			fmt.Printf("path: %s\n", sum.Path)
			fmt.Printf("sheets: %d\n", sum.SheetCount)
			for _, s := range sum.Sheets {
				fmt.Printf("  - %s\n", s)
			}
			fmt.Printf("dashboards: %d\n", sum.DashboardCount)
			for _, d := range sum.Dashboards {
				fmt.Printf("  - %s\n", d)
			}
			fmt.Printf("zones: %d\n", sum.ZoneCount)
			fmt.Printf("datasources: %d\n", sum.DatasourceCount)
			for _, d := range sum.Datasources {
				fmt.Printf("  - %s\n", d)
			}
			fmt.Printf("calcs: %d\n", sum.CalcCount)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	cmd.Flags().BoolVar(&compact, "compact", false, "compact JSON")
	return cmd
}

func newWorkbookLintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lint <path>",
		Short: "Lint workbook for illegal or broken XML patterns",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			wb, err := openWB(args[0])
			if err != nil {
				return err
			}
			issues := wb.Lint()
			if len(issues) == 0 {
				fmt.Println("ok: no issues")
				return nil
			}
			for _, iss := range issues {
				loc := iss.Path
				if loc != "" {
					loc = " @ " + loc
				}
				fmt.Fprintf(os.Stderr, "%s [%s]%s: %s\n", iss.Severity, iss.Code, loc, iss.Message)
			}
			if twb.HasErrors(issues) {
				return fail(cmdutil.ExitValidation, "lint failed with %d issue(s)", len(issues))
			}
			return nil
		},
	}
}

// --- calc ---

func newCalcCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "calc",
		Short: "List and add calculated fields",
	}
	cmd.AddCommand(newCalcListCmd())
	cmd.AddCommand(newCalcAddCmd())
	cmd.AddCommand(newCalcAddBatchCmd())
	cmd.AddCommand(newCalcPackYoYCmd())
	return cmd
}

func newCalcListCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "list <path>",
		Short: "List calculated fields",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			wb, err := openWB(args[0])
			if err != nil {
				return err
			}
			calcs := wb.ListCalcs()
			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(calcs)
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "CAPTION\tDATATYPE\tROLE\tFORMULA\tSOURCE")
			for _, c := range calcs {
				caption := c.Caption
				if caption == "" {
					caption = c.Name
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", caption, c.Datatype, c.Role, c.Formula, c.Source)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	return cmd
}

func newCalcAddCmd() *cobra.Command {
	var caption, formula, datatype, role, output string
	var inPlace, dryRun bool
	cmd := &cobra.Command{
		Use:   "add <path>",
		Short: "Add a calculated field",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			wb, err := openWB(args[0])
			if err != nil {
				return err
			}
			if err := wb.AddCalc(caption, formula, datatype, role); err != nil {
				return fail(cmdutil.ExitUsage, "%v", err)
			}
			return writeMutation(wb, args[0], output, inPlace, dryRun)
		},
	}
	cmd.Flags().StringVar(&caption, "caption", "", "field caption (required)")
	cmd.Flags().StringVar(&formula, "formula", "", "Tableau formula (required)")
	cmd.Flags().StringVar(&datatype, "datatype", "real", "datatype (real, integer, string, ...)")
	cmd.Flags().StringVar(&role, "role", "measure", "role (measure or dimension)")
	cmd.Flags().StringVar(&output, "output", "", "output .twb path")
	cmd.Flags().BoolVar(&inPlace, "in-place", false, "overwrite input file")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "mutate in memory only; do not write")
	_ = cmd.MarkFlagRequired("caption")
	_ = cmd.MarkFlagRequired("formula")
	return cmd
}

func newCalcAddBatchCmd() *cobra.Command {
	var file, output string
	var inPlace, dryRun bool
	cmd := &cobra.Command{
		Use:   "add-batch <path>",
		Short: "Add many calculated fields from a JSON array file (Ann-style bulk calcs)",
		Long:  `JSON file must be an array of objects: [{"caption":"...","formula":"...","datatype":"real","role":"measure"}, ...]`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := os.ReadFile(file)
			if err != nil {
				return fail(cmdutil.ExitIO, "read batch file: %v", err)
			}
			var specs []twb.CalcSpec
			if err := json.Unmarshal(raw, &specs); err != nil {
				return fail(cmdutil.ExitUsage, "parse batch JSON: %v", err)
			}
			wb, err := openWB(args[0])
			if err != nil {
				return err
			}
			if err := wb.AddCalcs(specs); err != nil {
				return fail(cmdutil.ExitUsage, "%v", err)
			}
			fmt.Fprintf(os.Stderr, "added %d calculated field(s)\n", len(specs))
			return writeMutation(wb, args[0], output, inPlace, dryRun)
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "path to JSON array of calc specs (required)")
	cmd.Flags().StringVar(&output, "output", "", "output .twb path")
	cmd.Flags().BoolVar(&inPlace, "in-place", false, "overwrite input file")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "mutate in memory only; do not write")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}

func newCalcPackYoYCmd() *cobra.Command {
	var measures, dateField, output string
	var cyYear, pyYear int
	var inPlace, dryRun bool
	cmd := &cobra.Command{
		Use:   "pack-yoy <path>",
		Short: "Add Ann-style CY/PY/Delta/YoY% calc pack for measures",
		Long:  "For each measure, adds four calculated fields: CY, PY, Delta, YoY %. Matches the bulk-calc path from Ann Jackson's experiment.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var ms []string
			for _, p := range strings.Split(measures, ",") {
				p = strings.TrimSpace(p)
				if p != "" {
					ms = append(ms, p)
				}
			}
			if len(ms) == 0 {
				return fail(cmdutil.ExitUsage, "--measures is required (comma-separated field names)")
			}
			specs := twb.BuildYoYPack(ms, dateField, cyYear, pyYear)
			wb, err := openWB(args[0])
			if err != nil {
				return err
			}
			if err := wb.AddCalcs(specs); err != nil {
				return fail(cmdutil.ExitUsage, "%v", err)
			}
			fmt.Fprintf(os.Stderr, "added %d calculated field(s) (YoY pack)\n", len(specs))
			return writeMutation(wb, args[0], output, inPlace, dryRun)
		},
	}
	cmd.Flags().StringVar(&measures, "measures", "", "comma-separated measure field names, e.g. Sales,Profit,Quantity (required)")
	cmd.Flags().StringVar(&dateField, "date-field", "Order Date", "date field for YEAR() split")
	cmd.Flags().IntVar(&cyYear, "cy-year", 2017, "current year for CY calcs (Ann demo used 2017)")
	cmd.Flags().IntVar(&pyYear, "py-year", 2016, "prior year for PY calcs")
	cmd.Flags().StringVar(&output, "output", "", "output .twb path")
	cmd.Flags().BoolVar(&inPlace, "in-place", false, "overwrite input file")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "mutate in memory only; do not write")
	_ = cmd.MarkFlagRequired("measures")
	return cmd
}

// --- sheet ---

func newSheetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sheet",
		Short: "List and clone worksheets",
	}
	cmd.AddCommand(newSheetListCmd())
	cmd.AddCommand(newSheetCloneCmd())
	return cmd
}

func newSheetListCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "list <path>",
		Short: "List worksheets",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			wb, err := openWB(args[0])
			if err != nil {
				return err
			}
			sheets := wb.ListSheets()
			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(sheets)
			}
			for _, s := range sheets {
				fmt.Println(s)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	return cmd
}

func newSheetCloneCmd() *cobra.Command {
	var from, to, output string
	var inPlace, dryRun bool
	cmd := &cobra.Command{
		Use:   "clone <path>",
		Short: "Deep-clone a worksheet under a new name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			wb, err := openWB(args[0])
			if err != nil {
				return err
			}
			if err := wb.CloneSheet(from, to); err != nil {
				return fail(cmdutil.ExitUsage, "%v", err)
			}
			return writeMutation(wb, args[0], output, inPlace, dryRun)
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "source sheet name (required)")
	cmd.Flags().StringVar(&to, "to", "", "new sheet name (required)")
	cmd.Flags().StringVar(&output, "output", "", "output .twb path")
	cmd.Flags().BoolVar(&inPlace, "in-place", false, "overwrite input file")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "mutate in memory only; do not write")
	_ = cmd.MarkFlagRequired("from")
	_ = cmd.MarkFlagRequired("to")
	return cmd
}

// --- style ---

func newStyleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "style",
		Short: "Copy style subtrees between worksheets",
	}
	cmd.AddCommand(newStyleApplyCmd())
	return cmd
}

func newStyleApplyCmd() *cobra.Command {
	var from, to, output string
	var inPlace, dryRun bool
	cmd := &cobra.Command{
		Use:   "apply <path>",
		Short: "Copy style-ish XML from one worksheet onto another",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			wb, err := openWB(args[0])
			if err != nil {
				return err
			}
			if err := wb.ApplyStyle(from, to); err != nil {
				return fail(cmdutil.ExitUsage, "%v", err)
			}
			return writeMutation(wb, args[0], output, inPlace, dryRun)
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "source sheet (required)")
	cmd.Flags().StringVar(&to, "to", "", "destination sheet (required)")
	cmd.Flags().StringVar(&output, "output", "", "output .twb path")
	cmd.Flags().BoolVar(&inPlace, "in-place", false, "overwrite input file")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "mutate in memory only; do not write")
	_ = cmd.MarkFlagRequired("from")
	_ = cmd.MarkFlagRequired("to")
	return cmd
}

// --- dashboard ---

func newDashboardCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "List dashboards and scaffold known-good layouts only",
	}
	cmd.AddCommand(newDashboardListCmd())
	cmd.AddCommand(newDashboardTemplatesCmd())
	cmd.AddCommand(newDashboardScaffoldCmd())
	cmd.AddCommand(newDashboardCloneCmd())
	return cmd
}

func newDashboardListCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "list <path>",
		Short: "List dashboards and zone counts",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			wb, err := openWB(args[0])
			if err != nil {
				return err
			}
			dashboards := wb.ListDashboards()
			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(dashboards)
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "NAME	ZONES")
			for _, d := range dashboards {
				fmt.Fprintf(tw, "%s	%d\n", d.Name, d.ZoneCount)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	return cmd
}

func newDashboardTemplatesCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "templates",
		Short: "List built-in known-good dashboard templates (no freeform zones)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			tpls := twb.ListDashboardTemplates()
			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(tpls)
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "ID	SHEETS	NAME	DESCRIPTION")
			for _, t := range tpls {
				fmt.Fprintf(tw, "%s	%d-%d	%s	%s\n", t.ID, t.MinSheets, t.MaxSheets, t.Name, t.Description)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	return cmd
}

func newDashboardScaffoldCmd() *cobra.Command {
	var name, template, sheetsCSV, output string
	var inPlace, dryRun bool
	cmd := &cobra.Command{
		Use:   "scaffold <path>",
		Short: "Add a dashboard from a known-good template only (never freeform XML)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var sheetNames []string
			for _, p := range strings.Split(sheetsCSV, ",") {
				p = strings.TrimSpace(p)
				if p != "" {
					sheetNames = append(sheetNames, p)
				}
			}
			wb, err := openWB(args[0])
			if err != nil {
				return err
			}
			if err := wb.ScaffoldDashboard(name, template, sheetNames); err != nil {
				return fail(cmdutil.ExitUsage, "%v", err)
			}
			return writeMutation(wb, args[0], output, inPlace, dryRun)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "new dashboard name (required)")
	cmd.Flags().StringVar(&template, "template", "", "template id: single | two-pane | three-row | quad (required)")
	cmd.Flags().StringVar(&sheetsCSV, "sheets", "", "comma-separated existing sheet names (required)")
	cmd.Flags().StringVar(&output, "output", "", "output .twb path")
	cmd.Flags().BoolVar(&inPlace, "in-place", false, "overwrite input file")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "mutate in memory only; do not write")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("template")
	_ = cmd.MarkFlagRequired("sheets")
	return cmd
}

func newDashboardCloneCmd() *cobra.Command {
	var from, to, output string
	var inPlace, dryRun bool
	cmd := &cobra.Command{
		Use:   "clone <path>",
		Short: "Clone an existing dashboard (copy-only; safe mimic path)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			wb, err := openWB(args[0])
			if err != nil {
				return err
			}
			if err := wb.CloneDashboard(from, to); err != nil {
				return fail(cmdutil.ExitUsage, "%v", err)
			}
			return writeMutation(wb, args[0], output, inPlace, dryRun)
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "source dashboard name (required)")
	cmd.Flags().StringVar(&to, "to", "", "new dashboard name (required)")
	cmd.Flags().StringVar(&output, "output", "", "output .twb path")
	cmd.Flags().BoolVar(&inPlace, "in-place", false, "overwrite input file")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "mutate in memory only; do not write")
	_ = cmd.MarkFlagRequired("from")
	_ = cmd.MarkFlagRequired("to")
	return cmd
}

// --- open-check ---

func newOpenCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open-check <path>",
		Short: "Lint workbook and report Tableau Desktop availability for open validation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			wb, err := openWB(args[0])
			if err != nil {
				return err
			}
			issues := wb.Lint()
			if len(issues) == 0 {
				fmt.Println("lint: ok")
			} else {
				for _, iss := range issues {
					loc := iss.Path
					if loc != "" {
						loc = " @ " + loc
					}
					fmt.Fprintf(os.Stderr, "%s [%s]%s: %s\n", iss.Severity, iss.Code, loc, iss.Message)
				}
			}
			desktop := findTableauDesktop()
			if desktop == "" {
				fmt.Println("desktop: not found (install Tableau Desktop for true open-check; offline lint still applies)")
			} else {
				fmt.Printf("desktop: found %s\n", desktop)
				fmt.Println("desktop: auto-open not enabled in v0.1 (lint is the agent gate); open manually to confirm")
			}
			if twb.HasErrors(issues) {
				return fail(cmdutil.ExitValidation, "open-check failed: lint errors present")
			}
			return nil
		},
	}
}

func findTableauDesktop() string {
	candidates := []string{
		"/Applications/Tableau Desktop 2025.1.app",
		"/Applications/Tableau Desktop 2024.3.app",
		"/Applications/Tableau Desktop 2024.2.app",
		"/Applications/Tableau Desktop 2024.1.app",
		"/Applications/Tableau Desktop.app",
		"/Applications/Tableau Public.app",
	}
	// Also scan /Applications for Tableau*.app
	if ents, err := os.ReadDir("/Applications"); err == nil {
		for _, e := range ents {
			n := e.Name()
			if strings.HasPrefix(n, "Tableau") && strings.HasSuffix(n, ".app") {
				candidates = append([]string{"/Applications/" + n}, candidates...)
			}
		}
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return c
		}
	}
	return ""
}
