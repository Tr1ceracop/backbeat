package tracker

import (
	"context"
	"sync"
	"time"

	"backbeat/internal/monitor"
	"backbeat/internal/store"

	"github.com/rs/zerolog"
)

type State int

const (
	StateIdle State = iota
	StateActive
	StateSleeping
)

func (s State) String() string {
	switch s {
	case StateIdle:
		return "idle"
	case StateActive:
		return "active"
	case StateSleeping:
		return "sleeping"
	default:
		return "unknown"
	}
}

type Tracker struct {
	store           *store.Store
	state           State
	mu              sync.RWMutex
	activeSessionID int64
	meetingActive   bool
	minSession      time.Duration
	logger          zerolog.Logger
}

func New(store *store.Store, minSession time.Duration, logger zerolog.Logger) *Tracker {
	return &Tracker{
		store:      store,
		state:      StateIdle,
		minSession: minSession,
		logger:     logger,
	}
}

func (t *Tracker) Run(ctx context.Context, events <-chan monitor.Event) {
	for {
		select {
		case <-ctx.Done():
			t.closeActiveSession(time.Now())
			return
		case ev := <-events:
			t.handleEvent(ev)
		}
	}
}

func (t *Tracker) handleEvent(ev monitor.Event) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.logger.Debug().Str("event", ev.Type.String()).Str("state", t.state.String()).Msg("handling event")

	switch ev.Type {
	case monitor.EventActive:
		t.onActive(ev.Timestamp)
	case monitor.EventIdle:
		t.onIdle(ev.Timestamp)
	case monitor.EventSleep:
		t.onSleep(ev.Timestamp)
	case monitor.EventWake:
		t.onWake(ev.Timestamp)
	case monitor.EventMeetingStart:
		t.onMeetingStart(ev.Timestamp)
	case monitor.EventMeetingEnd:
		t.onMeetingEnd(ev.Timestamp)
	}
}

func (t *Tracker) onActive(ts time.Time) {
	if t.state != StateIdle {
		return
	}
	t.state = StateActive
	id, err := t.store.StartSession(ts, t.meetingActive)
	if err != nil {
		t.logger.Error().Err(err).Msg("failed to start session")
		return
	}
	t.activeSessionID = id
	t.logger.Info().Int64("session_id", id).Time("start", ts).Msg("session started")
}

func (t *Tracker) onIdle(ts time.Time) {
	if t.state != StateActive {
		return
	}
	t.state = StateIdle
	t.endCurrentSession(ts)
}

func (t *Tracker) onSleep(ts time.Time) {
	if t.state == StateActive {
		t.endCurrentSession(ts)
	}
	t.state = StateSleeping
}

func (t *Tracker) onWake(ts time.Time) {
	if t.state != StateSleeping {
		return
	}
	t.state = StateIdle
	t.logger.Info().Msg("resumed from sleep, now idle")
}

func (t *Tracker) onMeetingStart(ts time.Time) {
	t.meetingActive = true
	if t.state == StateActive && t.activeSessionID > 0 {
		// Close current non-meeting session, start new meeting session
		t.endCurrentSession(ts)
		id, err := t.store.StartSession(ts, true)
		if err != nil {
			t.logger.Error().Err(err).Msg("failed to start meeting session")
			return
		}
		t.activeSessionID = id
		t.logger.Info().Int64("session_id", id).Msg("meeting session started")
	}
}

func (t *Tracker) onMeetingEnd(ts time.Time) {
	t.meetingActive = false
	if t.state == StateActive && t.activeSessionID > 0 {
		// Close meeting session, start new non-meeting session
		t.endCurrentSession(ts)
		id, err := t.store.StartSession(ts, false)
		if err != nil {
			t.logger.Error().Err(err).Msg("failed to start non-meeting session")
			return
		}
		t.activeSessionID = id
		t.logger.Info().Int64("session_id", id).Msg("non-meeting session started")
	}
}

func (t *Tracker) endCurrentSession(ts time.Time) {
	if t.activeSessionID == 0 {
		return
	}
	if err := t.store.EndSession(t.activeSessionID, ts); err != nil {
		t.logger.Error().Err(err).Int64("session_id", t.activeSessionID).Msg("failed to end session")
	} else {
		t.logger.Info().Int64("session_id", t.activeSessionID).Time("end", ts).Msg("session ended")
	}
	t.activeSessionID = 0
}

func (t *Tracker) closeActiveSession(ts time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.endCurrentSession(ts)
}

// State returns the current tracker state.
func (t *Tracker) CurrentState() State {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state
}

// ActiveSessionID returns the current active session ID (0 if none).
func (t *Tracker) ActiveSessionID() int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.activeSessionID
}

// IsMeeting returns whether a meeting is currently detected.
func (t *Tracker) IsMeeting() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.meetingActive
}
