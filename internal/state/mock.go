// internal/state/mock.go
package state

import (
	"database/sql"

	"github.com/llehouerou/waves/internal/ui/albumview"
)

// Mock is a test double for Manager.
type Mock struct {
	navState   *NavigationState
	queueState *QueueState
	presets    []albumview.Preset
	closed     bool
}

// NewMock creates a new mock state manager for testing.
func NewMock() *Mock {
	return &Mock{}
}

func (m *Mock) DB() *sql.DB { return nil }

func (m *Mock) SaveNavigation(_ NavigationState) {}

func (m *Mock) GetNavigation() (*NavigationState, error) {
	return m.navState, nil
}

func (m *Mock) SaveQueue(_ QueueState) error {
	return nil
}

func (m *Mock) GetQueue() (*QueueState, error) {
	return m.queueState, nil
}

func (m *Mock) Close() error {
	m.closed = true
	return nil
}

func (m *Mock) ListAlbumPresets() ([]albumview.Preset, error) {
	return m.presets, nil
}

func (m *Mock) SaveAlbumPreset(name string, settings albumview.Settings) (int64, error) {
	id := int64(len(m.presets) + 1)
	m.presets = append(m.presets, albumview.Preset{ID: id, Name: name, Settings: settings})
	return id, nil
}

func (m *Mock) DeleteAlbumPreset(id int64) error {
	for i, p := range m.presets {
		if p.ID == id {
			m.presets = append(m.presets[:i], m.presets[i+1:]...)
			break
		}
	}
	return nil
}

// Test helpers

func (m *Mock) SetNavigation(state *NavigationState) { m.navState = state }

func (m *Mock) SetQueue(state *QueueState) { m.queueState = state }

func (m *Mock) IsClosed() bool { return m.closed }

// Verify Mock implements Interface at compile time.
var _ Interface = (*Mock)(nil)
