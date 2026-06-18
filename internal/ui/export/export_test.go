package export

import (
	"testing"

	"github.com/llehouerou/waves/internal/export"
	"github.com/llehouerou/waves/internal/ui/action"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

func newTestExportPopup() *testutil.PopupHarness {
	m := New(nil) // nil repo - we'll set data via messages
	m.SetSize(80, 24)
	return testutil.NewPopupHarness(&m)
}

func newExportPopupWithTargets(targets []export.Target, volumes []export.Volume) *testutil.PopupHarness {
	m := New(nil)
	m.SetSize(80, 24)
	h := testutil.NewPopupHarness(&m)

	// Load targets and volumes
	h.SendMsg(TargetsLoadedMsg{Targets: targets})
	h.SendMsg(VolumesLoadedMsg{Volumes: volumes})

	return h
}

func getExportAction(t *testing.T, h *testutil.PopupHarness) action.Action {
	t.Helper()
	cmd := h.LastCommand()
	if cmd == nil {
		t.Fatal("expected command, got nil")
	}
	msg := testutil.ExecuteCmd(cmd)
	actionMsg, ok := msg.(action.Msg)
	if !ok {
		t.Fatalf("expected action.Msg, got %T", msg)
	}
	return actionMsg.Action
}

// Sample data for tests
func sampleTargets() []export.Target {
	return []export.Target{
		{ID: 1, Name: "USB Drive", DeviceUUID: "uuid-1", Subfolder: "/Music"},
		{ID: 2, Name: "Phone", DeviceUUID: "uuid-2", Subfolder: "/"},
		{ID: 3, Name: "Custom Folder", DeviceUUID: "", Subfolder: "/home/user/exports"},
	}
}

func sampleVolumes() []export.Volume {
	return []export.Volume{
		{UUID: "uuid-1", Label: "USB Drive", MountPath: "/media/usb"},
	}
}

// Close tests

func TestExport_CloseWithEscape(t *testing.T) {
	h := newTestExportPopup()

	h.SendEscape()

	act := getExportAction(t, h)
	if _, ok := act.(Close); !ok {
		t.Fatalf("expected Close, got %T", act)
	}
}

func TestExport_CloseWithQ(t *testing.T) {
	h := newTestExportPopup()

	h.SendKey("q")

	act := getExportAction(t, h)
	if _, ok := act.(Close); !ok {
		t.Fatalf("expected Close, got %T", act)
	}
}

// Navigation tests in StateSelectTarget

func TestExport_NavigateDown(t *testing.T) {
	targets := sampleTargets()
	h := newExportPopupWithTargets(targets, sampleVolumes())

	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}

	h.SendDown()

	if m.selectedIdx != 1 {
		t.Errorf("selectedIdx = %d, want 1", m.selectedIdx)
	}
}

func TestExport_NavigateWithJK(t *testing.T) {
	targets := sampleTargets()
	h := newExportPopupWithTargets(targets, sampleVolumes())

	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}

	h.SendKey("j") // -> 1
	h.SendKey("j") // -> 2
	h.SendKey("k") // -> 1

	if m.selectedIdx != 1 {
		t.Errorf("selectedIdx = %d, want 1", m.selectedIdx)
	}
}

func TestExport_NavigateBoundsTop(t *testing.T) {
	targets := sampleTargets()
	h := newExportPopupWithTargets(targets, sampleVolumes())

	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}

	h.SendUp()
	h.SendUp()

	if m.selectedIdx != 0 {
		t.Errorf("selectedIdx = %d, want 0", m.selectedIdx)
	}
}

// Delete target test

func TestExport_DeleteTarget(t *testing.T) {
	targets := sampleTargets()
	h := newExportPopupWithTargets(targets, sampleVolumes())

	h.SendKey("d")

	act := getExportAction(t, h)
	del, ok := act.(DeleteTarget)
	if !ok {
		t.Fatalf("expected DeleteTarget, got %T", act)
	}
	if del.ID != 1 {
		t.Errorf("ID = %d, want 1", del.ID)
	}
	if del.Name != "USB Drive" {
		t.Errorf("Name = %q, want 'USB Drive'", del.Name)
	}
}

