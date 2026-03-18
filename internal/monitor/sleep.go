package monitor

import (
	"context"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/rs/zerolog"
)

type SleepMonitor struct {
	logger zerolog.Logger
}

func NewSleepMonitor(logger zerolog.Logger) *SleepMonitor {
	return &SleepMonitor{logger: logger}
}

func (m *SleepMonitor) Run(ctx context.Context, events chan<- Event) {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		m.logger.Error().Err(err).Msg("failed to connect to system bus")
		return
	}
	defer conn.Close()

	if err := conn.AddMatchSignal(
		dbus.WithMatchInterface("org.freedesktop.login1.Manager"),
		dbus.WithMatchMember("PrepareForSleep"),
	); err != nil {
		m.logger.Error().Err(err).Msg("failed to add match signal")
		return
	}

	sigCh := make(chan *dbus.Signal, 8)
	conn.Signal(sigCh)

	for {
		select {
		case <-ctx.Done():
			return
		case sig := <-sigCh:
			if sig.Name != "org.freedesktop.login1.Manager.PrepareForSleep" {
				continue
			}
			goingToSleep, ok := sig.Body[0].(bool)
			if !ok {
				continue
			}

			now := time.Now()
			if goingToSleep {
				m.logger.Info().Msg("system going to sleep")
				events <- Event{Type: EventSleep, Timestamp: now}
			} else {
				m.logger.Info().Msg("system woke up")
				events <- Event{Type: EventWake, Timestamp: now}
			}
		}
	}
}
