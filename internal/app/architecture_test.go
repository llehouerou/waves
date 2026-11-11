package app

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// coreOwned are the domain packages the core has taken over: the TUI reaches
// them through internal/core only (ADR 0001). Each step of the core extraction
// (issue #88) adds its packages here.
var coreOwned = []string{
	"github.com/llehouerou/waves/internal/playback",
	"github.com/llehouerou/waves/internal/player",
	"github.com/llehouerou/waves/internal/radio",
}

// Test files are left out: they hand the core a player.Mock.
func TestTUIReachesCoreOwnedPackagesThroughTheCoreOnly(t *testing.T) {
	for _, root := range []string{".", "../ui"} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, imp := range file.Imports {
				pkg, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					return err
				}
				if slices.Contains(coreOwned, pkg) {
					t.Errorf("%s imports %s: go through internal/core", path, pkg)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
}
