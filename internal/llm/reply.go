package llm

import (
	"encoding/json"
	"strings"
)

// replyFormat asks the model to answer in a fixed JSON shape instead of free
// text. It is sent with every request, next to the tool definitions, so a
// reply is either a tool call or this JSON.
const replyFormat = `{
  "type": "json_schema",
  "json_schema": {
    "name": "kite_reply",
    "schema": {
      "type": "object",
      "properties": {
        "mode": { "type": "string", "enum": ["normal", "task", "unsupported"] },
        "answer": { "type": ["string", "null"] },
        "tasks": {
          "type": "array",
          "items": {
            "type": "object",
            "properties": {
              "id": { "type": "string" },
              "description": { "type": "string" },
              "dependencies": { "type": "array", "items": { "type": "string" } },
              "acceptance_criteria": { "type": "array", "items": { "type": "string" } },
              "creates": { "type": "array", "items": { "type": "string" } },
              "verification_policy": {
                "type": "object",
                "properties": {
                  "type": { "type": "string", "enum": ["none", "tool_result", "read_back", "test"] }
                },
                "required": ["type"]
              }
            },
            "required": ["id", "description", "dependencies", "acceptance_criteria", "creates", "verification_policy"]
          }
        }
      },
      "required": ["mode", "answer", "tasks"]
    }
  }
}`

// Reply is the JSON shape the model answers in. Tasks is only filled in "task" mode.
type Reply struct {
	Mode   string `json:"mode"`
	Answer string `json:"answer"`
	Tasks  []Task `json:"tasks"`
}

// Task is one step of a task-mode reply. Only the fields Kite uses are read:
// the description to send, the new files it will create, and the policy that
// decides whether it passed.
type Task struct {
	ID                 string   `json:"id"`
	Description        string   `json:"description"`
	Creates            []string `json:"creates"`
	VerificationPolicy struct {
		Type string `json:"type"`
	} `json:"verification_policy"`
}

// ParseReply reads the model's text as a Reply. ok is false when the text is
// not that JSON, so the caller can fall back to showing the text as it is.
func ParseReply(text string) (reply Reply, ok bool) {
	if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &reply); err != nil || reply.Mode == "" {
		return Reply{}, false
	}
	return reply, true
}
