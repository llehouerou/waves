package app

import (
	"go/ast"
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
	module + "importer",
	module + "rename",
	module + "lastfm",
	module + "listenbrainz",
	module + "releases",
	module + "export",
	module + "lyrics",
	module + "lrclib",
	module + "notify",
	module + "cover",
	module + "mpris",
	module + "state",
	module + "db",
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
	walkImports(t, root, false, fn)
}

// walkImports calls fn with every import of the Go files under root, the test
// files included when tests is set. Like the go command, it leaves out the
// directories vendor and testdata, and those whose name begins with . or _.
func walkImports(t *testing.T, root string, tests bool, fn func(file, pkg string)) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != root && ignoredDir(d.Name()) {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || (!tests && strings.HasSuffix(path, "_test.go")) {
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

// ignoredDir reports whether the go command leaves out a directory so named.
func ignoredDir(name string) bool {
	return name == "vendor" || name == "testdata" ||
		strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
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

// runewidth measures a text the way the locale says: the frame is laid out by
// lipgloss and Bubble Tea, whose measure reads no locale, so a second measure
// cuts names that fit and leaves columns short (issue #317).
const runewidth = "github.com/mattn/go-runewidth"

func TestTextIsMeasuredAsLipglossDoes(t *testing.T) {
	walkImports(t, "../..", true, func(file, pkg string) {
		if pkg == runewidth {
			t.Errorf("%s imports %s: measure a text with lipgloss.Width, ansi.StringWidth or internal/ui/render", file, pkg)
		}
	})
}

// requestPkg is the package that names the requests of the popups.
const requestPkg = module + "ui/request"

// isRequestID reports whether expr is the type request.ID, in a file that
// imports the request package as name.
func isRequestID(expr ast.Expr, name string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "ID" {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	return ok && name != "" && x.Name == name
}

// requestName is the name file imports the request package as, empty if it
// does not import it.
func requestName(file *ast.File) string {
	for _, imp := range file.Imports {
		if pkg, err := strconv.Unquote(imp.Path.Value); err != nil || pkg != requestPkg {
			continue
		}
		if imp.Name != nil {
			return imp.Name.Name
		}
		return "request"
	}
	return ""
}

// A message of the UI packages that carries an error is the answer to a
// request a popup made: it names the request, so that the popup drops any
// answer it does not wait for.
func TestPopupAnswersNameTheirRequest(t *testing.T) {
	err := filepath.WalkDir("../ui", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		name := requestName(file)
		ast.Inspect(file, func(n ast.Node) bool {
			spec, ok := n.(*ast.TypeSpec)
			if !ok || !strings.HasSuffix(spec.Name.Name, "Msg") {
				return true
			}
			st, ok := spec.Type.(*ast.StructType)
			if !ok {
				return true
			}
			var hasErr, hasID bool
			for _, field := range st.Fields.List {
				if isRequestID(field.Type, name) {
					hasID = true
				}
				if id, ok := field.Type.(*ast.Ident); ok && id.Name == "error" {
					for _, n := range field.Names {
						hasErr = hasErr || n.Name == "Err"
					}
				}
			}
			if hasErr && !hasID {
				t.Errorf("%s: %s carries an error and no request.ID: send its request through request.Pending", path, spec.Name.Name)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk ../ui: %v", err)
	}
}
