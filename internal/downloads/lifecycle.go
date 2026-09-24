package downloads

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/llehouerou/waves/internal/slskd"
)

// ErrNoSlskd is returned by the operations that need slskd when it isn't
// configured.
var ErrNoSlskd = errors.New("slskd is not configured")

// Queue queues the download's files on slskd, then records the download.
// Nothing is recorded when slskd refuses.
func (m *Manager) Queue(d Download) (int64, error) {
	if m.client == nil {
		return 0, ErrNoSlskd
	}
	if err := m.client.Download(d.SlskdUsername, slskdFiles(d.Files)); err != nil {
		return 0, err
	}
	return m.create(d)
}

// Sync reads slskd's transfers into the downloads' states, then checks
// completed files on disk. Without slskd there is nothing to sync.
func (m *Manager) Sync() error {
	if m.client == nil {
		return nil
	}
	transfers, err := m.client.GetDownloads()
	if err != nil {
		return err
	}
	if err := m.updateFromSlskd(transfers); err != nil {
		return err
	}
	return m.verifyOnDisk()
}

// Retry re-queues the files of a download that are failed now. slskd resumes
// from the partial file, if any.
func (m *Manager) Retry(id int64) error {
	if m.client == nil {
		return ErrNoSlskd
	}
	d, err := m.Get(id)
	if err != nil {
		return err
	}
	failed := d.FailedFiles()
	if len(failed) == 0 {
		return nil
	}
	return m.client.Download(d.SlskdUsername, slskdFiles(failed))
}

// Delete cancels a download's slskd transfers, removes its folder from the
// completed folder with everything in it, and drops it.
//
// ponytail: the folder is named after the last part of the slskd path only, so
// two downloads called "Album" or "CD1" share it and Delete takes both. Only
// ever on an explicit user action; per-download folders would fix it.
func (m *Manager) Delete(id int64) error {
	d, err := m.Get(id)
	if err != nil {
		return err
	}
	m.dropTransfers([]Download{*d})
	if dir, ok := m.folder(d); ok {
		if err := os.RemoveAll(dir); err != nil {
			return err // keep the row: the user can delete again
		}
	}
	return m.deleteRow(id)
}

// FinishImport ends a download whose tracks the import moved into the library:
// it drops its slskd transfer records and its row, and removes its folder if
// the import left it empty. A folder still holding anything is kept: it can be
// another download's that shares its name.
func (m *Manager) FinishImport(id int64) error {
	d, err := m.Get(id)
	if err != nil {
		return err
	}
	m.dropTransfers([]Download{*d})
	if dir, ok := m.folder(d); ok {
		_ = os.Remove(dir) //nolint:errcheck // not empty or already gone: nothing to clean
	}
	return m.deleteRow(id)
}

// ClearCompleted drops every completed download and its slskd transfer
// records, keeping its files.
func (m *Manager) ClearCompleted() error {
	all, err := m.List()
	if err != nil {
		return err
	}
	var completed []Download
	for i := range all {
		if all[i].Status == StatusCompleted {
			completed = append(completed, all[i])
		}
	}
	m.dropTransfers(completed)
	_, err = m.db.Exec(`DELETE FROM downloads WHERE status = ?`, StatusCompleted)
	return err
}

// dropTransfers cancels and removes the slskd transfers backing the given
// downloads, so a stale record can't match a later download of the same
// files. Best effort: slskd being unreachable must not block removal.
func (m *Manager) dropTransfers(dls []Download) {
	if m.client == nil || len(dls) == 0 {
		return
	}
	transfers, err := m.client.GetDownloads()
	if err != nil {
		return //nolint:nilerr // best effort: slskd being unreachable must not block removal
	}
	for i := range dls {
		d := &dls[i]
		if d.SlskdUsername == "" {
			continue
		}
		filenames := make([]string, 0, len(d.Files))
		for _, f := range d.Files {
			filenames = append(filenames, f.Filename)
		}
		for _, id := range slskd.TransferIDs(transfers, d.SlskdUsername, filenames) {
			_ = m.client.CancelDownload(d.SlskdUsername, id, true) //nolint:errcheck // best effort, as above
		}
	}
}

// folder is a download's folder in the completed folder. ok is false when
// there is no completed folder, or when the download has no folder of its own
// ("", ".", ".."): nothing above its folder may ever be removed.
func (m *Manager) folder(d *Download) (dir string, ok bool) {
	if m.completedPath == "" {
		return "", false
	}
	dir = BuildDiskPath(m.completedPath, d.SlskdDirectory)
	return dir, filepath.Dir(dir) == filepath.Clean(m.completedPath)
}

func slskdFiles(files []DownloadFile) []slskd.File {
	out := make([]slskd.File, 0, len(files))
	for _, f := range files {
		out = append(out, slskd.File{Filename: f.Filename, Size: f.Size})
	}
	return out
}
