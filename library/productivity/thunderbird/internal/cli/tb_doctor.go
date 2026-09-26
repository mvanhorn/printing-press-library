// pp:data-source local

package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/store"
	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile"
	"github.com/spf13/cobra"
)

// addThunderbirdDoctorChecks adds the local profile and store checks; it
// never touches the network.
func addThunderbirdDoctorChecks(cmd *cobra.Command, flags *rootFlags, report map[string]any) {
	profileDir, err := resolveTBProfile(flags)
	switch {
	case errors.Is(err, tbprofile.ErrNoProfile):
		report["profile"] = fmt.Sprintf("ERROR no Thunderbird profile under %s; pass --profile or set %s", tbprofile.RootDir(), tbprofile.EnvProfile)
	case err != nil:
		report["profile"] = "ERROR " + err.Error()
	default:
		report["profile"] = "ok"
		report["profile_path"] = profileDir
		if tbprofile.LockPresent(profileDir) {
			report["profile_lock"] = "INFO locked (Thunderbird is running; reads use snapshots)"
		} else {
			report["profile_lock"] = "ok (Thunderbird not running)"
		}
		prefs, perr := tbprofile.ParsePrefs(filepath.Join(profileDir, "prefs.js"))
		if perr != nil {
			report["prefs"] = "ERROR " + perr.Error()
		} else {
			report["prefs"] = "ok"
			accounts := prefs.Accounts(profileDir)
			report["accounts"] = fmt.Sprintf("ok (%d accounts)", len(accounts))
			total, offline := 0, 0
			for _, a := range accounts {
				fs, _, ferr := tbprofile.DiscoverFolders(a.Server.Directory, a.Key, nil)
				if ferr != nil {
					report["folders"] = "ERROR " + ferr.Error()
					break
				}
				for _, f := range fs {
					total++
					if f.Offline {
						offline++
					}
				}
			}
			if _, set := report["folders"]; !set {
				if offline == total {
					report["folders"] = fmt.Sprintf("ok (%d folders, all stored offline)", total)
				} else {
					report["folders"] = fmt.Sprintf("INFO %d folders, %d stored offline, %d server-only (not searchable)", total, offline, total-offline)
				}
			}
		}
	}

	dbPath := defaultDBPath(tbCLIName)
	report["store_path"] = dbPath
	if _, serr := os.Stat(dbPath); os.IsNotExist(serr) {
		report["store"] = fmt.Sprintf("INFO not synced yet; run: %s sync", tbCLIName)
		return
	}
	db, derr := store.OpenReadOnlyContext(cmd.Context(), dbPath)
	if derr != nil {
		report["store"] = "ERROR " + derr.Error()
		return
	}
	defer db.Close()
	n, _ := db.Count("messages")
	if last := tbLastSync(db); !last.IsZero() {
		report["last_sync"] = tbFormatTime(last)
		report["store"] = fmt.Sprintf("ok (%d messages)", n)
	} else {
		report["store"] = fmt.Sprintf("INFO not synced yet; run: %s sync", tbCLIName)
	}
}
