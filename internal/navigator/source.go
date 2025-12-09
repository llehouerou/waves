package navigator

// IconType represents the type of icon to display for a node.
type IconType int

const (
	IconFolder IconType = iota
	IconAudio
	IconArtist
	IconAlbum
	IconPlaylist
)

// Node represents an item that can be displayed and potentially navigated into.
type Node interface {
	ID() string
	DisplayName() string
	IsContainer() bool
	IconType() IconType
}

// PreviewProvider is an optional interface that nodes can implement
// to provide custom preview content for the right column.
type PreviewProvider interface {
	// PreviewLines returns lines to display in the preview column.
	// Returns nil to use default behavior (show children).
	PreviewLines() []string
}

// PreviewProviderWithWidth is an optional interface for nodes that need
// the column width to render their preview (e.g., for wrapping long text).
type PreviewProviderWithWidth interface {
	// PreviewLinesWithWidth returns lines to display, given the column width.
	// Returns nil to use default behavior (show children).
	PreviewLinesWithWidth(width int) []string
}

// TrackIDProvider is an optional interface for nodes that represent library tracks.
// Nodes implementing this can have their track ID extracted for favorites display.
type TrackIDProvider interface {
	// TrackID returns the library track ID, or 0 if not a library track.
	TrackID() int64
}

// Source provides data and navigation logic for the navigator.
type Source[T Node] interface {
	// Root returns the root container node.
	Root() T

	// Children returns the children of a container node.
	// Returns empty slice if node has no children.
	Children(parent T) ([]T, error)

	// Parent returns the parent of a node, or nil if at root.
	Parent(node T) *T

	// DisplayPath returns a human-readable path for display in the header.
	DisplayPath(node T) string

	// NodeFromID creates a node from its ID, if possible.
	NodeFromID(id string) (T, bool)
}
