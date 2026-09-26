// pp:data-source local

package cli

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/store"
	"github.com/spf13/cobra"
)

type tbStatsRow struct {
	Account        string `json:"account"`
	AccountName    string `json:"account_name"`
	Messages       int    `json:"messages"`
	Unread         int    `json:"unread"`
	Flagged        int    `json:"flagged"`
	Attachments    int    `json:"attachments"`
	SizeBytes      int64  `json:"size_bytes"`
	Folders        int    `json:"folders"`
	OfflineFolders int    `json:"offline_folders"`
	FirstMessage   string `json:"first_message"`
	LastMessage    string `json:"last_message"`
}

type tbStatsReport struct {
	Accounts []tbStatsRow `json:"accounts"`
	Overall  tbStatsRow   `json:"overall"`
	LastSync string       `json:"last_sync"`
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newTBStatsCmd(flags))
	})
}

func newTBStatsCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show per-account totals: messages, unread, flagged, folders, last message date",
		Long: `Summarise the synced store per account and overall: message, unread and
flagged counts, attachments, mailbox bytes, folder counts (and how many are
stored offline), and the first and last message dates.`,
		Example: strings.Trim(`
  thunderbird-pp-cli stats
  thunderbird-pp-cli stats --json --select overall`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "stats")
			}
			report := tbStatsReport{Accounts: make([]tbStatsRow, 0), Overall: tbStatsRow{Account: "all", AccountName: "all accounts"}}
			db, err := tbOpenStore(cmd)
			if err != nil {
				return err
			}
			if db != nil {
				defer db.Close()
				hintIfUnsynced(cmd, db, "messages")
				hintIfStale(cmd, db, "messages", flags.maxAge)
				if report, err = tbComputeStats(db); err != nil {
					return err
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			tw := newTabWriter(tbHumanOut(cmd))
			fmt.Fprintln(tw, "ACCOUNT\tMESSAGES\tUNREAD\tFLAGGED\tATTACHMENTS\tSIZE\tFOLDERS\tOFFLINE\tLAST MESSAGE")
			for _, r := range append(report.Accounts, report.Overall) {
				fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%d\t%s\t%d\t%d\t%s\n", r.AccountName, r.Messages, r.Unread, r.Flagged, r.Attachments,
					tbHumanBytes(r.SizeBytes), r.Folders, r.OfflineFolders, r.LastMessage)
			}
			return tw.Flush()
		},
	}
}

func tbComputeStats(db *store.Store) (tbStatsReport, error) {
	rep := tbStatsReport{Accounts: make([]tbStatsRow, 0), Overall: tbStatsRow{Account: "all", AccountName: "all accounts"}}
	byAcc := map[string]*tbStatsRow{}
	var order []string
	get := func(key string) *tbStatsRow {
		if r, ok := byAcc[key]; ok {
			return r
		}
		r := &tbStatsRow{Account: key, AccountName: key}
		byAcc[key] = r
		order = append(order, key)
		return r
	}
	accounts, err := tbLoadDocs[tbAccountRow](db, "accounts")
	if err != nil {
		return rep, err
	}
	for _, a := range accounts {
		get(a.ID).AccountName = a.Name
	}
	rows, err := db.DB().Query(`SELECT COALESCE(json_extract(data,'$.account'),''),
		COUNT(*),
		SUM(CASE WHEN json_extract(data,'$.read') THEN 0 ELSE 1 END),
		SUM(CASE WHEN json_extract(data,'$.flagged') THEN 1 ELSE 0 END),
		COALESCE(SUM(json_extract(data,'$.size_bytes')),0),
		MIN(NULLIF(json_extract(data,'$.date'),'')),
		MAX(NULLIF(json_extract(data,'$.date'),''))
		FROM resources WHERE resource_type = 'messages' GROUP BY 1`)
	if err != nil {
		return rep, err
	}
	for rows.Next() {
		var key string
		var n, unread, flagged int
		var size int64
		var first, last sql.NullString
		if err := rows.Scan(&key, &n, &unread, &flagged, &size, &first, &last); err != nil {
			_ = rows.Close()
			return rep, err
		}
		r := get(key)
		r.Messages, r.Unread, r.Flagged, r.SizeBytes, r.FirstMessage, r.LastMessage = n, unread, flagged, size, first.String, last.String
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return rep, err
	}
	if err := tbScanCounts(db, `SELECT COALESCE(json_extract(data,'$.account'),''), COUNT(*),
		SUM(CASE WHEN json_extract(data,'$.offline') THEN 1 ELSE 0 END)
		FROM resources WHERE resource_type = 'folders' GROUP BY 1`, func(key string, a, b int) {
		r := get(key)
		r.Folders, r.OfflineFolders = a, b
	}); err != nil {
		return rep, err
	}
	if err := tbScanCounts(db, `SELECT COALESCE(json_extract(data,'$.account'),''), COUNT(*), 0
		FROM resources WHERE resource_type = 'attachments' GROUP BY 1`, func(key string, a, _ int) {
		get(key).Attachments = a
	}); err != nil {
		return rep, err
	}
	o := &rep.Overall
	for _, key := range order {
		r := *byAcc[key]
		rep.Accounts = append(rep.Accounts, r)
		o.Messages += r.Messages
		o.Unread += r.Unread
		o.Flagged += r.Flagged
		o.Attachments += r.Attachments
		o.SizeBytes += r.SizeBytes
		o.Folders += r.Folders
		o.OfflineFolders += r.OfflineFolders
		if r.FirstMessage != "" && (o.FirstMessage == "" || r.FirstMessage < o.FirstMessage) {
			o.FirstMessage = r.FirstMessage
		}
		if r.LastMessage > o.LastMessage {
			o.LastMessage = r.LastMessage
		}
	}
	if t := tbLastSync(db); !t.IsZero() {
		rep.LastSync = tbFormatTime(t)
	}
	return rep, nil
}

func tbScanCounts(db *store.Store, query string, fn func(key string, a, b int)) error {
	rows, err := db.DB().Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var a, b int
		if err := rows.Scan(&key, &a, &b); err != nil {
			return err
		}
		fn(key, a, b)
	}
	return rows.Err()
}
