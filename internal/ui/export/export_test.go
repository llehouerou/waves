package export

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/action"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// fakeExports is the core's exports as the popup sees it: the targets saved
// and the devices mounted, or why they cannot be read. A target it is asked
// to save is saved, with the next ID.
type fakeExports struct {
	targets    []core.ExportTarget
	volumes    []core.ExportVolume
	targetsErr error
	volumesErr error
	createErr  error
}

func (f *fakeExports) Targets() ([]core.ExportTarget, error) {
	if f.targetsErr != nil {
		return nil, f.targetsErr
	}
	return slices.Clone(f.targets), nil
}

func (f *fakeExports) Volumes() ([]core.ExportVolume, error) {
	if f.volumesErr != nil {
		return nil, f.volumesErr
	}
	return slices.Clone(f.volumes), nil
}

func (f *fakeExports) CreateTarget(t core.ExportTarget) (int64, error) {
	if f.createErr != nil {
		return 0, f.createErr
	}
	t.ID = int64(len(f.targets) + 100)
	f.targets = append(f.targets, t)
	return t.ID, nil
}

// answers runs cmd, and every command it batches: it returns their answers,
// not delivered.
func answers(cmd tea.Cmd) []tea.Msg {
	msg := testutil.ExecuteCmd(cmd)
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{msg}
	}
	var msgs []tea.Msg
	for _, c := range batch {
		msgs = append(msgs, answers(c)...)
	}
	return msgs
}

// openPopup opens the popup on exports: it returns the answers of what its
// Init asked, not delivered.
func openPopup(exports *fakeExports) (*testutil.PopupHarness, *Model, []tea.Msg) {
	m := New(exports)
	m.SetSize(80, 24)
	h := testutil.NewPopupHarness(m)
	return h, m, answers(h.LastCommand())
}

// openedPopup is openPopup with the answers of its Init delivered.
func openedPopup(exports *fakeExports) (*testutil.PopupHarness, *Model) {
	h, m, init := openPopup(exports)
	for _, msg := range init {
		h.SendMsg(msg)
	}
	h.ClearCommands()
	return h, m
}

func newTestExportPopup() *testutil.PopupHarness {
	h, _, _ := openPopup(&fakeExports{})
	return h
}

