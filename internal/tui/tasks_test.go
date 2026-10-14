package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kite/internal/llm"
)

func TestTasksDoNotStartWhenAFileExists(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "docs.md")
	if err := os.WriteFile(existing, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := New()
	m.runCtx, m.cancelRun = context.WithCancel(context.Background())
	tasks := []llm.Task{
		{ID: "read", Description: "read main.go"},
		{ID: "write", Description: "write docs.md", Creates: []string{existing}},
	}

	cmd := m.startTasks(tasks)

	if cmd != nil {
		t.Error("a request was sent, want nothing to run")
	}
	if m.tasks.Active() {
		t.Error("task list was started")
	}
	last := m.history[len(m.history)-1]
	if last.Role != "assistant" || !strings.Contains(last.Content, "already exists") {
		t.Errorf("last history message = %+v, want the already-exists note", last)
	}
}

// checkingTask returns a model whose first task, a read_back one, has just
// answered, so judgeTask starts its check.
func checkingTask(t *testing.T) Model {
	t.Helper()
	m := New()
	m.runCtx, m.cancelRun = context.WithCancel(context.Background())
	write := llm.Task{ID: "write", Description: "write docs.md", Creates: []string{"docs.md"},
		AcceptanceCriteria: []string{"docs.md mentions loadEnv"}}
	write.VerificationPolicy.Type = "read_back"
	next := llm.Task{ID: "next", Description: "say done"}
	m.tasks.Start([]llm.Task{write, next})
	m.tasks.Running(0)
	m.taskErrors = map[string]string{}

	if cmd := m.judgeTask("I wrote docs.md."); cmd == nil {
		t.Fatal("no request was sent, want the check to start")
	}
	if !m.checking {
		t.Fatal("check did not start")
	}
	if len(m.taskHistory) != 1 || !strings.Contains(m.taskHistory[0].Content, "docs.md mentions loadEnv") {
		t.Fatalf("check history = %+v, want one fresh message with the criteria", m.taskHistory)
	}
	return m
}

func TestReadBackPassUsesTheVerdictMessage(t *testing.T) {
	m := checkingTask(t)

	m.onReply(replyMsg{response: `{"verdict":"PASS","message":"docs.md explains loadEnv"}`, finish: "stop"})

	if m.checking {
		t.Error("still checking after the verdict")
	}
	if m.tasks.Current() != 1 {
		t.Fatalf("current task = %d, want the next task to run", m.tasks.Current())
	}
	if !strings.Contains(m.taskHistory[0].Content, "docs.md explains loadEnv") {
		t.Errorf("next task got %q, want the verdict message as the earlier result", m.taskHistory[0].Content)
	}
}

func TestReadBackFailEndsTheRun(t *testing.T) {
	m := checkingTask(t)

	m.onReply(replyMsg{response: `{"verdict":"FAIL","message":"loadEnv is not mentioned"}`, finish: "stop"})

	if m.checking || m.tasks.Active() {
		t.Error("run did not end after FAIL")
	}
	last := m.history[len(m.history)-1]
	if !strings.Contains(last.Content, "loadEnv is not mentioned") {
		t.Errorf("summary = %q, want the FAIL reason", last.Content)
	}
}

func TestReadBackWithoutAVerdictFails(t *testing.T) {
	m := checkingTask(t)

	m.onReply(replyMsg{response: "looks good to me", finish: "stop"})

	if m.checking || m.tasks.Active() {
		t.Error("run did not end after a reply without a verdict")
	}
	last := m.history[len(m.history)-1]
	if !strings.Contains(last.Content, "did not return a verdict") {
		t.Errorf("summary = %q, want the no-verdict note", last.Content)
	}
}

func TestTasksStartWhenNewFilesAreFree(t *testing.T) {
	m := New()
	m.runCtx, m.cancelRun = context.WithCancel(context.Background())
	tasks := []llm.Task{{ID: "write", Description: "write new.md", Creates: []string{filepath.Join(t.TempDir(), "new.md")}}}

	if cmd := m.startTasks(tasks); cmd == nil {
		t.Error("no request was sent, want the first task to start")
	}
	if !m.tasks.Active() {
		t.Error("task list was not started")
	}
}
