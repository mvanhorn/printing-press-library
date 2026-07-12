package bambu

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMonitorMergesSparseReportsAndTransitions(t *testing.T) {
	monitor := NewMonitor("SERIAL123456")
	events, err := monitor.Ingest([]byte(`{"print":{"gcode_state":"FINISH","mc_percent":100}}`))
	if err != nil || len(events) != 0 {
		t.Fatalf("initial ingest: events=%v err=%v", events, err)
	}
	events, err = monitor.Ingest([]byte(`{"print":{"gcode_state":"RUNNING","subtask_name":"plate_one.gcode.3mf","mc_percent":1,"layer_num":2}}`))
	if err != nil || len(events) != 1 || events[0].Kind != "started" {
		t.Fatalf("start transition: %#v err=%v", events, err)
	}
	events, err = monitor.Ingest([]byte(`{"print":{"mc_percent":2}}`))
	if err != nil || len(events) != 0 {
		t.Fatalf("partial update: %#v err=%v", events, err)
	}
	if monitor.Snapshot().State != "RUNNING" || monitor.Snapshot().Percent == nil || *monitor.Snapshot().Percent != 2 {
		t.Fatalf("sparse merge lost state: %#v", monitor.Snapshot())
	}
}

func TestMonitorClassifiesStudioStopFailReasonAsCanceled(t *testing.T) {
	monitor := NewMonitor("SERIAL123456")
	if events, err := monitor.Ingest([]byte(`{"print":{"gcode_state":"RUNNING","task_id":"task","print_error":0,"fail_reason":"0"}}`)); err != nil || len(events) != 0 {
		t.Fatalf("initialize: events=%#v err=%v", events, err)
	}
	events, err := monitor.Ingest([]byte(`{"print":{"gcode_state":"FAILED","task_id":"task","print_error":0,"mc_print_error_code":"0","fail_reason":"50348044"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Kind != "canceled" || events[0].Snapshot.PrintError != CancelError {
		t.Fatalf("events = %#v, want one canceled event", events)
	}
}

func TestMonitorNormalizesLivePlateIndex(t *testing.T) {
	monitor := NewMonitor("SERIAL123456")
	if _, err := monitor.Ingest([]byte(`{"print":{"gcode_state":"RUNNING","plate_idx":1,"plate_id":9}}`)); err != nil {
		t.Fatal(err)
	}
	if got := monitor.Snapshot().PlateNumber; got == nil || *got != 1 {
		t.Fatalf("plate number = %v, want 1", got)
	}
}

func TestParseSSDPRejectsPublicAndAcceptsPrivatePrinter(t *testing.T) {
	payload := []byte("HTTP/1.1 200 OK\r\nST: " + SSDPService + "\r\nDevSerial.bambu.com: SERIAL123456\r\nLocation: http://192.168.1.25:80\r\nDevModel.bambu.com: O1D\r\nDevName.bambu.com: Workshop\r\n\r\n")
	result, ok := ParseSSDP(payload)
	if !ok || result.Host != "192.168.1.25" || result.Model != "O1D" {
		t.Fatalf("unexpected discovery: %#v ok=%v", result, ok)
	}
	publicPayload := bytes.ReplaceAll(payload, []byte("192.168.1.25"), []byte("8.8.8.8"))
	if _, ok := ParseSSDP(publicPayload); ok {
		t.Fatal("public SSDP location must be rejected")
	}
}

func TestExtract3MFSelectsPlateWeightAndThumbnail(t *testing.T) {
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	config, _ := zw.Create("Metadata/slice_info.config")
	_, _ = fmt.Fprint(config, `<config><plate><metadata key="index" value="1"/><metadata key="weight" value="449.51"/><object identify_id="7" name="part"/></plate></config>`)
	imageFile, _ := zw.Create("Metadata/plate_1.png")
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.White)
	_ = png.Encode(imageFile, img)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	plate := 1
	metadata, err := Extract3MF(archive.Bytes(), &plate)
	if err != nil {
		t.Fatal(err)
	}
	if metadata.WeightGrams == nil || *metadata.WeightGrams != 449.51 || len(metadata.Thumbnail) == 0 || len(metadata.Objects) != 1 {
		t.Fatalf("unexpected metadata: %#v", metadata)
	}
}

func TestExtract3MFRejectsMissingRequestedPlate(t *testing.T) {
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	config, _ := zw.Create("Metadata/slice_info.config")
	_, _ = fmt.Fprint(config, `<config><plate><metadata key="index" value="1"/></plate><plate><metadata key="index" value="2"/></plate></config>`)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	plate := 3
	if _, err := Extract3MF(archive.Bytes(), &plate); err == nil || !strings.Contains(err.Error(), "requested plate 3") {
		t.Fatalf("missing plate error = %v", err)
	}
}

func TestExtract3MFReadsProjectProfileAndObjectNames(t *testing.T) {
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	sliceConfig, _ := zw.Create("Metadata/slice_info.config")
	_, _ = fmt.Fprint(sliceConfig, `<config><plate><metadata key="index" value="1"/><object identify_id="170" name="3DBenchy.stl" skipped="false"/></plate></config>`)
	modelConfig, _ := zw.Create("3D/3dmodel.model")
	_, _ = fmt.Fprint(modelConfig, `<model><metadata name="Title">Benchy Bambu Pla Basic</metadata><metadata name="ProfileTitle">14min44s, Bambu PLA Basic, A1</metadata></model>`)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	plate := 1
	metadata, err := Extract3MF(archive.Bytes(), &plate)
	if err != nil {
		t.Fatal(err)
	}
	if metadata.ProjectName != "Benchy Bambu Pla Basic" || metadata.ProfileName != "14min44s, Bambu PLA Basic, A1" || len(metadata.Objects) != 1 || metadata.Objects[0].Name != "3DBenchy.stl" {
		t.Fatalf("metadata = %#v", metadata)
	}
}

func TestExtract3MFSelectsRequestedNonFirstPlate(t *testing.T) {
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	config, _ := zw.Create("Metadata/slice_info.config")
	_, _ = fmt.Fprint(config, `<config><plate><metadata key="index" value="1"/><metadata key="weight" value="10"/></plate><plate><metadata key="index" value="2"/><metadata key="weight" value="20"/></plate></config>`)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	plate := 2
	metadata, err := Extract3MF(archive.Bytes(), &plate)
	if err != nil {
		t.Fatal(err)
	}
	if metadata.PlateNumber == nil || *metadata.PlateNumber != 2 || metadata.WeightGrams == nil || *metadata.WeightGrams != 20 {
		t.Fatalf("selected metadata = %#v", metadata)
	}
}

func TestJobMetadataErrorsWithoutCandidates(t *testing.T) {
	ftps := &FTPS{}
	if _, err := ftps.JobMetadata(Snapshot{}); err == nil || !strings.Contains(err.Error(), "0 candidate") {
		t.Fatalf("metadata error = %v", err)
	}
}

func TestJobMetadataExplainsPrinterResidentDisplayPrint(t *testing.T) {
	ftps := &FTPS{}
	snapshot := Snapshot{Raw: map[string]any{"gcode_file": "/data/Metadata/plate_1.gcode"}}
	if _, err := ftps.JobMetadata(snapshot); err == nil || !strings.Contains(err.Error(), "printer-resident G-code") || !strings.Contains(err.Error(), "display-started") {
		t.Fatalf("metadata error = %v", err)
	}
}

func TestArchiveCandidatesPreferCurrentRootArchiveOverCache(t *testing.T) {
	candidates := ArchiveCandidates(Snapshot{SubtaskName: "current-job.3mf"})
	want := []string{"/current-job.3mf", "/cache/current-job.3mf"}
	if len(candidates) != len(want) {
		t.Fatalf("candidates = %#v, want %#v", candidates, want)
	}
	for index := range want {
		if candidates[index] != want[index] {
			t.Fatalf("candidates = %#v, want %#v", candidates, want)
		}
	}
}

func TestLegacyArchiveCandidatesRetainMissingAndSkewedTimestamps(t *testing.T) {
	observedAt := time.Date(2026, 7, 12, 5, 0, 0, 0, time.UTC)
	percent, remaining := 40, 30
	snapshot := Snapshot{ObservedAt: observedAt, Percent: &percent, RemainingMinutes: &remaining}
	files := []File{
		{Path: "/old.gcode.3mf", Name: "old.gcode.3mf", Size: 100, ModTime: observedAt.Add(-72 * time.Hour), Type: "file"},
		{Path: "/current.gcode.3mf", Name: "current.gcode.3mf", Size: 100, ModTime: observedAt.Add(-20 * time.Minute), Type: "file"},
		{Path: "/newer.gcode.3mf", Name: "newer.gcode.3mf", Size: 100, ModTime: observedAt.Add(-10 * time.Minute), Type: "file"},
		{Path: "/future.gcode.3mf", Name: "future.gcode.3mf", Size: 100, ModTime: observedAt.Add(10 * time.Minute), Type: "file"},
		{Path: "/unknown.gcode.3mf", Name: "unknown.gcode.3mf", Size: 100, Type: "file"},
		{Path: "/notes.txt", Name: "notes.txt", Size: 100, ModTime: observedAt, Type: "file"},
	}
	got := LegacyArchiveCandidates(snapshot, files)
	want := []string{"/unknown.gcode.3mf", "/future.gcode.3mf", "/newer.gcode.3mf", "/current.gcode.3mf"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy candidates = %#v, want %#v", got, want)
	}
}

func TestMetadataMatchesSnapshotProfileOrProject(t *testing.T) {
	snapshot := Snapshot{JobName: "12min42s, Bambu PLA Basic, A1 mini", SubtaskName: "fallback project"}
	if !MetadataMatchesSnapshot(Metadata{ProfileName: "12min42s Bambu PLA Basic A1 Mini"}, snapshot) {
		t.Fatal("normalized profile title must match current MQTT job")
	}
	if !MetadataMatchesSnapshot(Metadata{ProjectName: "Fallback_Project"}, snapshot) {
		t.Fatal("normalized project title must match current MQTT subtask")
	}
	if MetadataMatchesSnapshot(Metadata{ProfileName: "another print"}, snapshot) {
		t.Fatal("unrelated archive metadata must not match current MQTT job")
	}
}

func TestLegacyArchiveCandidatesRetainAllUnknownTimestamps(t *testing.T) {
	observedAt := time.Date(2026, 7, 12, 5, 0, 0, 0, time.UTC)
	files := make([]File, 25)
	for index := range files {
		files[index] = File{
			Path: fmt.Sprintf("/candidate-%02d.3mf", index),
			Name: fmt.Sprintf("candidate-%02d.3mf", index),
			Size: 100,
			Type: "file",
		}
	}
	files[len(files)-1].Path = "/zzzz-active.3mf"
	files[len(files)-1].Name = "zzzz-active.3mf"
	got := LegacyArchiveCandidates(Snapshot{ObservedAt: observedAt}, files)
	if len(got) != len(files) {
		t.Fatalf("legacy candidate count = %d, want %d", len(got), len(files))
	}
	if got[len(got)-1] != "/zzzz-active.3mf" {
		t.Fatalf("last missing-timestamp candidate was dropped: %#v", got)
	}
}

func TestExtract3MFRejectsExcessiveEntryCount(t *testing.T) {
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for index := 0; index <= MaxArchiveEntries; index++ {
		entry, err := writer.Create(fmt.Sprintf("Metadata/unused_%05d.txt", index))
		if err != nil {
			t.Fatal(err)
		}
		_, _ = entry.Write(nil)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Extract3MF(archive.Bytes(), nil); err == nil {
		t.Fatal("expected excessive entry-count error")
	}
}

func TestZIPEntryPreflightRejectsBeforeReader(t *testing.T) {
	payload := make([]byte, 22)
	binary.LittleEndian.PutUint32(payload[0:4], 0x06054b50)
	binary.LittleEndian.PutUint16(payload[10:12], uint16(MaxArchiveEntries+1))
	if err := preflightZIPDirectory(payload); err == nil || !strings.Contains(err.Error(), "entries") {
		t.Fatalf("preflight error = %v", err)
	}
}

func TestZIPEntryPreflightCountsHeadersInsteadOfTrustingEOCDModulo(t *testing.T) {
	const entries = MaxArchiveEntries + 1
	directoryBytes := entries * 46
	payload := make([]byte, directoryBytes+22)
	for index := 0; index < entries; index++ {
		binary.LittleEndian.PutUint32(payload[index*46:index*46+4], 0x02014b50)
	}
	eocd := payload[directoryBytes:]
	binary.LittleEndian.PutUint32(eocd[0:4], 0x06054b50)
	binary.LittleEndian.PutUint16(eocd[10:12], 0)
	binary.LittleEndian.PutUint32(eocd[12:16], uint32(directoryBytes))
	binary.LittleEndian.PutUint32(eocd[16:20], 0)
	if err := preflightZIPDirectory(payload); err == nil || !strings.Contains(err.Error(), "more than") {
		t.Fatalf("modulo-count preflight error = %v", err)
	}
}
