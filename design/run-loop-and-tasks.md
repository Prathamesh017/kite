# Kite: The Run Loop, Limits and Tasks

Decisions 10-23: how a run proceeds step by step, what stops it, how failures are retried, and how a request is broken into tasks. Back to the [architecture overview](../ARCHITECTURE.md). Previous: [ports, the UI and tools](ports-and-ui.md).

### Decision 10: The core keeps the run counters

**Context:** The old app counted `iterations` (model replies) and `toolCalls` (tools run) on the TUI model, so a run's limits depended on the UI. Something has to count them, and stop a run that goes on too long.

**Decision:** The core keeps both counters.

- `iterations` counts model replies. `toolCalls` counts tools actually run.
- Both are per loop: one question (and later, one task). They build up across the rounds of that loop and go back to 0 when the next question starts. They are not per conversation.

**Why:** A tool adapter is called one call at a time, and `Run("read_file", ...)` cannot know whether it is the first call of a question or the fifteenth. Only the core sees a whole run, from the question to the answer, so only the core can count across it and decide to stop. The same is true of the LLM adapter.

**Decided later:** the loop shape (Decision 11) and the limits (Decision 14).

### Decision 11: The loop is step by step, and `Ask` starts a run while `Step` continues it

**Context:** A question with tool calls takes several rounds: ask the model, run the tool, send the result back, repeat. The old app did this without a `for` loop. Each piece (a model call, a tool run) was a background command that sent a message back, and that message started the next piece. The core needs the same rhythm, and it must be able to tell a new question (counters at 0,0) from a continuation of a run.

**Decision:** The core works step by step, as the old flow did. One call does one piece of work and then returns to the caller. The caller calls again until the run is finished. There are two entry points, and the method that is called says which case it is:

```text
Ask("read main.go")  → starts a run: counters = 0,0, the question is added, the first step is done
Step()               → continues the run, never resets: runs the tool, or asks the model again
Step()               → ... until a step says the run is finished
```

- `Ask` is the only place where a run starts, so it is the only place where `iterations` and `toolCalls` are reset (Decision 10).
- `iterations` goes up when the model replies. `toolCalls` goes up when tools run. Each happens inside its own step.
- The state of a run (the counters and the messages being built) has to survive between calls, so it is kept on the `Agent`. `Ask` creates it fresh, and it is dropped when the run ends.

**Why:** Between steps the control is back with the caller, so a front end can redraw, show progress or cancel. A `Step` that returns cannot keep the counters in local variables, which is why the run state is stored. Choosing by method name avoids guessing "first call or loop call" from the counter values.

**Decided later:** what a step returns (Decision 12), the limits (Decision 14), progress in the UI (Decision 16), and cancel and timeouts (Decision 17).

### Decision 12: A step reports what happened, as facts, in three kinds

**Context:** After `Ask` or `Step` returns, the caller must know what just happened and whether to call `Step` again. The old app told the user in two ways, both built in the TUI: a temporary status line ("thinking", "reading main.go") and permanent lines in the conversation ("> using tool call - read_file", then the answer). The wording, such as `toolStatus` turning a read_file call into "reading main.go", lived in `internal/tui/loop.go`.

**Decision:** Each call returns a small result with a kind, and the kind carries facts, not display text. For now there are three kinds, the same three moments the old app notified the user about:

| Kind | Facts it carries | Old notification it replaces |
|---|---|---|
| asked for a tool | tool names and arguments | status "reading main.go" |
| tool ran | tool names | line "> using tool call - read_file" |
| answered | the answer text | the answer line |

The run is finished when the kind is "answered". Any other kind means the caller calls `Step` again.

```text
Ask("what is in main.go?")  → asked for a tool: read_file {"path":"main.go"}   TUI: status "reading main.go"
Step()                      → tool ran: read_file                              TUI: line "> using tool call - read_file"
Step()                      → answered: "main.go starts the TUI ..."           TUI: the answer
```

The TUI builds the wording from the facts, so it keeps the old style. Another UI is free to show the same facts differently.

**Why:** The core sends application-level results, not instructions about what to draw (section 5 of the [overview](../ARCHITECTURE.md)). The result is also the progress information, so the Activity pane can show what is happening without a separate events mechanism.

**Names in code:** `Result` and `Kind`, with the kinds `AskedForTool`, `ToolRan` and `Answered`. There is no separate "stopped" kind: a limit is reported as `Answered` (Decision 14).

### Decision 13: Running the tools and asking the model again are separate steps

