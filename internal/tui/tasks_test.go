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
