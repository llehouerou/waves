package radio

import (
	"slices"
	"testing"
)

// Issue #315: a fill reads radio's memory and leaves it untouched. The core
// records its seeds when it queues its tracks.

func TestRadio_FillRecordsNoSeed(t *testing.T) {
	r := newTestRadio(t, fakeLastfm(oneChart()))

	for i := 1; i <= 2; i++ {
		res := r.Fill("Low", nil)

		if res.Err != nil || len(res.Tracks) == 0 {
			t.Fatalf("fill %d = %+v, want tracks and no error", i, res)
		}
		if want := []string{"Codeine"}; !slices.Equal(res.Seeds, want) {
			t.Errorf("fill %d seeds = %v, want %v, the artists of its tracks", i, res.Seeds, want)
		}
	}
}

func TestRadio_FillSkipsTheRecordedSeeds(t *testing.T) {
	r := newTestRadio(t, fakeLastfm(oneChart()))

	r.AddRecentSeeds("Codeine")
	res := r.Fill("Low", nil)

	if res.Err != nil || len(res.Tracks) != 0 {
		t.Errorf("fill = %+v, want no track: Codeine is a recent seed", res)
	}
}
