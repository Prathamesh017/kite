package core

import (
	"encoding/json"
	"strings"
)

// ReplyPrompt is added to the system prompt for the user's own question, next to
// the reply format. It tells the model when to answer and when to break the request into tasks.
const ReplyPrompt = `Answer in the JSON shape. Before calling any tool, decide which kind of request this is:
1. A task: the request asks for several separate steps, for example reading or changing more than one file, or doing one thing and then another. Put the steps in "tasks" and set "answer" to null. Do not call tools yet. Each task is run on its own and only sees its description and the results of the tasks before it, so write each description so it can be done from that alone. In "creates", list the path of every new file the task will create, or [] if it creates none.
2. Anything else, including a request that none of your tools can do. If you need one file to answer, call a tool. When you can answer, put the answer in "answer" and set "tasks" to []. If none of your tools can do what is asked, say so in "answer".`

// replyFormat is the shape the model answers the user's question in: an answer, or a list of tasks.
var replyFormat = &ReplyFormat{
	Name: "kite_reply",
	Schema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "answer": { "type": ["string", "null"] },
    "tasks": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "description": { "type": "string" },
          "creates": { "type": "array", "items": { "type": "string" } }
        },
        "required": ["description", "creates"]
      }
    }
  },
  "required": ["answer", "tasks"]
}`),
}

// Reply is what the model answers in. Tasks is only filled when the model broke the request into tasks.
type Reply struct {
	Answer string `json:"answer"`
	Tasks  []Task `json:"tasks"`
}

// Task is one task: what to do, and the new files it will create.
type Task struct {
	Description string   `json:"description"`
	Creates     []string `json:"creates"`
}

// Mode is what the model's reply is.
type Mode int

const (
	ModeAnswer   Mode = iota // a plain answer
	ModeToolCall             // the model asks to run tools
	ModeTask                 // the model broke the request into tasks
)

// parseReply says what the model's reply is. For ModeAnswer and ModeTask the
// returned Reply holds the answer or the tasks. A reply that is not in the JSON
// shape is an answer, and comes back as it is.
func parseReply(reply Message) (Mode, Reply) {
	if len(reply.ToolCalls) > 0 {
		return ModeToolCall, Reply{}
	}

	var parsed Reply
	if err := json.Unmarshal([]byte(strings.TrimSpace(reply.Message)), &parsed); err != nil ||
		(parsed.Answer == "" && len(parsed.Tasks) == 0) {
		return ModeAnswer, Reply{Answer: reply.Message}
	}
	if len(parsed.Tasks) > 0 {
		return ModeTask, parsed
	}
	return ModeAnswer, parsed
}
