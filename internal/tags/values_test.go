package tags

import (
	"slices"
	"testing"
)

func TestSplitValues_GivesBackTheValuesJoinValuesJoined(t *testing.T) {
	for _, values := range [][]string{
		{"a"},
		{"a", "b"},
		{"89ad4ac3-39f7-470e-963a-56509c546377", "f27ec8db-af05-4f36-916e-3d57f91ecf5e", "5b11f4ce-a62d-471e-81fc-a69a8278c7da"},
	} {
		if got := SplitValues(JoinValues(values)); !slices.Equal(got, values) {
			t.Errorf("SplitValues(JoinValues(%q)) = %q", values, got)
		}
	}
}

func TestSplitValues_AValueJoinedAnotherWayIsOneValue(t *testing.T) {
	for _, joined := range []string{"idA", "idA;idB", "idA/idB"} {
		if got := SplitValues(joined); !slices.Equal(got, []string{joined}) {
			t.Errorf("SplitValues(%q) = %q, want one value", joined, got)
		}
	}
}

func TestSplitValues_AnEmptyTagHoldsNoValue(t *testing.T) {
	if got := SplitValues(""); len(got) != 0 {
		t.Errorf("SplitValues(\"\") = %q, want no value", got)
	}
}
