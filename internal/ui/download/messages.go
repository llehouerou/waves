package download

import "github.com/llehouerou/waves/internal/ui/request"

// SlskdDownloadQueuedMsg is the outcome of the queueing of the download of
// Album by Artist: queued, or why it was not.
type SlskdDownloadQueuedMsg struct {
	Request request.ID
	Artist  string
	Album   string
	Err     error
}
