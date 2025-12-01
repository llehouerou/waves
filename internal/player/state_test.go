package player

import "testing"

func TestState_String(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{Stopped, "Stopped"},
		{Playing, "Playing"},
		{Paused, "Paused"},
		{State(99), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.state.String(); got != tt.want {
				t.Errorf("State.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestState_IsActive(t *testing.T) {
	tests := []struct {
		state State
		want  bool
	}{
		{Stopped, false},
		{Playing, true},
		{Paused, true},
	}

	for _, tt := range tests {
		t.Run(tt.state.String(), func(t *testing.T) {
			if got := tt.state.IsActive(); got != tt.want {
				t.Errorf("State.IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestState_CanPause(t *testing.T) {
	tests := []struct {
		state State
		want  bool
	}{
		{Stopped, false},
		{Playing, true},
		{Paused, false},
	}

	for _, tt := range tests {
		t.Run(tt.state.String(), func(t *testing.T) {
			if got := tt.state.CanPause(); got != tt.want {
				t.Errorf("State.CanPause() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestState_CanResume(t *testing.T) {
	tests := []struct {
		state State
		want  bool
	}{
		{Stopped, false},
		{Playing, false},
		{Paused, true},
	}

	for _, tt := range tests {
		t.Run(tt.state.String(), func(t *testing.T) {
			if got := tt.state.CanResume(); got != tt.want {
				t.Errorf("State.CanResume() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestMock_StateTransitions validates the state machine using the Mock player.
func TestMock_StateTransitions(t *testing.T) {
	t.Run("Stopped to Playing via Play", func(t *testing.T) {
		m := NewMock()
		if m.State() != Stopped {
			t.Fatalf("initial state = %v, want Stopped", m.State())
		}

		_ = m.Play("/test.mp3")

		if m.State() != Playing {
			t.Errorf("state after Play = %v, want Playing", m.State())
		}
	})

	t.Run("Playing to Paused via Pause", func(t *testing.T) {
		m := NewMock()
		_ = m.Play("/test.mp3")

		m.Pause()

		if m.State() != Paused {
			t.Errorf("state after Pause = %v, want Paused", m.State())
		}
	})

	t.Run("Paused to Playing via Resume", func(t *testing.T) {
		m := NewMock()
		_ = m.Play("/test.mp3")
		m.Pause()

		m.Resume()

		if m.State() != Playing {
			t.Errorf("state after Resume = %v, want Playing", m.State())
		}
	})

	t.Run("Playing to Stopped via Stop", func(t *testing.T) {
		m := NewMock()
		_ = m.Play("/test.mp3")

		m.Stop()

		if m.State() != Stopped {
			t.Errorf("state after Stop = %v, want Stopped", m.State())
		}
	})

	t.Run("Paused to Stopped via Stop", func(t *testing.T) {
		m := NewMock()
		_ = m.Play("/test.mp3")
		m.Pause()

		m.Stop()

		if m.State() != Stopped {
			t.Errorf("state after Stop = %v, want Stopped", m.State())
		}
	})
}

func TestMock_Toggle(t *testing.T) {
	t.Run("Playing to Paused", func(t *testing.T) {
		m := NewMock()
		_ = m.Play("/test.mp3")

		m.Toggle()

		if m.State() != Paused {
			t.Errorf("state after Toggle = %v, want Paused", m.State())
		}
	})

	t.Run("Paused to Playing", func(t *testing.T) {
		m := NewMock()
		_ = m.Play("/test.mp3")
		m.Pause()

		m.Toggle()

		if m.State() != Playing {
			t.Errorf("state after Toggle = %v, want Playing", m.State())
		}
	})

	t.Run("Stopped remains Stopped", func(t *testing.T) {
		m := NewMock()

		m.Toggle()

		if m.State() != Stopped {
			t.Errorf("state after Toggle = %v, want Stopped", m.State())
		}
	})
}

func TestMock_NoOpTransitions(t *testing.T) {
	t.Run("Stop when Stopped is no-op", func(t *testing.T) {
		m := NewMock()

		m.Stop() // Should not panic

		if m.State() != Stopped {
			t.Errorf("state = %v, want Stopped", m.State())
		}
	})

	t.Run("Pause when Stopped is no-op", func(t *testing.T) {
		m := NewMock()

		m.Pause() // Should not panic

		if m.State() != Stopped {
			t.Errorf("state = %v, want Stopped", m.State())
		}
	})

	t.Run("Resume when Stopped is no-op", func(t *testing.T) {
		m := NewMock()

		m.Resume() // Should not panic

		if m.State() != Stopped {
			t.Errorf("state = %v, want Stopped", m.State())
		}
	})

	t.Run("Pause when Paused is no-op", func(t *testing.T) {
		m := NewMock()
		_ = m.Play("/test.mp3")
		m.Pause()

		m.Pause() // Should not panic

		if m.State() != Paused {
			t.Errorf("state = %v, want Paused", m.State())
		}
	})

	t.Run("Resume when Playing is no-op", func(t *testing.T) {
		m := NewMock()
		_ = m.Play("/test.mp3")

		m.Resume() // Should not panic

		if m.State() != Playing {
			t.Errorf("state = %v, want Playing", m.State())
		}
	})
}
