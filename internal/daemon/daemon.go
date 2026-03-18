package daemon

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"backbeat/internal/config"
	"backbeat/internal/ipc"
	"backbeat/internal/jira"
	"backbeat/internal/monitor"
	"backbeat/internal/store"
	"backbeat/internal/tempo"
	"backbeat/internal/tracker"

	"github.com/rs/zerolog"
)

type Daemon struct {
	cfg     *config.Config
	store   *store.Store
	tracker *tracker.Tracker
	ipcSrv  *ipc.Server
	jira    *jira.Client
	tempo   *tempo.Client
	cancel  context.CancelFunc
	logger  zerolog.Logger
}

func dbPath() string {
	dir := os.Getenv("XDG_DATA_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".local", "share")
	}
	dir = filepath.Join(dir, "backbeat")
	os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, "backbeat.db")
}

func New(cfg *config.Config, logger zerolog.Logger) (*Daemon, error) {
	st, err := store.Open(dbPath())
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}

	d := &Daemon{
		cfg:    cfg,
		store:  st,
		tracker: tracker.New(st, cfg.Tracking.MinSession.Duration, logger.With().Str("component", "tracker").Logger()),
		logger: logger,
	}

	if cfg.Jira.BaseURL != "" {
		d.jira = jira.NewClient(cfg.Jira)
	}
	if cfg.Tempo.APIToken != "" {
		d.tempo = tempo.NewClient(cfg.Tempo)
	}

	return d, nil
}

func (d *Daemon) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel

	events := make(chan monitor.Event, 32)

	// Start activity monitor
	actMon, err := monitor.NewActivityMonitor(
		d.cfg.Tracking.PollInterval.Duration,
		d.cfg.Tracking.IdleThreshold.Duration,
		d.logger.With().Str("component", "activity").Logger(),
	)
	if err != nil {
		cancel()
		d.store.Close()
		return fmt.Errorf("activity monitor: %w", err)
	}
	go actMon.Run(ctx, events)

	// Start sleep monitor
	sleepMon := monitor.NewSleepMonitor(
		d.logger.With().Str("component", "sleep").Logger(),
	)
	go sleepMon.Run(ctx, events)

	// Start meeting monitor
	meetMon := monitor.NewMeetingMonitor(
		d.cfg.Tracking.PollInterval.Duration,
		d.logger.With().Str("component", "meeting").Logger(),
	)
	go meetMon.Run(ctx, events)

	// Start tracker
	go d.tracker.Run(ctx, events)

	// Start IPC server
	ipcSrv, err := ipc.NewServer(d, d.logger.With().Str("component", "ipc").Logger())
	if err != nil {
		cancel()
		d.store.Close()
		return fmt.Errorf("ipc server: %w", err)
	}
	d.ipcSrv = ipcSrv
	go ipcSrv.Run(ctx)

	d.logger.Info().Str("socket", ipc.SocketPath()).Msg("backbeat daemon started")

	// Wait for signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	select {
	case sig := <-sigCh:
		d.logger.Info().Str("signal", sig.String()).Msg("shutting down")
	case <-ctx.Done():
	}

	d.shutdown()
	return nil
}

func (d *Daemon) shutdown() {
	if d.cancel != nil {
		d.cancel()
	}
	if d.ipcSrv != nil {
		d.ipcSrv.Close()
	}
	// Tracker closes active session on context cancel (in Run)
	// Clean up short sessions
	if deleted, err := d.store.DeleteShortSessions(d.cfg.Tracking.MinSession.Duration); err != nil {
		d.logger.Error().Err(err).Msg("failed to clean short sessions")
	} else if deleted > 0 {
		d.logger.Info().Int64("count", deleted).Msg("cleaned short sessions")
	}
	d.store.Close()
}

// IPC Handler implementation

func (d *Daemon) HandleStatus() (*ipc.StatusData, error) {
	state := d.tracker.CurrentState()
	activeSecs, meetingSecs, err := d.store.GetTodayTotals()
	if err != nil {
		return nil, err
	}

	data := &ipc.StatusData{
		State:        state.String(),
		IssueKey:     d.cfg.Tracking.IssueKey,
		IsMeeting:    d.tracker.IsMeeting(),
		TodayTotal:   activeSecs,
		TodayMeeting: meetingSecs,
	}

	if sessID := d.tracker.ActiveSessionID(); sessID > 0 {
		sess, err := d.store.GetActiveSession()
		if err == nil && sess != nil {
			data.SessionStart = sess.StartTime.Format(time.RFC3339)
		}
	}

	return data, nil
}

