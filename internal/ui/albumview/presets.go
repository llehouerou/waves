// internal/ui/albumview/presets.go
package albumview

import (
	"encoding/json"
)

// sortCriterionJSON is the JSON representation of a SortCriterion.
type sortCriterionJSON struct {
	Field int `json:"field"`
	Order int `json:"order"`
}

// settingsJSON is the extended JSON representation including all settings.
type settingsJSON struct {
	GroupFields    []int               `json:"groupFields"`
	GroupSortOrder int                 `json:"groupSortOrder"`
	GroupDateField int                 `json:"groupDateField"`
	SortCriteria   []sortCriterionJSON `json:"sortCriteria"`
	PresetName     string              `json:"presetName,omitempty"`
}

// ToJSON serializes Settings to JSON strings for database storage.
// groupFields contains the full settings JSON, sortCriteria is kept for compatibility.
func (s Settings) ToJSON() (groupFields, sortCriteria string, err error) {
	// Convert to JSON struct
	sj := settingsJSON{
		GroupFields:    make([]int, len(s.GroupFields)),
		GroupSortOrder: int(s.GroupSortOrder),
		GroupDateField: int(s.GroupDateField),
		SortCriteria:   make([]sortCriterionJSON, len(s.SortCriteria)),
		PresetName:     s.PresetName,
	}
	for i, f := range s.GroupFields {
		sj.GroupFields[i] = int(f)
	}
	for i, c := range s.SortCriteria {
		sj.SortCriteria[i] = sortCriterionJSON{Field: int(c.Field), Order: int(c.Order)}
	}

	gfBytes, err := json.Marshal(sj)
	if err != nil {
		return "", "", err
	}

	// Also serialize sort criteria separately for backward compatibility
	scBytes, err := json.Marshal(sj.SortCriteria)
	if err != nil {
		return "", "", err
	}

	return string(gfBytes), string(scBytes), nil
}

// SettingsFromJSON deserializes Settings from JSON strings.
func SettingsFromJSON(groupFields, sortCriteria string) (Settings, error) {
	var s Settings

	// Try to parse as new format first (full settings JSON)
	if groupFields != "" {
		var sj settingsJSON
		if err := json.Unmarshal([]byte(groupFields), &sj); err == nil && len(sj.SortCriteria) > 0 {
			// New format - parse from full settings
			s.GroupFields = make([]GroupField, len(sj.GroupFields))
			for i, f := range sj.GroupFields {
				s.GroupFields[i] = GroupField(f)
			}
			s.GroupSortOrder = SortOrder(sj.GroupSortOrder)
			s.GroupDateField = DateFieldType(sj.GroupDateField)
			s.SortCriteria = make([]SortCriterion, len(sj.SortCriteria))
			for i, c := range sj.SortCriteria {
				s.SortCriteria[i] = SortCriterion{Field: SortField(c.Field), Order: SortOrder(c.Order)}
			}
			s.PresetName = sj.PresetName
			return s, nil
		}

		// Old format - parse group fields as simple int array
		var gf []int
		if err := json.Unmarshal([]byte(groupFields), &gf); err == nil {
			s.GroupFields = make([]GroupField, len(gf))
			for i, f := range gf {
				s.GroupFields[i] = GroupField(f)
			}
		}
	}

	// Parse sort criteria (for old format or as fallback)
	if sortCriteria != "" {
		var sc []sortCriterionJSON
		if err := json.Unmarshal([]byte(sortCriteria), &sc); err == nil {
			s.SortCriteria = make([]SortCriterion, len(sc))
			for i, c := range sc {
				s.SortCriteria[i] = SortCriterion{Field: SortField(c.Field), Order: SortOrder(c.Order)}
			}
		}
	}

	return s, nil
}
