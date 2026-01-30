// Package lyrics provides a synchronized lyrics popup display.
package lyrics

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/lyrics"
	"github.com/llehouerou/waves/internal/ui"
	"github.com/llehouerou/waves/internal/ui/action"
	"github.com/llehouerou/waves/internal/ui/popup"
	"github.com/llehouerou/waves/internal/ui/styles"
)

// Compile-time check that Model implements popup.Popup.
var _ popup.Popup = (*Model)(nil)

// State represents the current state of the lyrics popup.
type State int

const (
	StateLoading State = iota
	StateLoaded
	StateNotFound
	StateError
)

// Model holds the state for the lyrics popup.
type Model struct {
	ui.Base
	source       *lyrics.Source
	lyrics       *lyrics.Lyrics
	state        State
	errorMsg     string
	currentLine  int
	scrollOffset int
	autoScroll   bool

	// Track info
	trackPath   string
	trackArtist string
	trackTitle  string
	trackAlbum  string
	duration    time.Duration
	position    time.Duration

	// Previous dimensions for stable loading display
	prevLineCount int
	prevMaxWidth  int
}

// New creates a new lyrics popup model.
func New(source *lyrics.Source) *Model {
	return &Model{
		source:     source,
		state:      StateLoading,
		autoScroll: true,
	}
}

// SetTrack sets the track to display lyrics for and triggers fetch.
func (m *Model) SetTrack(path, artist, title, album string, duration time.Duration) tea.Cmd {
	// Save previous dimensions for stable loading display
	if m.lyrics != nil && len(m.lyrics.Lines) > 0 {
		// Use actual displayed line count, not max visible height
		m.prevLineCount = min(len(m.lyrics.Lines), m.visibleHeight())
		m.prevMaxWidth = m.calculateMaxWidth()
	}

	m.trackPath = path
	m.trackArtist = artist
	m.trackTitle = title
	m.trackAlbum = album
	m.duration = duration
	m.lyrics = nil
	m.currentLine = -1
	m.scrollOffset = 0
	m.state = StateLoading
	m.autoScroll = true

	return m.fetchLyricsCmd()
}

// SetPosition updates the current playback position.
func (m *Model) SetPosition(pos time.Duration) {
	m.position = pos
	if m.lyrics != nil {
		newLine := m.lyrics.LineAt(pos)
		if newLine != m.currentLine {
			m.currentLine = newLine
			if m.autoScroll {
				m.centerCurrentLine()
			}
		}
	}
}

// centerCurrentLine adjusts scroll to center the current line.
func (m *Model) centerCurrentLine() {
	if m.currentLine < 0 || m.lyrics == nil {
		return
	}
	visibleHeight := m.visibleHeight()
	// Center the current line in the visible area
	m.scrollOffset = m.currentLine - visibleHeight/2
	m.scrollOffset = max(0, min(m.scrollOffset, m.maxScroll()))
}

// Init implements popup.Popup.
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update implements popup.Popup.
func (m *Model) Update(msg tea.Msg) (popup.Popup, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case FetchedMsg:
		return m.handleFetched(msg)
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (popup.Popup, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc", "q":
		return m, func() tea.Msg { return ActionMsg(Close{}) }
	case "j", "down":
		m.autoScroll = false
		maxScroll := m.maxScroll()
		if m.scrollOffset < maxScroll {
			m.scrollOffset++
		}
		return m, nil
	case "k", "up":
		m.autoScroll = false
		if m.scrollOffset > 0 {
			m.scrollOffset--
		}
		return m, nil
	case "g":
		m.autoScroll = false
		m.scrollOffset = 0
		return m, nil
	case "G":
		m.autoScroll = false
		m.scrollOffset = m.maxScroll()
		return m, nil
	case "c":
		// Re-enable auto-scroll and center on current line
		m.autoScroll = true
		m.centerCurrentLine()
		return m, nil
	default:
		// Pass unhandled keys to main handler for playback controls
		return m, func() tea.Msg { return ActionMsg(Passthrough{Key: msg}) }
	}
}

