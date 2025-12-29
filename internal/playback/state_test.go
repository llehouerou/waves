// internal/playback/state_test.go
package playback

import "testing"

func TestState_String(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{StateStopped, "Stopped"},
		{StatePlaying, "Playing"},
		{StatePaused, "Paused"},
		{State(99), "Unknown"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("%d.String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestState_IsActive(t *testing.T) {
	tests := []struct {
		state State
		want  bool
	}{
		{StateStopped, false},
		{StatePlaying, true},
		{StatePaused, true},
	}
	for _, tt := range tests {
		if got := tt.state.IsActive(); got != tt.want {
			t.Errorf("%v.IsActive() = %v, want %v", tt.state, got, tt.want)
		}
	}
}

func TestRepeatMode_String(t *testing.T) {
	tests := []struct {
		mode RepeatMode
		want string
	}{
		{RepeatOff, "Off"},
		{RepeatAll, "All"},
		{RepeatOne, "One"},
		{RepeatRadio, "Radio"},
		{RepeatMode(99), "Unknown"},
	}
	for _, tt := range tests {
		if got := tt.mode.String(); got != tt.want {
			t.Errorf("%d.String() = %q, want %q", tt.mode, got, tt.want)
		}
	}
}
