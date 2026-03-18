package monitor

import (
	"context"
	"encoding/json"
	"os/exec"
	"time"

	"github.com/rs/zerolog"
)

// CommandRunner abstracts command execution for testing.
type CommandRunner interface {
	Output(name string, args ...string) ([]byte, error)
}

type execRunner struct{}

func (e execRunner) Output(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}

type pwNode struct {
	ID   int    `json:"id"`
	Type string `json:"type"`
	Info *struct {
		State string         `json:"state"`
		Props map[string]any `json:"props"`
	} `json:"info"`
}

type MeetingMonitor struct {
	pollInterval time.Duration
	runner       CommandRunner
	logger       zerolog.Logger
	inMeeting    bool
}

func NewMeetingMonitor(pollInterval time.Duration, logger zerolog.Logger) *MeetingMonitor {
	return &MeetingMonitor{
		pollInterval: pollInterval,
		runner:       execRunner{},
		logger:       logger,
	}
}

// NewMeetingMonitorWithRunner creates a MeetingMonitor with a custom command runner (for testing).
func NewMeetingMonitorWithRunner(pollInterval time.Duration, runner CommandRunner, logger zerolog.Logger) *MeetingMonitor {
	return &MeetingMonitor{
		pollInterval: pollInterval,
		runner:       runner,
		logger:       logger,
	}
}

func (m *MeetingMonitor) Run(ctx context.Context, events chan<- Event) {
	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			meeting, err := m.checkMeeting()
			if err != nil {
				m.logger.Warn().Err(err).Msg("meeting detection failed")
				continue
			}

			now := time.Now()
			if meeting && !m.inMeeting {
				m.logger.Info().Msg("meeting detected")
				events <- Event{Type: EventMeetingStart, Timestamp: now}
				m.inMeeting = true
			} else if !meeting && m.inMeeting {
				m.logger.Info().Msg("meeting ended")
				events <- Event{Type: EventMeetingEnd, Timestamp: now}
				m.inMeeting = false
			}
		}
	}
}

func (m *MeetingMonitor) checkMeeting() (bool, error) {
	out, err := m.runner.Output("pw-dump")
	if err != nil {
		return false, err
	}

	var nodes []pwNode
	if err := json.Unmarshal(out, &nodes); err != nil {
		return false, err
	}

	for _, n := range nodes {
		if n.Type != "PipeWire:Interface:Node" || n.Info == nil {
			continue
		}
		if n.Info.State != "running" {
			continue
		}

		mediaClass, _ := n.Info.Props["media.class"].(string)
		switch mediaClass {
		case "Audio/Source", "Video/Source":
			return true, nil
		}
	}

	return false, nil
}
