# Kite: Ports, the UI and Tools

Decisions 1-9: how the core, the LLM, the tools and the terminal UI are connected. Back to the [architecture overview](../ARCHITECTURE.md). Next: [the run loop and tasks](run-loop-and-tasks.md).

### Decision 1: Composition root in a parallel `cmd/kite-v2`

**Context:** The agent loop currently lives inside the TUI, and we are moving it into a UI-independent core.

**Decision:** Add a new entry point, `cmd/kite-v2/main.go`, beside the existing `cmd/main.go`. It is the only place that knows every part. It builds the driven adapters (LLM, tools), builds the core with them, builds the TUI with the core, and runs the TUI.

**Why:** The old app keeps working while the new one is built, so we can compare them. Once v2 behaves the same, the old main and old loop code are deleted.

### Decision 2: The core defines both shapes, what it offers and what it needs

**Context:** There are two directions of communication between the core and the outside, and it must be clear which side owns each interface.

**Decision:** The core owns the interface in both directions.

1. **What the core offers (driving port).** An adapter calls the core through an interface the core publishes. It lives in `internal/core`.

   ```go
   // internal/core/kite.go
   type Kite interface {
       Ask(ctx context.Context, question string) (string, error)
   }
   ```

   The TUI receives a `core.Kite` and calls `Ask`. A future web UI would call the same `Ask`. Neither knows how the core answers.

2. **What the core needs (driven port).** The core calls an external system through an interface the core defines, in the shape the core needs. The adapter conforms to it. In Go the adapter does not even import or declare it, matching the method is enough.

   ```go
   // internal/core (defined by the core, because the core is the one that uses it)
   type LLM interface {
       Complete(ctx context.Context, history []Message) (Reply, error)
   }

   // internal/adapter/gemini (the adapter just has a matching method)
   func (c Client) Complete(ctx context.Context, history []core.Message) (core.Reply, error) { ... }
   ```

   Replacing Gemini with OpenAI means writing another type with a `Complete` method. The core does not change.

```
TUI adapter ──calls──▶ core.Kite     (core offers)
core ──calls──▶ core.LLM ◀──implements── Gemini adapter     (core needs)
```

**Why:** The dependency always points toward the core. Adapters depend on the core's interfaces, and the core never depends on an adapter. In the driven direction this follows the Go rule that the user of an interface defines it, shaped to what it actually needs.

**Note:** `core.Kite` exposes only what adapters need, not every internal function of the core. The method list in the example is not final.

### Decision 3: The LLM port is small, and the system prompt is passed separately

**Context:** The old `llm.Client.Complete` did many things: it added prompts, attached tools, picked a reply shape, and returned a finish reason and tool calls. The core needs a port for plain Q&A first.

**Decision:** The core defines the port in `internal/core/llm.go`, with its own `Message` type. The system prompt is a separate parameter, not a message inside the history.

```go
// internal/core/llm.go
type Message struct {
    Role    string // "user" or "assistant"
    Message string
}

type LLM interface {
    Complete(ctx context.Context, systemPrompt string, history []Message) (string, error)
}
```

The core owns the prompt text and the history (`Agent` in `internal/core/agent.go`). The adapter (`internal/adapter/gemini`) turns the `systemPrompt` and `[]core.Message` into the request its API expects, and returns the reply text. Tools, reply shapes and the finish reason are left out and come back one at a time.

**Why:**
- The core defines its own `Message`, so it never imports an adapter package.
- History holds only user and assistant turns. Trimming or summarising it later cannot drop the system prompt.
- Providers place the system prompt differently. The adapter decides where it goes, and the core does not change.
- `ctx` lets the adapter's HTTP call be cancelled or timed out.

**Wiring:** `cmd/kite-v2/main.go` loads `.env`, builds the Gemini client, passes it to `core.NewAgent`, and passes the agent to `tui.Init`. In the TUI, `Model` holds a `core.Kite`, and `askKite` (`commands.go`) calls `Ask` in the background and returns a `kiteReplyMsg`.

### Decision 4: The history stores only completed exchanges

**Context:** `Ask` used to append the question to the history before calling the LLM. When the call failed, the question stayed in the history. A retry added it again, so the next request held two user messages in a row.

