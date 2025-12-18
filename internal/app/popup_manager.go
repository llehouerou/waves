// internal/app/popup_manager.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/download"
	"github.com/llehouerou/waves/internal/downloads"
	importpopup "github.com/llehouerou/waves/internal/importer/popup"
	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/retag"
	"github.com/llehouerou/waves/internal/ui/albumview"
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
	PopupImport
	PopupRetag
	PopupAlbumGrouping
	PopupAlbumSorting
	PopupAlbumPresets
)

// popupPriority defines which popup takes precedence (highest priority first).
var popupPriority = []PopupType{
	PopupError,
	PopupScanReport,
	PopupHelp,
	PopupConfirm,
	PopupTextInput,
	PopupLibrarySources,
	PopupAlbumGrouping,
	PopupAlbumSorting,
	PopupAlbumPresets,
	PopupDownload,
	PopupImport,
	PopupRetag,
}

// popupRenderOrder defines the order popups are rendered (bottom to top).
var popupRenderOrder = []PopupType{
	PopupRetag,
	PopupImport,
	PopupDownload,
	PopupAlbumPresets,
	PopupAlbumSorting,
	PopupAlbumGrouping,
	PopupLibrarySources,
	PopupTextInput,
	PopupConfirm,
	PopupScanReport,
	PopupHelp,
	PopupError,
}

// PopupManager manages all modal popups and overlays.
type PopupManager struct {
	popups    map[PopupType]popup.Popup
	sizes     map[PopupType]popup.SizeConfig
	inputMode InputMode
	errorMsg  string
	width     int
	height    int
}

// NewPopupManager creates a new PopupManager with initialized components.
func NewPopupManager() PopupManager {
	return PopupManager{
		popups: make(map[PopupType]popup.Popup),
		sizes: map[PopupType]popup.SizeConfig{
			PopupDownload: popup.SizeLarge,
			PopupImport:   popup.SizeLarge,
			PopupRetag:    popup.SizeLarge,
			// All others default to SizeAuto
		},
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
	case PopupError:
		return p.errorMsg != ""
	case PopupTextInput:
		return p.inputMode != InputNone && p.popups[t] != nil
	case PopupHelp, PopupConfirm, PopupLibrarySources, PopupScanReport, PopupDownload, PopupImport,
		PopupRetag, PopupAlbumGrouping, PopupAlbumSorting, PopupAlbumPresets:
		return p.popups[t] != nil
	}
	return false
}

// ActivePopup returns which popup is currently active (highest priority).
func (p *PopupManager) ActivePopup() PopupType {
	for _, t := range popupPriority {
		if p.IsVisible(t) {
			return t
		}
	}
	return PopupNone
}

// Show displays a popup of the given type.
func (p *PopupManager) Show(t PopupType, pop popup.Popup) tea.Cmd {
	size := p.sizes[t]
	w, h := p.contentSize(size)
	pop.SetSize(w, h)
	p.popups[t] = pop
	return pop.Init()
}

// Hide hides the specified popup type.
func (p *PopupManager) Hide(t PopupType) {
	switch t {
	case PopupNone:
		// Nothing to hide
	case PopupError:
		p.errorMsg = ""
	case PopupTextInput:
		p.inputMode = InputNone
		delete(p.popups, t)
	case PopupHelp, PopupConfirm, PopupLibrarySources, PopupScanReport, PopupDownload, PopupImport,
		PopupRetag, PopupAlbumGrouping, PopupAlbumSorting, PopupAlbumPresets:
		delete(p.popups, t)
	}
}

// Get retrieves a popup for type assertion when needed.
func (p *PopupManager) Get(t PopupType) popup.Popup {
	return p.popups[t]
}

// contentSize calculates popup content dimensions based on size config.
func (p *PopupManager) contentSize(size popup.SizeConfig) (width, height int) {
	if size.WidthPct > 0 {
		w := p.width * size.WidthPct / 100
		h := p.height * size.HeightPct / 100
		return w, h
	}
	// Auto-fit: give full screen size, popup decides
	return p.width, p.height
}

// --- Show Methods (convenience wrappers) ---

// ShowHelp displays the help popup with the given contexts.
func (p *PopupManager) ShowHelp(contexts []string) tea.Cmd {
	help := helpbindings.New()
	help.SetContexts(contexts)
	return p.Show(PopupHelp, &help)
}

// ShowConfirm displays a confirmation dialog.
func (p *PopupManager) ShowConfirm(title, message string, context any) tea.Cmd {
	c := confirm.New()
	c.Show(title, message, context, p.width, p.height)
	return p.Show(PopupConfirm, &c)
}

// ShowConfirmWithOptions displays a confirmation dialog with custom options.
func (p *PopupManager) ShowConfirmWithOptions(title, message string, options []string, context any) tea.Cmd {
	c := confirm.New()
	c.ShowWithOptions(title, message, options, context, p.width, p.height)
	return p.Show(PopupConfirm, &c)
}

// ShowTextInput displays a text input popup.
func (p *PopupManager) ShowTextInput(mode InputMode, title, value string, context any) tea.Cmd {
	p.inputMode = mode
	ti := textinput.New()
	ti.Start(title, value, context, p.width, p.height)
	return p.Show(PopupTextInput, &ti)
}