**Context:** After the model asks for a tool, two things happen: the tool runs, and the result is sent back to the model. In the old app they were two background commands: `runTools` sent `toolsDoneMsg`, and only then `onToolsDone` started the next `ask`.

**Decision:** They stay two separate steps. The step that runs the tools reports `ToolRan`. The next step asks the model again.

```text
Ask   → asked for a tool: read_file     (the model said "read main.go")
Step  → tool ran: read_file             (only the tool ran, and the result went into the messages)
Step  → answered: "main.go starts ..."  (the model was asked again and answered)
```

**Why:** Each step ends with a result the UI can show. "Tool ran" is its own moment, as it was in the old app (the "> using tool call - read_file" line), and cancel and the limits are checked between steps.

### Decision 14: The limits come from `.env`, and are checked before the next step

**Context:** A model that keeps asking for tools would never finish. The old app capped it with `MAX_ITERATIONS` and `MAX_TOOL_CALLS`.

**Decision:**
- The core reads `MAX_ITERATIONS` (default 10) and `MAX_TOOL_CALLS` (default 20) from the environment, as the old app did. `main.go` loads `.env` into the environment. A missing or unreadable value falls back to the default.
- `iterations` counts the model replies that asked for tools. `toolCalls` counts the tools actually run. Both are per run (Decision 10).
- The limits are checked at the start of `Step`, before the next step:
  - before running a batch of tools: stop if the batch would push `toolCalls` past the limit
  - before asking the model again: stop if `iterations` has reached the limit
- **When a limit is hit**, as in the old app, a note ("Reached its tool-call limit. The task may be partially complete." or "Reached its execution-step limit before completing the task.") is returned as the answer, an `Answered` result. The work done so far stays in the history, followed by the note, so the user can ask to continue. A tool request that was refused is dropped from the history first, because a tool request without results would make the next request invalid.

```text
17 tools run, the model asks for 5 more → 5 > 20 - 17, stop before running them: "Reached its tool-call limit..."
10th round of tools has run             → iterations = 10, stop before asking the model: "Reached its execution-step limit..."
```

**Why:** Only the core sees a whole run, so it owns the limits. The old behavior was kept so the two apps match.

**Note:** `read_file` returns 200 lines at a time, so a file of about 2000 lines needs about ten rounds and can reach the iteration limit.

### Decision 15: Provider-specific data on a tool call is kept and given back unchanged

**Context:** Gemini 3 models attach a thought signature to each tool call, in `extra_content.google.thought_signature`. When the tool result is sent back, the signature must come back on the same call, or the request fails with a 400 ("Function call is missing a thought_signature"). The old client kept it as an opaque `ExtraContent` field on the tool call.

**Decision:** `core.ToolCall` has an `ExtraContent json.RawMessage` field. The Gemini adapter fills it from the response and writes it back when it builds the next request. The core never reads it. It only keeps it with the call, like the call's ID.

```go
// internal/core/llm.go
type ToolCall struct {
    ID           string
    Name         string
    Arguments    string
    ExtraContent json.RawMessage // provider-specific, kept and given back unchanged
}
```

**Why:** The adapter stays stateless, and the signature lives in the history, so it survives if the history is saved and restored later. The core stays provider-neutral, because another provider simply leaves the field empty. The alternative, the adapter remembering signatures by call ID, would make the adapter stateful and lose them after a restart.

### Decision 16: The Activity pane shows tool progress from the step results

**Context:** After Decision 12 the core sends facts for every step. The old pane showed a shimmering status ("reading main.go") and a permanent line per tool ("> using tool call - read_file"). The question is not in the pane's list until the answer arrives (Decision 6), so a tool line added to the list would appear above its question.

**Decision:** The pane draws, top to bottom: the finished conversation, the question being answered (`pending`), the tool lines of this run (`steps`), and the status line. When the answer arrives, the question, the tool lines and the answer join the conversation in that order.

```text
Enter            pending = "what is in main.go?"    status: thinking
AskedForTool     status  = "reading main.go"        (words built by toolStatus from the tool name and its path)
ToolRan          steps   = [read_file]              line "> using tool call - read_file", status: thinking
Answered         conversation += question, tool line, answer; pending and steps cleared
error            steps and pending dropped, the question goes back to the chat input
```

- `Status` gets a value `StatusTool`, and `Activity.detail` holds its text. `statusText()` replaces `Status.label()`, because the tool status needs the detail.
- The wording ("reading", "writing", "listing files", "using <name>") lives in the TUI (`toolStatus`). The core only sends the tool name and arguments, as in Decision 12.

