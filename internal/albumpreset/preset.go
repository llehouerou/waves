package albumpreset

// Preset represents a saved grouping/sorting configuration.
type Preset struct {
	ID       int64
	Name     string
	Settings Settings
}
