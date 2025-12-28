package playback

import "time"

const eventBufferSize = 16

// Subscription provides event channels for a subscriber.
type Subscription struct {
	StateChanged    <-chan StateChange
	TrackChanged    <-chan TrackChange
	PositionChanged <-chan PositionChange
	QueueChanged    <-chan QueueChange
	ModeChanged     <-chan ModeChange
	Error           <-chan ErrorEvent
	Done            <-chan struct{}

	// Internal write channels
	stateCh    chan StateChange
	trackCh    chan TrackChange
	positionCh chan PositionChange
	queueCh    chan QueueChange
	modeCh     chan ModeChange
	errorCh    chan ErrorEvent
	doneCh     chan struct{}
}

// newSubscription creates a new subscription with buffered channels.
func newSubscription() *Subscription {
	s := &Subscription{
		stateCh:    make(chan StateChange, eventBufferSize),
		trackCh:    make(chan TrackChange, eventBufferSize),
		positionCh: make(chan PositionChange, eventBufferSize),
		queueCh:    make(chan QueueChange, eventBufferSize),
		modeCh:     make(chan ModeChange, eventBufferSize),
		errorCh:    make(chan ErrorEvent, eventBufferSize),
		doneCh:     make(chan struct{}),
	}
	s.StateChanged = s.stateCh
	s.TrackChanged = s.trackCh
	s.PositionChanged = s.positionCh
	s.QueueChanged = s.queueCh
	s.ModeChanged = s.modeCh
	s.Error = s.errorCh
	s.Done = s.doneCh
	return s
}

// close signals subscribers to stop by closing doneCh.
func (s *Subscription) close() {
	close(s.doneCh)
}

// sendState sends a state change event (non-blocking).
func (s *Subscription) sendState(e StateChange) {
	select {
	case s.stateCh <- e:
	default:
		// Drop if buffer full
	}
}

// sendTrack sends a track change event (non-blocking).
func (s *Subscription) sendTrack(e TrackChange) {
	select {
	case s.trackCh <- e:
	default:
	}
}

// sendPosition sends a position change event (non-blocking).
func (s *Subscription) sendPosition(pos time.Duration) {
	select {
	case s.positionCh <- PositionChange{Position: pos}:
	default:
	}
}

// sendQueue sends a queue change event (non-blocking).
func (s *Subscription) sendQueue(e QueueChange) {
	select {
	case s.queueCh <- e:
	default:
	}
}

// sendMode sends a mode change event (non-blocking).
func (s *Subscription) sendMode(e ModeChange) {
	select {
	case s.modeCh <- e:
	default:
	}
}

// sendError sends an error event (non-blocking).
func (s *Subscription) sendError(e ErrorEvent) {
	select {
	case s.errorCh <- e:
	default:
	}
}