**Why:** The question always stays above its tool lines without sorting, and an error throws the run's lines away cleanly. The old look is kept.

### Decision 17: Timeouts, retry, cancel and the panic guard are ported from the old app

**Context:** The old app had safety pieces around a run. v2 had none, so a stalled request left the input blocked.

**Decision:** Each piece goes where its knowledge lives.

| Piece | Where | Behavior |
|---|---|---|
| Request timeout | Gemini adapter | `REQUEST_TIMEOUT`, 60s by default, on the HTTP client |
| Retry with backoff | Gemini adapter (moved to the core by Decision 22) | `MAX_RETRIES` (3), `RETRY_INITIAL_DELAY` (500ms), `RETRY_MAX_DELAY` (8s). Only 429, 500, 503 and 504 are retried. |
| Run timeout | TUI | One minute per question. The question goes back to the input with "Request timed out, please try again." |
| Cancel | TUI | `esc` cancels the run. What was shown stays, without an answer. A step that lands after `esc` is ignored. |
| Panic guard | TUI | A panic inside a step becomes an error message, and the question goes back to the input. |

The TUI holds the run's context (`runCtx`, `cancelRun`) and passes it to every `Ask` and `Step`, so the whole run shares one deadline.

### Decision 18: The model's reply has three modes, and the core recognises which one

**Context:** Besides plain answers and tool calls, a request with several separate steps should become tasks. The core needs to tell the three apart. The old app asked for a fixed JSON reply shape (`mode`: normal, task or unsupported, plus ids, dependencies and more) on every question. Several of those fields were never read.

**Decision:** The model decides, and the core only recognises its decision. The question is sent with a reply format, and the reply is one of three modes.

```go
// internal/core/reply.go
type Mode int
const (
    ModeAnswer   Mode = iota // a plain answer
    ModeToolCall             // the model asks to run tools
    ModeTask                 // the model broke the request into tasks
)
mode, parsed := parseReply(reply)
```

- The reply format is `{"answer": string|null, "tasks": [{description, creates, verification_policy, acceptance_criteria}]}`. A non-empty `tasks` means tasks. Otherwise `answer` is the answer. `id`, `dependencies` and `mode` are gone, because nothing read them. `ReplyPrompt` tells the model when to break a request into tasks.
- A reply that is not in that JSON shape is shown as the plain text it is.
- The history keeps the answer, not the JSON around it.
- `Agent.askModel` has one `switch` case per mode, and an error after it for a mode it does not know.

**The format is data, not an enum.** `ReplyFormat` is a name and a JSON schema, passed to `Complete` as one more argument (nil means plain text). The Gemini adapter wraps it into `response_format`. The core owns the schema and the parsing, because it knows what a task means. The adapter knows nothing about tasks. Another shape, such as the verdict of a check, is just another value.

**Why:** The core stays provider-neutral, and a new shape needs no change to the port or the adapter.

### Decision 19: A task runs in a sub-run of its own

**Context:** The old app started each task from a fresh history and reset its counters, with several switches (`loopHistory`, `tasks.Active()`) to pick the right state.

**Decision:** When the model returns tasks, the question's `run` keeps the task list, which task is current, and a sub-run for it. Each task gets a fresh `run` with its own messages and counters, dropped when the task ends.

```text
Ask       the model returns tasks          → TasksCreated
Step      startTask: a fresh run           → TaskStarted(0)
Step ...  the task's tool rounds           → AskedForTool / ToolRan  (same code as for a question)
Step      the model's plain reply          → judged → TaskFinished(0, passed)
Step      startTask for the next task ...
Step      none left, or one failed         → Answered: the summary
```

