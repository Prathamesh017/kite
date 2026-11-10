package core

import (
	"encoding/json"
	"strings"
)

// ReplyPrompt is added to the system prompt for the user's own question, next to
// the reply format. It tells the model when to answer and when to break the request into tasks.
const ReplyPrompt = `Answer in the JSON shape. Before calling any tool, decide which kind of request this is:
1. A task: the request asks for several separate steps, for example reading or changing more than one file, or doing one thing and then another. Put the steps in "tasks" and set "answer" to null. Do not call tools yet. Each task is run on its own and only sees its description and the results of the tasks before it, so write each description so it can be done from that alone. In "creates", list the path of every new file the task will create, or [] if it creates none. Choose each task's "verification_policy": "none" when nothing needs checking, for example the task only reads or explains; "tool_result" when it passes as long as its tools do not fail; "read_back" when the task creates files and a check should read them back and compare them with "acceptance_criteria". For "read_back", list in "acceptance_criteria" what the files must contain. Otherwise set it to [].
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
          "creates": { "type": "array", "items": { "type": "string" } },
          "verification_policy": { "type": "string", "enum": ["none", "tool_result", "read_back"] },
          "acceptance_criteria": { "type": "array", "items": { "type": "string" } }
        },
        "required": ["description", "creates", "verification_policy", "acceptance_criteria"]
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

// How a task is checked once the model has given its reply.
const (
	policyNone       = "none"        // nothing to check
	policyToolResult = "tool_result" // passes as long as its tools did not fail
	policyReadBack   = "read_back"   // a check reads the created files and compares them with the acceptance criteria
)

// Task is one task: what to do, the new files it will create, and how it is checked.
// AcceptanceCriteria is only used by the read_back check.
type Task struct {
	Description        string   `json:"description"`
	Creates            []string `json:"creates"`
	VerificationPolicy string   `json:"verification_policy"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
}

// Mode is what the model's reply is.
type Mode int

const (
	ModeAnswer   Mode = iota // a plain answer
	ModeToolCall             // the model asks to run tools
	ModeTask                 // the model broke the request into tasks
)

// cutOffNote is added to a reply the model cut off at its length limit.
const cutOffNote = "\n\nThe reply was cut off because it hit the model's length limit."

func markCutOff(answer string, truncated bool) string {
	if truncated {
		return answer + cutOffNote
	}
	return answer
}

// parseReply says what the model's reply is. For ModeAnswer and ModeTask the
// returned Reply holds the answer or the tasks. A reply that is not in the JSON
// shape is an answer, and comes back as it is. A reply that was cut off is never
// tasks, because its list may be incomplete.
func parseReply(reply Message) (Mode, Reply) {
	if len(reply.ToolCalls) > 0 {
		return ModeToolCall, Reply{}
	}
	if reply.Truncated {
		return ModeAnswer, Reply{Answer: reply.Message}
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
