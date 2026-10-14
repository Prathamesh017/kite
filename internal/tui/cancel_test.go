package tui

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"kite/internal/llm"
)

// running returns a model in the middle of a question that started a task run.
func running(t *testing.T) (Model, context.Context) {
	t.Helper()
	m := New()
	m.activity.AppendQuestion("summarize main.go and write docs.md")
	m.history = append(m.history, llm.Message{Role: "user", Content: "summarize main.go and write docs.md"})
	m.chat.busy = true
	m.tasks.Start([]llm.Task{{ID: "a", Description: "read main.go"}})
	m.taskHistory = []llm.Message{{Role: "user", Content: "Task: read main.go"}}
	m.runCtx, m.cancelRun = context.WithCancel(context.Background())
	return m, m.runCtx
}

func press(m Model, msg tea.Msg) Model {
	next, _ := m.Update(msg)
	return next.(Model)
}

func TestEscCancelsRun(t *testing.T) {
	m, ctx := running(t)

	m = press(m, tea.KeyMsg{Type: tea.KeyEsc})

	if ctx.Err() == nil {
		t.Error("run context was not cancelled")
	}
	if m.cancelRun != nil || m.runCtx != nil {
		t.Error("run is still kept on the model")
	}
	if m.tasks.Active() || m.taskHistory != nil {
		t.Error("task list or task history was not cleared")
	}
	if m.chat.busy {
		t.Error("chat input is still locked")
	}
	last := m.history[len(m.history)-1]
	if last.Role != "assistant" || last.Content != "(cancelled by the user before answering)" {
		t.Errorf("last history message = %+v, want the cancel note", last)
	}
	if got := m.history[len(m.history)-2].Content; got != "summarize main.go and write docs.md" {
		t.Errorf("question was not kept in history, got %q", got)
	}
}

func TestEscWithNothingRunning(t *testing.T) {
	m := New()
	m = press(m, tea.KeyMsg{Type: tea.KeyEsc})
	if len(m.history) != 0 {
		t.Errorf("Esc with nothing running changed history: %+v", m.history)
	}
}

func TestLateReplyAfterEscIsIgnored(t *testing.T) {
	m, _ := running(t)
	m = press(m, tea.KeyMsg{Type: tea.KeyEsc})
	historyLen, activityLen := len(m.history), len(m.activity.conversions)

	m = press(m, replyMsg{response: `{"mode":"normal","answer":"late","tasks":[]}`, finish: "stop"})
	m = press(m, toolsDoneMsg{})
	m = press(m, cancelledMsg{})

	if len(m.history) != historyLen {
		t.Errorf("late messages changed history: %+v", m.history[historyLen:])
	}
	if len(m.activity.conversions) != activityLen {
		t.Errorf("late messages changed activity: %+v", m.activity.conversions[activityLen:])
	}
}

func TestTimeoutPutsQuestionBack(t *testing.T) {
	m, ctx := running(t)

	m = press(m, timedOutMsg{})

	if ctx.Err() == nil || m.cancelRun != nil {
		t.Error("run was not released")
	}
	if m.tasks.Active() || m.taskHistory != nil {
		t.Error("task list or task history was not cleared")
	}
	if len(m.history) != 0 {
		t.Errorf("question was not dropped from history: %+v", m.history)
	}
	if m.chat.input != "summarize main.go and write docs.md" {
		t.Errorf("chat input = %q, want the question back", m.chat.input)
	}
	if m.chat.output != "Request timed out, please try again." || m.chat.busy {
		t.Errorf("chat = %+v, want the timeout reason and an unlocked input", m.chat)
	}
}

func TestRunToolsReportsTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()

	calls := []llm.ToolCall{{ID: "1", Function: llm.FunctionCall{Name: "list_files", Arguments: `{}`}}}
	msg := runTools(ctx, "", calls)()

	if _, ok := msg.(timedOutMsg); !ok {
		t.Errorf("runTools past its deadline returned %T, want timedOutMsg", msg)
	}
}

func TestRunToolsStopsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	calls := []llm.ToolCall{{ID: "1", Function: llm.FunctionCall{Name: "list_files", Arguments: `{}`}}}
	msg := runTools(ctx, "", calls)()

	if _, ok := msg.(cancelledMsg); !ok {
		t.Errorf("runTools with a cancelled context returned %T, want cancelledMsg", msg)
	}
}
