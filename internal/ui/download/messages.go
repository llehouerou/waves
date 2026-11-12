package download

// SlskdDownloadQueuedMsg is sent when download is queued.
type SlskdDownloadQueuedMsg struct {
	Err error
}
