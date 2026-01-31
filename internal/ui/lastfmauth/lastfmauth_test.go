package lastfmauth

import (
	"testing"

	"github.com/llehouerou/waves/internal/state"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

func newTestPopup() *testutil.PopupHarness {
	m := New()
	m.SetSize(80, 24)
	return testutil.NewPopupHarness(&m)
}

func newLinkedPopup(username string) *testutil.PopupHarness {
	m := New()
	m.SetSession(&state.LastfmSession{Username: username})
	m.SetSize(80, 24)
	return testutil.NewPopupHarness(&m)
}

func newWaitingPopup() *testutil.PopupHarness {
	m := New()
	m.SetWaitingCallback()
	m.SetSize(80, 24)
	return testutil.NewPopupHarness(&m)
}

func newErrorPopup(errMsg string) *testutil.PopupHarness {
	m := New()
	m.SetError(errMsg)
	m.SetSize(80, 24)
	return testutil.NewPopupHarness(&m)
}

func getAction(t *testing.T, h *testutil.PopupHarness) Action {
	t.Helper()
	cmd := h.LastCommand()
	if cmd == nil {
		t.Fatal("expected command, got nil")
	}
	msg := testutil.ExecuteCmd(cmd)
	actionMsg, ok := msg.(ActionMsg)
	if !ok {
		t.Fatalf("expected ActionMsg, got %T", msg)
	}
	return actionMsg.Action
}

// Not linked state tests

func TestNotLinked_StartAuth(t *testing.T) {
	h := newTestPopup()

	h.SendEnter()

	act := getAction(t, h)
	if act != ActionStartAuth {
		t.Errorf("Action = %v, want ActionStartAuth", act)
	}
}

func TestNotLinked_Close(t *testing.T) {
	h := newTestPopup()

	h.SendEscape()

	act := getAction(t, h)
	if act != ActionClose {
		t.Errorf("Action = %v, want ActionClose", act)
	}
}

func TestNotLinked_View(t *testing.T) {
	h := newTestPopup()

	if err := h.AssertViewContains("Not linked"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("Link"); err != "" {
		t.Error(err)
	}
}

// Waiting callback state tests

func TestWaiting_ConfirmAuth(t *testing.T) {
	h := newWaitingPopup()

	h.SendEnter()

	act := getAction(t, h)
	if act != ActionConfirmAuth {
		t.Errorf("Action = %v, want ActionConfirmAuth", act)
	}
}

func TestWaiting_Cancel(t *testing.T) {
	h := newWaitingPopup()

	h.SendEscape()

	act := getAction(t, h)
	if act != ActionClose {
		t.Errorf("Action = %v, want ActionClose", act)
	}
}

func TestWaiting_View(t *testing.T) {
	h := newWaitingPopup()

	if err := h.AssertViewContains("Authorizing"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("browser"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("I've authorized"); err != "" {
		t.Error(err)
	}
}

// Linked state tests

func TestLinked_Unlink(t *testing.T) {
	h := newLinkedPopup("testuser")

	h.SendKey("u")

	act := getAction(t, h)
	if act != ActionUnlink {
		t.Errorf("Action = %v, want ActionUnlink", act)
	}
}

func TestLinked_UnlinkUppercase(t *testing.T) {
	h := newLinkedPopup("testuser")

	h.SendKey("U")

	act := getAction(t, h)
	if act != ActionUnlink {
		t.Errorf("Action = %v, want ActionUnlink", act)
	}
}

func TestLinked_Close(t *testing.T) {
	h := newLinkedPopup("testuser")

	h.SendEscape()

	act := getAction(t, h)
	if act != ActionClose {
		t.Errorf("Action = %v, want ActionClose", act)
	}
}

func TestLinked_View(t *testing.T) {
	h := newLinkedPopup("musicfan")

	if err := h.AssertViewContains("Linked"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("musicfan"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("Scrobbling"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("Unlink"); err != "" {
		t.Error(err)
	}
}

// Error state tests

func TestError_Retry(t *testing.T) {
	h := newErrorPopup("Connection failed")

	h.SendEnter()

	act := getAction(t, h)
	if act != ActionStartAuth {
		t.Errorf("Action = %v, want ActionStartAuth", act)
	}
}

func TestError_Close(t *testing.T) {
	h := newErrorPopup("Connection failed")

	h.SendEscape()

	act := getAction(t, h)
	if act != ActionClose {
		t.Errorf("Action = %v, want ActionClose", act)
	}
}

func TestError_View(t *testing.T) {
	h := newErrorPopup("API rate limit exceeded")

	if err := h.AssertViewContains("Error"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("API rate limit exceeded"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("Retry"); err != "" {
		t.Error(err)
	}
}

// State transition tests

func TestSetSession_ToLinked(t *testing.T) {
	m := New()
	h := testutil.NewPopupHarness(&m)

	// Initially not linked
	if err := h.AssertViewContains("Not linked"); err != "" {
		t.Error(err)
	}

	// Set session
	m.SetSession(&state.LastfmSession{Username: "newuser"})

	if err := h.AssertViewContains("Linked"); err != "" {
		t.Error(err)
	}
	if err := h.AssertViewContains("newuser"); err != "" {
		t.Error(err)
	}
}

func TestSetSession_ToNotLinked(t *testing.T) {
	m := New()
	m.SetSession(&state.LastfmSession{Username: "olduser"})
	h := testutil.NewPopupHarness(&m)

	// Initially linked
	if err := h.AssertViewContains("Linked"); err != "" {
		t.Error(err)
	}

	// Clear session
	m.SetSession(nil)

	if err := h.AssertViewContains("Not linked"); err != "" {
		t.Error(err)
	}
}

func TestSetWaitingCallback(t *testing.T) {
	m := New()
	h := testutil.NewPopupHarness(&m)

	m.SetWaitingCallback()

	if err := h.AssertViewContains("Authorizing"); err != "" {
		t.Error(err)
	}
}

func TestSetError_ClearsOnNewSession(t *testing.T) {
	m := New()
	m.SetError("Some error")
	h := testutil.NewPopupHarness(&m)

	// Error shown
	if err := h.AssertViewContains("Error"); err != "" {
		t.Error(err)
	}

	// Session set clears error
	m.SetSession(&state.LastfmSession{Username: "user"})

	if err := h.AssertViewNotContains("Error"); err != "" {
		t.Error(err)
	}
}

// Title test

func TestView_ShowsTitle(t *testing.T) {
	h := newTestPopup()

	if err := h.AssertViewContains("Last.fm Settings"); err != "" {
		t.Error(err)
	}
}

// Unhandled keys test

func TestUnhandledKeys_NoAction(t *testing.T) {
	h := newTestPopup()
	h.ClearCommands()

	h.SendKey("x")
	h.SendKey("q")
	h.SendKey("j")

	if len(h.Commands()) != 0 {
		t.Error("unhandled keys should not produce commands")
	}
}
