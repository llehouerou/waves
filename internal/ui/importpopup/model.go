// Package importpopup is the popup that imports a completed download into the
// library: it shows what the core's import will do, lets the user pick the
// library source, and follows the import the core runs.
package importpopup

import (
	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui"
)

// Importer is what the popup asks of the core. The import it starts reports
// through core.ImportProgressed and core.ImportFinished, which the app hands
// to Update.
type Importer interface {
	PrepareImport(id int64) (core.ImportPlan, error)
	Import(plan core.ImportPlan, source string) error
}

// State represents the current state of the import popup.
type State int

const (
	StateTagPreview  State = iota // Two-column tag comparison
	StatePathPreview              // File path table + library selector
	StateImporting                // Progress view
	StateComplete                 // Results summary
)

// Model is the Bubble Tea model for the import popup.
type Model struct {
	state    State
	download *core.Download
	importer Importer

	// What the import will do, once the core has planned it
	preparing bool            // the core has not answered yet
	plan      core.ImportPlan // no release until then
	tagDiffs  []TagDiff       // current tags against the plan's
	err       string          // why there is no plan, why the import didn't start, or what failed after its files

	// Path preview data
	librarySources []string // Available destinations
	selectedSource int      // Cursor for source selection
	pathOffset     int      // Scroll offset for paths list

	// Where the import is, as the core last said
	files []core.FileProgress

	ui.Base
}

// TagDiff represents a difference between current and new tag values.
type TagDiff struct {
	Field    string
	OldValue string // Or "(N different)" for multi-value
	NewValue string
	Changed  bool
}

// New creates the import popup of a download. Its Init asks imp for the plan.
func New(download *core.Download, librarySources []string, imp Importer) *Model {
	m := &Model{
		state:          StateTagPreview,
		download:       download,
		importer:       imp,
		preparing:      true,
		librarySources: librarySources,
	}
	m.SetFocused(true)
	return m
}

// DownloadID is the download the popup imports.
func (m *Model) DownloadID() int64 {
	return m.download.ID
}
