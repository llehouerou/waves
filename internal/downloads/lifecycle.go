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
	return m.Create(d)
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

// Delete cancels a download's slskd transfers, removes its files from the
// completed folder and drops it.
func (m *Manager) Delete(id int64) error {
	d, err := m.Get(id)
	if err != nil {
		return err
	}
	m.dropTransfers([]Download{*d})
	if err := m.removeFolder(d); err != nil {
		return err // keep the row: the user can delete again
	}
	return m.deleteRow(id)
}

// Forget drops a download and its slskd transfer records, keeping its files.
func (m *Manager) Forget(id int64) error {
	d, err := m.Get(id)
	if err != nil {
		return err
	}
	m.dropTransfers([]Download{*d})
	return m.deleteRow(id)
}

// ClearCompleted forgets every completed download.
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

// removeFolder removes a download's folder from the completed folder, with
// everything in it. A download without a folder of its own ("", ".", "..")
// removes nothing: never anything above its folder.
func (m *Manager) removeFolder(d *Download) error {
	if m.completedPath == "" {
		return nil
	}
	dir := BuildDiskPath(m.completedPath, d.SlskdDirectory)
	if filepath.Dir(dir) != filepath.Clean(m.completedPath) {
		return nil
	}
	return os.RemoveAll(dir)
}

func slskdFiles(files []DownloadFile) []slskd.File {
	out := make([]slskd.File, 0, len(files))
	for _, f := range files {
		out = append(out, slskd.File{Filename: f.Filename, Size: f.Size})
	}
	return out
}
