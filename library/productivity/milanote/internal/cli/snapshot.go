// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/milanote/internal/milanote"
)

type boardSnapshot struct {
	ID       string                       `json:"id"`
	Board    string                       `json:"board"`
	Title    string                       `json:"title"`
	Name     string                       `json:"name,omitempty"`
	At       string                       `json:"at"`
	Elements map[string]*milanote.Element `json:"elements"`
}

type snapshotInfo struct {
	ID       string `json:"id"`
	Board    string `json:"board"`
	Title    string `json:"title"`
	Name     string `json:"name,omitempty"`
	At       string `json:"at"`
	Elements int    `json:"elements"`
}

func newNovelSnapshotCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "snapshot",
		Short:       "Save a board's state and see what changed since",
		Example:     "  milanote-pp-cli snapshot diff 1Ab2Cd3EfGh4iJ --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:parent-group": "true", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelSnapshotDiffCmd(flags))
	addNovelCommandIfAbsent(cmd, newSnapshotSaveCmd(flags))
	addNovelCommandIfAbsent(cmd, newSnapshotListCmd(flags))
	return cmd
}

var snapshotNameBad = regexp.MustCompile(`[^0-9A-Za-z_-]+`)

func snapshotDir(board string) (string, error) { return stateDir(filepath.Join("snapshots", board)) }

// captureBoard reads a board's own elements, leaving sub-boards' contents out.
func captureBoard(cmd *cobra.Command, flags *rootFlags, boardID string) (*boardSnapshot, error) {
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	c, err := mnClient(flags)
	if err != nil {
		return nil, err
	}
	payload, err := loadBoard(ctx, c, boardID)
	if err != nil {
		return nil, err
	}
	scoped := milanote.Scope(payload, boardID)
	snap := &boardSnapshot{Board: boardID, At: time.Now().UTC().Format(time.RFC3339), Elements: scoped.Elements}
	if el := scoped.Elements[boardID]; el != nil {
		snap.Title = el.Title()
	}
	return snap, nil
}

func newSnapshotSaveCmd(flags *rootFlags) *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:         "save <board>",
		Short:       "Save a board's current cards so you can compare later",
		Example:     "  milanote-pp-cli snapshot save 1Ab2Cd3EfGh4iJ --name before-rewrite",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "snapshot save")
			}
			boardID := boardRef(args[0])
			snap, err := captureBoard(cmd, flags, boardID)
			if err != nil {
				return err
			}
			snap.Name = snapshotNameBad.ReplaceAllString(name, "-")
			snap.ID = time.Now().Format("20060102-150405")
			if snap.Name != "" {
				snap.ID += "-" + snap.Name
			}
			dir, err := snapshotDir(boardID)
			if err != nil {
				return err
			}
			if err := writeJSONState(filepath.Join(dir, snap.ID+".json"), snap); err != nil {
				return err
			}
			count := len(snapElements(snap))
			info := snapshotInfo{ID: snap.ID, Board: boardID, Title: snap.Title, Name: snap.Name, At: snap.At, Elements: count}
			return emit(cmd, flags, info, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "%s saved snapshot %s of %q (%d elements)\n", green("✓"), snap.ID, snap.Title, count)
				return err
			})
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "A label to find this snapshot by")
	return cmd
}

func listSnapshots(board string) ([]snapshotInfo, error) {
	root, err := stateDir("snapshots")
	if err != nil {
		return nil, err
	}
	boards := []string{board}
	if board == "" {
		boards = nil
		entries, err := os.ReadDir(root)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() {
				boards = append(boards, e.Name())
			}
		}
	}
	out := []snapshotInfo{}
	for _, b := range boards {
		files, _ := filepath.Glob(filepath.Join(root, b, "*.json"))
		for _, f := range files {
			snap, err := readSnapshot(f)
			if err != nil {
				continue
			}
			out = append(out, snapshotInfo{ID: snap.ID, Board: snap.Board, Title: snap.Title, Name: snap.Name, At: snap.At, Elements: len(snapElements(snap))})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At < out[j].At })
	return out, nil
}

func readSnapshot(path string) (*boardSnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var snap boardSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("snapshot %s is damaged: %w", path, err)
	}
	return &snap, nil
}

func newSnapshotListCmd(flags *rootFlags) *cobra.Command {
	var board string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List saved snapshots",
		Example:     "  milanote-pp-cli snapshot list --board 1Ab2Cd3EfGh4iJ",
		Annotations: readAnnotations("local"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "snapshot list")
			}
			out, err := listSnapshots(boardRef(board))
			if err != nil {
				return err
			}
			return emit(cmd, flags, out, func(w io.Writer) error {
				if len(out) == 0 {
					_, err := fmt.Fprintln(w, "No snapshots yet.")
					return err
				}
				tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
				fmt.Fprintln(tw, "ID\tBOARD\tTITLE\tCARDS")
				for _, s := range out {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%d\n", s.ID, s.Board, s.Title, s.Elements)
				}
				return tw.Flush()
			})
		},
	}
	cmd.Flags().StringVar(&board, "board", "", "Only this board")
	return cmd
}

// resolveSnapshot finds a snapshot by id, or by unique part of its id or name.
func resolveSnapshot(board, ref string) (*boardSnapshot, error) {
	list, err := listSnapshots(board)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, notFoundErr(fmt.Errorf("no snapshots of board %s; run 'snapshot save %s' first", board, board))
	}
	dir, err := snapshotDir(board)
	if err != nil {
		return nil, err
	}
	if ref == "" || ref == "latest" {
		return readSnapshot(filepath.Join(dir, list[len(list)-1].ID+".json"))
	}
	var hits []snapshotInfo
	for _, s := range list {
		if s.ID == ref {
			hits = []snapshotInfo{s}
			break
		}
		if strings.Contains(s.ID, ref) {
			hits = append(hits, s)
		}
	}
	switch len(hits) {
	case 0:
		return nil, notFoundErr(fmt.Errorf("no snapshot matches %q", ref))
	case 1:
		return readSnapshot(filepath.Join(dir, hits[0].ID+".json"))
	}
	return nil, usageErr(errors.New("more than one snapshot matches " + ref + "; use the full id from 'snapshot list'"))
}
