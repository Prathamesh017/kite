package core

import (
	"context"
	"encoding/json"
)

// ToolSpec tells the model about one tool. Parameters is a JSON schema of its arguments.
type ToolSpec struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

// Tools is what the core needs to offer tools to the model and to run them.
type Tools interface {
	Specs() []ToolSpec
	Run(ctx context.Context, name, arguments string) string
}
