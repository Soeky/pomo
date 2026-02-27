package scheduler

import (
	"context"
	"time"
)

type Input struct {
	From time.Time
	To   time.Time
}

type PlannedEvent struct {
	Title      string
	Kind       string
	Domain     string
	Subtopic   string
	StartTime  time.Time
	EndTime    time.Time
	Source     string
	Status     string
	Dependency []int64
}

type Result struct {
	Generated []PlannedEvent
	Blocked   []PlannedEvent
}

type Engine interface {
	Generate(ctx context.Context, in Input) (Result, error)
}
