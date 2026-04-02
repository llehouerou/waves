package playerbar

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/icons"
	"github.com/llehouerou/waves/internal/ui/styles"
)

// playSymbol returns the play icon based on current icon style.
func playSymbol() string {
	return icons.Play()
}

// pauseSymbol returns the pause icon based on current icon style.
func pauseSymbol() string {
	return icons.Pause()
}

func barStyle() lipgloss.Style {
	t := styles.T()
	s := t.BaseStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(t.Border)
	if t.HasExplicitBackground {
		s = s.BorderBackground(t.BgBase)
	}
	return s
}

func expandedBarStyle() lipgloss.Style {
	t := styles.T()
	s := t.BaseStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Padding(0, 2)
	if t.HasExplicitBackground {
		s = s.BorderBackground(t.BgBase)
	}
	return s
}

func titleStyle() lipgloss.Style {
	return styles.T().S().Title
}

func artistStyle() lipgloss.Style {
	return styles.T().S().Base
}

func metaStyle() lipgloss.Style {
	return styles.T().S().Subtle
}

func progressTimeStyle() lipgloss.Style {
	return styles.T().S().Muted
}

func progressBarFilled() lipgloss.Style {
	return styles.T().BaseStyle().Foreground(styles.T().Primary)
}

func progressBarEmpty() lipgloss.Style {
	return styles.T().BaseStyle().Foreground(styles.T().FgSubtle)
}

func radioStyle() lipgloss.Style {
	return styles.T().BaseStyle().Foreground(styles.T().Secondary)
}