func (d *Daemon) HandleStop() error {
	d.logger.Info().Msg("stop requested via IPC")
	if d.cancel != nil {
		d.cancel()
	}
	return nil
}

func (d *Daemon) HandleTrack(args ipc.TrackArgs) error {
	d.cfg.Tracking.IssueKey = args.IssueKey

	// Assign to current active session
	if sessID := d.tracker.ActiveSessionID(); sessID > 0 {
		if err := d.store.SetSessionIssueKey(sessID, args.IssueKey); err != nil {
			return fmt.Errorf("set session issue: %w", err)
		}
	}

	// Assign to today's unassigned sessions
	affected, err := d.store.SetIssueKeyForUnassigned(args.IssueKey, time.Now())
	if err != nil {
		return fmt.Errorf("set unassigned issues: %w", err)
	}

	d.logger.Info().Str("issue", args.IssueKey).Int64("updated", affected).Msg("tracking issue")

	// Save to config
	return d.cfg.SetIssueKey(args.IssueKey)
}

func (d *Daemon) HandleSync() (*ipc.SyncData, error) {
	if d.tempo == nil {
		return nil, fmt.Errorf("tempo not configured (set tempo.api_token in config)")
	}
	if d.jira == nil {
		return nil, fmt.Errorf("jira not configured (set jira.base_url in config)")
	}

	sessions, err := d.store.GetUnsyncedSessions()
	if err != nil {
		return nil, fmt.Errorf("get unsynced sessions: %w", err)
	}

	if len(sessions) == 0 {
		return &ipc.SyncData{WorklogsCreated: 0}, nil
	}

	// Aggregate by (issue_key, date)
	type aggKey struct {
		IssueKey string
		Date     string
	}
	type aggValue struct {
		TotalSeconds int
		StartTime    string
		IDs          []int64
	}

	aggs := make(map[aggKey]*aggValue)
	for _, sess := range sessions {
		if sess.IssueKey == nil || sess.EndTime == nil {
			continue
		}
		key := aggKey{
			IssueKey: *sess.IssueKey,
			Date:     sess.StartTime.Format("2006-01-02"),
		}
		dur := int(sess.EndTime.Sub(sess.StartTime).Seconds())
		if dur <= 0 {
			continue
		}
		if v, ok := aggs[key]; ok {
			v.TotalSeconds += dur
			v.IDs = append(v.IDs, sess.ID)
		} else {
			aggs[key] = &aggValue{
				TotalSeconds: dur,
				StartTime:    sess.StartTime.Format("15:04:05"),
				IDs:          []int64{sess.ID},
			}
		}
	}

	created := 0
	for key, val := range aggs {
		// Resolve issue ID
		issueID, err := d.resolveIssueID(key.IssueKey)
		if err != nil {
			d.logger.Error().Err(err).Str("issue", key.IssueKey).Msg("failed to resolve issue ID")
			continue
		}

		worklog, err := d.tempo.CreateWorklog(tempo.CreateWorklogRequest{
			IssueID:          issueID,
			TimeSpentSeconds: val.TotalSeconds,
			StartDate:        key.Date,
			StartTime:        val.StartTime,
			Description:      "Tracked by backbeat",
		})
		if err != nil {
			d.logger.Error().Err(err).Str("issue", key.IssueKey).Str("date", key.Date).Msg("failed to create worklog")
			continue
		}

		if err := d.store.MarkSynced(val.IDs, int64(worklog.TempoWorklogID)); err != nil {
			d.logger.Error().Err(err).Msg("failed to mark sessions as synced")
			continue
		}

		d.logger.Info().
			Str("issue", key.IssueKey).
			Str("date", key.Date).
			Int("seconds", val.TotalSeconds).
			Int("worklog_id", worklog.TempoWorklogID).
			Msg("worklog created")
		created++
	}

	return &ipc.SyncData{WorklogsCreated: created}, nil
}

func (d *Daemon) resolveIssueID(issueKey string) (int, error) {
	// Check cache first
	if id, found, err := d.store.GetCachedIssueID(issueKey); err != nil {
		d.logger.Warn().Err(err).Msg("cache lookup failed")
	} else if found {
		return id, nil
	}

	// Resolve via Jira API
	id, err := d.jira.GetIssueID(issueKey)
	if err != nil {
		return 0, err
	}

	// Cache the result
	if err := d.store.CacheIssueID(issueKey, id); err != nil {
		d.logger.Warn().Err(err).Msg("failed to cache issue ID")
	}

	return id, nil
}
