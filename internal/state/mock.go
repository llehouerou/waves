// internal/state/mock.go
package state

import (
	"database/sql"
	"sync"
	"time"

	"github.com/llehouerou/waves/internal/albumpreset"
)

// Mock is a test double for Manager.
type Mock struct {
	mu          sync.Mutex // the core saves the queue and the volume from its own goroutine
	navState    *NavigationState
	queueState  *QueueState
	volumeState *VolumeState
	presets     []albumpreset.Preset
	closed      bool
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

func (m *Mock) SaveQueue(state QueueState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queueState = &state
	return nil
}

func (m *Mock) GetQueue() (*QueueState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.queueState, nil
}

func (m *Mock) GetVolume() (*VolumeState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.volumeState == nil {
		return &VolumeState{Volume: 1.0, Muted: false}, nil
	}
	return m.volumeState, nil
}

func (m *Mock) SaveVolume(volume float64, muted bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.volumeState = &VolumeState{Volume: volume, Muted: muted}
	return nil
}

// The mock holds no Last.fm session and keeps no pending scrobble.

func (m *Mock) GetLastfmSession() (*LastfmSession, error) {
	return nil, nil //nolint:nilnil // not linked, as Manager says it
}

func (m *Mock) SaveLastfmSession(_, _ string) error { return nil }

func (m *Mock) DeleteLastfmSession() error { return nil }

func (m *Mock) AddPendingScrobble(_ PendingScrobble) error { return nil }

func (m *Mock) GetPendingScrobbles() ([]PendingScrobble, error) { return nil, nil }

func (m *Mock) DeletePendingScrobble(_ int64) error { return nil }

func (m *Mock) UpdatePendingScrobbleAttempt(_ int64, _ string) error { return nil }

func (m *Mock) DeleteOldPendingScrobbles(_ time.Duration) error { return nil }

func (m *Mock) Close() error {
	m.closed = true
	return nil
}

func (m *Mock) ListAlbumPresets() ([]albumpreset.Preset, error) {
	return m.presets, nil
}

func (m *Mock) SaveAlbumPreset(name string, settings albumpreset.Settings) (int64, error) {
	id := int64(len(m.presets) + 1)
	m.presets = append(m.presets, albumpreset.Preset{ID: id, Name: name, Settings: settings})
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
