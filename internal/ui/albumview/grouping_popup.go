package albumview

import (
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/ui/popup"
)

// Key constants for popup navigation.
const (
	keyUp    = "up"
	keyDown  = "down"
	keyEnter = "enter"
	keyEsc   = "esc"
	keySpace = " "
)

// Compile-time check that GroupingPopup implements popup.Popup.
var _ popup.Popup = (*GroupingPopup)(nil)

var (
	gpTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212"))

	gpFieldStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	gpSelectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true)

	gpCursorStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236"))

	gpHintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	gpOrderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("141"))
)

// optionRow represents which option row is selected in the options section.
type optionRow int

const (
	optionRowNone      optionRow = iota // In field list
	optionRowSortOrder                  // Group sort order (Asc/Desc)
	optionRowDateField                  // Date field selection
)

// GroupingPopup allows configuring multi-layer grouping.
type GroupingPopup struct {
	selected  []GroupField  // Currently selected fields (in order)
	sortOrder SortOrder     // Group sort order (Asc/Desc)
	dateField DateFieldType // Which date field to use for date grouping
	cursor    int           // Cursor position in field list
	optionRow optionRow     // Which option row is selected (0 = in field list)
	width     int
	height    int
	active    bool
}

// NewGroupingPopup creates a new grouping popup.
func NewGroupingPopup() *GroupingPopup {
	return &GroupingPopup{}
}

// Show displays the grouping popup with current settings.
func (p *GroupingPopup) Show(current []GroupField, sortOrder SortOrder, dateField DateFieldType, width, height int) {
	p.selected = make([]GroupField, len(current))
	copy(p.selected, current)
	p.sortOrder = sortOrder
	p.dateField = dateField
	p.cursor = 0
	p.optionRow = optionRowNone
	p.width = width
	p.height = height
	p.active = true
}

// Reset clears the popup state.
func (p *GroupingPopup) Reset() {
	p.selected = nil
	p.cursor = 0
	p.active = false
}

// Active returns whether the popup is currently shown.
func (p GroupingPopup) Active() bool {
	return p.active
}

// Init implements popup.Popup.
func (p *GroupingPopup) Init() tea.Cmd {
	return nil
}

// SetSize implements popup.Popup.
func (p *GroupingPopup) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// Update implements popup.Popup.
func (p *GroupingPopup) Update(msg tea.Msg) (popup.Popup, tea.Cmd) {
	if !p.active {
		return p, nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}

	switch keyMsg.String() {
	case keyUp, "k":
		p.navigateUp()
	case keyDown, "j":
		p.navigateDown()
	case keySpace: // Space toggles selection or cycles options
		p.handleSpace()
	case "l", "right": // Cycle option forward
		p.handleRight()
	case "h", "left": // Cycle option backward
		p.handleLeft()
	case "K", "shift+up": // Move selected field up
		if p.optionRow == optionRowNone {
			p.moveFieldUp()
		}
	case "J", "shift+down": // Move selected field down
		if p.optionRow == optionRowNone {
			p.moveFieldDown()
		}
	case keyEnter:
		p.active = false
		selected := make([]GroupField, len(p.selected))
		copy(selected, p.selected)
		sortOrder := p.sortOrder
		dateField := p.dateField
		return p, func() tea.Msg {
			return GroupingActionMsg(GroupingApplied{
				Fields:    selected,
				SortOrder: sortOrder,
				DateField: dateField,
			})
		}
	case keyEsc:
		p.active = false
		return p, func() tea.Msg {
			return GroupingActionMsg(GroupingCanceled{})
		}
	}
	return p, nil
}

// navigateUp moves cursor up through fields and options.
func (p *GroupingPopup) navigateUp() {
	switch p.optionRow {
	case optionRowNone:
		if p.cursor > 0 {
			p.cursor--
		}
	case optionRowSortOrder:
		// Go back to field list
		p.optionRow = optionRowNone
		p.cursor = GroupFieldCount - 1
	case optionRowDateField:
		// Go to sort order
		p.optionRow = optionRowSortOrder
	}
}

// navigateDown moves cursor down through fields and options.
func (p *GroupingPopup) navigateDown() {
	switch p.optionRow {
	case optionRowNone:
		if p.cursor < GroupFieldCount-1 {
			p.cursor++
		} else if len(p.selected) > 0 {
			// Move to options section (sort order first)
			p.optionRow = optionRowSortOrder
		}
	case optionRowSortOrder:
		if p.hasDateBasedGrouping() {
			p.optionRow = optionRowDateField
		}
		// Otherwise stay on sort order (bottom of options)
	case optionRowDateField:
		// Already at bottom
	}
}

// handleSpace toggles field selection or cycles current option.
func (p *GroupingPopup) handleSpace() {
	switch p.optionRow {
	case optionRowNone:
		field := GroupField(p.cursor)
		if idx := slices.Index(p.selected, field); idx >= 0 {
			p.selected = slices.Delete(p.selected, idx, idx+1)
		} else {
			p.selected = append(p.selected, field)
		}
	case optionRowSortOrder:
		p.toggleSortOrder()
	case optionRowDateField:
		p.cycleDateField(1)
	}
}

// handleRight cycles option forward.
func (p *GroupingPopup) handleRight() {
	switch p.optionRow {
	case optionRowNone:
		// No action in field list
	case optionRowSortOrder:
		p.toggleSortOrder()
	case optionRowDateField:
		p.cycleDateField(1)
	}
}

