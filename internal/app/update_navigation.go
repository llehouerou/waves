// internal/app/update_navigation.go
package app

import tea "github.com/charmbracelet/bubbletea"

// handleNavigationMsg routes navigation-related messages.
func (m Model) handleNavigationMsg(msg NavigationMessage) (tea.Model, tea.Cmd) {
	if scanMsg, ok := msg.(ScanResultMsg); ok {
		return m.handleScanResult(scanMsg)
	}
	return m, nil
}

// handleScanResult processes directory scan results for deep search.
func (m Model) handleScanResult(msg ScanResultMsg) (tea.Model, tea.Cmd) {
	m.Input.UpdateScanResults(msg.Items, !msg.Done)
	if !msg.Done {
		return m, m.waitForScan()
	}
	return m, nil
}
