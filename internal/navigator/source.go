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
