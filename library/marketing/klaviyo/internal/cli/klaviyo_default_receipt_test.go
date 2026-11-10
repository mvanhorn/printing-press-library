// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
)

func TestTSECGatedCommandReceiptsAreOptIn(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want int
	}{
		{name: "default", want: 0},
		{name: "explicit opt in", args: []string{"--receipt"}, want: 1},
		{name: "explicit opt out", args: []string{"--receipt=false"}, want: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			raCLIEnvironment(t)
			args := append([]string{"whoami", "--client-profile", "tenant-a", "--json"}, test.args...)
			if _, err := runTSECCommand(t, nil, args...); err != nil {
				t.Fatal(err)
			}
			session, err := platform.PrepareProfileSource("tenant-a", "klaviyo-pp-cli", "test-source")
			if err != nil {
				t.Fatal(err)
			}
			if session.Paths.ReceiptsDir != filepath.Join(session.Paths.StateDir, "receipts") {
				t.Fatalf("receipts escaped profile state directory: %s", session.Paths.ReceiptsDir)
			}
			var receipts []string
			err = filepath.WalkDir(session.Paths.ReceiptsDir, func(filename string, entry fs.DirEntry, walkErr error) error {
				if os.IsNotExist(walkErr) {
					return nil
				}
				if walkErr != nil {
					return walkErr
				}
				if !entry.IsDir() && filepath.Ext(filename) == ".json" {
					receipts = append(receipts, filename)
				}
				return nil
			})
			if err != nil || len(receipts) != test.want {
				t.Fatalf("receipt files=%v error=%v want=%d", receipts, err, test.want)
			}
			for _, filename := range receipts {
				data, err := os.ReadFile(filename)
				if err != nil {
					t.Fatal(err)
				}
				var receipt platform.Receipt
				if err := json.Unmarshal(data, &receipt); err != nil {
					t.Fatal(err)
				}
				if receipt.Client.Profile != "tenant-a" || receipt.Client.Gate != platform.GateVerified || receipt.Status != "success" || receipt.ExitCode != 0 {
					t.Fatalf("requested receipt lost invocation outcome: %+v", receipt)
				}
				if info, err := os.Stat(filename); err != nil || info.Mode().Perm() != 0o600 {
					t.Fatalf("receipt is not private: %v %v", info, err)
				}
			}
		})
	}
}
