// Copyright 2026 Todd Dailey. Licensed under Apache-2.0. See LICENSE.

// Package mcp registers the Peloton CLI's read-side workflows as MCP
// tools. Unlike generator-emitted CLIs whose MCP layer is a generic
// HTTP-method/path dispatcher, this CLI's MCP just calls the typed
// methods on internal/client + internal/store and returns the same
// JSON shapes the CLI emits — so agent consumers see identical
// responses across the two transports.
//
// Auth and database paths are shared with the CLI:
//   ~/.config/peloton-pp-cli/config.toml          (bearer token)
//   ~/.local/share/peloton-pp-cli/peloton.db      (synced store)
//
// `peloton-pp-cli auth login` is intentionally NOT an MCP tool — Chrome
// spawning belongs at the user's desk, not inside an agent loop.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/mvanhorn/printing-press-library/library/other/peloton/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/peloton/internal/config"
	"github.com/mvanhorn/printing-press-library/library/other/peloton/internal/store"
)

// RegisterTools wires every public tool onto s. Add new read-side tools
// here; `sync` is the only mutating tool (writes to the local SQLite
// store), and even it doesn't change Peloton-side state.
func RegisterTools(s *server.MCPServer) {
	s.AddTool(
		mcplib.NewTool("me",
			mcplib.WithDescription("Print the cached Peloton identity (user_id, username, token age) without spawning Chrome."),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(false),
		),
		handleMe,
	)
	s.AddTool(
		mcplib.NewTool("workouts_list",
			mcplib.WithDescription("List recent Peloton workouts, newest-first. Returns id, ride_id, workout_date, title, instructor, duration_seconds, total_output_kj, calories, and HR fields per workout."),
			mcplib.WithNumber("limit", mcplib.Description("Max workouts to return (default 50)")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
		),
		handleWorkoutsList,
	)
	s.AddTool(
		mcplib.NewTool("workouts_show",
			mcplib.WithDescription("Show one Peloton workout by id with the same shape as a workouts_list element."),
			mcplib.WithString("id", mcplib.Required(), mcplib.Description("Workout id (32-char hex from workouts_list)")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
		),
		handleWorkoutsShow,
	)
	s.AddTool(
		mcplib.NewTool("ride_show",
			mcplib.WithDescription("Show ride metadata + playlist (song order, artists, liked-flag, start_time_offset). Pair with workouts_show: the workout's ride_id is the input here."),
			mcplib.WithString("ride_id", mcplib.Required(), mcplib.Description("Ride id (32-char hex from a workout)")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
		),
		handleRideShow,
	)
	s.AddTool(
		mcplib.NewTool("discoveries",
			mcplib.WithDescription("List in-class songs you liked across recent rides, deduped by song id with a times_played counter."),
			mcplib.WithNumber("limit", mcplib.Description("How many recent workouts to scan (default 30)")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
		),
		handleDiscoveries,
	)
	s.AddTool(
		mcplib.NewTool("search",
			mcplib.WithDescription("Full-text (FTS5) search over the synced local store: workouts (title + instructor) and songs (title + artists + album), interleaved by bm25. Run sync first to populate."),
			mcplib.WithString("query", mcplib.Required(), mcplib.Description("FTS5 query — phrases (\"low impact\"), prefixes (cure*), and NEAR( ) all work")),
			mcplib.WithNumber("limit", mcplib.Description("Max hits to return (default 20)")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(false),
		),
		handleSearch,
	)
	s.AddTool(
		mcplib.NewTool("sync",
			mcplib.WithDescription("Mirror new Peloton workouts (and optional ride playlists) into the local SQLite store. Incremental by default; full=true disables the known-ids early-stop. hydrate_rides=false skips playlist fetches."),
			mcplib.WithNumber("limit", mcplib.Description("Max workouts to scan in this run (default 200)")),
			mcplib.WithBoolean("full", mcplib.Description("Disable the known-ids early-stop; walk every page up to limit (default false)")),
			mcplib.WithBoolean("hydrate_rides", mcplib.Description("Backfill ride playlists for rides not yet hydrated (default true)")),
			mcplib.WithReadOnlyHintAnnotation(false),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(false),
		),
		handleSync,
	)
}

// loadAuthedClient resolves the saved bearer token, ensures a usable
// user_id (re-fetching /api/me if the cached one was harvested before
// the auth0|<id> prefix-strip), and returns a ready Client + the cfg.
//
// Centralizing this shrinks every handler to "load → call → marshal".
func loadAuthedClient(ctx context.Context) (*client.Client, *config.Config, error) {
	cfg, err := config.Load("")
	if err != nil {
		return nil, nil, fmt.Errorf("load config: %w", err)
	}
	if cfg.Token == "" {
		return nil, nil, fmt.Errorf("no Peloton token saved — run `peloton-pp-cli auth login` from a shell first; MCP cannot spawn the browser")
	}
	c := client.New(cfg.Token)
	if cfg.UserID == "" || containsPipe(cfg.UserID) {
		id, username, err := c.Me()
		if err != nil {
			return nil, nil, fmt.Errorf("fetch /api/me: %w", err)
		}
		cfg.UserID = id
		cfg.Username = username
		_ = cfg.Save()
	}
	return c, cfg, nil
}

func containsPipe(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '|' {
			return true
		}
	}
	return false
}

func intArg(args map[string]any, key string, def int) int {
	if v, ok := args[key]; ok {
		switch x := v.(type) {
		case float64:
			return int(x)
		case int:
			return x
		}
	}
	return def
}

func boolArg(args map[string]any, key string, def bool) bool {
	if v, ok := args[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return def
}

func stringArg(args map[string]any, key string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func jsonResult(v any) (*mcplib.CallToolResult, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return mcplib.NewToolResultError("marshal: " + err.Error()), nil
	}
	return mcplib.NewToolResultText(string(b)), nil
}

func handleMe(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	cfg, err := config.Load("")
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	if cfg.Token == "" {
		return mcplib.NewToolResultError("no Peloton token saved — run `peloton-pp-cli auth login` from a shell"), nil
	}
	return jsonResult(map[string]any{
		"user_id":      cfg.UserID,
		"username":     cfg.Username,
		"harvested_at": cfg.HarvestedAt,
	})
}

func handleWorkoutsList(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	c, cfg, err := loadAuthedClient(ctx)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	args := req.GetArguments()
	limit := intArg(args, "limit", 50)
	workouts, err := c.ListWorkouts(cfg.UserID, limit, nil)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	return jsonResult(workouts)
}

func handleWorkoutsShow(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	c, _, err := loadAuthedClient(ctx)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	id := stringArg(req.GetArguments(), "id")
	if id == "" {
		return mcplib.NewToolResultError("required argument: id"), nil
	}
	w, err := c.GetWorkout(id)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	return jsonResult(w)
}

func handleRideShow(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	c, _, err := loadAuthedClient(ctx)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	rideID := stringArg(req.GetArguments(), "ride_id")
	if rideID == "" {
		return mcplib.NewToolResultError("required argument: ride_id"), nil
	}
	rd, err := c.GetRideDetails(rideID)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	return jsonResult(rd)
}

func handleDiscoveries(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	c, cfg, err := loadAuthedClient(ctx)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	args := req.GetArguments()
	limit := intArg(args, "limit", 30)
	workouts, err := c.ListWorkouts(cfg.UserID, limit, nil)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	ds, err := collectDiscoveriesMCP(c, workouts)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	return jsonResult(ds)
}

func handleSearch(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	query := stringArg(args, "query")
	if query == "" {
		return mcplib.NewToolResultError("required argument: query"), nil
	}
	limit := intArg(args, "limit", 20)
	st, err := store.Open(ctx, "")
	if err != nil {
		return mcplib.NewToolResultError("open store: " + err.Error()), nil
	}
	defer st.Close()
	hits, err := st.Search(ctx, query, limit)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	return jsonResult(hits)
}

func handleSync(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	c, cfg, err := loadAuthedClient(ctx)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	args := req.GetArguments()
	limit := intArg(args, "limit", 200)
	full := boolArg(args, "full", false)
	hydrate := boolArg(args, "hydrate_rides", true)

	st, err := store.Open(ctx, "")
	if err != nil {
		return mcplib.NewToolResultError("open store: " + err.Error()), nil
	}
	defer st.Close()

	known, err := st.KnownWorkoutIDs(ctx)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	knownArg := known
	if full {
		knownArg = nil
	}
	workouts, err := c.ListWorkouts(cfg.UserID, limit, knownArg)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	newWorkouts, err := st.UpsertWorkouts(ctx, workouts)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	newRides := 0
	if hydrate {
		ids, err := st.RideIDsMissingDetails(ctx)
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		for _, id := range ids {
			rd, err := c.GetRideDetails(id)
			if err != nil {
				// Skip individual 404s (retired rides); fail on auth/rate
				// errors so the agent sees them.
				if isFatalAPIError(err) {
					return mcplib.NewToolResultError(err.Error()), nil
				}
				continue
			}
			if err := st.UpsertRideDetails(ctx, rd); err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}
			newRides++
		}
	}
	counts, err := st.Counts(ctx)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	return jsonResult(map[string]any{
		"new_workouts": newWorkouts,
		"new_rides":    newRides,
		"counts":       counts,
		"db_path":      st.Path(),
	})
}
