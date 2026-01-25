package albumpreset

import (
	"testing"
)

func TestToJSON_FromJSON_RoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		settings Settings
	}{
		{
			name:     "default settings",
			settings: DefaultSettings(),
		},
		{
			name: "multiple group fields",
			settings: Settings{
				GroupFields:    []GroupField{GroupFieldArtist, GroupFieldYear},
				GroupSortOrder: SortAsc,
				GroupDateField: DateFieldOriginal,
				SortCriteria: []SortCriterion{
					{Field: SortFieldArtist, Order: SortAsc},
					{Field: SortFieldAlbum, Order: SortAsc},
				},
			},
		},
		{
			name: "no grouping",
			settings: Settings{
				GroupFields:    nil,
				GroupSortOrder: SortDesc,
				GroupDateField: DateFieldBest,
				SortCriteria: []SortCriterion{
					{Field: SortFieldAddedAt, Order: SortDesc},
				},
			},
		},
		{
			name:     "empty settings",
			settings: Settings{},
		},
		{
			name: "all sort fields",
			settings: Settings{
				GroupFields:    []GroupField{GroupFieldGenre},
				GroupSortOrder: SortDesc,
				GroupDateField: DateFieldRelease,
				SortCriteria: []SortCriterion{
					{Field: SortFieldOriginalDate, Order: SortDesc},
					{Field: SortFieldReleaseDate, Order: SortAsc},
					{Field: SortFieldAddedAt, Order: SortDesc},
					{Field: SortFieldTrackCount, Order: SortAsc},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Serialize
			groupFields, sortCriteria, err := tt.settings.ToJSON()
			if err != nil {
				t.Fatalf("ToJSON() error = %v", err)
			}

			// Deserialize
			got, err := FromJSON(groupFields, sortCriteria)
			if err != nil {
				t.Fatalf("FromJSON() error = %v", err)
			}

			// Compare
			if !settingsEqual(got, tt.settings) {
				t.Errorf("round-trip mismatch:\ngot:  %+v\nwant: %+v", got, tt.settings)
			}
		})
	}
}

func TestFromJSON_OldFormat(t *testing.T) {
	// Old format: groupFields is a simple int array, sortCriteria is separate
	groupFields := `[0,3]` // GroupFieldArtist, GroupFieldYear
	sortCriteria := `[{"field":0,"order":1},{"field":3,"order":0}]`

	got, err := FromJSON(groupFields, sortCriteria)
	if err != nil {
		t.Fatalf("FromJSON() error = %v", err)
	}

	// Check group fields
	if len(got.GroupFields) != 2 {
		t.Errorf("GroupFields length = %d, want 2", len(got.GroupFields))
	}
	if got.GroupFields[0] != GroupFieldArtist {
		t.Errorf("GroupFields[0] = %v, want GroupFieldArtist", got.GroupFields[0])
	}
	if got.GroupFields[1] != GroupFieldYear {
		t.Errorf("GroupFields[1] = %v, want GroupFieldYear", got.GroupFields[1])
	}

	// Check sort criteria
	if len(got.SortCriteria) != 2 {
		t.Errorf("SortCriteria length = %d, want 2", len(got.SortCriteria))
	}
	if got.SortCriteria[0].Field != SortFieldOriginalDate || got.SortCriteria[0].Order != SortAsc {
		t.Errorf("SortCriteria[0] = %+v, want {OriginalDate, Asc}", got.SortCriteria[0])
	}
}

func TestFromJSON_EmptyInputs(t *testing.T) {
	got, err := FromJSON("", "")
	if err != nil {
		t.Fatalf("FromJSON() error = %v", err)
	}

	if len(got.GroupFields) != 0 {
		t.Errorf("GroupFields = %v, want empty", got.GroupFields)
	}
	if len(got.SortCriteria) != 0 {
		t.Errorf("SortCriteria = %v, want empty", got.SortCriteria)
	}
}

func TestFromJSON_InvalidJSON(t *testing.T) {
	// Invalid JSON should not error, just return empty settings
	got, err := FromJSON("not json", "also not json")
	if err != nil {
		t.Fatalf("FromJSON() error = %v", err)
	}

	if len(got.GroupFields) != 0 {
		t.Errorf("GroupFields = %v, want empty", got.GroupFields)
	}
	if len(got.SortCriteria) != 0 {
		t.Errorf("SortCriteria = %v, want empty", got.SortCriteria)
	}
}

func TestDefaultSettings(t *testing.T) {
	s := DefaultSettings()

	// Check group fields
	if len(s.GroupFields) != 1 {
		t.Errorf("GroupFields length = %d, want 1", len(s.GroupFields))
	}
	if s.GroupFields[0] != GroupFieldMonth {
		t.Errorf("GroupFields[0] = %v, want GroupFieldMonth", s.GroupFields[0])
	}

	// Check sort order
	if s.GroupSortOrder != SortDesc {
		t.Errorf("GroupSortOrder = %v, want SortDesc", s.GroupSortOrder)
	}

	// Check date field
	if s.GroupDateField != DateFieldAdded {
		t.Errorf("GroupDateField = %v, want DateFieldAdded", s.GroupDateField)
	}

	// Check sort criteria
	if len(s.SortCriteria) != 1 {
		t.Errorf("SortCriteria length = %d, want 1", len(s.SortCriteria))
	}
	if s.SortCriteria[0].Field != SortFieldAddedAt {
		t.Errorf("SortCriteria[0].Field = %v, want SortFieldAddedAt", s.SortCriteria[0].Field)
	}
	if s.SortCriteria[0].Order != SortDesc {
		t.Errorf("SortCriteria[0].Order = %v, want SortDesc", s.SortCriteria[0].Order)
	}
}

// settingsEqual compares two Settings for equality.
func settingsEqual(a, b Settings) bool {
	if len(a.GroupFields) != len(b.GroupFields) {
		return false
	}
	for i := range a.GroupFields {
		if a.GroupFields[i] != b.GroupFields[i] {
			return false
		}
	}

	if a.GroupSortOrder != b.GroupSortOrder {
		return false
	}
	if a.GroupDateField != b.GroupDateField {
		return false
	}

	if len(a.SortCriteria) != len(b.SortCriteria) {
		return false
	}
	for i := range a.SortCriteria {
		if a.SortCriteria[i] != b.SortCriteria[i] {
			return false
		}
	}

	return true
}