func (m *Model) handleFetched(msg FetchedMsg) (popup.Popup, tea.Cmd) {
	// Ignore stale results from previous track
	if msg.TrackPath != m.trackPath {
		return m, nil
	}
	if msg.Err != nil {
		m.state = StateError
		m.errorMsg = msg.Err.Error()
		// Clear prev dimensions so error shows at natural size
		m.prevLineCount = 0
		m.prevMaxWidth = 0
		return m, nil
	}
	if msg.Result.Lyrics == nil {
		m.state = StateNotFound
		// Clear prev dimensions so not-found shows at natural size
		m.prevLineCount = 0
		m.prevMaxWidth = 0
		return m, nil
	}
	m.lyrics = msg.Result.Lyrics
	m.state = StateLoaded
	// Clear prev dimensions - new lyrics determine size
	m.prevLineCount = 0
	m.prevMaxWidth = 0
	// Update current line based on position
	m.currentLine = m.lyrics.LineAt(m.position)
	m.centerCurrentLine()
	return m, nil
}

// View implements popup.Popup.
func (m *Model) View() string {
	if m.Width() == 0 || m.Height() == 0 {
		return ""
	}
	return m.render()
}

func (m *Model) render() string {
	t := styles.T()
	titleStyle := t.S().Title
	footerStyle := t.S().Subtle

	var content string
	switch m.state {
	case StateLoading:
		content = m.renderLoading()
	case StateNotFound:
		content = m.renderNotFound()
	case StateError:
		content = m.renderError()
	case StateLoaded:
		content = m.renderLyrics()
	}

	var result strings.Builder
	result.WriteString(titleStyle.Render("Lyrics"))
	result.WriteString("\n\n")
	result.WriteString(content)
	result.WriteString("\n\n")
	result.WriteString(footerStyle.Render(m.buildFooter()))

	return result.String()
}

func (m *Model) renderLoading() string {
	t := styles.T()
	subtle := t.S().Subtle

	// Build loading message
	loadingMsg := "Loading lyrics..."
	trackInfo := m.trackTitle
	if m.trackArtist != "" {
		trackInfo += " - " + m.trackArtist
	}

	// If we have previous dimensions, pad to maintain popup size
	if m.prevLineCount > 0 && m.prevMaxWidth > 0 {
		lines := make([]string, m.prevLineCount)

		// Center the loading message vertically
		centerLine := m.prevLineCount / 2
		trackInfoLine := centerLine + 1

		for i := range lines {
			switch i {
			case centerLine:
				lines[i] = m.centerToWidth(subtle.Render(loadingMsg), m.prevMaxWidth)
			case trackInfoLine:
				lines[i] = m.centerToWidth(subtle.Render(trackInfo), m.prevMaxWidth)
			default:
				lines[i] = strings.Repeat(" ", m.prevMaxWidth)
			}
		}
		return strings.Join(lines, "\n")
	}

	// No previous dimensions - use simple format
	var sb strings.Builder
	sb.WriteString(subtle.Render(loadingMsg))
	sb.WriteString("\n\n")
	sb.WriteString(subtle.Render(trackInfo))
	return sb.String()
}

func (m *Model) renderNotFound() string {
	t := styles.T()
	subtle := t.S().Subtle

	notFoundMsg := "No lyrics found"
	trackInfo := m.trackTitle
	if m.trackArtist != "" {
		trackInfo += " - " + m.trackArtist
	}

	var sb strings.Builder
	sb.WriteString(subtle.Render(notFoundMsg))
	sb.WriteString("\n\n")
	sb.WriteString(subtle.Render(trackInfo))
	return sb.String()
}

func (m *Model) renderError() string {
	t := styles.T()
	subtle := t.S().Subtle
	errorStyle := lipgloss.NewStyle().Foreground(t.Error)

	var sb strings.Builder
	sb.WriteString(errorStyle.Render("Error loading lyrics"))
	sb.WriteString("\n\n")
	sb.WriteString(subtle.Render(m.errorMsg))
	return sb.String()
}

