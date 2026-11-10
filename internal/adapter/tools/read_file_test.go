package tools

import (
	"context"
	"strings"
	"testing"
)

func TestReadFileBlocksSecretFiles(t *testing.T) {
	for _, path := range []string{".env", ".env.local", "certs/server.pem", ".git/config"} {
		got := Tools{}.Run(context.Background(), "read_file", `{"path":"`+path+`"}`)
		want := "error: " + path + " is blocked"
		if !strings.HasPrefix(got, want) {
			t.Errorf("read_file %s = %q, want it blocked", path, got)
		}
	}
}

func TestReadFileAllowsNormalFiles(t *testing.T) {
	got := Tools{}.Run(context.Background(), "read_file", `{"path":"read_file.go"}`)
	if !strings.HasPrefix(got, "lines ") {
		t.Errorf("read_file read_file.go = %q, want its lines", got[:min(len(got), 100)])
	}
}

func TestRunUnknownTool(t *testing.T) {
	got := Tools{}.Run(context.Background(), "nope", `{}`)
	if got != "error: unknown tool nope" {
		t.Errorf("got %q", got)
	}
}

func TestSpecsListsTools(t *testing.T) {
	specs := Tools{}.Specs()
	if len(specs) != 3 || specs[0].Name != "list_files" || specs[1].Name != "read_file" || specs[2].Name != "write_file" {
		t.Errorf("specs = %+v, want list_files, read_file and write_file", specs)
	}
}
