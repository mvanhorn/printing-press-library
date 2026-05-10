package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// CompanyContext stores company-specific facts that calibrate all advice.
// Persisted at ~/.betriebsrat/context.json.
type CompanyContext struct {
	Employees   int      `json:"employees"`             // Anzahl Arbeitnehmer im Betrieb
	Sector      string   `json:"sector,omitempty"`      // Branche (IT, Handel, Industrie, etc.)
	Tariff      bool     `json:"tariff"`                // Tarifvertrag vorhanden?
	TariffName  string   `json:"tariff_name,omitempty"` // Name des Tarifvertrags
	BRSize      int      `json:"br_size"`               // Anzahl BR-Mitglieder
	ExistingBVs []string `json:"existing_bvs,omitempty"` // Themen mit bestehenden BVs
	Notes       string   `json:"notes,omitempty"`        // Freitext
}

// Thresholds summarises which §§ apply given employee count.
type contextThreshold struct {
	Employees int
	Applied   []string
	Missing   []string
}

func companyContextPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home dir: %w", err)
	}
	dir := filepath.Join(home, ".betriebsrat")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating state dir: %w", err)
	}
	return filepath.Join(dir, "context.json"), nil
}

func loadCompanyContext() (*CompanyContext, error) {
	p, err := companyContextPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var ctx CompanyContext
	if err := json.Unmarshal(data, &ctx); err != nil {
		return nil, fmt.Errorf("parsing context.json: %w", err)
	}
	return &ctx, nil
}

func saveCompanyContext(ctx *CompanyContext) error {
	p, err := companyContextPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(ctx, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

func thresholdsFor(n int) []string {
	var t []string
	if n >= 5 {
		t = append(t, "§ 1 BetrVG: BR-Gründung möglich")
	}
	if n >= 20 {
		t = append(t, "§ 111 BetrVG: Betriebsänderung — Unterrichtungs- und Beratungsrechte")
	}
	if n >= 100 {
		t = append(t, "§ 106 BetrVG: Wirtschaftsausschuss verpflichtend")
	}
	if n >= 200 {
		t = append(t, "§ 38 BetrVG: Vollfreistellung von BR-Mitgliedern")
	}
	if n >= 300 {
		t = append(t, "§ 112 BetrVG: Erzwingbarer Sozialplan auch bei <10% Belegschaft möglich")
	}
	return t
}

func newContextCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Store and display company profile for context-aware advice",
		Long: `Persists company-specific facts so all advice is calibrated to your situation.
Stored at ~/.betriebsrat/context.json.

The profile affects which BetrVG thresholds apply:
  <5 AN   — No BR possible
  ≥20 AN  — § 111 Betriebsänderung rights apply
  ≥100 AN — § 106 Wirtschaftsausschuss is mandatory
  ≥200 AN — § 38 full-time BR member release required`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
	}
	cmd.AddCommand(newContextSetCmd(flags))
	cmd.AddCommand(newContextShowCmd(flags))
	cmd.AddCommand(newContextResetCmd(flags))
	return cmd
}

