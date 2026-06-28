package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// doctorCheck is one health-check result.
type doctorCheck struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

// doctorReport is the doctor command's structured output.
type doctorReport struct {
	OK          bool          `json:"ok"`
	Version     string        `json:"version"`
	ProfilePath string        `json:"profile_path"`
	Checks      []doctorCheck `json:"checks"`
}

func newDoctorCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Verify the registry loads, the profile is usable, and paths are writable",
		Long:  "Doctor runs local health checks: the provider registry parses, the applicant profile (if any) loads and validates, and the profile directory is writable. No network calls.",
		RunE: func(cmd *cobra.Command, args []string) error {
			rep := doctorReport{Version: version, ProfilePath: f.resolvedProfilePath(), OK: true}
			add := func(name string, ok bool, detail string) {
				rep.Checks = append(rep.Checks, doctorCheck{Name: name, OK: ok, Detail: detail})
				if !ok {
					rep.OK = false
				}
			}

			// 1. Registry loads.
			reg, err := f.loadRegistry()
			if err != nil {
				add("registry", false, err.Error())
			} else {
				add("registry", true, fmt.Sprintf("%d providers loaded from %s", len(reg.Providers), reg.Source))
			}

			// 2. Profile present + valid (optional).
			ppath := f.resolvedProfilePath()
			if _, statErr := os.Stat(ppath); statErr != nil {
				add("profile", true, fmt.Sprintf("no profile yet at %s - run 'insurance-finder intake'", ppath))
			} else if p, lerr := f.loadProfile(); lerr != nil {
				add("profile", false, lerr.Error())
			} else {
				problems := p.Validate()
				if len(problems) == 0 {
					add("profile", true, fmt.Sprintf("profile at %s is complete", ppath))
				} else {
					add("profile", false, fmt.Sprintf("%d field(s) incomplete: %v", len(problems), problems))
				}
			}

			// 3. Profile directory writable.
			dir := filepath.Dir(ppath)
			if dir == "" {
				dir = "."
			}
			if writable(dir) {
				add("profile-dir-writable", true, dir)
			} else {
				add("profile-dir-writable", false, "cannot write to "+dir)
			}

			err = f.emit(cmd, func(w io.Writer) { renderDoctor(w, rep) }, rep)
			if err != nil {
				return err
			}
			if !rep.OK {
				return dataErr(fmt.Errorf("doctor found problems"))
			}
			return nil
		},
	}
}

func writable(dir string) bool {
	tmp, err := os.CreateTemp(dir, ".insfinder-write-*")
	if err != nil {
		return false
	}
	name := tmp.Name()
	tmp.Close()
	os.Remove(name)
	return true
}

func renderDoctor(w io.Writer, rep doctorReport) {
	status := green("OK")
	if !rep.OK {
		status = red("PROBLEMS FOUND")
	}
	fmt.Fprintf(w, "%s  insurance-finder %s\n", bold("Doctor:"), rep.Version)
	fmt.Fprintf(w, "Status: %s\n\n", status)
	for _, c := range rep.Checks {
		mark := green("ok ")
		if !c.OK {
			mark = red("FAIL")
		}
		fmt.Fprintf(w, "  [%s] %-22s %s\n", mark, c.Name, c.Detail)
	}
}
