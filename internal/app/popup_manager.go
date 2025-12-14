// internal/app/popup_manager.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/download"
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
	PopupDownload
)

// PopupManager manages all modal popups and overlays.
type PopupManager struct {
	help           helpbindings.Model
	confirm        confirm.Model
	textInput      textinput.Model
	librarySources librarysources.Model
	scanReport     *scanreport.Model
	download       *download.Model
	errorMsg       string
	inputMode      InputMode

	visible map[PopupType]bool
	width   int
	height  int
}

// NewPopupManager creates a new PopupManager with initialized components.
func NewPopupManager() PopupManager {
	return PopupManager{
		help:           helpbindings.New(),
		confirm:        confirm.New(),
		textInput:      textinput.New(),
		librarySources: librarysources.New(),
		visible:        make(map[PopupType]bool),
	}
}

// SetSize updates the dimensions for popup rendering.
func (p *PopupManager) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// IsVisible returns true if the specified popup type is visible.
func (p *PopupManager) IsVisible(t PopupType) bool {
	switch t {
	case PopupNone:
		return false
	case PopupHelp:
		return p.visible[PopupHelp]
	case PopupConfirm:
		return p.confirm.Active()
	case PopupTextInput:
		return p.inputMode != InputNone
	case PopupLibrarySources:
		return p.visible[PopupLibrarySources]
	case PopupScanReport:
		return p.scanReport != nil
	case PopupError:
		return p.errorMsg != ""
	case PopupDownload:
		return p.download != nil
	}
	return false
}

// ActivePopup returns which popup is currently active (highest priority).
func (p *PopupManager) ActivePopup() PopupType {
	// Check in priority order (error has highest priority)
	if p.IsVisible(PopupError) {
		return PopupError
	}
	if p.IsVisible(PopupScanReport) {
		return PopupScanReport
	}
	if p.IsVisible(PopupHelp) {
		return PopupHelp
	}
	if p.IsVisible(PopupConfirm) {
		return PopupConfirm
	}
	if p.IsVisible(PopupTextInput) {
		return PopupTextInput
	}
	if p.IsVisible(PopupLibrarySources) {
		return PopupLibrarySources
	}
	if p.IsVisible(PopupDownload) {
		return PopupDownload
	}
	return PopupNone
}

// Hide hides the specified popup type.
func (p *PopupManager) Hide(t PopupType) {
	switch t {
	case PopupNone:
		// Nothing to hide
	case PopupHelp:
		p.visible[PopupHelp] = false
	case PopupConfirm:
		p.confirm.Reset()
	case PopupTextInput:
		p.inputMode = InputNone
		p.textInput.Reset()
	case PopupLibrarySources:
		p.visible[PopupLibrarySources] = false
		p.librarySources.Reset()
	case PopupScanReport:
		p.scanReport = nil
	case PopupError:
		p.errorMsg = ""
	case PopupDownload:
		if p.download != nil {
			p.download.Reset()
		}
		p.download = nil
	}
}

// --- Show Methods ---

// ShowHelp displays the help popup with the given contexts.
func (p *PopupManager) ShowHelp(contexts []string) {
	p.help.SetContexts(contexts)
	p.help.SetSize(p.width, p.height)
	p.visible[PopupHelp] = true
}

// ShowConfirm displays a confirmation dialog.
func (p *PopupManager) ShowConfirm(title, message string, context any) {
	p.confirm.Show(title, message, context, p.width, p.height)
}

// ShowConfirmWithOptions displays a confirmation dialog with custom options.
func (p *PopupManager) ShowConfirmWithOptions(title, message string, options []string, context any) {
	p.confirm.ShowWithOptions(title, message, options, context, p.width, p.height)
}

// ShowTextInput displays a text input popup.
func (p *PopupManager) ShowTextInput(mode InputMode, title, value string, context any) {
	p.inputMode = mode
	p.textInput.Start(title, value, context, p.width, p.height)
}

// ShowLibrarySources displays the library sources popup.
func (p *PopupManager) ShowLibrarySources(sources []string) {
	p.librarySources.SetSources(sources)
	p.librarySources.SetSize(p.width, p.height)
	p.visible[PopupLibrarySources] = true
}