func newContextSetCmd(flags *rootFlags) *cobra.Command {
	var employees int
	var sector string
	var tariff bool
	var tariffName string
	var brSize int
	var bvs string
	var notes string

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set company profile fields",
		Example: strings.Trim(`
  betriebsrat context set --employees 150 --sector IT --tariff --br-size 7
  betriebsrat context set --employees 350 --tariff --tariff-name "TV-L" --bvs "Homeoffice,Arbeitszeit"
  betriebsrat context set --employees 80`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCompanyContext()
			if err != nil {
				return err
			}
			if ctx == nil {
				ctx = &CompanyContext{}
			}

			if cmd.Flags().Changed("employees") {
				ctx.Employees = employees
			}
			if cmd.Flags().Changed("sector") {
				ctx.Sector = sector
			}
			if cmd.Flags().Changed("tariff") {
				ctx.Tariff = tariff
			}
			if cmd.Flags().Changed("tariff-name") {
				ctx.TariffName = tariffName
			}
			if cmd.Flags().Changed("br-size") {
				ctx.BRSize = brSize
			}
			if cmd.Flags().Changed("bvs") {
				if bvs == "" {
					ctx.ExistingBVs = nil
				} else {
					parts := strings.Split(bvs, ",")
					for i := range parts {
						parts[i] = strings.TrimSpace(parts[i])
					}
					ctx.ExistingBVs = parts
				}
			}
			if cmd.Flags().Changed("notes") {
				ctx.Notes = notes
			}

			if err := saveCompanyContext(ctx); err != nil {
				return fmt.Errorf("saving context: %w", err)
			}

			if flags.asJSON || flags.agent {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(ctx)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Kontext gespeichert.")
			return printContext(cmd, ctx)
		},
	}
	cmd.Flags().IntVar(&employees, "employees", 0, "Anzahl Arbeitnehmer im Betrieb")
	cmd.Flags().StringVar(&sector, "sector", "", "Branche (z.B. IT, Handel, Industrie, Pflege)")
	cmd.Flags().BoolVar(&tariff, "tariff", false, "Tarifvertrag vorhanden")
	cmd.Flags().StringVar(&tariffName, "tariff-name", "", "Name des Tarifvertrags (z.B. TV-L, TVöD, IG Metall)")
	cmd.Flags().IntVar(&brSize, "br-size", 0, "Anzahl Betriebsratsmitglieder")
	cmd.Flags().StringVar(&bvs, "bvs", "", "Komma-getrennte Themen mit bestehenden Betriebsvereinbarungen (z.B. Homeoffice,Arbeitszeit,Datenschutz)")
	cmd.Flags().StringVar(&notes, "notes", "", "Freitext-Notizen")
	return cmd
}

func newContextShowCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Display current company profile and applicable BetrVG thresholds",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCompanyContext()
			if err != nil {
				return err
			}
			if ctx == nil {
				fmt.Fprintln(cmd.OutOrStdout(), "Kein Kontext gespeichert. Starten Sie mit:")
				fmt.Fprintln(cmd.OutOrStdout(), "  betriebsrat context set --employees <n> ...")
				return nil
			}
			if flags.asJSON || flags.agent {
				type showResult struct {
					Profile    *CompanyContext `json:"profile"`
					Thresholds []string        `json:"applicable_thresholds"`
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(showResult{
					Profile:    ctx,
					Thresholds: thresholdsFor(ctx.Employees),
				})
			}
			return printContext(cmd, ctx)
		},
	}
}

func newContextResetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "reset",
		Short: "Delete stored company profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := companyContextPath()
			if err != nil {
				return err
			}
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Kontext gelöscht.")
			return nil
		},
	}
}

func printContext(cmd *cobra.Command, ctx *CompanyContext) error {
	w := cmd.OutOrStdout()
	fmt.Fprintln(w, "\nUnternehmensprofil:")
	if ctx.Employees > 0 {
		fmt.Fprintf(w, "  Arbeitnehmer:      %d\n", ctx.Employees)
	}
	if ctx.Sector != "" {
		fmt.Fprintf(w, "  Branche:           %s\n", ctx.Sector)
	}
	if ctx.Tariff {
		tv := "ja"
		if ctx.TariffName != "" {
			tv = ctx.TariffName
		}
		fmt.Fprintf(w, "  Tarifvertrag:      %s\n", tv)
	} else {
		fmt.Fprintln(w, "  Tarifvertrag:      nein")
	}
	if ctx.BRSize > 0 {
		fmt.Fprintf(w, "  BR-Größe:          %d Mitglieder\n", ctx.BRSize)
	}
	if len(ctx.ExistingBVs) > 0 {
		fmt.Fprintf(w, "  Bestehende BVs:    %s\n", strings.Join(ctx.ExistingBVs, ", "))
	}
	if ctx.Notes != "" {
		fmt.Fprintf(w, "  Notizen:           %s\n", ctx.Notes)
	}

	if ctx.Employees > 0 {
		thresholds := thresholdsFor(ctx.Employees)
		fmt.Fprintln(w, "\nAnwendbare BetrVG-Schwellenwerte:")
		for _, t := range thresholds {
			fmt.Fprintf(w, "  ✓ %s\n", t)
		}
		// Warn about thresholds NOT met
		if ctx.Employees < 20 {
			fmt.Fprintln(w, "  ✗ § 111 BetrVG gilt NICHT (weniger als 20 AN)")
		}
		if ctx.Employees < 100 {
			fmt.Fprintln(w, "  ✗ § 106 Wirtschaftsausschuss noch nicht verpflichtend")
		}
	}
	return nil
}
