package cli

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/store"
)

// The only Protect path the generator offered as a write target disables alarms, so an
// "import" of ordinary records would silently stand down an alarm system.
func TestImportHasNoProtectWriteTarget(t *testing.T) {
	if path, err := resourceWritePath("protect"); err == nil {
		t.Fatalf("protect import writes to %q", path)
	}
	if _, err := resourceWritePath("carrier"); err != nil {
		t.Fatalf("carrier import should remain: %v", err)
	}
}

func TestPlanUndoUsesTheMethodTheEndpointAccepts(t *testing.T) {
	before := json.RawMessage(`{"id":"x","name":"old","modelKey":"camera","enabled":true}`)
	cases := []struct {
		name       string
		method     string
		path       string
		request    string
		wantMethod string
		wantBody   map[string]any
	}{
		{
			name:       "protect restores only the changed field",
			method:     http.MethodPatch,
			path:       "/protect/integration/v1/cameras/x",
			request:    `{"name":"new"}`,
			wantMethod: http.MethodPatch,
			wantBody:   map[string]any{"name": "old"},
		},
		{
			name:       "a patch elsewhere is undone by a patch",
			method:     http.MethodPatch,
			path:       "/network/integration/v1/sites/s/firewall/policies/x",
			request:    `{"enabled":false}`,
			wantMethod: http.MethodPatch,
			wantBody:   map[string]any{"enabled": true},
		},
		{
			name:       "a put is undone by a full put",
			method:     http.MethodPut,
			path:       "/network/integration/v1/sites/s/wifi/broadcasts/x",
			request:    `{"name":"new","modelKey":"camera","enabled":true}`,
			wantMethod: http.MethodPut,
			wantBody:   map[string]any{"name": "old", "modelKey": "camera", "enabled": true},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			method, _, body := planUndo(store.JournalEntry{Kind: changeKindUpdate, Method: tc.method, Path: tc.path, ReadPath: tc.path, RequestBody: json.RawMessage(tc.request), BeforeImage: before})
			if method != tc.wantMethod || !reflect.DeepEqual(body, any(tc.wantBody)) {
				t.Fatalf("got %s %v, want %s %v", method, body, tc.wantMethod, tc.wantBody)
			}
		})
	}
}

// A rollback that is only assumed to have worked would tell someone their network is back
// to normal when it is not.
func TestRollbackIsConfirmedByReadBack(t *testing.T) {
	t.Run("auto-revert of a create checks it is gone", func(t *testing.T) {
		_, obj, db, collection := changeFixture(t)
		obj.mangle, obj.ignoreDelete = true, true
		spec := map[string]any{"method": "POST", "path": collection, "body": map[string]any{"name": "new", "enabled": true}}
		run := runUniFiCLI(t, "change", "apply", writeChangeFile(t, spec), "--console", "C1", "--db", db, "--confirm", "--auto-revert", "--json")
		result := parseChange(t, run)
		if run.err == nil || result.State != journalUnresolved || result.Reverted {
			t.Fatalf("%v %+v", run.err, result)
		}
	})
	t.Run("undo of a delete needs an id", func(t *testing.T) {
		_, obj, db, collection := changeFixture(t)
		spec := map[string]any{"method": "DELETE", "path": collection + "/pol-1"}
		deleted := parseChange(t, runUniFiCLI(t, "change", "apply", writeChangeFile(t, spec), "--console", "C1", "--db", db, "--confirm", "--json"))
		obj.noID = true
		run := runUniFiCLI(t, "change", "undo", deleted.JournalID, "--console", "C1", "--db", db, "--confirm", "--json")
		if undo := parseChange(t, run); run.err == nil || undo.State != journalUnresolved {
			t.Fatalf("%v %+v", run.err, undo)
		}
	})
	t.Run("undo of a delete reads the recreated object back", func(t *testing.T) {
		_, obj, db, collection := changeFixture(t)
		spec := map[string]any{"method": "DELETE", "path": collection + "/pol-1"}
		deleted := parseChange(t, runUniFiCLI(t, "change", "apply", writeChangeFile(t, spec), "--console", "C1", "--db", db, "--confirm", "--json"))
		obj.mangle = true
		run := runUniFiCLI(t, "change", "undo", deleted.JournalID, "--console", "C1", "--db", db, "--confirm", "--json")
		if undo := parseChange(t, run); run.err == nil || undo.State != journalUnresolved {
			t.Fatalf("%v %+v", run.err, undo)
		}
	})
}
