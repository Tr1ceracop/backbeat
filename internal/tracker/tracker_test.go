package tracker

import (
	"context"
	"os"
	"testing"
	"time"

	"backbeat/internal/monitor"
	"backbeat/internal/store"

	"github.com/rs/zerolog"
)

func testTracker(t *testing.T) (*Tracker, *store.Store) {
	t.Helper()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)
	tr := New(s, time.Minute, logger)
	return tr, s
}

func TestIdleToActive(t *testing.T) {
	tr, s := testTracker(t)

	now := time.Now()
	events := make(chan monitor.Event, 10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go tr.Run(ctx, events)

	events <- monitor.Event{Type: monitor.EventActive, Timestamp: now}
	time.Sleep(50 * time.Millisecond)

	if tr.CurrentState() != StateActive {
		t.Fatalf("expected active, got %s", tr.CurrentState())
	}

	sess, _ := s.GetActiveSession()
	if sess == nil {
		t.Fatal("expected active session")
	}
}

func TestActiveToIdle(t *testing.T) {
	tr, s := testTracker(t)

	now := time.Now()
	events := make(chan monitor.Event, 10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go tr.Run(ctx, events)

	events <- monitor.Event{Type: monitor.EventActive, Timestamp: now}
	time.Sleep(50 * time.Millisecond)

	events <- monitor.Event{Type: monitor.EventIdle, Timestamp: now.Add(30 * time.Minute)}
	time.Sleep(50 * time.Millisecond)

	if tr.CurrentState() != StateIdle {
		t.Fatalf("expected idle, got %s", tr.CurrentState())
	}

	sess, _ := s.GetActiveSession()
	if sess != nil {
		t.Fatal("expected no active session")
	}
}

func TestSleepWakeCycle(t *testing.T) {
	tr, _ := testTracker(t)

	now := time.Now()
	events := make(chan monitor.Event, 10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go tr.Run(ctx, events)

	events <- monitor.Event{Type: monitor.EventActive, Timestamp: now}
	time.Sleep(50 * time.Millisecond)

	events <- monitor.Event{Type: monitor.EventSleep, Timestamp: now.Add(time.Hour)}
	time.Sleep(50 * time.Millisecond)

	if tr.CurrentState() != StateSleeping {
		t.Fatalf("expected sleeping, got %s", tr.CurrentState())
	}

	events <- monitor.Event{Type: monitor.EventWake, Timestamp: now.Add(2 * time.Hour)}
	time.Sleep(50 * time.Millisecond)

	if tr.CurrentState() != StateIdle {
		t.Fatalf("expected idle after wake, got %s", tr.CurrentState())
	}
}

func TestMeetingSplitsSession(t *testing.T) {
	tr, s := testTracker(t)

	now := time.Now()
	events := make(chan monitor.Event, 10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go tr.Run(ctx, events)

	// Start active
	events <- monitor.Event{Type: monitor.EventActive, Timestamp: now}
	time.Sleep(50 * time.Millisecond)

	// Meeting starts -> should split session
	events <- monitor.Event{Type: monitor.EventMeetingStart, Timestamp: now.Add(15 * time.Minute)}
	time.Sleep(50 * time.Millisecond)

	if !tr.IsMeeting() {
		t.Fatal("expected meeting active")
	}

	// Meeting ends -> split again
	events <- monitor.Event{Type: monitor.EventMeetingEnd, Timestamp: now.Add(45 * time.Minute)}
	time.Sleep(50 * time.Millisecond)

	if tr.IsMeeting() {
		t.Fatal("expected meeting inactive")
	}

	// End
	events <- monitor.Event{Type: monitor.EventIdle, Timestamp: now.Add(time.Hour)}
	time.Sleep(50 * time.Millisecond)

	sessions, _ := s.GetSessionsForDate(now)
	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions (pre-meeting, meeting, post-meeting), got %d", len(sessions))
	}
	if !sessions[1].IsMeeting {
		t.Fatal("expected middle session to be a meeting")
	}
}

func TestIdleSleepNoSession(t *testing.T) {
	tr, _ := testTracker(t)

	now := time.Now()
	events := make(chan monitor.Event, 10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go tr.Run(ctx, events)

	// Sleep while idle - should just transition state
	events <- monitor.Event{Type: monitor.EventSleep, Timestamp: now}
	time.Sleep(50 * time.Millisecond)

	if tr.CurrentState() != StateSleeping {
		t.Fatalf("expected sleeping, got %s", tr.CurrentState())
	}

	events <- monitor.Event{Type: monitor.EventWake, Timestamp: now.Add(time.Hour)}
	time.Sleep(50 * time.Millisecond)

	if tr.CurrentState() != StateIdle {
		t.Fatalf("expected idle, got %s", tr.CurrentState())
	}
}

func TestContextCancelClosesSession(t *testing.T) {
	tr, s := testTracker(t)

	now := time.Now()
	events := make(chan monitor.Event, 10)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		tr.Run(ctx, events)
		close(done)
	}()

	events <- monitor.Event{Type: monitor.EventActive, Timestamp: now}
	time.Sleep(50 * time.Millisecond)

	cancel()
	<-done

	sess, _ := s.GetActiveSession()
	if sess != nil {
		t.Fatal("expected session to be closed on context cancel")
	}
}
