package core

import (
	"context"
)

// Kite is what the core offers to any interface that wants to use it.
type Kite interface {
	// Ask starts a run for a question and does its first step.
	Ask(ctx context.Context, question string) (Result, error)
	// Step does the next step of the run. Call it until a result is Answered.
	Step(ctx context.Context) (Result, error)
}
