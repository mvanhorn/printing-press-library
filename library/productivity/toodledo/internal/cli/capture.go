// Copyright 2026 wwilson1017 and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Novel feature: batch capture. Reads task titles (one per line) from a file or
// stdin and creates them via the tasks=<JSON> batch param in chunks of 50 (the
// per-call limit), resolving folder/context names to ids. Prints by default;
// --apply performs the writes.

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/toodledo/internal/cliutil"
	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelCaptureCmd(flags *rootFlags) *cobra.Command {
	var flagFile, flagFolder, flagContext, flagPriority, flagStatus, flagDue, dbPath string
	var flagStar, apply bool
	cmd := &cobra.Command{
		Use:   "capture",
		Short: "Bulk-add tasks from a file or stdin (one title per line), in batches of 50",
		Long: `Read task titles (one per line) from a file or stdin and create them in
Toodledo, resolving --folder and --context names to ids and batching writes in
groups of 50 (the per-call limit) to stay under the rate budget.

Prints what it would create by default; pass --apply to actually create them.`,
		Example: strings.Trim(`
  toodledo-pp-cli capture --file ~/inbox.txt --folder Inbox
  printf 'call dentist\nbuy milk\n' | toodledo-pp-cli capture --context @errands --apply`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			titles, err := readCaptureTitles(cmd, flagFile)
			if err != nil {
				return err
			}
			if len(titles) == 0 {
				return usageErr(fmt.Errorf("no task titles provided (use --file <path> or pipe titles on stdin)"))
			}

			base := map[string]any{}
			if flagPriority != "" {
				p, ok := parsePriority(flagPriority)
				if !ok {
					return usageErr(fmt.Errorf("invalid --priority %q (use negative/low/medium/high/top)", flagPriority))
				}
				base["priority"] = p
			}
			if flagStatus != "" {
				s, ok := parseStatus(flagStatus)
				if !ok {
					return usageErr(fmt.Errorf("invalid --status %q", flagStatus))
				}
				base["status"] = s
			}
			if flagStar {
				base["star"] = 1
			}
			if flagDue != "" {
				due, err := parseDueDate(flagDue)
				if err != nil {
					return usageErr(err)
				}
				base["duedate"] = due
			}

			if flagFolder != "" || flagContext != "" {
				db, ok, err := openLocalMirror(cmd, toodledoDBPath(dbPath))
				if err != nil {
					return err
				}
				if !ok {
					return notFoundErr(fmt.Errorf("cannot resolve --folder/--context names without a local mirror; run 'toodledo-pp-cli sync' first, or pass numeric ids"))
				}
				defer db.Close()
				if flagFolder != "" {
					id, found, _ := resolveRefID(db, "folders", flagFolder)
					if !found {
						return usageErr(fmt.Errorf("folder %q not found. available: %s", flagFolder, strings.Join(availableNames(db, "folders"), ", ")))
					}
					base["folder"] = id
				}
				if flagContext != "" {
					id, found, _ := resolveRefID(db, "contexts", flagContext)
					if !found {
						return usageErr(fmt.Errorf("context %q not found. available: %s", flagContext, strings.Join(availableNames(db, "contexts"), ", ")))
					}
					base["context"] = id
				}
			}

			tasks := make([]map[string]any, 0, len(titles))
			for _, t := range titles {
				obj := map[string]any{"title": t}
				for k, v := range base {
					obj[k] = v
				}
				tasks = append(tasks, obj)
			}
			chunks := (len(tasks) + 49) / 50

			// Print by default; short-circuit under the verifier; require --apply to write.
			if !apply || cliutil.IsVerifyEnv() {
				w := cmd.OutOrStdout()
				if flags.asJSON || flags.agent {
					return printJSONFiltered(w, map[string]any{
						"would_create": len(tasks), "chunks": chunks, "applied": false, "tasks": tasks,
					}, flags)
				}
				fmt.Fprintf(w, "Would create %d task(s) in %d batch(es) of 50. Re-run with --apply to create them.\n", len(tasks), chunks)
				for i, t := range tasks {
					if i >= 10 {
						fmt.Fprintf(w, "  ... and %d more\n", len(tasks)-10)
						break
					}
					fmt.Fprintf(w, "  - %v\n", t["title"])
				}
				return nil
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			created := 0
			for i := 0; i < len(tasks); i += 50 {
				end := i + 50
				if end > len(tasks) {
					end = len(tasks)
				}
				batch := tasks[i:end]
				payload, mErr := json.Marshal(batch)
				if mErr != nil {
					return mErr
				}
				form := url.Values{"tasks": {string(payload)}}
				resp, status, postErr := c.PostForm(ctx, "/tasks/add.php", form)
				if postErr != nil {
					return classifyAPIError(fmt.Errorf("creating tasks (batch %d; %d already created): %w", i/50+1, created, postErr), flags)
				}
				if status >= 400 {
					return apiErr(fmt.Errorf("tasks/add.php returned HTTP %d (%d already created): %s", status, created, cliutil.SanitizeErrorBody(string(resp))))
				}
				var arr []json.RawMessage
				if json.Unmarshal(resp, &arr) == nil {
					created += len(arr)
				} else {
					created += len(batch)
				}
			}
			w := cmd.OutOrStdout()
			if flags.asJSON || flags.agent {
				return printJSONFiltered(w, map[string]any{"created": created, "chunks": chunks, "applied": true}, flags)
			}
			fmt.Fprintf(w, "Created %d task(s) in %d batch(es).\n", created, chunks)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagFile, "file", "", "File of task titles, one per line (default: read stdin)")
	cmd.Flags().StringVar(&flagFolder, "folder", "", "Folder (project) name or id to file every task under")
	cmd.Flags().StringVar(&flagContext, "context", "", "Context name or id to apply to every task")
	cmd.Flags().StringVar(&flagPriority, "priority", "", "Priority for every task (negative/low/medium/high/top)")
	cmd.Flags().StringVar(&flagStatus, "status", "", "GTD status for every task (e.g. next_action)")
	cmd.Flags().StringVar(&flagDue, "due", "", "Due date for every task (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&flagStar, "star", false, "Star every task")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually create the tasks (default: preview only)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local mirror path for name resolution")
	return cmd
}

func readCaptureTitles(cmd *cobra.Command, file string) ([]string, error) {
	var r io.Reader
	if strings.TrimSpace(file) != "" {
		f, err := os.Open(file) // #nosec G304 -- path is the user's own --file flag value; opening the named file is the command's purpose
		if err != nil {
			return nil, fmt.Errorf("opening --file: %w", err)
		}
		defer f.Close()
		r = f
	} else {
		r = cmd.InOrStdin()
	}
	var titles []string
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		titles = append(titles, line)
	}
	return titles, sc.Err()
}
