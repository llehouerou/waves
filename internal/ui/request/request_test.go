package request

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

type answerMsg struct{ id ID }

// ask asks p and returns the ID the answer carries.
func ask(p *Pending) ID {
	cmd := p.Ask(func(id ID) tea.Cmd {
		return func() tea.Msg { return answerMsg{id: id} }
	})
	answer, _ := cmd().(answerMsg) //nolint:errcheck // the command above answers nothing else
	return answer.id
}

func TestPending_TakesTheAnswerOfTheRequestWaitedForOnce(t *testing.T) {
	t.Parallel()
	var p Pending
	id := ask(&p)
	if !p.Take(id) {
		t.Fatal("the answer waited for is not taken")
	}
	if p.Take(id) {
		t.Error("the answer is taken twice")
	}
}

func TestPending_DropsTheAnswerOfAnEarlierRequest(t *testing.T) {
	t.Parallel()
	var p Pending
	earlier := ask(&p)
	later := ask(&p)
	if p.Take(earlier) {
		t.Error("the answer of the earlier request is taken")
	}
	if !p.Take(later) {
		t.Error("the answer of the later request is not taken")
	}
}

func TestPending_DropsTheAnswerOfAnotherPending(t *testing.T) {
	t.Parallel()
	var p, other Pending
	ask(&p)
	if p.Take(ask(&other)) {
		t.Error("the answer of another popup is taken")
	}
}

func TestPending_NeverTakesTheZeroID(t *testing.T) {
	t.Parallel()
	var p Pending
	if p.Take(ID{}) {
		t.Error("the zero ID is taken by a Pending waiting for nothing")
	}
	ask(&p)
	if p.Take(ID{}) {
		t.Error("the zero ID is taken by a Pending waiting for a request")
	}
}

func TestPending_ForgetDropsTheAnswer(t *testing.T) {
	t.Parallel()
	var p Pending
	id := ask(&p)
	p.Forget()
	if p.Take(id) {
		t.Error("the answer of a forgotten request is taken")
	}
}
