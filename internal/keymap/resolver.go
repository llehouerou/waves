package keymap

// Resolver maps key strings to actions.
type Resolver struct {
	bindings map[string]Action   // key -> action
	byAction map[Action][]string // action -> keys (for help/documentation)
}

// NewResolver creates a resolver from bindings.
func NewResolver(bindings []Binding) *Resolver {
	r := &Resolver{
		bindings: make(map[string]Action),
		byAction: make(map[Action][]string),
	}
	for _, b := range bindings {
		for _, key := range b.Keys {
			r.bindings[key] = b.Action
		}
		// Collect all keys for each action (may have duplicates from different contexts)
		r.byAction[b.Action] = append(r.byAction[b.Action], b.Keys...)
	}
	// Deduplicate keys per action
	for action, keys := range r.byAction {
		r.byAction[action] = dedupe(keys)
	}
	return r
}

// Resolve returns the action for a key, or empty string if not bound.
func (r *Resolver) Resolve(key string) Action {
	return r.bindings[key]
}

// KeysFor returns the keys bound to an action (for help/documentation).
func (r *Resolver) KeysFor(action Action) []string {
	return r.byAction[action]
}

// dedupe removes duplicate strings from a slice.
func dedupe(s []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(s))
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}
