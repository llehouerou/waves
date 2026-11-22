// Package request is how a popup tells the answers to its own requests from
// the others. A popup sends each request through a Pending: the request gets
// an ID no other request of the process has, its answer carries the ID, and
// the popup takes the answer only if it still waits for that request, once.
// Any other answer, the one of a popup closed since or of a request made
// before, is dropped.
package request

import (
	"sync/atomic"

	tea "github.com/charmbracelet/bubbletea"
)

// ID names one request. Only Pending.Ask hands one out: the zero ID is no
// request, and is never taken.
type ID struct {
	n uint64
}

// last is the last ID handed out, shared by every popup.
var last atomic.Uint64

// Pending is the request a popup waits for the answer of, for one kind of
// answer: none at first, and none again once its answer is taken.
type Pending struct {
	id ID
}

// Ask makes a request: it hands a fresh ID to build, which returns the
// command whose answer carries it, and makes it the request waited for, in
// place of any other.
func (p *Pending) Ask(build func(ID) tea.Cmd) tea.Cmd {
	p.id = ID{n: last.Add(1)}
	return build(p.id)
}

// Take reports whether id is the request waited for. If it is, its answer
// is taken: the popup waits for nothing until it asks again.
func (p *Pending) Take(id ID) bool {
	if id == (ID{}) || id != p.id {
		return false
	}
	p.id = ID{}
	return true
}

// Waiting reports whether a request is waited for: asked, and its answer
// neither taken nor forgotten.
func (p *Pending) Waiting() bool {
	return p.id != (ID{})
}

// Forget stops waiting: the answer of the request waited for is dropped.
func (p *Pending) Forget() {
	p.id = ID{}
}
