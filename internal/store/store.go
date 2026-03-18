package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type Session struct {
	ID        int64
	StartTime time.Time
	EndTime   *time.Time
	IssueKey  *string
	IsMeeting bool
	Synced    bool
	WorklogID *int64
}

func Open(dsn string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return s, nil
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			id          INTEGER PRIMARY KEY,
			start_time  TEXT NOT NULL,
			end_time    TEXT,
			issue_key   TEXT,
			is_meeting  INTEGER DEFAULT 0,
			synced      INTEGER DEFAULT 0,
			worklog_id  INTEGER
		);

		CREATE TABLE IF NOT EXISTS issue_cache (
			issue_key   TEXT PRIMARY KEY,
			issue_id    INTEGER NOT NULL,
			cached_at   TEXT NOT NULL
		);
	`)
	return err
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) StartSession(startTime time.Time, isMeeting bool) (int64, error) {
	res, err := s.db.Exec(
		"INSERT INTO sessions (start_time, is_meeting) VALUES (?, ?)",
		startTime.Format(time.RFC3339), boolToInt(isMeeting),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) EndSession(id int64, endTime time.Time) error {
	_, err := s.db.Exec(
		"UPDATE sessions SET end_time = ? WHERE id = ?",
		endTime.Format(time.RFC3339), id,
	)
	return err
}

func (s *Store) GetActiveSession() (*Session, error) {
	return s.scanSession(s.db.QueryRow(
		"SELECT id, start_time, end_time, issue_key, is_meeting, synced, worklog_id FROM sessions WHERE end_time IS NULL ORDER BY id DESC LIMIT 1",
	))
}

func (s *Store) SetSessionIssueKey(id int64, issueKey string) error {
	_, err := s.db.Exec("UPDATE sessions SET issue_key = ? WHERE id = ?", issueKey, id)
	return err
}

func (s *Store) SetIssueKeyForUnassigned(issueKey string, date time.Time) (int64, error) {
	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	dayEnd := dayStart.Add(24 * time.Hour)

	res, err := s.db.Exec(
		"UPDATE sessions SET issue_key = ? WHERE issue_key IS NULL AND start_time >= ? AND start_time < ?",
		issueKey, dayStart.Format(time.RFC3339), dayEnd.Format(time.RFC3339),
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) GetUnsyncedSessions() ([]Session, error) {
	rows, err := s.db.Query(
		"SELECT id, start_time, end_time, issue_key, is_meeting, synced, worklog_id FROM sessions WHERE synced = 0 AND end_time IS NOT NULL AND issue_key IS NOT NULL ORDER BY start_time",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		sess, err := s.scanSessionRow(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, *sess)
	}
	return sessions, rows.Err()
}

func (s *Store) MarkSynced(ids []int64, worklogID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("UPDATE sessions SET synced = 1, worklog_id = ? WHERE id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, id := range ids {
		if _, err := stmt.Exec(worklogID, id); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) GetSessionsForDate(date time.Time) ([]Session, error) {
	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	dayEnd := dayStart.Add(24 * time.Hour)

	rows, err := s.db.Query(
		"SELECT id, start_time, end_time, issue_key, is_meeting, synced, worklog_id FROM sessions WHERE start_time >= ? AND start_time < ? ORDER BY start_time",
		dayStart.Format(time.RFC3339), dayEnd.Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		sess, err := s.scanSessionRow(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, *sess)
	}
	return sessions, rows.Err()
}

func (s *Store) GetTodayTotals() (activeSecs, meetingSecs int, err error) {
	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	rows, err := s.db.Query(
		"SELECT start_time, end_time, is_meeting FROM sessions WHERE start_time >= ? ORDER BY start_time",
		dayStart.Format(time.RFC3339),
	)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var startStr string
		var endPtr *string
		var meeting int

		if err := rows.Scan(&startStr, &endPtr, &meeting); err != nil {
			return 0, 0, err
		}

		start, _ := time.Parse(time.RFC3339, startStr)
		end := now
		if endPtr != nil {
			end, _ = time.Parse(time.RFC3339, *endPtr)
		}

		dur := int(end.Sub(start).Seconds())
		activeSecs += dur
		if meeting == 1 {
			meetingSecs += dur
		}
	}

	return activeSecs, meetingSecs, rows.Err()
}

// Issue cache methods

func (s *Store) GetCachedIssueID(issueKey string) (int, bool, error) {
	var issueID int
	err := s.db.QueryRow(
		"SELECT issue_id FROM issue_cache WHERE issue_key = ?", issueKey,
	).Scan(&issueID)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return issueID, true, nil
}

func (s *Store) CacheIssueID(issueKey string, issueID int) error {
	_, err := s.db.Exec(
		"INSERT OR REPLACE INTO issue_cache (issue_key, issue_id, cached_at) VALUES (?, ?, ?)",
		issueKey, issueID, time.Now().Format(time.RFC3339),
	)
	return err
}

// DeleteShortSessions removes closed sessions shorter than the given duration.
func (s *Store) DeleteShortSessions(minDuration time.Duration) (int64, error) {
	rows, err := s.db.Query(
		"SELECT id, start_time, end_time FROM sessions WHERE end_time IS NOT NULL AND synced = 0",
	)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var toDelete []int64
	for rows.Next() {
		var id int64
		var startStr, endStr string
		if err := rows.Scan(&id, &startStr, &endStr); err != nil {
			return 0, err
		}
		start, _ := time.Parse(time.RFC3339, startStr)
		end, _ := time.Parse(time.RFC3339, endStr)
		if end.Sub(start) < minDuration {
			toDelete = append(toDelete, id)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	if len(toDelete) == 0 {
		return 0, nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("DELETE FROM sessions WHERE id = ?")
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	for _, id := range toDelete {
		if _, err := stmt.Exec(id); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int64(len(toDelete)), nil
}

func (s *Store) scanSession(row *sql.Row) (*Session, error) {
	var sess Session
	var startStr string
	var endStr, issueKey *string
	var meeting, synced int
	var worklogID *int64

	err := row.Scan(&sess.ID, &startStr, &endStr, &issueKey, &meeting, &synced, &worklogID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	sess.StartTime, _ = time.Parse(time.RFC3339, startStr)
	if endStr != nil {
		t, _ := time.Parse(time.RFC3339, *endStr)
		sess.EndTime = &t
	}
	sess.IssueKey = issueKey
	sess.IsMeeting = meeting == 1
	sess.Synced = synced == 1
	sess.WorklogID = worklogID

	return &sess, nil
}

type scannable interface {
	Scan(dest ...any) error
}

func (s *Store) scanSessionRow(row scannable) (*Session, error) {
	var sess Session
	var startStr string
	var endStr, issueKey *string
	var meeting, synced int
	var worklogID *int64

	err := row.Scan(&sess.ID, &startStr, &endStr, &issueKey, &meeting, &synced, &worklogID)
	if err != nil {
		return nil, err
	}

	sess.StartTime, _ = time.Parse(time.RFC3339, startStr)
	if endStr != nil {
		t, _ := time.Parse(time.RFC3339, *endStr)
		sess.EndTime = &t
	}
	sess.IssueKey = issueKey
	sess.IsMeeting = meeting == 1
	sess.Synced = synced == 1
	sess.WorklogID = worklogID

	return &sess, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