// handleLeft cycles option backward.
func (p *GroupingPopup) handleLeft() {
	switch p.optionRow {
	case optionRowNone:
		// No action in field list
	case optionRowSortOrder:
		p.toggleSortOrder()
	case optionRowDateField:
		p.cycleDateField(-1)
	}
}

// toggleSortOrder toggles between Asc and Desc.
func (p *GroupingPopup) toggleSortOrder() {
	if p.sortOrder == SortAsc {
		p.sortOrder = SortDesc
	} else {
		p.sortOrder = SortAsc
	}
}

// hasDateBasedGrouping checks if any selected grouping uses date fields.
func (p *GroupingPopup) hasDateBasedGrouping() bool {
	for _, f := range p.selected {
		if f == GroupFieldYear || f == GroupFieldMonth || f == GroupFieldWeek {
			return true
		}
	}
	return false
}

// cycleDateField cycles through date field options.
func (p *GroupingPopup) cycleDateField(delta int) {
	p.dateField = DateFieldType((int(p.dateField) + delta + DateFieldTypeCount) % DateFieldTypeCount)
}

// moveFieldUp moves the current field up in the selection order.
func (p *GroupingPopup) moveFieldUp() {
	field := GroupField(p.cursor)
	idx := slices.Index(p.selected, field)
	if idx > 0 {
		p.selected[idx], p.selected[idx-1] = p.selected[idx-1], p.selected[idx]
	}
}

// moveFieldDown moves the current field down in the selection order.
func (p *GroupingPopup) moveFieldDown() {
	field := GroupField(p.cursor)
	idx := slices.Index(p.selected, field)
	if idx >= 0 && idx < len(p.selected)-1 {
		p.selected[idx], p.selected[idx+1] = p.selected[idx+1], p.selected[idx]
	}
}

// renderOptionsSection renders the options section of the popup.
func (p *GroupingPopup) renderOptionsSection() string {
	if len(p.selected) == 0 {
		return ""
	}

	optionLines := make([]string, 0, 2)

	// Sort order option (always shown)
	optionLines = append(optionLines, p.renderSortOrderOption())

	// Date field option (only when date-based grouping)
	if p.hasDateBasedGrouping() {
		optionLines = append(optionLines, p.renderDateFieldOption())
	}

	return "\n\n" + gpTitleStyle.Render("Options") + "\n" + strings.Join(optionLines, "\n")
}

// renderSortOrderOption renders the sort order option line.
func (p *GroupingPopup) renderSortOrderOption() string {
	prefix := "  "
	if p.optionRow == optionRowSortOrder {
		prefix = "> "
	}
	sortValue := "Descending"
	if p.sortOrder == SortAsc {
		sortValue = "Ascending"
	}
	line := prefix + gpFieldStyle.Render("Order: ") + gpSelectedStyle.Render(sortValue)
	if p.optionRow == optionRowSortOrder {
		line = gpCursorStyle.Render(line)
	}
	return line
}

// renderDateFieldOption renders the date field option line.
func (p *GroupingPopup) renderDateFieldOption() string {
	prefix := "  "
	if p.optionRow == optionRowDateField {
		prefix = "> "
	}
	dateValue := DateFieldTypeName(p.dateField)
	line := prefix + gpFieldStyle.Render("Date field: ") + gpSelectedStyle.Render(dateValue)
	if p.optionRow == optionRowDateField {
		line = gpCursorStyle.Render(line)
	}
	return line
}

// View implements popup.Popup.
func (p *GroupingPopup) View() string {
	if !p.active || p.width == 0 || p.height == 0 {
		return ""
	}

	title := gpTitleStyle.Render("Album Grouping")

	// Build field list
	lines := make([]string, 0, GroupFieldCount)
	for i := range GroupFieldCount {
		field := GroupField(i)
		name := GroupFieldName(field)

		// Check if selected and get order
		order := ""
		if idx := slices.Index(p.selected, field); idx >= 0 {
			order = gpOrderStyle.Render("[" + string('1'+rune(idx)) + "] ")
		} else {
			order = "    "
		}

		// Prefix for cursor
		prefix := "  "
		if i == p.cursor && p.optionRow == optionRowNone {
			prefix = "> "
		}

		// Apply style
		var line string
		if slices.Contains(p.selected, field) {
			line = prefix + order + gpSelectedStyle.Render(name)
		} else {
			line = prefix + order + gpFieldStyle.Render(name)
		}

		// Apply cursor background
		if i == p.cursor && p.optionRow == optionRowNone {
			line = gpCursorStyle.Render(line)
		}

		lines = append(lines, line)
	}

	fieldList := strings.Join(lines, "\n")

	// Options section (shown when grouping is selected)
	optionsSection := p.renderOptionsSection()

	// Current selection summary
	var summary string
	if len(p.selected) == 0 {
		summary = "No grouping"
	} else {
		names := make([]string, len(p.selected))
		for i, f := range p.selected {
			names[i] = GroupFieldName(f)
		}
		sortDir := arrowDown
		if p.sortOrder == SortAsc {
			sortDir = arrowUp
		}
		summary = "Group by: " + strings.Join(names, " > ") + " " + sortDir
		if p.hasDateBasedGrouping() {
			summary += " (" + DateFieldTypeName(p.dateField) + ")"
		}
	}
	summaryLine := gpFieldStyle.Render(summary)

	hint := gpHintStyle.Render("↑↓ navigate · space toggle · J/K reorder · ←→ options · enter apply · esc cancel")

	return title + "\n\n" + fieldList + optionsSection + "\n\n" + summaryLine + "\n\n" + hint
}
