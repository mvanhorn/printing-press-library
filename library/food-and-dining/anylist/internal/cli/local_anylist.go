package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/config"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/store"

	"github.com/google/uuid"
)

func openLocalStore(flags *rootFlags) (*config.Config, *store.Store, error) {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return nil, nil, configErr(err)
	}
	st, err := store.Open(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("no local data found — run 'anylist-pp-cli sync' first")
	}
	return cfg, st, nil
}

func openAuthedLocalStore(flags *rootFlags) (*config.Config, *store.Store, error) {
	cfg, st, err := openLocalStore(flags)
	if err != nil {
		return nil, nil, err
	}
	if cfg.AccessToken == "" {
		st.Close()
		return nil, nil, authErr(fmt.Errorf("not authenticated — run 'anylist-pp-cli auth login' first"))
	}
	return cfg, st, nil
}

func syncStoreFromLive(ctx context.Context, cfg *config.Config, st *store.Store) error {
	alClient := anylist.New(cfg)
	userData, err := alClient.GetUserData(ctx)
	if err != nil {
		return err
	}
	return st.SyncFromUserData(userData)
}

func itemRowToPB(item *store.ItemRow, userID string) *pb.ListItem {
	if item == nil {
		return nil
	}
	return &pb.ListItem{
		Identifier:      item.ID,
		ListId:          item.ListID,
		Name:            item.Name,
		Quantity:        item.Quantity,
		Details:         item.Details,
		Checked:         item.Checked,
		Category:        item.Category,
		UserId:          userID,
		CategoryMatchId: item.CategoryMatchID,
		StoreIds:        item.StoreIDs,
		ManualSortIndex: int32(item.SortIndex),
	}
}

func readStdinJSONMap() (map[string]any, error) {
	stdinData, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("reading stdin: %w", err)
	}
	var body map[string]any
	if err := json.Unmarshal(stdinData, &body); err != nil {
		return nil, fmt.Errorf("parsing stdin JSON: %w", err)
	}
	return body, nil
}

func stringFromBody(body map[string]any, key string) string {
	if body == nil {
		return ""
	}
	if v, ok := body[key].(string); ok {
		return v
	}
	return ""
}

