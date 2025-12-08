// internal/app/popup_manager.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/ui/confirm"
	"github.com/llehouerou/waves/internal/ui/helpbindings"
	"github.com/llehouerou/waves/internal/ui/librarysources"
	"github.com/llehouerou/waves/internal/ui/popup"
	"github.com/llehouerou/waves/internal/ui/scanreport"
	"github.com/llehouerou/waves/internal/ui/textinput"
)

// PopupType identifies which popup is currently active.
type PopupType int

const (
	PopupNone PopupType = iota
	PopupHelp
	PopupConfirm
	PopupTextInput
	PopupLibrarySources
	PopupScanReport
	PopupError
)

// PopupManager manages all modal popups and overlays.
type PopupManager struct {
	help           helpbindings.Model
	showHelp       bool
	confirm        confirm.Model
	textInput      textinput.Model
	inputMode      InputMode
	librarySources librarysources.Model
	showLibSources bool
	scanReport     *scanreport.Model
	errorMsg       string

	// Dimensions for popup rendering
	width  int
	height int
}

// NewPopupManager creates a new PopupManager with initialized components.
func NewPopupManager() PopupManager {
	return PopupManager{
		help:           helpbindings.New(),
		confirm:        confirm.New(),
		textInput:      textinput.New(),
		librarySources: librarysources.New(),
	}
}

// SetSize updates the dimensions for popup rendering.
func (p *PopupManager) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// ActivePopup returns which popup is currently active (if any).
func (p *PopupManager) ActivePopup() PopupType {
	// Check in priority order (error has highest priority)
	if p.errorMsg != "" {
		return PopupError
	}
	if p.scanReport != nil {
		return PopupScanReport
	}
	if p.showHelp {
		return PopupHelp
	}
	if p.confirm.Active() {
		return PopupConfirm
	}
	if p.inputMode != InputNone {
		return PopupTextInput
	}
	if p.showLibSources {
		return PopupLibrarySources
	}
	return PopupNone
}

// HasActivePopup returns true if any popup is visible.
func (p *PopupManager) HasActivePopup() PopupType {
	return p.ActivePopup()
}

// --- Help Popup ---

// ShowHelp displays the help popup with the given contexts.
func (p *PopupManager) ShowHelp(contexts []string) {
	p.help.SetContexts(contexts)
	p.help.SetSize(p.width, p.height)
	p.showHelp = true
}

// HideHelp hides the help popup.
func (p *PopupManager) HideHelp() {
	p.showHelp = false
}

// IsHelpVisible returns true if the help popup is visible.
func (p *PopupManager) IsHelpVisible() bool {
	return p.showHelp
}

// Help returns the help popup model for direct access.
func (p *PopupManager) Help() *helpbindings.Model {
	return &p.help
}

// --- Confirm Popup ---

// ShowConfirm displays a confirmation dialog.
func (p *PopupManager) ShowConfirm(title, message string, context any) {
	p.confirm.Show(title, message, context, p.width, p.height)
}

// ShowConfirmWithOptions displays a confirmation dialog with custom options.
func (p *PopupManager) ShowConfirmWithOptions(title, message string, options []string, context any) {
	p.confirm.ShowWithOptions(title, message, options, context, p.width, p.height)
}

// HideConfirm hides the confirmation popup.
func (p *PopupManager) HideConfirm() {
	p.confirm.Reset()
}

// IsConfirmVisible returns true if the confirmation popup is visible.
func (p *PopupManager) IsConfirmVisible() bool {
	return p.confirm.Active()
}

// Confirm returns the confirm model for direct access.
func (p *PopupManager) Confirm() *confirm.Model {
	return &p.confirm
}

// --- Text Input Popup ---

// ShowTextInput displays a text input popup.
func (p *PopupManager) ShowTextInput(mode InputMode, title, value string, context any) {
	p.inputMode = mode
	p.textInput.Start(title, value, context, p.width, p.height)
}

// HideTextInput hides the text input popup.
func (p *PopupManager) HideTextInput() {
	p.inputMode = InputNone
	p.textInput.Reset()
}

// IsTextInputVisible returns true if the text input popup is visible.
func (p *PopupManager) IsTextInputVisible() bool {
	return p.inputMode != InputNone
}

// InputMode returns the current input mode.
func (p *PopupManager) InputMode() InputMode {
	return p.inputMode
}

// TextInput returns the text input model for direct access.
func (p *PopupManager) TextInput() *textinput.Model {
	return &p.textInput
}

// --- Library Sources Popup ---