func (m *Model) renderLyrics() string {
	if m.lyrics == nil || len(m.lyrics.Lines) == 0 {
		return m.renderNotFound()
	}

	t := styles.T()
	currentStyle := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	normalStyle := t.S().Subtle

	lines := make([]string, len(m.lyrics.Lines))
	maxWidth := 0
	for i, line := range m.lyrics.Lines {
		if i == m.currentLine {
			lines[i] = currentStyle.Render("▶ " + line.Text)
		} else {
			lines[i] = normalStyle.Render("  " + line.Text)
		}
		if w := lipgloss.Width(lines[i]); w > maxWidth {
			maxWidth = w
		}
	}

	// Calculate visible area
	visibleHeight := m.visibleHeight()
	if visibleHeight <= 0 {
		visibleHeight = len(lines)
	}

	// Apply scroll offset
	startLine := m.scrollOffset
	endLine := min(startLine+visibleHeight, len(lines))
	startLine = min(startLine, len(lines))

	visibleLines := lines[startLine:endLine]

	// Pad lines to max width for consistent popup sizing
	for i, line := range visibleLines {
		if w := lipgloss.Width(line); w < maxWidth {
			visibleLines[i] = line + strings.Repeat(" ", maxWidth-w)
		}
	}

	return strings.Join(visibleLines, "\n")
}

func (m *Model) buildFooter() string {
	var parts []string

	// Loading indicator when fetching new lyrics
	if m.state == StateLoading {
		parts = append(parts, "loading...")
	}

	// Position/duration
	if m.duration > 0 {
		parts = append(parts, fmt.Sprintf("%s / %s",
			formatDuration(m.position),
			formatDuration(m.duration)))
	}

	// Sync indicator (only when loaded, not during loading)
	if m.state == StateLoaded && m.lyrics != nil {
		parts = append(parts, m.renderSyncIndicator())
	}

	// Scroll info
	if m.state == StateLoaded && m.maxScroll() > 0 {
		parts = append(parts, "j/k scroll")
	}

	parts = append(parts, "esc close")

	return strings.Join(parts, " · ")
}

func (m *Model) visibleHeight() int {
	// Leave room for popup chrome (title, footer, borders, margins)
	return max(m.Height()-10, 5)
}

func (m *Model) maxScroll() int {
	if m.lyrics == nil {
		return 0
	}
	total := len(m.lyrics.Lines)
	visible := m.visibleHeight()
	if total <= visible {
		return 0
	}
	return total - visible
}

func (m *Model) fetchLyricsCmd() tea.Cmd {
	// Capture track path to identify stale results
	trackPath := m.trackPath
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		result := m.source.Fetch(ctx, lyrics.TrackInfo{
			FilePath: trackPath,
			Artist:   m.trackArtist,
			Title:    m.trackTitle,
			Album:    m.trackAlbum,
			Duration: m.duration,
		})
		return FetchedMsg{TrackPath: trackPath, Result: result, Err: result.Err}
	}
}

// formatDuration formats a duration as mm:ss.
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	m := d / time.Minute
	s := (d % time.Minute) / time.Second
	return fmt.Sprintf("%d:%02d", m, s)
}

// renderSyncIndicator returns the styled sync/unsync indicator.
func (m *Model) renderSyncIndicator() string {
	t := styles.T()
	if !m.lyrics.IsSynced() {
		return lipgloss.NewStyle().Foreground(t.Error).Render("unsynced")
	}
	syncStyle := lipgloss.NewStyle().Foreground(t.Primary)
	if m.autoScroll {
		return syncStyle.Render("synced")
	}
	return syncStyle.Render("c sync")
}

// calculateMaxWidth returns the maximum width of the lyrics lines.
func (m *Model) calculateMaxWidth() int {
	if m.lyrics == nil {
		return 0
	}
	maxW := 0
	for _, line := range m.lyrics.Lines {
		// Account for the prefix ("▶ " or "  ")
		w := lipgloss.Width(line.Text) + 2
		if w > maxW {
			maxW = w
		}
	}
	return maxW
}

// centerToWidth centers a string within the specified width.
func (m *Model) centerToWidth(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	pad := (width - w) / 2
	return strings.Repeat(" ", pad) + s + strings.Repeat(" ", width-w-pad)
}

// ActionMsg creates an action.Msg for a lyrics action.
func ActionMsg(a action.Action) action.Msg {
	return action.Msg{Source: "lyrics", Action: a}
}
