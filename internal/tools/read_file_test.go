package tools

import (
	"strings"
	"testing"
)

func TestReadFileBlocksSecretFiles(t *testing.T) {
	for _, path := range []string{".env", ".env.local", "certs/server.pem", ".git/config"} {
		got := readFile(`{"path":"` + path + `"}`)
		want := "error: " + path + " is blocked"
		if !strings.HasPrefix(got, want) {
			t.Errorf("read_file %s = %q, want it blocked", path, got)
		}
	}
}

func TestReadFileAllowsNormalFiles(t *testing.T) {
	got := readFile(`{"path":"read_file.go"}`)
	if !strings.HasPrefix(got, "lines ") {
		t.Errorf("read_file read_file.go = %q, want its lines", got[:min(len(got), 100)])
	}
}
