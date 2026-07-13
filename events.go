package main

import "time"

// EventKind identifies the type of pipeline event.
type EventKind int

const (
	EventStepStart EventKind = iota
	EventStepOutput
	EventStepComplete
	EventStepStream
	EventFileChanges
	EventStepRetry
	EventStepSkip
	EventDone
)

// Event is a typed message emitted by the pipeline executor.
type Event struct {
	Kind       EventKind
	StepIndex  int
	Output     string
	Line       string
	Duration   time.Duration
	Err        error
	Changes    []string
	Attempt    int
	MaxRetries int
	Prompt     string
}

// emit sends an event to the channel without blocking.
// If ch is nil or full, the event is silently dropped.
func emit(ch chan<- Event, e Event) {
	if ch == nil {
		return
	}
	select {
	case ch <- e:
	default:
		// Drop event to avoid blocking the executor.
	}
}
