// internal/app/popupctl/manager.go
package popupctl

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/albumpreset"
	"github.com/llehouerou/waves/internal/download"
	"github.com/llehouerou/waves/internal/downloads"
	"github.com/llehouerou/waves/internal/export"
	importpopup "github.com/llehouerou/waves/internal/importer/popup"
	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/rename"
	"github.com/llehouerou/waves/internal/retag"
	"github.com/llehouerou/waves/internal/state"
	"github.com/llehouerou/waves/internal/ui/albumview"
	"github.com/llehouerou/waves/internal/ui/confirm"
	exportui "github.com/llehouerou/waves/internal/ui/export"
	"github.com/llehouerou/waves/internal/ui/helpbindings"
	"github.com/llehouerou/waves/internal/ui/lastfmauth"
	"github.com/llehouerou/waves/internal/ui/librarysources"
	"github.com/llehouerou/waves/internal/ui/popup"
	"github.com/llehouerou/waves/internal/ui/scanreport"
	"github.com/llehouerou/waves/internal/ui/textinput"
)

// Manager manages all modal popups and overlays.
type Manager struct {
	popups    map[Type]popup.Popup
	sizes     map[Type]popup.SizeConfig
	inputMode InputMode
	errorMsg  string
	width     int
	height    int
}

// New creates a new Manager with initialized components.
func New() *Manager {
	return &Manager{
		popups: make(map[Type]popup.Popup),
		sizes: map[Type]popup.SizeConfig{
			Download: popup.SizeLarge,
			Import:   popup.SizeLarge,
			Retag:    popup.SizeLarge,
			// All others default to SizeAuto
		},
	}
}

// SetSize updates the dimensions for popup rendering.
func (p *Manager) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// IsVisible returns true if the specified popup type is visible.
func (p *Manager) IsVisible(t Type) bool {
	switch t {
	case None:
		return false
	case Error:
		return p.errorMsg != ""
	case TextInput:
		return p.inputMode != InputNone && p.popups[t] != nil
	case Help, Confirm, LibrarySources, ScanReport, Download, Import,
		Retag, AlbumGrouping, AlbumSorting, AlbumPresets, LastfmAuth, Export:
		return p.popups[t] != nil
	}
	return false
}

// ActivePopup returns which popup is currently active (highest priority).
func (p *Manager) ActivePopup() Type {
	for _, t := range Priority {
		if p.IsVisible(t) {
			return t
		}
	}
	return None
}

// Show displays a popup of the given type.
func (p *Manager) Show(t Type, pop popup.Popup) tea.Cmd {
	size := p.sizes[t]
	w, h := p.contentSize(size)
	pop.SetSize(w, h)
	p.popups[t] = pop
	return pop.Init()
}

// Hide hides the specified popup type.
func (p *Manager) Hide(t Type) {
	switch t {
	case None:
		// Nothing to hide
	case Error:
		p.errorMsg = ""
	case TextInput:
		p.inputMode = InputNone
		delete(p.popups, t)
	case Help, Confirm, LibrarySources, ScanReport, Download, Import,
		Retag, AlbumGrouping, AlbumSorting, AlbumPresets, LastfmAuth, Export:
		delete(p.popups, t)
	}
}

// Get retrieves a popup for type assertion when needed.
func (p *Manager) Get(t Type) popup.Popup {
	return p.popups[t]
}