**Decision:** `Ask` sends the LLM a copy of the history plus the new question. The question and the answer are appended to the history together, only after the reply succeeded.

```go
// internal/core/agent.go
user := Message{Role: "user", Message: question}
reply, err := a.llm.Complete(ctx, systemPrompt, append(slices.Clone(a.history), user))
if err != nil {
    return "", err // history is untouched
}
a.history = append(a.history, user, Message{Role: "assistant", Message: reply})
```

**Why:** A failed question is not part of the conversation, so it should not be in the history. The history is always a clean sequence of user and assistant turns, and no cleanup code is needed on failure.

### Decision 5: The UI owns the list it displays

**Context:** The pane shows the conversation, and the core also keeps a history. A first version had the pane read the core's history on every redraw through `GetHistory()`. But `Ask` appends to the history in a background goroutine while `View` reads it on the UI goroutine, which is a data race.

**Decision:** The history is maintained by the core, and the core remains the owner of what is sent to the model. The TUI (`Activity.conversations`) also maintains its own, separate version of the conversation, for display only. Two copies exist on purpose. The core exposes no way to read its history. When a reply arrives, `Update` adds the question and the answer to the UI's list. The core only sends data out (the reply); the UI never reaches into the core's memory.

| | Core history | UI list (`Activity.conversations`) |
|---|---|---|
| Purpose | what is sent to the model | what is shown to the user |
| Owner | `core.Agent` | `tui.Activity` |
| Written by | `Ask`, after a successful reply | `Update`, when a reply arrives |
| Read by | `Ask`, on the next request | `View`, on every redraw |

A different UI (for example a web UI) would keep its own list in the same way.

```go
// internal/core/kite.go
type Kite interface {
    Ask(ctx context.Context, question string) (string, error)
}

// internal/adapter/tui/model.go, in Update, when a reply arrives
m.activity.conversations = append(m.activity.conversations,
    core.Message{Role: "user", Message: question},
    core.Message{Role: "assistant", Message: msg.reply},
)
```

The pane does not get told about new entries. Bubble Tea calls `View` after every `Update`, and `kiteReplyMsg` is what triggers an `Update` when a reply arrives.

**Why:** In Go, data should have one owner, and others learn about it by being sent a message, not by sharing memory. The core owns what is sent to the model. The UI owns what is displayed. Only one goroutine touches each list, so there is no race and no lock. A copy of a message is cheap, because a Go string is a pointer and a length over bytes that are never changed.

**Why not `GetHistory()`:** A read is fine for a one-off need, like loading a saved session at start-up. It is wrong to poll it every frame while another goroutine changes the data. The displayed list and the stored history will also differ once tool steps and summaries exist.

### Decision 6: The UI keeps the unanswered question in `pending`

**Context:** Because of Decisions 4 and 5, a question joins the history and the UI's list only once it was answered. Without more, the question would vanish from the screen while the model thinks.

**Decision:** The UI holds the unanswered question in `Activity.pending` and draws it after the conversation. It is set on Enter and cleared when the reply or the error arrives. On an error it is the text put back into the chat input.

```
Enter       pending = "what is Go?"          pane: [list...] > you  what is Go?
reply ok    pending = ""                     pane: [list..., you: what is Go?, kite: ...]
reply err   pending = "", input = question   pane: [list...]  chat: question + error
```

**Why:** `pending` is one string of UI state, not a second history. The core stays clean and the user still sees their question straight away.

### Decision 7: `Status` is a typed set of values

**Context:** The pane shows what Kite is doing while it waits ("thinking" now, more states with tools). A plain string would accept any text, including typos.

**Decision:** `Status` is an integer type with named constants, and a `label()` method gives the text shown. `StatusIdle` is the zero value and means nothing is shown.

```go
// internal/adapter/tui/activity.go
type Status int

const (
    StatusIdle Status = iota
    StatusThinking
)

func (s Status) label() string {
    switch s {
    case StatusThinking:
        return "thinking"
    }
    return ""
}
```

The model sets `StatusThinking` on Enter and `StatusIdle` when the reply or error arrives. A new state is one constant and one `case` in `label()`.

**Why:** An integer-based type means the compiler rejects anything that is not a defined constant. A string-based type would accept `"thinkin"` without an error, because Go lets an untyped string constant be assigned to it.