// ShowScanReport displays the scan report popup.
func (p *PopupManager) ShowScanReport(report scanreport.Model) {
	p.scanReport = &report
}

// ShowDownload displays the download popup.
func (p *PopupManager) ShowDownload(slskdURL, slskdAPIKey string) tea.Cmd {
	p.download = download.New(slskdURL, slskdAPIKey)
	// Size: 80% width, 70% height
	popupWidth := p.width * 80 / 100
	popupHeight := p.height * 70 / 100
	p.download.SetSize(popupWidth, popupHeight)
	p.download.SetFocused(true)
	return p.download.Init()
}

// ShowError displays an error message popup.
func (p *PopupManager) ShowError(msg string) {
	p.errorMsg = msg
}

// --- Accessors ---

// Help returns the help popup model for direct access.
func (p *PopupManager) Help() *helpbindings.Model {
	return &p.help
}

// LibrarySources returns the library sources popup model for direct access.
func (p *PopupManager) LibrarySources() *librarysources.Model {
	return &p.librarySources
}

// Download returns the download popup model for direct access.
func (p *PopupManager) Download() *download.Model {
	return p.download
}

// InputMode returns the current input mode.
func (p *PopupManager) InputMode() InputMode {
	return p.inputMode
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

	if p.IsVisible(PopupError) {
		p.Hide(PopupError)
		return true, nil
	}

	if p.IsVisible(PopupScanReport) {
		if key == "enter" || key == "escape" {
			p.Hide(PopupScanReport)
		}
		return true, nil
	}

	if p.IsVisible(PopupHelp) {
		var cmd tea.Cmd
		p.help, cmd = p.help.Update(msg)
		return true, cmd
	}

	if p.IsVisible(PopupConfirm) {
		var cmd tea.Cmd
		p.confirm, cmd = p.confirm.Update(msg)
		return true, cmd
	}

	if p.IsVisible(PopupTextInput) {
		var cmd tea.Cmd
		p.textInput, cmd = p.textInput.Update(msg)
		return true, cmd
	}

	if p.IsVisible(PopupLibrarySources) {
		var cmd tea.Cmd
		p.librarySources, cmd = p.librarySources.Update(msg)
		return true, cmd
	}

	if p.IsVisible(PopupDownload) && p.download != nil {
		// Route keys to download popup
		dl, cmd := p.download.Update(msg)
		p.download = dl
		return true, cmd
	}

	return false, nil
}

// --- Rendering ---

// RenderOverlay renders active popup(s) on top of the base view.
func (p *PopupManager) RenderOverlay(base string) string {
	if p.IsVisible(PopupTextInput) {
		base = popup.Compose(base, p.textInput.View(), p.width, p.height)
	}
	if p.IsVisible(PopupConfirm) {
		base = popup.Compose(base, p.confirm.View(), p.width, p.height)
	}
	if p.IsVisible(PopupLibrarySources) {
		base = popup.Compose(base, p.librarySources.View(), p.width, p.height)
	}
	if p.IsVisible(PopupError) {
		base = popup.Compose(base, p.renderError(), p.width, p.height)
	}
	if p.IsVisible(PopupScanReport) {
		base = popup.Compose(base, p.scanReport.Render(), p.width, p.height)
	}
	if p.IsVisible(PopupDownload) {
		base = popup.Compose(base, p.renderDownload(), p.width, p.height)
	}
	if p.IsVisible(PopupHelp) {
		base = popup.Compose(base, p.help.View(), p.width, p.height)
	}
	return base
}

func (p *PopupManager) renderDownload() string {
	if p.download == nil {
		return ""
	}

	// Get download content
	content := p.download.View()

	// Calculate popup dimensions (80% width, 70% height)
	popupWidth := p.width * 80 / 100
	popupHeight := p.height * 70 / 100

	// Create bordered box
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Width(popupWidth-2). // Account for border
		Height(popupHeight-2).
		Padding(1, 2)

	box := boxStyle.Render(content)

	return popup.Center(box, p.width, p.height)
}

func (p *PopupManager) renderError() string {
	pop := popup.New()
	pop.Title = "Error"
	pop.Content = p.errorMsg
	pop.Footer = "Press any key to dismiss"
	return pop.Render(p.width, p.height)
}
