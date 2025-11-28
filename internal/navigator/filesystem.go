package navigator

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/llehouerou/waves/internal/player"
)

// FileNode represents a file or directory.
type FileNode struct {
	path  string
	name  string
	isDir bool
}

func (n FileNode) ID() string { return n.path }

func (n FileNode) DisplayName() string { return n.name }

func (n FileNode) IsContainer() bool { return n.isDir }

// FileSource provides filesystem navigation.
type FileSource struct {
	root string
}

// NewFileSource creates a new filesystem source starting at the given path.
func NewFileSource(startPath string) (*FileSource, error) {
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return nil, err
	}
	return &FileSource{root: absPath}, nil
}

func (s *FileSource) Root() FileNode {
	info, err := os.Stat(s.root)
	if err != nil {
		return FileNode{path: s.root, name: filepath.Base(s.root), isDir: true}
	}
	return FileNode{path: s.root, name: info.Name(), isDir: info.IsDir()}
}

func (s *FileSource) Children(parent FileNode) ([]FileNode, error) {
	if !parent.isDir {
		return nil, nil
	}

	entries, err := os.ReadDir(parent.path)
	if err != nil {
		return nil, err
	}

	nodes := make([]FileNode, 0, len(entries))
	for _, e := range entries {
		name := e.Name()

		// Skip hidden files and directories
		if strings.HasPrefix(name, ".") {
			continue
		}

		// Only include directories and music files
		path := filepath.Join(parent.path, name)
		if !e.IsDir() && !player.IsMusicFile(path) {
			continue
		}

		nodes = append(nodes, FileNode{
			path:  path,
			name:  name,
			isDir: e.IsDir(),
		})
	}

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].isDir != nodes[j].isDir {
			return nodes[i].isDir
		}
		return nodes[i].name < nodes[j].name
	})

	return nodes, nil
}

func (s *FileSource) Parent(node FileNode) *FileNode {
	parentPath := filepath.Dir(node.path)
	if parentPath == node.path {
		return nil // at root
	}

	return &FileNode{
		path:  parentPath,
		name:  filepath.Base(parentPath),
		isDir: true,
	}
}

func (s *FileSource) DisplayPath(node FileNode) string {
	return node.path
}

func (s *FileSource) NodeFromID(id string) (FileNode, bool) {
	info, err := os.Stat(id)
	if err != nil {
		return FileNode{}, false
	}
	return FileNode{
		path:  id,
		name:  filepath.Base(id),
		isDir: info.IsDir(),
	}, true
}
