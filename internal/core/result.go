package core

// Kind says what a step did.
type Kind int

const (
	AskedForTool Kind = iota // the model asked to run tools
	ToolRan                  // the tools ran, and their results wait for the next model call
	Answered                 // the model gave its answer, the run is finished
)

// Result is what Ask and Step report. The run is finished when Kind is Answered.
// For any other kind the caller calls Step again.
type Result struct {
	Kind   Kind
	Tools  []ToolCall // AskedForTool: the calls the model asked for. ToolRan: the calls that ran.
	Answer string     // Answered: the answer text
}
