package export

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/export"
)

// VolumesLoadedMsg contains detected volumes.
type VolumesLoadedMsg struct {
	Volumes []export.Volume
	Err     error
}

// TargetsLoadedMsg contains saved targets.
type TargetsLoadedMsg struct {
	Targets []export.Target
	Err     error
}

// TargetCreatedMsg signals a new target was created.
type TargetCreatedMsg struct {
	Target export.Target
	Err    error
}

// LoadVolumesCmd detects mounted volumes.
func LoadVolumesCmd() tea.Cmd {
	return func() tea.Msg {
		volumes, err := export.DetectVolumes()
		return VolumesLoadedMsg{Volumes: volumes, Err: err}
	}
}

// LoadTargetsCmd loads saved targets.
func LoadTargetsCmd(repo *export.TargetRepository) tea.Cmd {
	return func() tea.Msg {
		targets, err := repo.List()
		return TargetsLoadedMsg{Targets: targets, Err: err}
	}
}

// CreateTargetCmd saves a new target.
func CreateTargetCmd(repo *export.TargetRepository, target export.Target) tea.Cmd {
	return func() tea.Msg {
		id, err := repo.Create(target)
		if err == nil {
			target.ID = id
		}
		return TargetCreatedMsg{Target: target, Err: err}
	}
}

// DirectoriesLoadedMsg contains subdirectories for browsing.
type DirectoriesLoadedMsg struct {
	Path string   // The path that was listed
	Dirs []string // Subdirectory names (sorted, no hidden)
	Err  error
}

// ListDirectoriesCmd lists subdirectories in a path.
func ListDirectoriesCmd(basePath, subPath string) tea.Cmd {
	return func() tea.Msg {
		fullPath := filepath.Join(basePath, subPath)
		entries, err := os.ReadDir(fullPath)
		if err != nil {
			return DirectoriesLoadedMsg{Path: subPath, Err: err}
		}

		var dirs []string
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()
			// Skip hidden directories
			if strings.HasPrefix(name, ".") {
				continue
			}
			dirs = append(dirs, name)
		}
		sort.Strings(dirs)

		return DirectoriesLoadedMsg{Path: subPath, Dirs: dirs, Err: nil}
	}
}
