package navigator

// Node represents an item that can be displayed and potentially navigated into.
type Node interface {
	ID() string
	DisplayName() string
	IsContainer() bool
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
}
