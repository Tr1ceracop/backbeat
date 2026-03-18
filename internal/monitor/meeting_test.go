package monitor

import (
	"context"
	"testing"
	"time"

	"os"

	"github.com/rs/zerolog"
)

type mockRunner struct {
	output []byte
	err    error
}

func (m *mockRunner) Output(name string, args ...string) ([]byte, error) {
	return m.output, m.err
}

func TestMeetingDetectsRunningAudioSource(t *testing.T) {
	runner := &mockRunner{
		output: []byte(`[
			{
				"id": 1,
				"type": "PipeWire:Interface:Node",
				"info": {
					"state": "running",
					"props": {
						"media.class": "Audio/Source",
						"node.name": "alsa_input.pci"
					}
				}
			}
		]`),
	}

	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)
	mon := NewMeetingMonitorWithRunner(100*time.Millisecond, runner, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan Event, 10)
	go mon.Run(ctx, events)

	select {
	case ev := <-events:
		if ev.Type != EventMeetingStart {
			t.Fatalf("expected meeting start, got %s", ev.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for meeting event")
	}
}

func TestMeetingIgnoresSuspendedSource(t *testing.T) {
	runner := &mockRunner{
		output: []byte(`[
			{
				"id": 1,
				"type": "PipeWire:Interface:Node",
				"info": {
					"state": "suspended",
					"props": {
						"media.class": "Audio/Source",
						"node.name": "alsa_input.pci"
					}
				}
			}
		]`),
	}

	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)
	mon := NewMeetingMonitorWithRunner(100*time.Millisecond, runner, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan Event, 10)
	go mon.Run(ctx, events)

	select {
	case ev := <-events:
		t.Fatalf("expected no event, got %s", ev.Type)
	case <-time.After(300 * time.Millisecond):
		// OK - no event expected
	}
}

func TestMeetingDetectsVideoSource(t *testing.T) {
	runner := &mockRunner{
		output: []byte(`[
			{
				"id": 2,
				"type": "PipeWire:Interface:Node",
				"info": {
					"state": "running",
					"props": {
						"media.class": "Video/Source",
						"node.name": "v4l2_input"
					}
				}
			}
		]`),
	}

	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)
	mon := NewMeetingMonitorWithRunner(100*time.Millisecond, runner, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan Event, 10)
	go mon.Run(ctx, events)

	select {
	case ev := <-events:
		if ev.Type != EventMeetingStart {
			t.Fatalf("expected meeting start, got %s", ev.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for meeting event")
	}
}

func TestMeetingEndEvent(t *testing.T) {
	runner := &mockRunner{
		output: []byte(`[
			{
				"id": 1,
				"type": "PipeWire:Interface:Node",
				"info": {
					"state": "running",
					"props": {
						"media.class": "Audio/Source"
					}
				}
			}
		]`),
	}

	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)
	mon := NewMeetingMonitorWithRunner(100*time.Millisecond, runner, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan Event, 10)
	go mon.Run(ctx, events)

	// Wait for meeting start
	<-events

	// Switch to no meeting
	runner.output = []byte(`[
		{
			"id": 1,
			"type": "PipeWire:Interface:Node",
			"info": {
				"state": "suspended",
				"props": {
					"media.class": "Audio/Source"
				}
			}
		}
	]`)

	select {
	case ev := <-events:
		if ev.Type != EventMeetingEnd {
			t.Fatalf("expected meeting end, got %s", ev.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for meeting end event")
	}
}
