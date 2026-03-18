package store

import (
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestStartAndEndSession(t *testing.T) {
	s := openTestStore(t)

	start := time.Now().Truncate(time.Second)
	id, err := s.StartSession(start, false)
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero session ID")
	}

	// Should be active
	sess, err := s.GetActiveSession()
	if err != nil {
		t.Fatalf("get active session: %v", err)
	}
	if sess == nil {
		t.Fatal("expected active session")
	}
	if sess.ID != id {
		t.Fatalf("expected session ID %d, got %d", id, sess.ID)
	}
	if sess.EndTime != nil {
		t.Fatal("expected nil end time")
	}

	// End it
	end := start.Add(30 * time.Minute)
	if err := s.EndSession(id, end); err != nil {
		t.Fatalf("end session: %v", err)
	}

	// Should not be active anymore
	sess, err = s.GetActiveSession()
	if err != nil {
		t.Fatalf("get active session: %v", err)
	}
	if sess != nil {
		t.Fatal("expected no active session")
	}
}

func TestMeetingSession(t *testing.T) {
	s := openTestStore(t)

	id, err := s.StartSession(time.Now(), true)
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	sess, err := s.GetActiveSession()
	if err != nil {
		t.Fatalf("get active: %v", err)
	}
	if !sess.IsMeeting {
		t.Fatal("expected meeting session")
	}
	_ = id
}

func TestSetIssueKeyForUnassigned(t *testing.T) {
	s := openTestStore(t)
	now := time.Now().Truncate(time.Second)

	// Create sessions: one unassigned, one assigned
	id1, _ := s.StartSession(now.Add(-2*time.Hour), false)
	s.EndSession(id1, now.Add(-1*time.Hour))

	id2, _ := s.StartSession(now.Add(-1*time.Hour), false)
	s.EndSession(id2, now)
	s.SetSessionIssueKey(id2, "OTHER-1")

	id3, _ := s.StartSession(now.Add(-30*time.Minute), false)
	s.EndSession(id3, now.Add(-15*time.Minute))

	affected, err := s.SetIssueKeyForUnassigned("PROJ-123", now)
	if err != nil {
		t.Fatalf("set issue key: %v", err)
	}
	if affected != 2 {
		t.Fatalf("expected 2 affected, got %d", affected)
	}
}

func TestGetUnsyncedSessions(t *testing.T) {
	s := openTestStore(t)
	now := time.Now().Truncate(time.Second)

	// Completed + assigned = should appear
	id1, _ := s.StartSession(now.Add(-2*time.Hour), false)
	s.EndSession(id1, now.Add(-1*time.Hour))
	s.SetSessionIssueKey(id1, "PROJ-1")

	// Completed + unassigned = should NOT appear
	id2, _ := s.StartSession(now.Add(-1*time.Hour), false)
	s.EndSession(id2, now)

	// Active = should NOT appear
	s.StartSession(now, false)

	sessions, err := s.GetUnsyncedSessions()
	if err != nil {
		t.Fatalf("get unsynced: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 unsynced session, got %d", len(sessions))
	}
	if sessions[0].ID != id1 {
		t.Fatalf("expected session %d, got %d", id1, sessions[0].ID)
	}
}

func TestMarkSynced(t *testing.T) {
	s := openTestStore(t)
	now := time.Now().Truncate(time.Second)

	id, _ := s.StartSession(now, false)
	s.EndSession(id, now.Add(time.Hour))
	s.SetSessionIssueKey(id, "PROJ-1")

	if err := s.MarkSynced([]int64{id}, 42); err != nil {
		t.Fatalf("mark synced: %v", err)
	}

	sessions, _ := s.GetUnsyncedSessions()
	if len(sessions) != 0 {
		t.Fatal("expected no unsynced sessions")
	}
}

func TestDeleteShortSessions(t *testing.T) {
	s := openTestStore(t)
	now := time.Now().Truncate(time.Second)

	// Short session (30s)
	id1, _ := s.StartSession(now, false)
	s.EndSession(id1, now.Add(30*time.Second))

	// Long session (10m)
	id2, _ := s.StartSession(now.Add(time.Minute), false)
	s.EndSession(id2, now.Add(11*time.Minute))

	deleted, err := s.DeleteShortSessions(time.Minute)
	if err != nil {
		t.Fatalf("delete short: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 deleted, got %d", deleted)
	}

	sessions, _ := s.GetSessionsForDate(now)
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session remaining, got %d", len(sessions))
	}
}

func TestIssueCacheRoundtrip(t *testing.T) {
	s := openTestStore(t)

	_, found, err := s.GetCachedIssueID("PROJ-1")
	if err != nil {
		t.Fatalf("get cache: %v", err)
	}
	if found {
		t.Fatal("expected not found")
	}

	if err := s.CacheIssueID("PROJ-1", 12345); err != nil {
		t.Fatalf("cache: %v", err)
	}

	id, found, err := s.GetCachedIssueID("PROJ-1")
	if err != nil {
		t.Fatalf("get cache: %v", err)
	}
	if !found {
		t.Fatal("expected found")
	}
	if id != 12345 {
		t.Fatalf("expected 12345, got %d", id)
	}
}
