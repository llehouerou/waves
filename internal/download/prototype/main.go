// Throwaway PROTOTYPE (issue #61): date-grouped releases list (variant B),
// split in two modes: Recent (downloadable) and Upcoming (informational).
// Run: go run ./internal/download/prototype   (-dump prints both modes)
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/icons"
	"github.com/llehouerou/waves/internal/ui/render"
	"github.com/llehouerou/waves/internal/ui/styles"
)

var (
	modeNames   = []string{"Recent", "Upcoming"}
	filterNames = []string{"all", "library", "discoveries"}
	iconModes   = []string{"nerd", "unicode", "none"}
	widths      = []int{100, 78, 58}
)

type model struct {
	all      []release
	mode     int // 0 = recent (past), 1 = upcoming (future)
	filter   int
	iconMode int
	widthIdx int
	stress   bool
	cursor   int
	offset   int
	termW    int
	termH    int
}

func main() {
	dump := flag.Bool("dump", false, "print both modes to stdout")
	flag.Parse()

	rs, err := load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "load:", err)
		os.Exit(1)
	}
	icons.Init("nerd")
	m := model{all: rs, termW: 120, termH: 40}

	if *dump {
		for mode := range modeNames {
			m.mode = mode
			fmt.Println(m.View())
			fmt.Println()
		}
		return
	}
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termW, m.termH = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "tab", "left", "right", "h", "l":
			m.mode = 1 - m.mode
			m.cursor, m.offset = 0, 0
		case "f":
			m.filter = (m.filter + 1) % len(filterNames)
			m.cursor, m.offset = 0, 0
		case "i":
			m.iconMode = (m.iconMode + 1) % len(iconModes)
			icons.Init(iconModes[m.iconMode])
		case "w":
			m.widthIdx = (m.widthIdx + 1) % len(widths)
		case "x":
			m.stress = !m.stress
			m.cursor, m.offset = 0, 0
		case "down", "j":
			m.cursor++
		case "up", "k":
			m.cursor--
		}
	}
	return m, nil
}

// rows returns the releases for the current mode and filter.
// Recent: newest first. Upcoming: soonest first.
func (m model) rows() []release {
	src := m.all
	if m.stress {
		src = append(append([]release{}, stressRows()...), m.all...)
		sortReleases(src)
	}
	today := time.Now().Format("2006-01-02")
	var out []release
	for _, r := range src {
		if (r.Date > today) != (m.mode == 1) {
			continue
		}
		if m.filter != 0 && (m.filter == 2) != r.Discovery {
			continue
		}
		out = append(out, r)
	}
	return out
}

func (m model) counts() (past, future int) {
	today := time.Now().Format("2006-01-02")
	for _, r := range m.all {
		if r.Date > today {
			future++
		} else {
			past++
		}
	}
	return past, future
}

func (m model) View() string {
	rows := m.rows()
	w := min(widths[m.widthIdx], max(m.termW-4, 30))
	inner := w - 6 // box borders + padding + 2-char cursor prefix

	m.cursor = clamp(m.cursor, 0, max(len(rows)-1, 0))
	bodyLines := max(m.termH-8, 6)
	body, first, last := m.viewGroups(rows, inner, bodyLines)
	if m.cursor < first || m.cursor > last {
		// keep the cursor row on screen
		if m.cursor < first {
			m.offset = m.cursor
		} else {
			m.offset += m.cursor - last
		}
		body, _, _ = m.viewGroups(rows, inner, bodyLines)
	}

	s := styles.T().S()
	past, future := m.counts()
	head := s.Title.Render("New releases") + "   " +
		tab("Recent", past, m.mode == 0) + " " + tab("Upcoming", future, m.mode == 1) + "   " +
		s.Muted.Render("filter: "+filterNames[m.filter])

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.T().BorderFocus).
		Padding(0, 1).Width(w - 2).
		Render(head + "\n\n" + body)

	bar := s.Muted.Render(fmt.Sprintf(
		"tab mode · f filter · i icons (%s) · w width (%d) · x stress rows (%v) · j/k scroll · q quit",
		iconModes[m.iconMode], widths[m.widthIdx], m.stress))
	return box + "\n" + bar
}

func tab(name string, n int, active bool) string {
	label := fmt.Sprintf(" %s %d ", name, n)
	if active {
		return styles.T().S().Cursor.Bold(true).Render(label)
	}
	return styles.T().S().Subtle.Render(label)
}

// viewGroups renders rows grouped by date, capped at maxLines rendered lines.
// It returns the first and last row index actually rendered.
func (m model) viewGroups(rows []release, w, maxLines int) (body string, first, last int) {
	var b strings.Builder
	lines := 0
	last = m.offset
	lastDate := ""
	for i := m.offset; i < len(rows); i++ {
		r := rows[i]
		need := 1
		if r.Date != lastDate {
			need = 2
		}
		if lines+need > maxLines {
			break
		}
		if r.Date != lastDate {
			lastDate = r.Date
			b.WriteString(styles.T().S().Warning.Render(dashes(" "+humanDate(r.Date)+" ", w+2)) + "\n")
		}
		text := r.Artist + " — " + r.Title
		meta := r.Type
		if s := why(r); s != "" {
			meta += " · " + s
		}
		line := "  " + marker(r) + " " +
			render.TruncateAndPadEllipsis(text, max(w-7-lipgloss.Width(meta), 10)) + "  " +
			styles.T().S().Subtle.Render(meta)
		b.WriteString(row(line, i == m.cursor, r) + "\n")
		lines += need
		last = i
	}
	return b.String(), m.offset, last
}

func row(line string, selected bool, r release) string {
	if selected {
		return styles.T().S().Cursor.Render("> " + line)
	}
	if r.Owned {
		return styles.T().S().Subtle.Render("  " + line)
	}
	return styles.T().S().Base.Render("  " + line)
}

// marker shows library vs discovery, and whether the album is already owned.
func marker(r release) string {
	switch {
	case r.Owned:
		return pad2(icons.InLibrary())
	case r.Discovery:
		return pad2(icons.Radio())
	default:
		return pad2(iconOrText(icons.FormatArtist("x"), "-"))
	}
}

func iconOrText(formatted, fallback string) string {
	if formatted == "x" { // "none" icon style returns the bare name
		return fallback
	}
	return strings.TrimSuffix(formatted, "x")
}

func pad2(s string) string { return render.TruncateAndPadEllipsis(strings.TrimSpace(s), 2) }

func why(r release) string {
	if !r.Discovery || len(r.From) == 0 {
		return ""
	}
	if len(r.From) > 2 {
		return fmt.Sprintf("← %s, %s +%d", r.From[0], r.From[1], len(r.From)-2)
	}
	return "← " + strings.Join(r.From, ", ")
}

func humanDate(d string) string {
	t, err := time.Parse("2006-01-02", d)
	if err != nil {
		return d
	}
	switch daysFromToday(t) {
	case 0:
		return "Today"
	case 1:
		return "Tomorrow"
	case -1:
		return "Yesterday"
	}
	return t.Format("Mon 2 Jan 2006")
}

func daysFromToday(t time.Time) int {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return int(t.Sub(today).Hours() / 24)
}

func dashes(label string, w int) string {
	return "──" + label + strings.Repeat("─", max(w-lipgloss.Width(label)-2, 0))
}

func clamp(v, lo, hi int) int { return max(lo, min(v, hi)) }
