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

// DefaultPath is where the state database of the user is: in the waves
// folder of their data directory, which it creates when missing.
func DefaultPath() (string, error) {
	return xdg.DataFile(filepath.Join(appName, dbFileName))
}

// OpenAt opens the state database at dbPath, creating it when missing.
func OpenAt(dbPath string) (*Manager, error) {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}

	// Pragmas that live on the connection go in the DSN, so that every
	// connection of the pool gets them: a PRAGMA run through db.Exec reaches one
	// connection only. Wait up to 5s for locks, or a second writer on another
	// connection fails at once with SQLITE_BUSY. Enforce foreign keys, so the
	// ON DELETE CASCADE of the schema fires whatever connection deletes.
	//
	// Every transaction takes the write lock when it begins (they all write).
	// Begun as a reader, one whose first statement reads before it writes
	// (connecting the FTS5 table does) cannot wait for the lock: SQLite fails
	// it at once with SQLITE_BUSY, whatever the timeout.
	db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)"+
		"&_pragma=foreign_keys(1)"+
		"&_pragma=synchronous(NORMAL)"+ // Good balance of safety/speed with WAL
		"&_txlock=immediate")
	if err != nil {
		return nil, err
	}

	// Stored in the file: once is enough.
	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		db.Close()
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

func (m *Manager) GetQueue() (*QueueState, error) {
	return getQueue(m.db)
}

func (m *Manager) SaveQueue(state QueueState) error {
	return saveQueue(m.db, state)
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
