// pp:data-source live
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"tabelog-pp-cli/internal/domain"
	"tabelog-pp-cli/internal/notebook"

	"github.com/spf13/cobra"
)

type listFactChange struct {
	Field         string `json:"field"`
	Previous      any    `json:"previous"`
	Current       any    `json:"current"`
	PreviousState string `json:"previous_state"`
	CurrentState  string `json:"current_state"`
}

type listRefreshItem struct {
	savedRestaurant
	RefreshStatus string           `json:"refresh_status"`
	Changes       []listFactChange `json:"changes"`
	AddedEvidence []listFactChange `json:"added_evidence"`
	Error         string           `json:"error,omitempty"`
}

var refreshFactFields = []string{
	"name", "url", "status", "source_status", "status_notice", "source_warnings", "rating", "review_count", "categories", "area", "nearest_station", "nearest_station_distance_m",
	"lunch_budget", "dinner_budget", "review_lunch_budget", "review_dinner_budget", "closures", "hours", "payment",
	"reservation", "address", "transportation", "service_charge", "facilities", "awards",
}

func listSnapshotChanges(previous, current domain.Restaurant) ([]listFactChange, []listFactChange, error) {
	beforeRaw, err := json.Marshal(previous)
	if err != nil {
		return nil, nil, err
	}
	afterRaw, err := json.Marshal(current)
	if err != nil {
		return nil, nil, err
	}
	var before, after map[string]any
	if err := json.Unmarshal(beforeRaw, &before); err != nil {
		return nil, nil, err
	}
	if err := json.Unmarshal(afterRaw, &after); err != nil {
		return nil, nil, err
	}
	changes, added := make([]listFactChange, 0), make([]listFactChange, 0)
	for _, field := range refreshFactFields {
		oldState, newState := savedFieldState(previous, field), savedFieldState(current, field)
		if oldState == newState && reflect.DeepEqual(before[field], after[field]) {
			continue
		}
		change := listFactChange{field, before[field], after[field], oldState, newState}
		if oldState == "detail_not_fetched" || (oldState != "known" && newState == "known") {
			added = append(added, change)
		} else {
			changes = append(changes, change)
		}
	}
	return changes, added, nil
}

func newTabelogListRefreshCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "refresh <list> [id...]", Short: "Refresh selected saved facts and preserve personal notes", Long: "Fetch current detail facts for up to 20 saved restaurants, at most two at a time within a 20-second operation deadline. Report changed source facts separately from newly obtained evidence. A failed record retains its last valid snapshot and note; any per-record failure gives a non-success exit. Use lists audit for a network-free evidence check.", Annotations: listWriteAnnotations()}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := listArgCount(flags, args, 1, -1); err != nil {
			return err
		}
		if flags.dataSource == "local" {
			return fmt.Errorf("no local data source for refresh; use lists audit or lists compare for saved evidence")
		}
		if len(args) > 0 {
			if err := notebook.ValidateName(args[0]); err != nil {
				return err
			}
		}
		selected := make([]string, 0, len(listIDs(args)))
		seen := map[string]bool{}
		for _, id := range listIDs(args) {
			if err := notebook.ValidateID(id); err != nil {
				return err
			}
			if !seen[id] {
				seen[id] = true
				selected = append(selected, id)
			}
		}
		if len(selected) > 20 {
			return fmt.Errorf("refresh accepts at most 20 selected saved restaurants; narrow the ID selection")
		}
		name := listName(args)
		if flags.dryRun {
			meta := domain.Meta{Source: "live", Coverage: "dry_run", Criteria: map[string]any{"list": name, "operation": "refresh", "ids": selected, "max_venues": 20, "concurrency": 2}, Note: "No storage or network access; saved membership and source detail lookups remain planned."}
			return tripPrint(cmd, flags, []map[string]any{{"operation": "refresh", "list": name, "ids": selected, "planned": true}}, meta)
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		ctx, deadlineCancel := context.WithTimeout(ctx, 20*time.Second)
		defer deadlineCancel()
		db, nb, err := tripOpen(ctx, flags)
		if err != nil {
			return err
		}
		defer db.Close()
		entries, err := nb.Entries(ctx, name, selected)
		if err != nil {
			return err
		}
		if len(entries) > 20 {
			return fmt.Errorf("saved list has %d candidates; select at most 20 IDs for refresh", len(entries))
		}
		previous, err := decodeSaved(entries, flags.maxAge)
		if err != nil {
			return err
		}
		type fetchedResult struct {
			restaurant domain.Restaurant
			err        error
			changes    []listFactChange
			added      []listFactChange
		}
		fetched := make([]fetchedResult, len(entries))
		jobs := make(chan int, len(entries))
		for i := range entries {
			jobs <- i
		}
		close(jobs)
		var calls atomic.Int64
		var workers sync.WaitGroup
		workerCount := 2
		if len(entries) < workerCount {
			workerCount = len(entries)
		}
		for worker := 0; worker < workerCount; worker++ {
			workers.Add(1)
			go func() {
				defer workers.Done()
				liveFlags := *flags
				liveFlags.dataSource = "live"
				for i := range jobs {
					if err := ctx.Err(); err != nil {
						fetched[i].err = err
						continue
					}
					calls.Add(1)
					r, err := tripDetail(ctx, &liveFlags, previous[i].URL)
					if err == nil && r.ID != previous[i].ID {
						err = fmt.Errorf("source restaurant identity changed from %s to %s", previous[i].ID, r.ID)
					}
					var changes, added []listFactChange
					if err == nil {
						changes, added, err = listSnapshotChanges(previous[i].Restaurant, r)
					}
					if err == nil {
						err = tripReplace(ctx, nb, r)
					}
					fetched[i] = fetchedResult{r, err, changes, added}
				}
			}()
		}
		workers.Wait()
		items := make([]listRefreshItem, 0, len(entries))
		failed := 0
		for i, entry := range entries {
			result := listRefreshItem{savedRestaurant: previous[i], RefreshStatus: "error", Changes: make([]listFactChange, 0), AddedEvidence: make([]listFactChange, 0)}
			err := fetched[i].err
			if err == nil {
				result.savedRestaurant = savedView(entry, fetched[i].restaurant, flags.maxAge)
				result.RefreshStatus = "ok"
				result.Changes = fetched[i].changes
				result.AddedEvidence = fetched[i].added
			}
			if err != nil {
				failed++
				result.Error = err.Error()
			}
			items = append(items, result)
		}
		meta := domain.Meta{Source: "live", Coverage: "selected_saved_candidates", Returned: len(items), Scanned: len(entries), Requests: int(calls.Load()), PartialFailure: failed > 0, Failed: failed, Criteria: map[string]any{"list": name, "operation": "refresh", "ids": selected, "max_venues": 20, "concurrency": 2}, Note: "Changes compare the same source fields; added_evidence records previously unavailable detail facts. Failed records retain their previous snapshots."}
		if err := tripPrint(cmd, flags, items, meta); err != nil {
			return err
		}
		if failed > 0 {
			return fmt.Errorf("refresh failed for %d of %d saved restaurants; last valid snapshots retained", failed, len(items))
		}
		return nil
	}
	return cmd
}
