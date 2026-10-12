package tools

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// writeFileDefinition tells the model it can create one new file with the given contents.
const writeFileDefinition = `{
  "type": "function",
  "function": {
    "name": "write_file",
    "description": "Create a new text file inside the current workspace with the given contents. It cannot change or overwrite a file that already exists, and it cannot create folders, so the folder must already exist.",
    "parameters": {
      "type": "object",
      "properties": {
        "path": {
          "type": "string",
          "description": "Path of the new file"
        },
        "content": {
          "type": "string",
          "description": "Full contents of the new file"
        }
      },
      "required": ["path", "content"],
      "additionalProperties": false
    }
  }
}`

// writeFile creates one new file inside the workspace.
// The path must stay inside the folder Kite was started in, its folder must
// already exist, and no file may exist at that path yet.
func writeFile(arguments string) string {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil || strings.TrimSpace(args.Path) == "" {
		return `error: write_file needs a path and content, for example {"path":"notes.md","content":"hello"}`
	}

	full, ok := insideWorkspace(args.Path)
	if !ok {
		return "error: " + args.Path + " is outside the workspace"
	}

	folder, err := os.Stat(filepath.Dir(full))
	if err != nil || !folder.IsDir() {
		return "error: the folder for " + args.Path + " does not exist, and write_file cannot create folders"
	}

	// O_EXCL makes the create fail if anything already exists at the path.
	file, err := os.OpenFile(full, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, os.ErrExist) {
		return "error: " + args.Path + " already exists, write_file can only create new files"
	}
	if err != nil {
		return "error: " + err.Error()
	}
	defer file.Close()

	if _, err := file.WriteString(args.Content); err != nil {
		return "error: " + err.Error()
	}
	return "file written successfully: " + args.Path
}
