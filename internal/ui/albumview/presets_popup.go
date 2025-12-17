package albumview

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/ui/popup"
)

// Compile-time check that PresetsPopup implements popup.Popup.
var _ popup.Popup = (*PresetsPopup)(nil)

var (
	ppTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212"))

	ppPresetStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	ppCursorStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236"))

	ppHintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	ppInputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	ppEmptyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true)
)

// Preset represents a saved grouping/sorting configuration.
type Preset struct {
	ID       int64
	Name     string
	Settings Settings
}

// PresetMode represents the current mode of the presets popup.
type PresetMode int

const (
	PresetModeList PresetMode = iota // Browsing presets
	PresetModeSave                   // Entering name for new preset
)

// PresetsPopup allows managing saved presets.
type PresetsPopup struct {
	presets []Preset
	current Settings // Current settings (for saving)
	cursor  int
	mode    PresetMode
	input   string // Text input for save mode
	width   int
	height  int
	active  bool
}

// NewPresetsPopup creates a new presets popup.
func NewPresetsPopup() *PresetsPopup {
	return &PresetsPopup{}
}

// Show displays the presets popup.
func (p *PresetsPopup) Show(presets []Preset, current Settings, width, height int) {
	p.presets = presets
	p.current = current
	p.cursor = 0
	p.mode = PresetModeList
	p.input = ""
	p.width = width
	p.height = height
	p.active = true
}

// Reset clears the popup state.
func (p *PresetsPopup) Reset() {
	p.presets = nil
	p.cursor = 0
	p.mode = PresetModeList
	p.input = ""
	p.active = false
}

// Active returns whether the popup is currently shown.
func (p PresetsPopup) Active() bool {
	return p.active
}

// Init implements popup.Popup.
func (p *PresetsPopup) Init() tea.Cmd {
	return nil
}

// SetSize implements popup.Popup.
func (p *PresetsPopup) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// Update implements popup.Popup.
func (p *PresetsPopup) Update(msg tea.Msg) (popup.Popup, tea.Cmd) {
	if !p.active {
		return p, nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}

	if p.mode == PresetModeSave {
		return p.handleSaveMode(keyMsg)
	}

	return p.handleListMode(keyMsg)
}

func (p *PresetsPopup) handleListMode(msg tea.KeyMsg) (popup.Popup, tea.Cmd) {
	switch msg.String() {
	case keyUp, "k":
		if p.cursor > 0 {
			p.cursor--
		}
	case keyDown, "j":
		if p.cursor < len(p.presets)-1 {
			p.cursor++
		}
	case keyEnter:
		if len(p.presets) > 0 && p.cursor < len(p.presets) {
			p.active = false
			preset := p.presets[p.cursor]
			// Set the preset name in the settings
			settings := preset.Settings
			settings.PresetName = preset.Name
			return p, func() tea.Msg {
				return PresetsActionMsg(PresetLoaded{Settings: settings, PresetID: preset.ID})
			}
		}
	case "s": // Save current as new preset
		p.mode = PresetModeSave
		p.input = ""
	case "d", "delete": // Delete selected preset
		if len(p.presets) > 0 && p.cursor < len(p.presets) {
			preset := p.presets[p.cursor]
			return p, func() tea.Msg {
				return PresetsActionMsg(PresetDeleted{ID: preset.ID})
			}
		}
	case keyEsc:
		p.active = false
		return p, func() tea.Msg {
			return PresetsActionMsg(PresetsClosed{})
		}
	}
	return p, nil
}

func (p *PresetsPopup) handleSaveMode(msg tea.KeyMsg) (popup.Popup, tea.Cmd) {
	switch msg.String() {
	case keyEnter:
		if p.input != "" {
			p.active = false
			name := p.input
			settings := p.current
			return p, func() tea.Msg {
				return PresetsActionMsg(PresetSaved{Name: name, Settings: settings})
			}
		}
	case keyEsc:
		p.mode = PresetModeList
		p.input = ""
	case "backspace":
		if p.input != "" {
			p.input = p.input[:len(p.input)-1]
		}
	default:
		// Add printable characters
		if len(msg.String()) == 1 && msg.String()[0] >= 32 {
			p.input += msg.String()
		}
	}
	return p, nil
}

// View implements popup.Popup.
func (p *PresetsPopup) View() string {
	if !p.active || p.width == 0 || p.height == 0 {
		return ""
	}

	if p.mode == PresetModeSave {
		return p.viewSaveMode()
	}

	return p.viewListMode()
}

func (p *PresetsPopup) viewListMode() string {
	title := ppTitleStyle.Render("Album View Presets")

	// Build preset list
	var lines []string
	if len(p.presets) == 0 {
		lines = append(lines, ppEmptyStyle.Render("  No saved presets"))
	} else {
		for i, preset := range p.presets {
			prefix := "  "
			if i == p.cursor {
				prefix = "> "
			}

			// Build description
			desc := p.formatPresetDescription(preset.Settings)
			line := prefix + ppPresetStyle.Render(preset.Name) + " " + ppHintStyle.Render("("+desc+")")

			if i == p.cursor {
				line = ppCursorStyle.Render(line)
			}

			lines = append(lines, line)
		}
	}

	presetList := strings.Join(lines, "\n")

	hint := ppHintStyle.Render("↑↓ navigate · enter load · s save current · d delete · esc close")

	return title + "\n\n" + presetList + "\n\n" + hint
}

func (p *PresetsPopup) viewSaveMode() string {
	title := ppTitleStyle.Render("Save Preset")

	prompt := ppPresetStyle.Render("Name: ")
	input := ppInputStyle.Render(p.input + "█")

	hint := ppHintStyle.Render("enter save · esc cancel")

	return title + "\n\n" + prompt + input + "\n\n" + hint
}

func (p *PresetsPopup) formatPresetDescription(s Settings) string {
	var parts []string

	if len(s.GroupFields) == 0 {
		parts = append(parts, "no grouping")
	} else {
		names := make([]string, len(s.GroupFields))
		for i, f := range s.GroupFields {
			names[i] = strings.ToLower(GroupFieldName(f))
		}
		parts = append(parts, "by "+strings.Join(names, ">"))
	}

	if len(s.SortCriteria) > 0 {
		sortParts := make([]string, len(s.SortCriteria))
		for i, c := range s.SortCriteria {
			dir := arrowDown
			if c.Order == SortAsc {
				dir = arrowUp
			}
			sortParts[i] = strings.ToLower(SortFieldName(c.Field)) + dir
		}
		parts = append(parts, strings.Join(sortParts, ","))
	}

	return strings.Join(parts, ", ")
}
