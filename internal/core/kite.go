package core

import (
	"context"
)

// Kite is what the core offers to any interface that wants to use it.
type Kite interface {
	Ask(ctx context.Context, question string) (string, error)
}