// ShowLibrarySources displays the library sources popup.
func (p *PopupManager) ShowLibrarySources(sources []string) tea.Cmd {
	ls := librarysources.New()
	ls.SetSources(sources)
	return p.Show(PopupLibrarySources, &ls)
}

// ShowScanReport displays the scan report popup.
func (p *PopupManager) ShowScanReport(stats *library.ScanStats) tea.Cmd {
	report := scanreport.New(stats)
	return p.Show(PopupScanReport, &report)
}

// ShowDownload displays the download popup.
func (p *PopupManager) ShowDownload(slskdURL, slskdAPIKey string, filters download.FilterConfig) tea.Cmd {
	dl := download.New(slskdURL, slskdAPIKey, filters)
	dl.SetFocused(true)
	return p.Show(PopupDownload, dl)
}

// ShowError displays an error message popup.
func (p *PopupManager) ShowError(msg string) {
	p.errorMsg = msg
}

// ShowImport displays the import popup for a completed download.
func (p *PopupManager) ShowImport(dl *downloads.Download, completedPath string, librarySources []string, mbClient *musicbrainz.Client) tea.Cmd {
	imp := importpopup.New(dl, completedPath, librarySources, mbClient)
	return p.Show(PopupImport, imp)
}

// ShowRetag displays the retag popup for an existing album.
func (p *PopupManager) ShowRetag(albumArtist, albumName string, trackPaths []string, mbClient *musicbrainz.Client, lib *library.Library) tea.Cmd {
	rt := retag.New(albumArtist, albumName, trackPaths, mbClient, lib)
	return p.Show(PopupRetag, rt)
}

// ShowAlbumGrouping displays the album grouping popup.
func (p *PopupManager) ShowAlbumGrouping(current []albumview.GroupField, sortOrder albumview.SortOrder, dateField albumview.DateFieldType) tea.Cmd {
	gp := albumview.NewGroupingPopup()
	gp.Show(current, sortOrder, dateField, p.width, p.height)
	return p.Show(PopupAlbumGrouping, gp)
}

// ShowAlbumSorting displays the album sorting popup.
func (p *PopupManager) ShowAlbumSorting(current []albumview.SortCriterion) tea.Cmd {
	sp := albumview.NewSortingPopup()
	sp.Show(current, p.width, p.height)
	return p.Show(PopupAlbumSorting, sp)
}

// ShowAlbumPresets displays the album presets popup.
func (p *PopupManager) ShowAlbumPresets(presets []albumview.Preset, current albumview.Settings) tea.Cmd {
	pp := albumview.NewPresetsPopup()
	pp.Show(presets, current, p.width, p.height)
	return p.Show(PopupAlbumPresets, pp)
}

// --- Accessors ---

// InputMode returns the current input mode.
func (p *PopupManager) InputMode() InputMode {
	return p.inputMode
}

// ErrorMsg returns the current error message.
func (p *PopupManager) ErrorMsg() string {
	return p.errorMsg
}

// Download returns the download popup model for direct access.
func (p *PopupManager) Download() *download.Model {
	if pop := p.popups[PopupDownload]; pop != nil {
		if dl, ok := pop.(*download.Model); ok {
			return dl
		}
	}
	return nil
}

// Import returns the import popup model for direct access.
func (p *PopupManager) Import() *importpopup.Model {
	if pop := p.popups[PopupImport]; pop != nil {
		if imp, ok := pop.(*importpopup.Model); ok {
			return imp
		}
	}
	return nil
}

// Retag returns the retag popup model for direct access.
func (p *PopupManager) Retag() *retag.Model {
	if pop := p.popups[PopupRetag]; pop != nil {
		if rt, ok := pop.(*retag.Model); ok {
			return rt
		}
	}
	return nil
}

// LibrarySources returns the library sources popup model for direct access.
func (p *PopupManager) LibrarySources() *librarysources.Model {
	if pop := p.popups[PopupLibrarySources]; pop != nil {
		if ls, ok := pop.(*librarysources.Model); ok {
			return ls
		}
	}
	return nil
}

// --- Key Handling ---

// HandleKey routes key events to the active popup.
// Returns (handled, cmd) where handled is true if a popup consumed the key.
func (p *PopupManager) HandleKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	// Error popup: dismiss on any key
	if p.errorMsg != "" {
		p.errorMsg = ""
		return true, nil
	}

	// Find highest-priority active popup
	active := p.ActivePopup()
	if active == PopupNone {
		return false, nil
	}

	// ScanReport special handling (no Update method with keys)
	if active == PopupScanReport {
		key := msg.String()
		if key == "enter" || key == "escape" {
			p.Hide(PopupScanReport)
		}
		return true, nil
	}

	pop := p.popups[active]
	if pop == nil {
		return false, nil
	}

	// Route to popup's Update
	updated, cmd := pop.Update(msg)
	p.popups[active] = updated
	return true, cmd
}

// --- Rendering ---

// RenderOverlay renders active popup(s) on top of the base view.
func (p *PopupManager) RenderOverlay(base string) string {
	for _, t := range popupRenderOrder {
		if !p.IsVisible(t) {
			continue
		}

		if t == PopupError {
			base = popup.Compose(base, p.renderError(), p.width, p.height)
			continue
		}

		pop := p.popups[t]
		if pop == nil {
			continue
		}

		content := pop.View()
		size := p.sizes[t]
		rendered := popup.RenderBordered(content, p.width, p.height, size)
		base = popup.Compose(base, rendered, p.width, p.height)
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
