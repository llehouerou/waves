package handler

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNotHandled(t *testing.T) {
	if NotHandled.Handled {
		t.Error("NotHandled.Handled should be false")
	}
	if NotHandled.Cmd != nil {
		t.Error("NotHandled.Cmd should be nil")
	}
}

func TestHandledNoCmd(t *testing.T) {
	if !HandledNoCmd.Handled {
		t.Error("HandledNoCmd.Handled should be true")
	}
	if HandledNoCmd.Cmd != nil {
		t.Error("HandledNoCmd.Cmd should be nil")
	}
}

func TestHandled(t *testing.T) {
	// Test with nil command
	t.Run("nil command", func(t *testing.T) {
		result := Handled(nil)
		if !result.Handled {
			t.Error("Handled(nil).Handled should be true")
		}
		if result.Cmd != nil {
			t.Error("Handled(nil).Cmd should be nil")
		}
	})

	// Test with actual command
	t.Run("with command", func(t *testing.T) {
		cmd := func() tea.Msg { return "test" }
		result := Handled(cmd)
		if !result.Handled {
			t.Error("Handled(cmd).Handled should be true")
		}
		if result.Cmd == nil {
			t.Error("Handled(cmd).Cmd should not be nil")
		}
	})
}

func TestChain_NoHandlers(t *testing.T) {
	handled, cmd := Chain()
	if handled {
		t.Error("Chain() with no handlers should return handled=false")
	}
	if cmd != nil {
		t.Error("Chain() with no handlers should return cmd=nil")
	}
}

func TestChain_SingleHandler(t *testing.T) {
	t.Run("handler returns NotHandled", func(t *testing.T) {
		h := func() Result { return NotHandled }
		handled, cmd := Chain(h)
		if handled {
			t.Error("Chain should return handled=false when handler returns NotHandled")
		}
		if cmd != nil {
			t.Error("Chain should return cmd=nil when handler returns NotHandled")
		}
	})

	t.Run("handler returns HandledNoCmd", func(t *testing.T) {
		h := func() Result { return HandledNoCmd }
		handled, cmd := Chain(h)
		if !handled {
			t.Error("Chain should return handled=true when handler returns HandledNoCmd")
		}
		if cmd != nil {
			t.Error("Chain should return cmd=nil when handler returns HandledNoCmd")
		}
	})

	t.Run("handler returns Handled with cmd", func(t *testing.T) {
		testCmd := func() tea.Msg { return "test" }
		h := func() Result { return Handled(testCmd) }
		handled, cmd := Chain(h)
		if !handled {
			t.Error("Chain should return handled=true when handler returns Handled")
		}
		if cmd == nil {
			t.Error("Chain should return the command when handler returns Handled with cmd")
		}
	})
}

func TestChain_MultipleHandlers(t *testing.T) {
	t.Run("first handler handles", func(t *testing.T) {
		callCount := 0
		h1 := func() Result {
			callCount++
			return HandledNoCmd
		}
		h2 := func() Result {
			callCount++
			return HandledNoCmd
		}

		handled, _ := Chain(h1, h2)
		if !handled {
			t.Error("Chain should return handled=true")
		}
		if callCount != 1 {
			t.Errorf("Only first handler should be called, got %d calls", callCount)
		}
	})

	t.Run("second handler handles", func(t *testing.T) {
		callCount := 0
		h1 := func() Result {
			callCount++
			return NotHandled
		}
		h2 := func() Result {
			callCount++
			return HandledNoCmd
		}

		handled, _ := Chain(h1, h2)
		if !handled {
			t.Error("Chain should return handled=true")
		}
		if callCount != 2 {
			t.Errorf("Both handlers should be called, got %d calls", callCount)
		}
	})

	t.Run("no handler handles", func(t *testing.T) {
		callCount := 0
		h1 := func() Result {
			callCount++
			return NotHandled
		}
		h2 := func() Result {
			callCount++
			return NotHandled
		}
		h3 := func() Result {
			callCount++
			return NotHandled
		}

		handled, cmd := Chain(h1, h2, h3)
		if handled {
			t.Error("Chain should return handled=false when no handler handles")
		}
		if cmd != nil {
			t.Error("Chain should return cmd=nil when no handler handles")
		}
		if callCount != 3 {
			t.Errorf("All handlers should be called, got %d calls", callCount)
		}
	})

	t.Run("middle handler handles with command", func(t *testing.T) {
		testCmd := func() tea.Msg { return "middle" }
		callOrder := []int{}

		h1 := func() Result {
			callOrder = append(callOrder, 1)
			return NotHandled
		}
		h2 := func() Result {
			callOrder = append(callOrder, 2)
			return Handled(testCmd)
		}
		h3 := func() Result {
			callOrder = append(callOrder, 3)
			return HandledNoCmd
		}

		handled, cmd := Chain(h1, h2, h3)
		if !handled {
			t.Error("Chain should return handled=true")
		}
		if cmd == nil {
			t.Error("Chain should return the command from h2")
		}
		if len(callOrder) != 2 || callOrder[0] != 1 || callOrder[1] != 2 {
			t.Errorf("Expected call order [1, 2], got %v", callOrder)
		}
	})
}

func TestResult_ZeroValue(t *testing.T) {
	var r Result
	if r.Handled {
		t.Error("Zero value Result.Handled should be false")
	}
	if r.Cmd != nil {
		t.Error("Zero value Result.Cmd should be nil")
	}
}