### Decision 8: The core defines the tools port, and the filesystem tools are an adapter

**Context:** The agent will call tools in a loop. The core must decide when to run a tool, but reading a file touches the filesystem, so the code that does it is an adapter. The old `internal/tools` mixed both: the definition was a provider-format JSON string, and `Definitions` and `Registry` were two globals kept in sync by hand.

**Decision:** The core defines what a tool is and what it needs, in its own words, in `internal/core/tools.go`. A filesystem adapter implements it in `internal/adapter/tools`.

```go
// internal/core/tools.go
type ToolSpec struct {
    Name        string
    Description string
    Parameters  json.RawMessage // JSON schema of the arguments
}

type Tools interface {
    Specs() []ToolSpec
    Run(ctx context.Context, name, arguments string) string // errors come back as "error: ..." text
}

// internal/adapter/tools: the adapter matches the port
var _ core.Tools = Tools{}
```

One adapter value provides both the list and the runner, so they cannot drift apart. The tools are `list_files`, `read_file` and `write_file`, ported from the old app. The adapter keeps the old safety rules: the path must stay inside the folder Kite was started in, `.env`, `*.pem` and `.git` are blocked, binary files are rejected, and the output is capped.

**Why:**
- The `LLM` adapter, not the core, turns a `ToolSpec` into the provider's JSON (`{"type":"function",...}`), so a provider with a different shape needs no core change.
- The core decides when to run a tool and what happens with the result. The adapter only does the work.
- MCP tools (Phase 5) become another type with `Specs` and `Run`, or a wrapper that joins several.

**Why a port when there is only one kind of tool today?** A port is only worth having when a second implementation is really coming (see section 6 of the [overview](../ARCHITECTURE.md): do not create abstractions merely to follow a pattern). Three are coming, and one is needed at the next step:
1. **A fake tool set to test the loop.** The loop in `Agent.Ask` (tool call, result, LLM again) can be tested with a fake `Tools` and no real files. Without the port, the loop test would read the real filesystem.
2. **MCP tools (Phase 5)** are a second real implementation.
3. **Permissions and hooks (Phase 3)** wrap the tools: a wrapper checks "may this run?" and then calls the real tools. That only fits cleanly if there is a port to wrap.

There is also a structural reason. The core cannot call the filesystem tools directly without importing the adapter, which would break the rule that the core never depends on an adapter. The interface costs two methods. If tools were only ever a few filesystem functions and the loop was never tested on its own, a port would be overkill.

### Decision 9: The LLM port receives the tool list and can return tool calls

**Context:** Decision 3 gave the LLM port a plain text reply: `Complete(ctx, systemPrompt, history) (string, error)`. With tools, the model must be told which tools exist, and a reply can be a request to run a tool instead of text.

**Decision:** `Complete` takes the tool specs and returns the assistant's `Message`. A `Message` may carry `ToolCalls`. This replaces the signature shown in Decision 3.

```go
// internal/core/llm.go
type ToolCall struct {
    ID        string
    Name      string
    Arguments string // JSON string
}

type Message struct {
    Role      string // "user", "assistant"
    Message   string
    ToolCalls []ToolCall // set when the model asks to run tools instead of answering
}

type LLM interface {
    Complete(ctx context.Context, systemPrompt string, history []Message, tools []ToolSpec) (Message, error)
}
```

The core gets the specs from `Tools.Specs()` and passes them through untouched. The `gemini` adapter maps each `ToolSpec` to the provider's `{"type":"function","function":{...}}` shape and reads `tool_calls` from the response back into `core.ToolCall`. The wiring is in `cmd/kite-v2/main.go`: the tools adapter is handed to `core.NewAgent(llm, tools)`.

`Agent.Ask` tells the reply kinds apart with a `switch`:

```go
switch {
case len(reply.ToolCalls) > 0:
    // the model wants to run tools instead of answering (not handled yet)
// case task: how a task reply is recognised is not decided yet
default:
    // a regular message, stored in the history and returned
}
```

**Why:**
- The reply is the assistant's own message, so it goes into the history as it is. The provider needs that exact assistant message before the tool results.
- `Ask`, in the core, decides what a reply means. The UI does not classify replies.
- A different provider only needs a different mapping in its adapter.

