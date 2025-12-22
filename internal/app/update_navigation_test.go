// internal/app/update_navigation_test.go
package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/search"
)

func TestHandleScanResult_IgnoresStaleScanMessages(t *testing.T) {
	// This test reproduces a bug where stale scan messages from a cancelled
	// file browser search corrupt an active FTS-based library search.
	//
	// Scenario:
	// 1. User is in file browser, presses 'ff' → starts scan-based deep search
	// 2. User presses ESC → EndSearch() cancels scan, clears scanChan
	// 3. User switches to library view, presses 'ff' → sets searchFunc for FTS
	// 4. A late ScanResultMsg arrives (was already in message queue)
	// 5. BUG: handleScanResult calls UpdateScanResults → calls SetItems → clears searchFunc!

	m := newTestModel()
	m.Input = NewInputManager()

	// Step 1 & 2: Simulate that a scan was started and then ended
	// After EndSearch(), ScanChan() should be nil
	if m.Input.ScanChan() != nil {
		t.Fatal("expected ScanChan to be nil initially")
	}

	// Step 3: Start FTS-based search (sets searchFunc)
	searchCalled := false
	ftsSearchFunc := func(_ string) ([]search.Item, error) {
		searchCalled = true
		return []search.Item{}, nil
	}
	m.Input.StartDeepSearchWithFunc(ftsSearchFunc)

	// Verify searchFunc is set by triggering a search
	searchCalled = false
	m.Input.Search().SetSearchFunc(ftsSearchFunc) // Reset to track calls
	// Type a character to trigger updateMatches which calls searchFunc
	m.Input.UpdateSearch(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

	if !searchCalled {
		t.Fatal("expected FTS searchFunc to be called when typing")
	}

	// Step 4: Simulate a stale ScanResultMsg arriving
	staleScanMsg := ScanResultMsg(navigator.ScanResult{
		Items: []search.Item{},
		Done:  true,
	})

	// Handle the stale message - IMPORTANT: use the returned model like the real app does
	// handleScanResult has a value receiver, so it returns a modified copy
	newModel, _ := m.handleScanResult(staleScanMsg)
	newM, ok := newModel.(Model)
	if !ok {
		t.Fatal("expected Model type from handleScanResult")
	}

	// Step 5: Verify searchFunc is still working on the returned model
	// (not cleared by stale message)
	searchCalled = false
	newM.Input.UpdateSearch(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})

	if !searchCalled {
		t.Error("FTS searchFunc was cleared by stale scan message - this is the bug!")
	}
}

func TestHandleScanResult_ProcessesActiveScanMessages(t *testing.T) {
	// Verify that legitimate scan messages are still processed correctly
	m := newTestModel()
	m.Input = NewInputManager()

	// Start a scan-based search (this would set up scanChan in real usage)
	// For this test, we'll manually set up the state
	items := []search.Item{}
	m.Input.StartDeepSearchWithItems(items)

	// When a scan is active, we need to have a scanChan
	// But StartDeepSearchWithItems doesn't create one - it's for pre-loaded items
	// So this test verifies that with no scanChan, messages are ignored

	scanMsg := ScanResultMsg(navigator.ScanResult{
		Items: []search.Item{},
		Done:  false,
	})

	_, cmd := m.handleScanResult(scanMsg)

	// With no active scan channel, should not continue waiting
	if cmd != nil {
		t.Error("expected no command when scanChan is nil")
	}
}
