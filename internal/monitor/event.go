package monitor

import "time"

type EventType int

const (
	EventActive EventType = iota
	EventIdle
	EventSleep
	EventWake
	EventMeetingStart
	EventMeetingEnd
)

func (e EventType) String() string {
	switch e {
	case EventActive:
		return "active"
	case EventIdle:
		return "idle"
	case EventSleep:
		return "sleep"
	case EventWake:
		return "wake"
	case EventMeetingStart:
		return "meeting_start"
	case EventMeetingEnd:
		return "meeting_end"
	default:
		return "unknown"
	}
}

type Event struct {
	Type      EventType
	Timestamp time.Time
}
