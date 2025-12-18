package albumview

import (
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/ui/popup"
	"github.com/llehouerou/waves/internal/ui/styles"
)

// Compile-time check that SortingPopup implements popup.Popup.
var _ popup.Popup = (*SortingPopup)(nil)

func spTitleStyle() lipgloss.Style {
	return styles.T().S().Title
}

func spFieldStyle() lipgloss.Style {
	return styles.T().S().Base
}

func spSelectedStyle() lipgloss.Style {
	return styles.T().S().Playing.Bold(true)
}

func spCursorStyle() lipgloss.Style {
	return styles.T().S().Cursor
}

func spHintStyle() lipgloss.Style {
	return styles.T().S().Subtle
}

func spOrderStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(styles.T().Secondary)
}

func spAscStyle() lipgloss.Style {
	return styles.T().S().Success
}

func spDescStyle() lipgloss.Style {
	return styles.T().S().Warning
}

// SortingPopup allows configuring multi-field sorting.
type SortingPopup struct {
	selected []SortCriterion // Currently selected criteria (in order)
	cursor   int             // Current cursor position (0 to SortFieldCount-1)
	width    int
	height   int
	active   bool
}

// NewSortingPopup creates a new sorting popup.
func NewSortingPopup() *SortingPopup {
	return &SortingPopup{}
}

// Show displays the sorting popup with current settings.
func (p *SortingPopup) Show(current []SortCriterion, width, height int) {
	p.selected = make([]SortCriterion, len(current))
	copy(p.selected, current)
	p.cursor = 0
	p.width = width
	p.height = height
	p.active = true
}

// Reset clears the popup state.
func (p *SortingPopup) Reset() {
	p.selected = nil
	p.cursor = 0
	p.active = false
}

// Active returns whether the popup is currently shown.
func (p SortingPopup) Active() bool {
	return p.active
}

// Init implements popup.Popup.
func (p *SortingPopup) Init() tea.Cmd {
	return nil
}

// SetSize implements popup.Popup.
func (p *SortingPopup) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// findCriterion returns the index of the criterion with the given field, or -1.
func (p *SortingPopup) findCriterion(field SortField) int {
	for i, c := range p.selected {
		if c.Field == field {
			return i
		}
	}
	return -1
}

// Update implements popup.Popup.
func (p *SortingPopup) Update(msg tea.Msg) (popup.Popup, tea.Cmd) {
	if !p.active {
		return p, nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}

	switch keyMsg.String() {
	case keyUp, "k":
		if p.cursor > 0 {
			p.cursor--
		}
	case keyDown, "j":
		if p.cursor < SortFieldCount-1 {
			p.cursor++
		}
	case keySpace: // Space toggles selection
		field := SortField(p.cursor)
		if idx := p.findCriterion(field); idx >= 0 {
			// Remove from selected
			p.selected = slices.Delete(p.selected, idx, idx+1)
		} else {
			// Add to selected (default to descending)
			p.selected = append(p.selected, SortCriterion{Field: field, Order: SortDesc})
		}
	case "a": // Toggle to ascending
		field := SortField(p.cursor)
		if idx := p.findCriterion(field); idx >= 0 {
			p.selected[idx].Order = SortAsc
		}
	case "d": // Toggle to descending
		field := SortField(p.cursor)
		if idx := p.findCriterion(field); idx >= 0 {
			p.selected[idx].Order = SortDesc
		}
	case "K", "shift+up": // Move selected field up
		p.moveFieldUp()
	case "J", "shift+down": // Move selected field down
		p.moveFieldDown()
	case keyEnter:
		p.active = false
		selected := make([]SortCriterion, len(p.selected))
		copy(selected, p.selected)
		return p, func() tea.Msg {
			return SortingActionMsg(SortingApplied{Criteria: selected})
		}
	case keyEsc:
		p.active = false
		return p, func() tea.Msg {
			return SortingActionMsg(SortingCanceled{})
		}
	}
	return p, nil
}

// moveFieldUp moves the current field up in the selection order.
func (p *SortingPopup) moveFieldUp() {
	field := SortField(p.cursor)
	idx := p.findCriterion(field)
	if idx > 0 {
		p.selected[idx], p.selected[idx-1] = p.selected[idx-1], p.selected[idx]
	}
}

// moveFieldDown moves the current field down in the selection order.
func (p *SortingPopup) moveFieldDown() {
	field := SortField(p.cursor)
	idx := p.findCriterion(field)
	if idx >= 0 && idx < len(p.selected)-1 {
		p.selected[idx], p.selected[idx+1] = p.selected[idx+1], p.selected[idx]
	}
}

// View implements popup.Popup.
func (p *SortingPopup) View() string {
	if !p.active || p.width == 0 || p.height == 0 {
		return ""
	}

	title := spTitleStyle().Render("Album Sorting")

	// Build field list
	lines := make([]string, 0, SortFieldCount)
	for i := range SortFieldCount {
		field := SortField(i)
		name := SortFieldName(field)

		// Check if selected and get order
		order := ""
		orderIndicator := ""
		if idx := p.findCriterion(field); idx >= 0 {
			order = spOrderStyle().Render("[" + string('1'+rune(idx)) + "] ")
			if p.selected[idx].Order == SortAsc {
				orderIndicator = spAscStyle().Render(" ↑asc")
			} else {
				orderIndicator = spDescStyle().Render(" ↓desc")
			}
		} else {
			order = "    "
		}

		// Prefix for cursor
		prefix := "  "
		if i == p.cursor {
			prefix = "> "
		}

		// Apply style
		var line string
		if p.findCriterion(field) >= 0 {
			line = prefix + order + spSelectedStyle().Render(name) + orderIndicator
		} else {
			line = prefix + order + spFieldStyle().Render(name)
		}

		// Apply cursor background
		if i == p.cursor {
			line = spCursorStyle().Render(line)
		}

		lines = append(lines, line)
	}

	fieldList := strings.Join(lines, "\n")

	// Current selection summary
	var summary string
	if len(p.selected) == 0 {
		summary = "Default sort order"
	} else {
		parts := make([]string, len(p.selected))
		for i, c := range p.selected {
			dir := "desc"
			if c.Order == SortAsc {
				dir = "asc"
			}
			parts[i] = SortFieldName(c.Field) + " " + dir
		}
		summary = "Sort by: " + strings.Join(parts, ", ")
	}
	summaryLine := spFieldStyle().Render(summary)

	hint := spHintStyle().Render("↑↓ navigate · space toggle · a/d asc/desc · J/K reorder · enter apply")

	return title + "\n\n" + fieldList + "\n\n" + summaryLine + "\n\n" + hint
}