// Rename target test

func TestExport_EnterRenameMode(t *testing.T) {
	targets := sampleTargets()
	h := newExportPopupWithTargets(targets, sampleVolumes())

	h.SendKey("r")

	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}

	if m.state != StateRenameTarget {
		t.Errorf("state = %d, want StateRenameTarget", m.state)
	}
}

func TestExport_RenameTargetSubmit(t *testing.T) {
	targets := sampleTargets()
	h := newExportPopupWithTargets(targets, sampleVolumes())

	h.SendKey("r") // Enter rename mode

	// Clear existing name "USB Drive" (9 characters)
	for range 9 {
		h.SendKey("backspace")
	}

	h.SendKey("N")
	h.SendKey("e")
	h.SendKey("w")
	h.SendEnter()

	act := getExportAction(t, h)
	rename, ok := act.(RenameTarget)
	if !ok {
		t.Fatalf("expected RenameTarget, got %T", act)
	}
	if rename.NewName != "New" {
		t.Errorf("NewName = %q, want 'New'", rename.NewName)
	}
}

func TestExport_RenameCancel(t *testing.T) {
	targets := sampleTargets()
	h := newExportPopupWithTargets(targets, sampleVolumes())

	h.SendKey("r") // Enter rename mode
	h.SendKey("t")
	h.SendKey("e")
	h.SendKey("s")
	h.SendKey("t")
	h.SendEscape() // Cancel

	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}

	if m.state != StateSelectTarget {
		t.Errorf("state = %d, want StateSelectTarget", m.state)
	}
}

// New target test

func TestExport_EnterNewTargetMode(t *testing.T) {
	targets := sampleTargets()
	h := newExportPopupWithTargets(targets, sampleVolumes())

	// Navigate to "New target" option (past all existing targets)
	for range targets {
		h.SendDown()
	}
	h.SendEnter()

	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}

	if m.state != StateNewTarget {
		t.Errorf("state = %d, want StateNewTarget", m.state)
	}
}

func TestExport_NewTargetCancel(t *testing.T) {
	targets := sampleTargets()
	h := newExportPopupWithTargets(targets, sampleVolumes())

	// Navigate to "New target"
	for range targets {
		h.SendDown()
	}
	h.SendEnter()
	h.SendEscape() // Cancel

	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}

	if m.state != StateSelectTarget {
		t.Errorf("state = %d, want StateSelectTarget", m.state)
	}
}

// FLAC conversion toggle

func TestExport_ToggleFLACConversion(t *testing.T) {
	targets := sampleTargets()
	h := newExportPopupWithTargets(targets, sampleVolumes())

	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}

	// Set tracks with FLAC files
	m.SetTracks([]export.Track{
		{SrcPath: "/music/track.flac", Extension: ".flac"},
	}, "Test Album")

	initialConvert := m.convertFLAC
	h.SendKey(" ") // Toggle

	if m.convertFLAC == initialConvert {
		t.Error("convertFLAC should toggle")
	}
}

// View tests

func TestExport_ViewShowsTitle(t *testing.T) {
	h := newTestExportPopup()

	if err := h.AssertViewContains("Export"); err != "" {
		t.Error(err)
	}
}

func TestExport_ViewShowsTargets(t *testing.T) {
	targets := sampleTargets()
	h := newExportPopupWithTargets(targets, sampleVolumes())

	if err := h.AssertViewContains("USB Drive"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("Phone"); err != "" {
		t.Error(err)
	}
}

func TestExport_ViewShowsTrackCount(t *testing.T) {
	h := newTestExportPopup()

	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}
	m.SetTracks([]export.Track{
		{SrcPath: "/a.mp3"},
		{SrcPath: "/b.mp3"},
		{SrcPath: "/c.mp3"},
	}, "")

	if err := h.AssertViewContains("3 tracks"); err != "" {
		t.Error(err)
	}
}