- A task sees only `Task: <description>` plus `Results of the earlier tasks:` (each finished task's description and reply), not the conversation. It answers in plain text, with no reply format.
- Tool errors are remembered per tool. A later success of the same tool clears the error.
- A task passes when no tool error is left, unless its policy is `none`. A task fails on a leftover tool error, a limit hit inside it (the limit note is its reply), or a failed model request. A cancel or timeout is not a task failure. It goes back to the caller.
- **After a failure, no later task starts.** They show as `not run`.
- The run ends with a summary as the answer: `N of M tasks passed`, then each task with its status and reply indented under it. The history keeps the question and the summary, not the work inside the tasks.
- `Agent.stepTasks` decides the next step from three states: a task is running (continue it), `finished` is set (end the run), or a task just ended and more are left (start the next). `finished` is set in `finishTask`, when a task fails or the last one ends.

**Why:** Fresh state per task falls out of creating a new run. The tool and limit code is reused as it is, because `runTools` works on whichever run it is given.

### Decision 20: Task progress is one result kind per moment, and the pane follows it

**Context:** Extends Decision 12. The UI needs to show a task list that changes, and the old app showed rows with statuses and a note for each finished task.

**Decision:** Four more result kinds, in the same style as the tool kinds. They carry facts.

| Kind | Facts | The TUI shows |
|---|---|---|
| `TasksCreated` | the tasks | the task list pane, all `pending`, status "creating todo list" |
| `TaskStarted` | which task | the row `running`, status "thinking" |
| `TaskChecking` | which task | status "checking the result" |
| `TaskFinished` | which task, whether it passed | the row `passed` or `failed`, and for a pass the line "> task N completed" |

The TUI keeps its own `TaskList` ([Decision 5](ports-and-ui.md) applied to tasks) and clears it when the run ends, fails or is cancelled. The list takes at most half of the middle band. The "task N completed" lines join the conversation with the tool lines, as in Decision 16.

### Decision 21: The model chooses a task's verification policy, and `read_back` is a check with a yes or no

**Context:** A task that has no failed tool is not necessarily a good result. The old app let the model choose a policy for each task.

**Decision:** Each task carries `verification_policy` (`none`, `tool_result` or `read_back`) and `acceptance_criteria`, chosen by the model, as the old app did. `ReplyPrompt` explains the three.

- `none`: the task passes even if a tool failed.
- `tool_result`: it passes when no tool error is left.
- `read_back`: as `tool_result`, and then a check runs. The check is another sub-run with its own counters. Its message names the task, the criteria and the files in `creates`, and asks the model to read them with `read_file`. The model answers `{"passed": true|false, "message": "..."}` (`verdictFormat`, sent with every request of the check). A yes or no marks the task passed or failed, and the message becomes the task's reply. A reply with no yes or no, or a tool error during the check, fails the task.

### Decision 22: A retry is a step in the core

**Context:** Decision 17 put the retry loop in the Gemini adapter, so the UI could not show that a request was being sent again. The adapter cannot report progress to the UI, and the core already works step by step (Decision 13).

**Decision:** The adapter no longer retries. The core decides, as a step.

```text
the request fails with a retryable error
  → the core remembers a wait in the run and returns  Retrying   TUI: status "retrying..."
the next Step waits that long (cancel and timeout still work during the wait), then asks again
a success resets the run's retry count; too many failures fail the question or the task as before
```

- The adapter's error says whether it may pass: `APIError` has a `Retryable()` method (429, 500, 503, 504). The core only asks `errors.As` for anything with that method, so it knows no provider.
- The count and the delays are read in the core, like the limits: `MAX_RETRIES` (3), `RETRY_INITIAL_DELAY` (500ms), `RETRY_MAX_DELAY` (8s). The wait doubles each retry, capped at the maximum, and is random up to that cap.
- It works the same for a question, a task and a read_back check, because all three go through `retryLater` and `waitToRetry` in `core/retry.go`. A cancel or timeout is never retried. A task whose retries run out fails with "The request failed", as before.
- The adapter keeps the request timeout (`REQUEST_TIMEOUT`, 60s) for one attempt.

**Why:** It fits the step-by-step flow, and the UI learns of the retry with no extra mechanism. Which errors are retryable stays provider knowledge, in the adapter.

### Decision 23: Three behaviors ported from the old app: scrolling, the existing-file check, the cut-off note

**Scrolling.** The Activity pane scrolls with the mouse wheel (3 rows a notch, only over the pane) and the `up` and `down` keys. It follows the newest line until the user scrolls up, then stays where it is.

**Existing-file check.** When the model returns tasks, the core checks the files they plan to create before the list is shown. If any already exists, the run ends with "Not started: notes.md already exists, and Kite can only create new files." The core asks the tools for `list_files` once and compares normalized paths. If that call fails, nothing is refused. Hidden folders are not listed, so a file inside one is not seen.

**Cut-off note.** A reply the model cut off at its length limit carries `Truncated` on its `Message`, read from `finish_reason: "length"` in the adapter. The core appends "The reply was cut off because it hit the model's length limit." to a question's answer and to a task's reply, and never treats a cut-off reply as tasks, because its list may be incomplete.
