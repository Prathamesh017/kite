# Kite: Architecture

The architecture style, the system boundary, what the core owns, and the rules the design follows. Every decision was made by the author, [@Prathamesh017](https://github.com/Prathamesh017), and recorded with the problem it solved.

For the full reasoning behind each decision:

- [Ports, the UI and tools](design/ports-and-ui.md): Decisions 1-9, how the core, the LLM, the tools and the terminal UI connect.
- [The run loop, limits and tasks](design/run-loop-and-tasks.md): Decisions 10-23, how a run proceeds, what stops it, retries, and tasks.

## 1. Architecture Style

Kite will follow **Hexagonal Architecture (Ports and Adapters)**.

The goal is to keep the application's core functionality independent of external systems and infrastructure.

The core should not depend on specific implementations such as:
- Terminal UI or Web UI
- Gemini or OpenAI
- Filesystem or database implementations

External systems communicate with the core through ports, and adapters implement those ports.

## 2. System Boundary

The system boundary defines what belongs to Kite and what belongs outside it.

**Inside Kite:**
- Application behaviour and execution flow.
- Conversation and task management.
- Application state.
- Decisions about which actions to perform.
- Coordination of LLM requests and tool execution.

**Outside Kite:**
- User interfaces such as TUI and Web UI.
- External LLM providers such as Gemini and OpenAI.
- Storage systems such as SQLite and PostgreSQL.
- External infrastructure and services.

A useful way to identify ownership:

- If something is necessary for Kite to perform its job, it belongs to the application.
- If something is necessary only for presentation or user interaction, it belongs to the UI.
- If something provides an external capability, it belongs outside the core and communicates through a port.

## 3. Application Core

Kite is an AI agent.

Its primary responsibility is to:

- Receive user requests.
- Understand and process requests.
- Decide what actions to perform.
- Execute tools when necessary.
- Manage conversations and tasks.
- Return results to the user.

The application core owns the application's behaviour, state and execution flow.

It should not contain UI-specific logic or depend on specific external service implementations.

### 3.1 Application Interface

The core exposes its capabilities through an interface.

This interface defines the operations that other components can use when interacting with Kite.

For example:

```go
type Kite interface {
    Ask(question string) (string, error)
    Cancel() error
}
```

**What is a contract?**

A contract is an agreement that defines which operations are available and how they can be called.

The interface defines what can be done, not how it is done.

**Who implements the contract?**

The core provides the actual implementation of the interface methods.

For example, the core implements the `Ask()` method, which contains the actual Q&A logic.

In Go, the core doesn't need to explicitly declare that it implements the interface. Go automatically considers it to satisfy the interface when its methods match.

**Who calls the contract?**

Adapters call the methods exposed through the interface.

For example:
- TUI calls `Ask()` when a user submits a question.
- A future Web UI can call the same `Ask()` method.
- Both use the same core implementation.

Adapters don't need to know how the core works internally. They only need to know which operations are available.

We should expose only the functionality that other components need, not every internal function of the core.

In Go, interfaces are generally defined by the package that consumes them. The core provides the implementation, and the consuming adapter defines what it needs.

### 3.2 Communication with the UI

The UI acts as an interface to the application.

It should:
- Collect user input.
- Send user requests to the application core.
- Display results and application updates.
- Maintain UI-specific state such as theme, scroll position and window dimensions.
- Maintain its own display copy of the conversation. The core owns the real history; the UI does not read it (see [Decision 5](design/ports-and-ui.md)).

The core should:
- Manage application state and behaviour.
- Process requests from the UI.
- Return application-level results and updates.

The core should not know which UI is interacting with it.

### 3.3 LLM Communication

The core controls the agent's execution flow, while LLM adapters handle communication with individual providers.

The core should:
- Prepare conversation context and requests.
- Decide when an LLM request is needed.
- Process the response.
- Decide whether to execute a tool, continue the conversation or return an answer.

LLM adapters should:
- Implement the LLM communication contract defined by the application.
- Handle provider-specific API communication.
- Translate application requests into provider-specific formats.
- Translate provider responses into a format understood by the application.

The LLM communication contract is a **port**. Gemini, OpenAI and other providers can be connected through adapters that implement this port.

Replacing an LLM provider should not require changing the core's execution logic.

### 3.4 Tool Execution

The core controls tool execution.

It should:
- Process tool calls requested by the LLM.
- Validate whether a tool call is permitted.
- Coordinate tool execution.
- Process tool results and decide what happens next.

The LLM adapter only translates tool-call requests between the provider's format and the application's format. It does not decide whether a tool is allowed or execute it.

## 4. Ports and Adapters

Ports define contracts for communication between the application core and external systems.

Adapters implement or connect to those contracts.

Examples of potential adapters:

| Capability | Possible adapters |
|---|---|
| LLM communication | Gemini, OpenAI, Anthropic |
| User interaction | Terminal UI, Web UI |
| Data storage | SQLite, PostgreSQL |
| File access | Local filesystem |

The core should depend on port contracts rather than concrete adapter implementations.

New adapters should be introducible without requiring substantial changes to the core.

## 5. Separation of Responsibilities

- Application logic belongs in the application core.
- UI components handle user interaction, rendering and UI state.
- External adapters handle communication with external systems.
- Application state should not be tightly coupled to UI state.
- The application should communicate application-level results, not instructions about which UI components to update.

## 6. Development Principles

- Prefer simple designs over unnecessary abstractions.
- Introduce interfaces when they establish a useful boundary.
- Do not create abstractions merely to follow a pattern.
- Keep responsibilities clear and avoid mixing unrelated concerns.
- Preserve existing behaviour when refactoring.
- Add tests for important application behaviour.
- Design around actual requirements rather than hypothetical future needs.
