package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Definitions are sent with every request so the model knows which tools exist.
// They are raw JSON, so they reach the model exactly as written.
var Definitions = []json.RawMessage{
	json.RawMessage(listFilesDefinition),
	json.RawMessage(readFileDefinition),
}

// Registry maps a tool name from the model to the function that runs it.
// Each function gets the model's arguments as a JSON string and returns the
// text sent back to the model. Problems are returned as text starting with "error:".
var Registry = map[string]func(arguments string) string{
	"list_files": listFiles,
	"read_file":  readFile,
}

// insideWorkspace resolves path against the folder Kite was started in, and
// reports whether the result stays inside that folder.
func insideWorkspace(path string) (string, bool) {
	root, err := os.Getwd()
	if err != nil {
		return "", false
	}
	full := path
	if !filepath.IsAbs(full) {
		full = filepath.Join(root, full)
	}
	full = filepath.Clean(full)

	rel, err := filepath.Rel(root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return full, true
}
