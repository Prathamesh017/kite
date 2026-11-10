# Kite

A coding agent built from scratch, one capability at a time (`docs/roadmap.md`). It is being refactored to hexagonal architecture (ports and adapters).

- New code: `cmd/kite-v2`, `internal/core`, `internal/adapter/{tui,gemini,tools}`. Run it with `task run-v2`.
- Old code: `internal/tui`, `internal/llm`, `internal/tools`, `cmd/main.go`. It stays until v2 behaves the same, and is then deleted.

## Read first

- `docs/Architecture.md`: the architecture and every decision made so far (section 7). `docs/` is gitignored.
- `docs/flow.md`: how the old app works. It is the reference for "do not break the flow".

## How to work

The author is practicing design and wants to understand each step, not just get it done.

- **Do not make design decisions.** Types, where state lives, how two parts talk, loop shape, limits and what a limit does are all the author's call. Give the options with a recommendation, ask one question, and wait. Implement only the option they pick.
- **Explain the concept first**, with a small example, then do one small step. Do not add methods, fields, tests or cleanups that were not asked for.
- **Picture the flow before filling it in.** Leave blanks for the author to place.
- Read a file before writing to it. Prefer small edits over rewriting a file.
- Check the old code before mapping it to the new design, and say exactly what the old code does.
- After each step run `go build ./...`, `go vet` and `go test -race` on the packages touched.
- Add a decision to `docs/Architecture.md` only after the author has made it. Never record an unconfirmed choice as settled.
- Do not commit unless asked.

## Rule: ask before designing

The author is practicing low-level design. Do not choose structure for them.

Before adding or changing any of these, stop and ask:

- A new type, field, or function whose job is a design choice
- Where state lives, and what is stored versus derived
- How two parts talk (chat to parent, parent to the model client)
- What a user action does after the input is captured

Ask one or two questions, then wait. Implement only the option they pick. When they are picturing a flow, follow "picture the flow first" and leave the blanks empty.

Proceed without asking for a bugfix, a rename they already named, or a change they specified step by step.

```go
// Bad: inventing a second model of the same conversation
type shown struct { role, text string; fail bool }
type History struct { Role, Text string }

// Good: they said "keep input and the model call; we will design the list"
// so the code only stores input and the last reply until they decide.
```

## Rule: picture the flow first

When the author is working out a flow, do not fill it in for them.

- Restate only the parts they already placed.
- Leave a blank where they have not said what happens.
- Ask them to place the next blank. One or two blanks at a time.
- Skip types, fields, function signatures, and code until they ask to build what they pictured.

A short opinion is fine when they ask "what do you think". An opinion names whether the picture holds together. It does not add the missing steps.

## Rule: start with real problems

Don't design from theory. Design from observation.

When starting a feature or phase:

1. **Identify the real problem** from the actual code and flow
   - Run a task, see what fails or wastes effort
   - Read `docs/flow.md` to understand the current architecture
   - Trace through concrete scenarios
   - Don't assume problems; observe them

2. **Picture the flow** showing the problem
   - Current state: what happens now?
   - Desired state: what should happen?
   - Where does it break?

3. **Ask design questions** one at a time
   - Based on the real problem, not theory
   - Wait for answers before moving on
   - Let corrections reshape the design

4. **Document the design** before building
   - Record decisions and why they were made in `docs/Architecture.md` (section 7)
   - Include the real problem it solves

```
Bad: "We should add semantic search to memory because it's more sophisticated"

Good: "Agent re-reads main.go in Task 1 and Task 2 (observed waste).
       We'll save a summary so Task 2 doesn't have to re-read.
       Semantic search can come later if summaries aren't relevant enough."
```
