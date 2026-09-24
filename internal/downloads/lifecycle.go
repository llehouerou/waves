package downloads

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/llehouerou/waves/internal/slskd"
)

// ErrNoSlskd is returned by the operations that need slskd when it isn't
// configured.
var ErrNoSlskd = errors.New("slskd is not configured")

// Queue queues the download's files on slskd, then records the download.
// Nothing is queued or recorded when one of its download folders is taken (see
// checkFolder), and nothing is recorded when slskd refuses.
func (m *Manager) Queue(d Download) (int64, error) {
	if m.client == nil {
		return 0, ErrNoSlskd
	}
	if err := m.checkFolder(d); err != nil {
		return 0, err
	}
	if err := m.client.Download(d.SlskdUsername, slskdFiles(d.Files)); err != nil {
		return 0, err
	}
	return m.create(d)
}

// checkFolder refuses a download any of whose download folders is already
// another's: a download waves still tracks, whatever its status, or a
// directory already in the completed folder. slskd names the folder after the
// last part of the slskd path only, so two downloads sharing it would mix their
// files.
func (m *Manager) checkFolder(d Download) error {
	all, err := m.List()
	if err != nil {
		return err
	}
	takenBy := make(map[string]*Download)
	for i := range all {
		for _, dir := range all[i].Folders() {
			takenBy[ExtractFolderName(dir)] = &all[i]
		}
	}
	for _, dir := range d.Folders() {
		path := BuildDiskPath(m.completedPath, dir)
		if other, ok := takenBy[ExtractFolderName(dir)]; ok {
			return fmt.Errorf("download folder %s is already used by %s / %s",
				path, other.MBArtistName, other.MBAlbumTitle)
		}
		if m.completedPath == "" {
			continue
		}
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return fmt.Errorf("download folder %s already exists", path)
		}
	}
	return nil
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

// Delete cancels a download's slskd transfers, removes its folders from the
// completed folder with everything in them, and drops it. Queue gives each
// download folders of its own, so nothing else is in them.
func (m *Manager) Delete(id int64) error {
	d, err := m.Get(id)
	if err != nil {
		return err
	}
	m.dropTransfers([]Download{*d})
	for _, dir := range m.folders(d) {
		if err := os.RemoveAll(dir); err != nil {
			return err // keep the row: the user can delete again
		}
	}
	return m.deleteRow(id)
}

// FinishImport ends a download whose tracks the import moved into the library:
// it drops its slskd transfer records and its row, and removes each of its
// folders the import left empty. A folder still holding anything (extras such
// as .nfo or .cue) is kept.
func (m *Manager) FinishImport(id int64) error {
	d, err := m.Get(id)
	if err != nil {
		return err
	}
	m.dropTransfers([]Download{*d})
	for _, dir := range m.folders(d) {
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

// folders are a download's folders in the completed folder. None when there is
// no completed folder; a file with no folder of its own ("", ".", "..") adds
// none: nothing above its folder may ever be removed.
func (m *Manager) folders(d *Download) []string {
	if m.completedPath == "" {
		return nil
	}
	var dirs []string
	for _, slskdDir := range d.Folders() {
		if dir := BuildDiskPath(m.completedPath, slskdDir); filepath.Dir(dir) == filepath.Clean(m.completedPath) {
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

func slskdFiles(files []DownloadFile) []slskd.File {
	out := make([]slskd.File, 0, len(files))
	for _, f := range files {
		out = append(out, slskd.File{Filename: f.Filename, Size: f.Size})
	}
	return out
}