func newExportPopupWithTargets(targets []core.ExportTarget, volumes []core.ExportVolume) *testutil.PopupHarness {
	h, _ := openedPopup(&fakeExports{targets: targets, volumes: volumes})
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
func sampleTargets() []core.ExportTarget {
	return []core.ExportTarget{
		{ID: 1, Name: "USB Drive", DeviceUUID: "uuid-1", Subfolder: "/Music"},
		{ID: 2, Name: "Phone", DeviceUUID: "uuid-2", Subfolder: "/"},
		{ID: 3, Name: "Custom Folder", DeviceUUID: "", Subfolder: "/home/user/exports"},
	}
}

func sampleVolumes() []core.ExportVolume {
	return []core.ExportVolume{
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

func TestExport_RenameTargetCyrillic(t *testing.T) {
	targets := sampleTargets()
	h := newExportPopupWithTargets(targets, sampleVolumes())

	h.SendKey("r") // Enter rename mode

	// Clear existing name "USB Drive" (9 characters)
	for range 9 {
		h.SendKey("backspace")
	}

	for _, r := range "Плеер" {
		h.SendKey(string(r))
	}
	h.SendEnter()

	act := getExportAction(t, h)
	rename, ok := act.(RenameTarget)
	if !ok {
		t.Fatalf("expected RenameTarget, got %T", act)
	}
	if rename.NewName != "Плеер" {
		t.Errorf("NewName = %q, want %q", rename.NewName, "Плеер")
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
	m.SetTracks([]core.ExportTrack{
		{SrcPath: "/music/track.flac", Extension: ".flac"},
	}, "Test Album")

	initialConvert := m.convertFLAC
	h.SendKey(" ") // Toggle

	if m.convertFLAC == initialConvert {
		t.Error("convertFLAC should toggle")
	}
}

// Starting the export

// "Phone" was not plugged in when the popup opened. The core looks again when
// it is asked to start: the popup does not decide for it.
func TestExport_EnterAsksForTheExportEvenToATargetSeenNotConnected(t *testing.T) {
	h := newExportPopupWithTargets(sampleTargets(), sampleVolumes())
	m, ok := h.Popup().(*Model)
	if !ok {
		t.Fatal("expected *Model")
	}
	m.SetTracks([]core.ExportTrack{{SrcPath: "/music/track.flac", Extension: ".flac"}}, "Test Album")

	h.SendDown()   // Phone
	h.SendKey(" ") // convert the FLAC
	h.SendEnter()

	start, ok := getExportAction(t, h).(StartExport)
	if !ok {
		t.Fatalf("expected StartExport, got %T", getExportAction(t, h))
	}
	if start.Target.Name != "Phone" || len(start.Tracks) != 1 || !start.ConvertFLAC {
		t.Errorf("StartExport = %+v, want the track to Phone, converted", start)
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
	m.SetTracks([]core.ExportTrack{
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
	m.SetTracks([]core.ExportTrack{{SrcPath: "/a.mp3"}}, "Best Album Ever")

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
	h, _ := openedPopup(&fakeExports{targets: []core.ExportTarget{{ID: 1, Name: "Test Target"}}})

	if err := h.AssertViewContains("Test Target"); err != "" {
		t.Error(err)
	}
}

// VolumesLoadedMsg handling

func TestExport_VolumesLoadedMsg(t *testing.T) {
	h, _ := openedPopup(&fakeExports{
		targets: []core.ExportTarget{{ID: 1, Name: "USB", DeviceUUID: "test-uuid"}},
		volumes: []core.ExportVolume{{UUID: "test-uuid", Label: "USB", MountPath: "/media/usb"}},
	})

	// Should not show "not connected"
	if err := h.AssertViewNotContains("not connected"); err != "" {
		t.Error(err)
	}
}

// Empty targets

func TestExport_EmptyTargetsShowsNewTarget(t *testing.T) {
	h, _ := openedPopup(&fakeExports{})

	if err := h.AssertViewContains("New target"); err != "" {
		t.Error(err)
	}
}

// Delete on empty list

func TestExport_DeleteOnEmptyDoesNothing(t *testing.T) {
	h, _ := openedPopup(&fakeExports{})

	h.SendKey("d")

	if len(h.Commands()) != 0 {
		t.Error("delete on empty list should not produce command")
	}
}

// === Answers the popup does not wait for ===

// closedPopupsReads are the volumes and the targets, and their failed reads,
// each read for a popup closed before they came.
func closedPopupsReads() map[string]tea.Msg {
	reads := map[string]tea.Msg{}
	for prefix, exports := range map[string]*fakeExports{
		"":       {targets: []core.ExportTarget{{ID: 9, Name: "Closed Target", DeviceUUID: "uuid-9"}}, volumes: []core.ExportVolume{{UUID: "uuid-9", Label: "Closed Device", MountPath: "/media/closed"}}},
		"failed": {targetsErr: errors.New("database locked"), volumesErr: errors.New("udisks unavailable")},
	} {
		_, _, init := openPopup(exports)
		for _, msg := range init {
			switch msg.(type) {
			case VolumesLoadedMsg:
				reads[strings.TrimSpace(prefix+" volumes")] = msg
			case TargetsLoadedMsg:
				reads[strings.TrimSpace(prefix+" targets")] = msg
			}
		}
	}
	return reads
}

// assertSelected checks that the popup shows targets, in order, the one
// named selected.
func assertSelected(t *testing.T, h *testutil.PopupHarness, m *Model, selected string, targets ...string) {
	t.Helper()
	names := make([]string, 0, len(m.targets))
	for _, target := range m.targets {
		names = append(names, target.Name)
	}
	if !slices.Equal(names, targets) {
		t.Errorf("targets = %q, want %q", names, targets)
	}
	if got, ok := m.SelectedTarget(); !ok || got.Name != selected {
		t.Errorf("selected %+v, want %q", got, selected)
	}
	if line := testutil.FindLine(testutil.StripANSI(h.View()), "> "); !strings.Contains(line, selected) {
		t.Errorf("cursor on %q, want %q", line, selected)
	}
}

// A popup drops the reads of a popup closed since, while it waits for its own
// and once it took them, and takes its own.
func TestExport_DropsTheReadsOfAClosedPopup(t *testing.T) {
	closed := closedPopupsReads()
	for name, msg := range closed {
		t.Run("loading/"+name, func(t *testing.T) {
			h, m, own := openPopup(&fakeExports{targets: sampleTargets(), volumes: sampleVolumes()})
			h.AssertDrops(t, msg)
			for _, msg := range own {
				h.SendMsg(msg)
			}
			assertSelected(t, h, m, "USB Drive", "USB Drive", "Phone", "Custom Folder")
		})
		t.Run("loaded/"+name, func(t *testing.T) {
			h, m := openedPopup(&fakeExports{targets: sampleTargets(), volumes: sampleVolumes()})
			h.SendDown()
			h.AssertDrops(t, msg)
			assertSelected(t, h, m, "Phone", "USB Drive", "Phone", "Custom Folder")
		})
	}
}

// The volumes and the targets the popup took, delivered again, leave the
// list and its cursor as they are.
func TestExport_TakesItsReadsOnce(t *testing.T) {
	h, m, own := openPopup(&fakeExports{targets: sampleTargets(), volumes: sampleVolumes()})
	for _, msg := range own {
		h.SendMsg(msg)
	}
	h.SendDown()

	for _, msg := range own {
		h.AssertDrops(t, msg)
	}
	assertSelected(t, h, m, "Phone", "USB Drive", "Phone", "Custom Folder")
}

// After a rename or a delete, the popup reads its targets again: the read
// asked before is dropped when it comes after.
func TestExport_ReloadTargetsReplacesTheReadAskedBefore(t *testing.T) {
	exports := &fakeExports{targets: sampleTargets(), volumes: sampleVolumes()}
	h, m, init := openPopup(exports)
	exports.targets = exports.targets[1:] // the first one deleted
	reload := m.ReloadTargets()

	for _, msg := range init {
		if _, ok := msg.(TargetsLoadedMsg); ok {
			h.AssertDrops(t, msg)
			continue
		}
		h.SendMsg(msg)
	}
	h.ExecuteAndSend(reload)

	names := make([]string, 0, len(m.targets))
	for _, target := range m.targets {
		names = append(names, target.Name)
	}
	if !slices.Equal(names, []string{"Phone", "Custom Folder"}) {
		t.Errorf("targets = %q, want the ones read again", names)
	}
}

// === Folders of a device ===

// mount is a device mounted at a temporary folder, with the folders dirs.
func mount(t *testing.T, uuid, label string, dirs ...string) core.ExportVolume {
	t.Helper()
	root := t.TempDir()
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	return core.ExportVolume{UUID: uuid, Label: label, MountPath: root}
}

// newTargetPopup is a popup on the device list of a new target, with two
// devices: X, with the folders Alpha and Beta, and Y, with Gamma.
func newTargetPopup(t *testing.T) (*testutil.PopupHarness, *Model) {
	t.Helper()
	exports := &fakeExports{volumes: []core.ExportVolume{
		mount(t, "uuid-x", "X", "Alpha", "Beta", "Alpha/Inner", "Beta/Deep"),
		mount(t, "uuid-y", "Y", "Gamma"),
	}}
	h, m := openedPopup(exports)
	h.SendEnter() // "New target", the only row
	if m.state != StateNewTarget {
		t.Fatalf("state = %d, want the device list", m.state)
	}
	return h, m
}

// pickDevice picks the device at row i of the device list: it returns the
// listing of its root, not delivered.
func pickDevice(t *testing.T, h *testutil.PopupHarness, m *Model, i int) tea.Cmd {
	t.Helper()
	for range i {
		h.SendDown()
	}
	cmd := h.SendEnter()
	if m.state != StateNewTargetFolder || cmd == nil {
		t.Fatalf("state = %d, want the folders of the device loading", m.state)
	}
	return cmd
}

// #310: the folders of a device left with Esc are not shown under another.
func TestExport_TheFoldersOfADeviceLeftAreNotShownUnderAnother(t *testing.T) {
	h, m := newTargetPopup(t)
	x := pickDevice(t, h, m, 0)
	h.SendEscape()
	left := testutil.ExecuteCmd(x)
	h.AssertDrops(t, left)

	y := pickDevice(t, h, m, 1)
	h.AssertDrops(t, left)
	h.ExecuteAndSend(y)

	view := testutil.StripANSI(h.View())
	if !strings.Contains(view, "Device: Y") || !strings.Contains(view, "Gamma") || strings.Contains(view, "Alpha") {
		t.Errorf("view:\n%s\nwant the folders of Y only", view)
	}
}

// #310: of two listings in flight in one device, only the last one asked is
// shown, whatever order they come in.
func TestExport_OnlyTheLastListingAskedIsShown(t *testing.T) {
	for name, lastFirst := range map[string]bool{"in order": false, "last first": true} {
		t.Run(name, func(t *testing.T) {
			h, m := newTargetPopup(t)
			h.ExecuteAndSend(pickDevice(t, h, m, 0))
			alpha := h.SendEnter() // into Alpha
			h.SendDown()
			beta := h.SendEnter() // into Beta, Alpha not listed yet
			earlier, later := testutil.ExecuteCmd(alpha), testutil.ExecuteCmd(beta)

			if lastFirst {
				h.SendMsg(later)
				h.AssertDrops(t, earlier)
			} else {
				h.AssertDrops(t, earlier)
				h.SendMsg(later)
			}

			if m.currentPath != "/Beta" || !slices.Equal(m.directories, []string{"Deep"}) {
				t.Errorf("path %q with %q, want /Beta with Deep", m.currentPath, m.directories)
			}
			h.AssertDrops(t, later)
		})
	}
}

// A popup drops the listing of a popup closed since, and its failure.
func TestExport_DropsTheListingOfAClosedPopup(t *testing.T) {
	closedH, closedM := newTargetPopup(t)
	listing := testutil.ExecuteCmd(pickDevice(t, closedH, closedM, 1))
	failed := failedListing(t)

	h, m := newTargetPopup(t)
	own := pickDevice(t, h, m, 0)
	h.AssertDrops(t, listing)
	h.AssertDrops(t, failed)
	h.ExecuteAndSend(own)
	if !slices.Equal(m.directories, []string{"Alpha", "Beta"}) {
		t.Errorf("folders = %q, want those of X", m.directories)
	}
	h.AssertDrops(t, listing)
	h.AssertDrops(t, failed)
}

// failedListing is the listing of the root of a device unplugged since it
// was picked, by a popup closed before it came.
func failedListing(t *testing.T) tea.Msg {
	t.Helper()
	gone := mount(t, "uuid-z", "Z")
	h, m := openedPopup(&fakeExports{volumes: []core.ExportVolume{gone}})
	h.SendEnter()
	if err := os.Remove(gone.MountPath); err != nil {
		t.Fatal(err)
	}
	msg, ok := testutil.ExecuteCmd(pickDevice(t, h, m, 0)).(DirectoriesLoadedMsg)
	if !ok || msg.Err == nil {
		t.Fatalf("listing = %#v, want a failure", msg)
	}
	return msg
}

// === A new target ===

// savingPopup is a popup on exports whose user went through the screens of a
// new target, a custom folder, and pressed Enter on the last: it returns the
// answers of its Init and the save, not delivered.
func savingPopup(t *testing.T, exports *fakeExports) (*testutil.PopupHarness, *Model, []tea.Msg, tea.Cmd) {
	t.Helper()
	h, m, init := openPopup(exports)
	h.SendEnter() // "New target", the only row before the targets are read
	h.SendEnter() // "Custom folder", the only row before the volumes are read
	for _, r := range "/tmp/saved" {
		h.SendKey(string(r))
	}
	h.SendEnter()
	save := h.SendEnter()
	if save == nil {
		t.Fatalf("state = %d, want the target saving", m.state)
	}
	return h, m, init, save
}

// #310: a popup that saved a target lists it once, selected, with the other
// saved targets, whatever order its targets read and its save come in.
func TestExport_ASavedTargetIsListedOnceAndSelected(t *testing.T) {
	orders := map[string]func(h *testutil.PopupHarness, read tea.Msg, save tea.Cmd){
		"read, then save": func(h *testutil.PopupHarness, read tea.Msg, save tea.Cmd) {
			h.SendMsg(read)
			_, reread := h.ExecuteAndSend(save)
			h.ExecuteAndSend(reread)
		},
		"save, then read": func(h *testutil.PopupHarness, read tea.Msg, save tea.Cmd) {
			_, reread := h.ExecuteAndSend(save)
			h.SendMsg(read)
			h.ExecuteAndSend(reread)
		},
		"save, read again, then read": func(h *testutil.PopupHarness, read tea.Msg, save tea.Cmd) {
			_, reread := h.ExecuteAndSend(save)
			h.ExecuteAndSend(reread)
			h.SendMsg(read)
		},
	}
	for name, deliver := range orders {
		t.Run(name, func(t *testing.T) {
			exports := &fakeExports{targets: sampleTargets(), volumes: sampleVolumes()}
			h, m, init, save := savingPopup(t, exports)
			var read tea.Msg
			for _, msg := range init {
				if _, ok := msg.(TargetsLoadedMsg); ok {
					read = msg
					continue
				}
				h.SendMsg(msg) // the volumes
			}

			deliver(h, read, save)

			if m.state != StateSelectTarget {
				t.Fatalf("state = %d, want the targets", m.state)
			}
			assertSelected(t, h, m, "saved", "USB Drive", "Phone", "Custom Folder", "saved")
		})
	}
}

// A save delivered again adds nothing.
func TestExport_ASaveDeliveredTwiceIsTakenOnce(t *testing.T) {
	exports := &fakeExports{targets: sampleTargets(), volumes: sampleVolumes()}
	h, m, init, save := savingPopup(t, exports)
	for _, msg := range init {
		h.SendMsg(msg)
	}
	saved, reread := h.ExecuteAndSend(save)
	h.ExecuteAndSend(reread)

	h.AssertDrops(t, saved)
	assertSelected(t, h, m, "saved", "USB Drive", "Phone", "Custom Folder", "saved")
}

// #310: the save of a popup closed since adds nothing, and leaves the screen
// shown.
func TestExport_DropsTheSaveOfAClosedPopup(t *testing.T) {
	closed := map[string]tea.Msg{}
	for name, err := range map[string]error{"saved": nil, "failed": errors.New("database locked")} {
		_, _, _, save := savingPopup(t, &fakeExports{createErr: err})
		closed[name] = testutil.ExecuteCmd(save)
	}
	screens := map[string]func(t *testing.T) (*testutil.PopupHarness, *Model){
		"targets": func(*testing.T) (*testutil.PopupHarness, *Model) {
			return openedPopup(&fakeExports{targets: sampleTargets(), volumes: sampleVolumes()})
		},
		"device list": newTargetPopup,
		"folder structure": func(t *testing.T) (*testutil.PopupHarness, *Model) {
			t.Helper()
			h, m := newTargetPopup(t)
			h.ExecuteAndSend(pickDevice(t, h, m, 0))
			h.SendKey(" ")
			return h, m
		},
	}
	for screen, open := range screens {
		for name, msg := range closed {
			t.Run(screen+"/"+name, func(t *testing.T) {
				h, m := open(t)
				state, targets := m.state, len(m.targets)
				h.AssertDrops(t, msg)
				if m.state != state || len(m.targets) != targets {
					t.Errorf("state %d with %d targets, want %d with %d", m.state, len(m.targets), state, targets)
				}
			})
		}
	}
}

// === Failures ===

// assertShows checks that the view of the popup has a line with want.
func assertShows(t *testing.T, h *testutil.PopupHarness, want string) {
	t.Helper()
	if view := testutil.StripANSI(h.View()); !strings.Contains(view, want) {
		t.Errorf("view:\n%s\nwant a line with %q", view, want)
	}
}

// #309: targets or devices the core cannot read are said, not shown as none.
func TestExport_ShowsTheTargetsAndDevicesItCannotRead(t *testing.T) {
	h, _ := openedPopup(&fakeExports{
		targetsErr: errors.New("database locked"),
		volumesErr: errors.New("udisks unavailable"),
	})
	assertShows(t, h, "Failed to load export targets: database locked")
	assertShows(t, h, "Failed to detect volumes: udisks unavailable")

	h.SendEnter() // "New target", the only row
	assertShows(t, h, "udisks unavailable")
	if strings.Contains(testutil.StripANSI(h.View()), "No removable devices detected") {
		t.Errorf("view:\n%s\nwant no claim that there is no device", testutil.StripANSI(h.View()))
	}
}

// #309: a new target the core cannot save is said, on the screen saved from,
// and Esc still goes back.
func TestExport_ShowsATargetItCannotSave(t *testing.T) {
	h, m, init, save := savingPopup(t, &fakeExports{createErr: errors.New("disk full")})
	for _, msg := range init {
		h.SendMsg(msg)
	}
	h.ExecuteAndSend(save)

	if m.state != StateCustomFolderConfig {
		t.Fatalf("state = %d, want the screen saved from", m.state)
	}
	assertShows(t, h, "Failed to save export target: disk full")

	h.SendEscape()
	if m.state != StateCustomFolder {
		t.Errorf("state = %d, want Esc to go back", m.state)
	}
	if strings.Contains(testutil.StripANSI(h.View()), "disk full") {
		t.Errorf("view:\n%s\nwant the failure gone with its screen", testutil.StripANSI(h.View()))
	}
}

// #309: a folder that cannot be listed is said, and the browser stays where
// it was, usable.
func TestExport_ShowsAFolderItCannotList(t *testing.T) {
	h, m := newTargetPopup(t)
	h.ExecuteAndSend(pickDevice(t, h, m, 0))
	if err := os.RemoveAll(filepath.Join(m.mountPath, "Beta")); err != nil {
		t.Fatal(err)
	}
	h.SendDown()
	h.ExecuteAndSend(h.SendEnter()) // into Beta, gone

	if m.currentPath != "/" || m.dirIdx != 1 {
		t.Errorf("path %q, row %d: want the browser where it was, on Beta", m.currentPath, m.dirIdx)
	}
	assertShows(t, h, "Failed to list folder '/Beta'")
	assertShows(t, h, "no such file or directory")

	h.SendUp()
	h.ExecuteAndSend(h.SendEnter()) // into Alpha
	if m.currentPath != "/Alpha" {
		t.Errorf("path = %q, want /Alpha", m.currentPath)
	}
	if strings.Contains(testutil.StripANSI(h.View()), "no such file or directory") {
		t.Errorf("view:\n%s\nwant the failure gone once a folder is listed", testutil.StripANSI(h.View()))
	}
}

// #309: a folder picked with Space leaves the browser: a listing asked
// before, failed, is not said on the next screen.
func TestExport_DropsAListingAskedBeforeTheFolderWasPicked(t *testing.T) {
	h, m := newTargetPopup(t)
	h.ExecuteAndSend(pickDevice(t, h, m, 0))
	if err := os.RemoveAll(filepath.Join(m.mountPath, "Alpha")); err != nil {
		t.Fatal(err)
	}
	listing := testutil.ExecuteCmd(h.SendEnter()) // into Alpha, gone
	h.SendKey(" ")
	if m.state != StateNewTargetConfig {
		t.Fatalf("state = %d, want the folder structure", m.state)
	}

	h.AssertDrops(t, listing)
}

// #337: a symbolic link to a folder is listed as a folder, and entered under
// its own name; a link to a file or to nothing is not.
func TestExport_ListsASymbolicLinkToAFolder(t *testing.T) {
	device := mount(t, "uuid-l", "L", "real/sub")
	for link, target := range map[string]string{
		"link":     "real",
		"dangling": "missing",
		"file":     "song.mp3",
	} {
		if err := os.Symlink(target, filepath.Join(device.MountPath, link)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(device.MountPath, "song.mp3"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	h, m := openedPopup(&fakeExports{volumes: []core.ExportVolume{device}})
	h.SendEnter() // "New target", the only row
	h.ExecuteAndSend(pickDevice(t, h, m, 0))

	if !slices.Equal(m.directories, []string{"link", "real"}) {
		t.Fatalf("folders = %q, want link and real", m.directories)
	}

	h.ExecuteAndSend(h.SendEnter()) // into link
	if m.currentPath != "/link" || !slices.Equal(m.directories, []string{"sub"}) {
		t.Errorf("path %q with %q, want /link with sub", m.currentPath, m.directories)
	}
}
