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

const module = "github.com/llehouerou/waves/internal/"

// coreOwned are the domain packages the core has taken over: the TUI reaches
// them, and the packages under them, through internal/core only (ADR 0001).
// Each step of the core extraction (issue #88) adds its packages here.
var coreOwned = []string{
	module + "playback",
	module + "player",
	module + "radio",
	module + "library",
	module + "playlists",
	module + "playlist",
	module + "downloads",
	module + "slskd",
	module + "musicbrainz",
}

// presentation are the packages that draw the TUI, and the framework it is
// built on. A domain the core owns imports none of them: the core would carry
// the TUI into every client.
var presentation = []string{
	module + "navigator",
	module + "icons",
	module + "ui",
	"github.com/charmbracelet",
}

// within reports whether pkg is one of pkgs or a package under one of them.
func within(pkg string, pkgs []string) bool {
	return slices.ContainsFunc(pkgs, func(p string) bool {
		return pkg == p || strings.HasPrefix(pkg, p+"/")
	})
}

// eachImport calls fn with every import of the non-test Go files under root.
// Test files are left out: they hand the core a player.Mock, or build a
// library on a test database.
func eachImport(t *testing.T, root string, fn func(file, pkg string)) {
	t.Helper()
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
			fn(path, pkg)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
}

func TestTUIReachesCoreOwnedPackagesThroughTheCoreOnly(t *testing.T) {
	for _, root := range []string{".", "../ui"} {
		eachImport(t, root, func(file, pkg string) {
			if within(pkg, coreOwned) {
				t.Errorf("%s imports %s: go through internal/core", file, pkg)
			}
		})
	}
}

func TestCoreOwnedPackagesHoldNoPresentation(t *testing.T) {
	for _, owned := range coreOwned {
		eachImport(t, "../"+strings.TrimPrefix(owned, module), func(file, pkg string) {
			if within(pkg, presentation) {
				t.Errorf("%s imports %s: presentation belongs to the client", file, pkg)
			}
		})
	}
}