// ShowLibrarySources displays the library sources popup.
func (p *PopupManager) ShowLibrarySources(sources []string) {
	p.librarySources.SetSources(sources)
	p.librarySources.SetSize(p.width, p.height)
	p.showLibSources = true
}

// HideLibrarySources hides the library sources popup.
func (p *PopupManager) HideLibrarySources() {
	p.showLibSources = false
	p.librarySources.Reset()
}

// IsLibrarySourcesVisible returns true if the library sources popup is visible.
func (p *PopupManager) IsLibrarySourcesVisible() bool {
	return p.showLibSources
}

// LibrarySources returns the library sources model for direct access.
func (p *PopupManager) LibrarySources() *librarysources.Model {
	return &p.librarySources
}

// --- Scan Report Popup ---

// ShowScanReport displays the scan report popup.
func (p *PopupManager) ShowScanReport(report scanreport.Model) {
	p.scanReport = &report
}

// HideScanReport hides the scan report popup.
func (p *PopupManager) HideScanReport() {
	p.scanReport = nil
}

// IsScanReportVisible returns true if the scan report popup is visible.
func (p *PopupManager) IsScanReportVisible() bool {
	return p.scanReport != nil
}

// ScanReport returns the scan report model (may be nil).
func (p *PopupManager) ScanReport() *scanreport.Model {
	return p.scanReport
}

// --- Error Popup ---

// ShowError displays an error message popup.
func (p *PopupManager) ShowError(msg string) {
	p.errorMsg = msg
}

// HideError hides the error popup.
func (p *PopupManager) HideError() {
	p.errorMsg = ""
}

// IsErrorVisible returns true if the error popup is visible.
func (p *PopupManager) IsErrorVisible() bool {
	return p.errorMsg != ""
}

// ErrorMsg returns the current error message.
func (p *PopupManager) ErrorMsg() string {
	return p.errorMsg
}

// --- Key Handling ---

// HandleKey routes key events to the active popup.
// Returns (handled, cmd) where handled is true if a popup consumed the key.
func (p *PopupManager) HandleKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	key := msg.String()

	// Handle error overlay - any key dismisses it
	if p.errorMsg != "" {
		p.errorMsg = ""
		return true, nil
	}

	// Handle scan report popup - Enter/Escape dismisses it
	if p.scanReport != nil {
		if key == "enter" || key == "escape" {
			p.scanReport = nil
		}
		return true, nil
	}

	// Handle help popup
	if p.showHelp {
		var cmd tea.Cmd
		p.help, cmd = p.help.Update(msg)
		return true, cmd
	}

	// Handle confirmation dialog
	if p.confirm.Active() {
		var cmd tea.Cmd
		p.confirm, cmd = p.confirm.Update(msg)
		return true, cmd
	}

	// Handle text input mode
	if p.inputMode != InputNone {
		var cmd tea.Cmd
		p.textInput, cmd = p.textInput.Update(msg)
		return true, cmd
	}

	// Handle library sources popup
	if p.showLibSources {
		var cmd tea.Cmd
		p.librarySources, cmd = p.librarySources.Update(msg)
		return true, cmd
	}

	return false, nil
}

// --- Rendering ---

// RenderOverlay renders active popup(s) on top of the base view.
func (p *PopupManager) RenderOverlay(base string) string {
	// Overlay text input popup if active
	if p.inputMode != InputNone {
		inputView := p.textInput.View()
		base = popup.Compose(base, inputView, p.width, p.height)
	}

	// Overlay confirmation popup if active
	if p.confirm.Active() {
		confirmView := p.confirm.View()
		base = popup.Compose(base, confirmView, p.width, p.height)
	}

	// Overlay library sources popup if active
	if p.showLibSources {
		sourcesView := p.librarySources.View()
		base = popup.Compose(base, sourcesView, p.width, p.height)
	}

	// Overlay error popup if present
	if p.errorMsg != "" {
		errorView := p.renderError()
		base = popup.Compose(base, errorView, p.width, p.height)
	}

	// Overlay scan report popup if present
	if p.scanReport != nil {
		reportView := p.scanReport.Render()
		base = popup.Compose(base, reportView, p.width, p.height)
	}

	// Overlay help popup if active
	if p.showHelp {
		helpView := p.help.View()
		base = popup.Compose(base, helpView, p.width, p.height)
	}

	return base
}

func (p *PopupManager) renderError() string {
	pop := popup.New()
	pop.Title = "Error"
	pop.Content = p.errorMsg
	pop.Footer = "Press any key to dismiss"
	return pop.Render(p.width, p.height)
}
