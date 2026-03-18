package monitor

import (
	"context"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/rs/zerolog"
)

const (
	mutterDest  = "org.gnome.Mutter.IdleMonitor"
	mutterPath  = "/org/gnome/Mutter/IdleMonitor/Core"
	mutterIface = "org.gnome.Mutter.IdleMonitor"
)

type ActivityMonitor struct {
	conn          *dbus.Conn
	pollInterval  time.Duration
	idleThreshold time.Duration
	logger        zerolog.Logger
	wasIdle       bool
}

func NewActivityMonitor(pollInterval, idleThreshold time.Duration, logger zerolog.Logger) (*ActivityMonitor, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, err
	}

	return &ActivityMonitor{
		conn:          conn,
		pollInterval:  pollInterval,
		idleThreshold: idleThreshold,
		logger:        logger,
		wasIdle:       true, // start as idle, transition to active on first activity
	}, nil
}

func (m *ActivityMonitor) Run(ctx context.Context, events chan<- Event) {
	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()
	defer m.conn.Close()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			idle, err := m.getIdleTime()
			if err != nil {
				m.logger.Warn().Err(err).Msg("failed to get idle time, attempting reconnect")
				if err := m.reconnect(); err != nil {
					m.logger.Error().Err(err).Msg("reconnect failed")
				}
				continue
			}

			now := time.Now()
			isIdle := idle >= m.idleThreshold

			if m.wasIdle && !isIdle {
				m.logger.Debug().Dur("idle_was", idle).Msg("user became active")
				events <- Event{Type: EventActive, Timestamp: now}
				m.wasIdle = false
			} else if !m.wasIdle && isIdle {
				// Emit idle event backdated to when idleness began
				idleStart := now.Add(-idle)
				m.logger.Debug().Dur("idle_for", idle).Msg("user became idle")
				events <- Event{Type: EventIdle, Timestamp: idleStart}
				m.wasIdle = true
			}
		}
	}
}

func (m *ActivityMonitor) getIdleTime() (time.Duration, error) {
	obj := m.conn.Object(mutterDest, mutterPath)
	call := obj.Call(mutterIface+".GetIdletime", 0)
	if call.Err != nil {
		return 0, call.Err
	}

	var idleMs uint64
	if err := call.Store(&idleMs); err != nil {
		return 0, err
	}

	return time.Duration(idleMs) * time.Millisecond, nil
}

func (m *ActivityMonitor) reconnect() error {
	if m.conn != nil {
		m.conn.Close()
	}
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return err
	}
	m.conn = conn
	return nil
}
