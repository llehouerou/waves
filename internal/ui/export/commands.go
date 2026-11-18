package export

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/linkfs"
	"github.com/llehouerou/waves/internal/ui/request"
)

// VolumesLoadedMsg contains detected volumes.
type VolumesLoadedMsg struct {
	Request request.ID
	Volumes []core.ExportVolume
	Err     error
}

// TargetsLoadedMsg contains saved targets.
type TargetsLoadedMsg struct {
	Request request.ID
	Targets []core.ExportTarget
	Err     error
}

// TargetCreatedMsg signals a new target was created.
type TargetCreatedMsg struct {
	Request request.ID
	Target  core.ExportTarget
	Err     error
}

// loadVolumes detects mounted volumes.
func loadVolumes(exports Exports) func(request.ID) tea.Cmd {
	return func(id request.ID) tea.Cmd {
		return func() tea.Msg {
			volumes, err := exports.Volumes()
			return VolumesLoadedMsg{Request: id, Volumes: volumes, Err: err}
		}
	}
}

// loadTargets loads saved targets.
func loadTargets(exports Exports) func(request.ID) tea.Cmd {
	return func(id request.ID) tea.Cmd {
		return func() tea.Msg {
			targets, err := exports.Targets()
			return TargetsLoadedMsg{Request: id, Targets: targets, Err: err}
		}
	}
}

// createTarget saves a new target.
func createTarget(exports Exports, target core.ExportTarget) func(request.ID) tea.Cmd {
	return func(id request.ID) tea.Cmd {
		return func() tea.Msg {
			targetID, err := exports.CreateTarget(target)
			if err == nil {
				target.ID = targetID
			}
			return TargetCreatedMsg{Request: id, Target: target, Err: err}
		}
	}
}

// DirectoriesLoadedMsg contains subdirectories for browsing.
type DirectoriesLoadedMsg struct {
	Request request.ID
	Path    string   // The path that was listed
	Dirs    []string // Subdirectory names (sorted, no hidden)
	Err     error
}

// listDirectories lists subdirectories in a path.
func listDirectories(basePath, subPath string) func(request.ID) tea.Cmd {
	return func(id request.ID) tea.Cmd {
		return func() tea.Msg {
			dirs, err := subdirectories(filepath.Join(basePath, subPath))
			return DirectoriesLoadedMsg{Request: id, Path: subPath, Dirs: dirs, Err: err}
		}
	}
}

// subdirectories are the names of the folders in path, a symbolic link to a
// folder among them, sorted, hidden ones left out.
func subdirectories(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, entry := range entries {
		if linkfs.KindOf(filepath.Join(path, entry.Name()), entry) != linkfs.Dir {
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
	return dirs, nil
}
