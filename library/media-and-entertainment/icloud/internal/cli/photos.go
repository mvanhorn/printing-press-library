// Copyright 2026 Matias Sanchez Moises and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/spf13/cobra"

func newPhotosCmd(f *rootFlags) *cobra.Command {
	photos := &cobra.Command{
		Use:   "photos",
		Short: "Query and manage your Photos library",
		Long: `Read your macOS Photos library directly — no Photos.app launch required
for read operations.

Read commands use Photos.sqlite in read-only mode.
The delete command requires Photos.app and moves items to Recently Deleted.`,
	}

	photos.AddCommand(newTopCmd(f))
	photos.AddCommand(newVideosCmd(f))
	photos.AddCommand(newStorageCmd(f))
	photos.AddCommand(newStatsCmd(f))
	photos.AddCommand(newDeleteCmd(f))
	photos.AddCommand(newDownloadCmd(f))

	return photos
}