// contentSize calculates popup content dimensions based on size config.
func (p *Manager) contentSize(size popup.SizeConfig) (width, height int) {
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
func (p *Manager) ShowHelp(contexts []string) tea.Cmd {
	help := helpbindings.New()
	help.SetContexts(contexts)
	return p.Show(Help, &help)
}

// ShowConfirm displays a confirmation dialog.
func (p *Manager) ShowConfirm(title, message string, context any) tea.Cmd {
	c := confirm.New()
	c.Show(title, message, context, p.width, p.height)
	return p.Show(Confirm, &c)
}

// ShowConfirmWithOptions displays a confirmation dialog with custom options.
func (p *Manager) ShowConfirmWithOptions(title, message string, options []string, context any) tea.Cmd {
	c := confirm.New()
	c.ShowWithOptions(title, message, options, context, p.width, p.height)
	return p.Show(Confirm, &c)
}

// ShowTextInput displays a text input popup.
func (p *Manager) ShowTextInput(mode InputMode, title, value string, context any) tea.Cmd {
	p.inputMode = mode
	ti := textinput.New()
	ti.Start(title, value, context, p.width, p.height)
	return p.Show(TextInput, &ti)
}

// ShowLibrarySources displays the library sources popup.
func (p *Manager) ShowLibrarySources(sources []string) tea.Cmd {
	ls := librarysources.New()
	ls.SetSources(sources)
	return p.Show(LibrarySources, &ls)
}

// ShowScanReport displays the scan report popup.
func (p *Manager) ShowScanReport(stats *library.ScanStats) tea.Cmd {
	report := scanreport.New(stats)
	return p.Show(ScanReport, &report)
}

// ShowDownload displays the download popup.
func (p *Manager) ShowDownload(slskdURL, slskdAPIKey string, filters download.FilterConfig) tea.Cmd {
	dl := download.New(slskdURL, slskdAPIKey, filters)
	dl.SetFocused(true)
	return p.Show(Download, dl)
}

// ShowError displays an error message popup.
func (p *Manager) ShowError(msg string) {
	p.errorMsg = msg
}

// ShowImport displays the import popup for a completed download.
func (p *Manager) ShowImport(dl *downloads.Download, completedPath string, librarySources []string, mbClient *musicbrainz.Client, renameConfig rename.Config) tea.Cmd {
	imp := importpopup.New(dl, completedPath, librarySources, mbClient, renameConfig)
	return p.Show(Import, imp)
}

// ShowRetag displays the retag popup for an existing album.
func (p *Manager) ShowRetag(albumArtist, albumName string, trackPaths []string, mbClient *musicbrainz.Client, lib *library.Library) tea.Cmd {
	rt := retag.New(albumArtist, albumName, trackPaths, mbClient, lib)
	return p.Show(Retag, rt)
}

// ShowAlbumGrouping displays the album grouping popup.
func (p *Manager) ShowAlbumGrouping(current []albumview.GroupField, sortOrder albumview.SortOrder, dateField albumview.DateFieldType) tea.Cmd {
	gp := albumview.NewGroupingPopup()
	gp.Show(current, sortOrder, dateField, p.width, p.height)
	return p.Show(AlbumGrouping, gp)
}

// ShowAlbumSorting displays the album sorting popup.
func (p *Manager) ShowAlbumSorting(current []albumview.SortCriterion) tea.Cmd {
	sp := albumview.NewSortingPopup()
	sp.Show(current, p.width, p.height)
	return p.Show(AlbumSorting, sp)
}

// ShowAlbumPresets displays the album presets popup.
func (p *Manager) ShowAlbumPresets(presets []albumview.Preset, current albumpreset.Settings) tea.Cmd {
	pp := albumview.NewPresetsPopup()
	pp.Show(presets, current, p.width, p.height)
	return p.Show(AlbumPresets, pp)
}

// ShowLastfmAuth displays the Last.fm authentication popup.
func (p *Manager) ShowLastfmAuth(session *state.LastfmSession) tea.Cmd {
	lfm := lastfmauth.New()
	lfm.SetSession(session)
	return p.Show(LastfmAuth, &lfm)
}

// ShowExport displays the export popup.
func (p *Manager) ShowExport(repo *export.TargetRepository) tea.Cmd {
	exp := exportui.New(repo)
	return p.Show(Export, &exp)
}

// Export returns the export popup model for direct access.
func (p *Manager) Export() *exportui.Model {
	if pop := p.popups[Export]; pop != nil {
		if exp, ok := pop.(*exportui.Model); ok {
			return exp
		}
	}
	return nil
}

// --- Accessors ---

// InputMode returns the current input mode.
func (p *Manager) InputMode() InputMode {
	return p.inputMode
}

// ErrorMsg returns the current error message.
func (p *Manager) ErrorMsg() string {
	return p.errorMsg
}

// Download returns the download popup model for direct access.
func (p *Manager) Download() *download.Model {
	if pop := p.popups[Download]; pop != nil {
		if dl, ok := pop.(*download.Model); ok {
			return dl
		}
	}
	return nil
}

// Import returns the import popup model for direct access.
func (p *Manager) Import() *importpopup.Model {
	if pop := p.popups[Import]; pop != nil {
		if imp, ok := pop.(*importpopup.Model); ok {
			return imp
		}
	}
	return nil
}

// Retag returns the retag popup model for direct access.
func (p *Manager) Retag() *retag.Model {
	if pop := p.popups[Retag]; pop != nil {
		if rt, ok := pop.(*retag.Model); ok {
			return rt
		}
	}
	return nil
}

// LibrarySources returns the library sources popup model for direct access.
func (p *Manager) LibrarySources() *librarysources.Model {
	if pop := p.popups[LibrarySources]; pop != nil {
		if ls, ok := pop.(*librarysources.Model); ok {
			return ls
		}
	}
	return nil
}

// --- Key Handling ---

// HandleKey routes key events to the active popup.
// Returns (handled, cmd) where handled is true if a popup consumed the key.
func (p *Manager) HandleKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	// Error popup: dismiss on any key
	if p.errorMsg != "" {
		p.errorMsg = ""
		return true, nil
	}

	// Find highest-priority active popup
	active := p.ActivePopup()
	if active == None {
		return false, nil
	}

	// ScanReport special handling (no Update method with keys)
	if active == ScanReport {
		key := msg.String()
		if key == "enter" || key == "escape" {
			p.Hide(ScanReport)
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
func (p *Manager) RenderOverlay(base string) string {
	for _, t := range RenderOrder {
		if !p.IsVisible(t) {
			continue
		}

		if t == Error {
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

func (p *Manager) renderError() string {
	pop := popup.New()
	pop.Title = "Error"
	pop.Content = p.errorMsg
	pop.Footer = "Press any key to dismiss"
	return pop.Render(p.width, p.height)
}
