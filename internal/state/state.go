package state

import (
	"database/sql"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/adrg/xdg"
	_ "modernc.org/sqlite" // SQLite driver
)

const (
	appName      = "waves"
	dbFileName   = "waves.db"
	saveDebounce = 500 * time.Millisecond
)

type Manager struct {
	db        *sql.DB
	saveMu    sync.Mutex
	saveTimer *time.Timer
	pending   *NavigationState
}

func Open() (*Manager, error) {
	dbPath, err := getDBPath()
	if err != nil {
		return nil, err
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	if err := initSchema(db); err != nil {
		db.Close()
		return nil, err
	}

	return &Manager{db: db}, nil
}

func (m *Manager) Close() error {
	m.saveMu.Lock()
	if m.saveTimer != nil {
		m.saveTimer.Stop()
	}
	pending := m.pending
	m.pending = nil
	m.saveMu.Unlock()

	// Flush pending state
	if pending != nil {
		_ = saveNavigation(m.db, *pending)
	}

	return m.db.Close()
}

func (m *Manager) GetNavigation() (*NavigationState, error) {
	return getNavigation(m.db)
}

func (m *Manager) DB() *sql.DB {
	return m.db
}

func (m *Manager) SaveNavigation(state NavigationState) {
	m.saveMu.Lock()
	defer m.saveMu.Unlock()

	m.pending = &state

	if m.saveTimer != nil {
		m.saveTimer.Stop()
	}

	m.saveTimer = time.AfterFunc(saveDebounce, func() {
		m.saveMu.Lock()
		pending := m.pending
		m.pending = nil
		m.saveMu.Unlock()

		if pending != nil {
			_ = saveNavigation(m.db, *pending)
		}
	})
}

func getDBPath() (string, error) {
	return xdg.DataFile(filepath.Join(appName, dbFileName))
}