func intFromBody(body map[string]any, key string) int {
	if body == nil {
		return 0
	}
	switch v := body[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}

func defaultMealRange(from, to string) (string, string, error) {
	if from == "" {
		now := time.Now()
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		monday := now.AddDate(0, 0, -(weekday - 1))
		sunday := monday.AddDate(0, 0, 6)
		from = monday.Format("2006-01-02")
		to = sunday.Format("2006-01-02")
	}
	if to == "" {
		fromTime, err := time.Parse("2006-01-02", from)
		if err != nil {
			return "", "", fmt.Errorf("invalid --from date: %w", err)
		}
		to = fromTime.AddDate(0, 0, 6).Format("2006-01-02")
	}
	// PATCH: Validate explicit ranges even when both endpoints are supplied.
	fromTime, err := time.Parse("2006-01-02", from)
	if err != nil {
		return "", "", fmt.Errorf("invalid --from date: %w", err)
	}
	toTime, err := time.Parse("2006-01-02", to)
	if err != nil {
		return "", "", fmt.Errorf("invalid --to date: %w", err)
	}
	if toTime.Before(fromTime) {
		return "", "", fmt.Errorf("--to date must be on or after --from date")
	}
	return from, to, nil
}

func currentRecipeData(ctx context.Context, cfg *config.Config) (*pb.PBUserDataResponse, string, error) {
	alClient := anylist.New(cfg)
	userData, err := alClient.GetUserData(ctx)
	if err != nil {
		return nil, "", err
	}
	rdr := userData.GetRecipeDataResponse()
	if rdr == nil || rdr.GetRecipeDataId() == "" {
		return nil, "", fmt.Errorf("recipe data id not found in AnyList user data")
	}
	return userData, rdr.GetRecipeDataId(), nil
}

func findLiveRecipeByName(userData *pb.PBUserDataResponse, name string) (*pb.PBRecipe, error) {
	rdr := userData.GetRecipeDataResponse()
	if rdr == nil {
		return nil, fmt.Errorf("recipe %q not found", name)
	}
	lower := strings.ToLower(name)
	for _, recipe := range rdr.GetRecipes() {
		if strings.EqualFold(recipe.GetName(), name) {
			return recipe, nil
		}
	}
	for _, recipe := range rdr.GetRecipes() {
		if strings.Contains(strings.ToLower(recipe.GetName()), lower) {
			return recipe, nil
		}
	}
	return nil, fmt.Errorf("recipe %q not found", name)
}

func currentListFolderData(ctx context.Context, cfg *config.Config) (*pb.PBUserDataResponse, string, string, error) {
	alClient := anylist.New(cfg)
	userData, err := alClient.GetUserData(ctx)
	if err != nil {
		return nil, "", "", err
	}
	lfr := userData.GetListFoldersResponse()
	if lfr == nil || lfr.GetListDataId() == "" {
		return nil, "", "", fmt.Errorf("list folder data id not found in AnyList user data")
	}
	return userData, lfr.GetListDataId(), lfr.GetRootFolderId(), nil
}

func findLiveRecipeCollectionByName(userData *pb.PBUserDataResponse, name string) (*pb.PBRecipeCollection, error) {
	rdr := userData.GetRecipeDataResponse()
	if rdr == nil {
		return nil, fmt.Errorf("recipe collection %q not found", name)
	}
	lower := strings.ToLower(name)
	for _, collection := range rdr.GetRecipeCollections() {
		if strings.EqualFold(collection.GetName(), name) {
			return collection, nil
		}
	}
	for _, collection := range rdr.GetRecipeCollections() {
		if strings.Contains(strings.ToLower(collection.GetName()), lower) {
			return collection, nil
		}
	}
	return nil, fmt.Errorf("recipe collection %q not found", name)
}

func findLiveListFolderByName(userData *pb.PBUserDataResponse, name string) (*pb.PBListFolder, error) {
	lfr := userData.GetListFoldersResponse()
	if lfr == nil {
		return nil, fmt.Errorf("list folder %q not found", name)
	}
	lower := strings.ToLower(name)
	for _, folder := range lfr.GetListFolders() {
		if strings.EqualFold(folder.GetName(), name) {
			return folder, nil
		}
	}
	for _, folder := range lfr.GetListFolders() {
		if strings.Contains(strings.ToLower(folder.GetName()), lower) {
			return folder, nil
		}
	}
	return nil, fmt.Errorf("list folder %q not found", name)
}

func newRecipeID() string {
	return strings.ReplaceAll(uuid.NewString(), "-", "")
}

const maxImportedRecipeBytes = 10 << 20

func importedRecipeFromURL(ctx context.Context, recipeURL string) (*pb.PBRecipe, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, recipeURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 anylist-pp-cli")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching recipe URL: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetching recipe URL failed (HTTP %d)", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxImportedRecipeBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading recipe URL: %w", err)
	}
	if len(body) > maxImportedRecipeBytes {
		return nil, fmt.Errorf("reading recipe URL: response exceeds %d bytes", maxImportedRecipeBytes)
	}
	recipe, err := extractRecipeFromHTML(string(body), recipeURL)
	if err != nil {
		return nil, err
	}
	return recipe, nil
}

func extractRecipeFromHTML(page, recipeURL string) (*pb.PBRecipe, error) {
	for _, raw := range extractScriptJSON(page, `(?is)<script[^>]+type=["']application/ld\+json["'][^>]*>(.*?)</script>`) {
		if recipe := recipeFromJSONBlob(raw, recipeURL); recipe != nil {
			return recipe, nil
		}
	}
	for _, raw := range extractScriptJSON(page, `(?is)<script[^>]+id=["']__NEXT_DATA__["'][^>]*>(.*?)</script>`) {
		if recipe := recipeFromJSONBlob(raw, recipeURL); recipe != nil {
			return recipe, nil
		}
	}
	return nil, fmt.Errorf("no recipe metadata found at %s", recipeURL)
}

func extractScriptJSON(page, pattern string) []string {
	re := regexp.MustCompile(pattern)
	matches := re.FindAllStringSubmatch(page, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			out = append(out, html.UnescapeString(match[1]))
		}
	}
	return out
}

func recipeFromJSONBlob(raw, recipeURL string) *pb.PBRecipe {
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return nil
	}
	if candidate := findRecipeMap(v); candidate != nil {
		return recipeFromMap(candidate, recipeURL)
	}
	return nil
}

