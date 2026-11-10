package core

import (
	"encoding/json"
	"strings"
)

// verdictFormat is the shape of a read_back check's reply: a yes or no, and a line saying why.
var verdictFormat = &ReplyFormat{
	Name: "kite_verdict",
	Schema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "passed": { "type": "boolean" },
    "message": { "type": "string" }
  },
  "required": ["passed", "message"]
}`),
}

// Verdict is a read_back check's reply. Message says what the files hold, or why the task fails.
type Verdict struct {
	Passed  *bool  `json:"passed"`
	Message string `json:"message"`
}

// parseVerdict reads the model's text as a Verdict. ok is false when the text is
// not that JSON, or has no yes or no.
func parseVerdict(text string) (verdict Verdict, ok bool) {
	if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &verdict); err != nil || verdict.Passed == nil {
		return Verdict{}, false
	}
	return verdict, true
}