func TestExport_ViewShowsAlbumName(t *testing.T) {
	h := newTestExportPopup()

	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}
	m.SetTracks([]export.Track{{SrcPath: "/a.mp3"}}, "Best Album Ever")

	if err := h.AssertViewContains("Best Album Ever"); err != "" {
		t.Error(err)
	}
}

func TestExport_ViewShowsNotConnected(t *testing.T) {
	targets := sampleTargets()
	// Only mount first volume - Phone (uuid-2) won't be connected
	h := newExportPopupWithTargets(targets, sampleVolumes())

	if err := h.AssertViewContains("not connected"); err != "" {
		t.Error(err)
	}
}

func TestExport_ViewShowsNewTargetOption(t *testing.T) {
	h := newTestExportPopup()

	if err := h.AssertViewContains("New target"); err != "" {
		t.Error(err)
	}
}

// Custom folder tests

func TestExport_EnterCustomFolderMode(t *testing.T) {
	targets := sampleTargets()
	volumes := sampleVolumes()
	h := newExportPopupWithTargets(targets, volumes)

	// Navigate to "New target"
	for range targets {
		h.SendDown()
	}
	h.SendEnter()

	// Navigate past volumes to "Custom folder" option
	for range volumes {
		h.SendDown()
	}
	h.SendEnter()

	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}

	if m.state != StateCustomFolder {
		t.Errorf("state = %d, want StateCustomFolder", m.state)
	}
}

func TestExport_CustomFolderCancel(t *testing.T) {
	targets := sampleTargets()
	volumes := sampleVolumes()
	h := newExportPopupWithTargets(targets, volumes)

	// Navigate to custom folder mode
	for range targets {
		h.SendDown()
	}
	h.SendEnter()
	for range volumes {
		h.SendDown()
	}
	h.SendEnter()

	h.SendEscape() // Cancel

	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}

	if m.state != StateNewTarget {
		t.Errorf("state = %d, want StateNewTarget", m.state)
	}
}

// TargetsLoadedMsg handling

func TestExport_TargetsLoadedMsg(t *testing.T) {
	h := newTestExportPopup()

	targets := []export.Target{
		{ID: 1, Name: "Test Target"},
	}
	h.SendMsg(TargetsLoadedMsg{Targets: targets})

	if err := h.AssertViewContains("Test Target"); err != "" {
		t.Error(err)
	}
}

// VolumesLoadedMsg handling

func TestExport_VolumesLoadedMsg(t *testing.T) {
	h := newTestExportPopup()

	// Add a target first
	h.SendMsg(TargetsLoadedMsg{Targets: []export.Target{
		{ID: 1, Name: "USB", DeviceUUID: "test-uuid"},
	}})

	// Add matching volume - should show as connected
	h.SendMsg(VolumesLoadedMsg{Volumes: []export.Volume{
		{UUID: "test-uuid", Label: "USB", MountPath: "/media/usb"},
	}})

	// Should not show "not connected"
	if err := h.AssertViewNotContains("not connected"); err != "" {
		t.Error(err)
	}
}

// Empty targets

func TestExport_EmptyTargetsShowsNewTarget(t *testing.T) {
	h := newTestExportPopup()
	h.SendMsg(TargetsLoadedMsg{Targets: nil})

	if err := h.AssertViewContains("New target"); err != "" {
		t.Error(err)
	}
}

// Delete on empty list

func TestExport_DeleteOnEmptyDoesNothing(t *testing.T) {
	h := newTestExportPopup()
	h.SendMsg(TargetsLoadedMsg{Targets: nil})
	h.ClearCommands()

	h.SendKey("d")

	if len(h.Commands()) != 0 {
		t.Error("delete on empty list should not produce command")
	}
}