func findRecipeMap(v any) map[string]any {
	switch x := v.(type) {
	case map[string]any:
		if isRecipeMap(x) {
			return x
		}
		for _, key := range []string{"@graph", "recipe", "content", "props", "pageProps"} {
			if child, ok := x[key]; ok {
				if found := findRecipeMap(child); found != nil {
					return found
				}
			}
		}
		for _, child := range x {
			if found := findRecipeMap(child); found != nil {
				return found
			}
		}
	case []any:
		for _, child := range x {
			if found := findRecipeMap(child); found != nil {
				return found
			}
		}
	}
	return nil
}

func isRecipeMap(m map[string]any) bool {
	if _, ok := m["recipeIngredient"]; ok {
		return true
	}
	if _, ok := m["recipeIngredients"]; ok {
		return true
	}
	if typ, ok := m["@type"]; ok && strings.Contains(strings.ToLower(fmt.Sprint(typ)), "recipe") {
		return true
	}
	return false
}

func recipeFromMap(m map[string]any, recipeURL string) *pb.PBRecipe {
	now := float64(time.Now().Unix())
	sourceName := ""
	if parsed, err := url.Parse(recipeURL); err == nil {
		sourceName = parsed.Hostname()
	}
	recipe := &pb.PBRecipe{
		Identifier:        newRecipeID(),
		Timestamp:         now,
		CreationTimestamp: now,
		Name:              firstString(m, "name", "headline", "title"),
		Note:              firstString(m, "description", "summary"),
		SourceName:        sourceName,
		SourceUrl:         recipeURL,
		Servings:          servingsFromAny(firstValue(m, "recipeYield", "yield", "servings")),
		PrepTime:          int32(durationSeconds(firstString(m, "prepTime"))),
		CookTime:          int32(durationSeconds(firstString(m, "cookTime"))),
	}
	for _, raw := range stringSliceFromAny(firstValue(m, "recipeIngredient", "recipeIngredients", "ingredients")) {
		recipe.Ingredients = append(recipe.Ingredients, &pb.PBIngredient{
			RawIngredient: raw,
			Name:          raw,
		})
	}
	for _, step := range instructionStrings(firstValue(m, "recipeInstructions", "instructions", "directions")) {
		recipe.PreparationSteps = append(recipe.PreparationSteps, step)
	}
	if recipe.Name == "" || len(recipe.Ingredients) == 0 {
		return nil
	}
	return recipe
}

func firstValue(m map[string]any, keys ...string) any {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			return v
		}
	}
	return nil
}

func firstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if s, ok := m[key].(string); ok {
			return strings.TrimSpace(html.UnescapeString(s))
		}
	}
	return ""
}

func servingsFromAny(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case float64:
		return strconv.Itoa(int(x))
	case []any:
		parts := stringSliceFromAny(x)
		return strings.Join(parts, ", ")
	}
	return ""
}

func stringSliceFromAny(v any) []string {
	switch x := v.(type) {
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			if s := stringFromAny(item); s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return x
	case string:
		if x == "" {
			return nil
		}
		return []string{x}
	}
	return nil
}

func stringFromAny(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(html.UnescapeString(x))
	case map[string]any:
		return firstString(x, "text", "name", "description")
	}
	return ""
}

func instructionStrings(v any) []string {
	switch x := v.(type) {
	case []any:
		var out []string
		for _, item := range x {
			out = append(out, instructionStrings(item)...)
		}
		return out
	case map[string]any:
		if steps := firstValue(x, "itemListElement"); steps != nil {
			return instructionStrings(steps)
		}
		if s := stringFromAny(x); s != "" {
			return []string{s}
		}
	case string:
		if s := stringFromAny(x); s != "" {
			return []string{s}
		}
	}
	return nil
}

func durationSeconds(s string) int {
	if s == "" {
		return 0
	}
	if d, err := time.ParseDuration(strings.ToLower(strings.TrimPrefix(s, "PT"))); err == nil {
		return int(d.Seconds())
	}
	re := regexp.MustCompile(`(?i)(\d+)\s*(hour|hr|h|minute|min|m)`)
	total := 0
	for _, match := range re.FindAllStringSubmatch(s, -1) {
		n, _ := strconv.Atoi(match[1])
		unit := strings.ToLower(match[2])
		if strings.HasPrefix(unit, "h") {
			total += n * 3600
		} else {
			total += n * 60
		}
	}
	return total
}
